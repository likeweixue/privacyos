package media

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"privacyos/backend/internal/config"
	"privacyos/backend/internal/files"
	"privacyos/backend/internal/store"
	"privacyos/backend/internal/thumb"
)

// Movies indexes and serves the per-user movie library.
type Movies struct {
	Cfg    *config.Config
	Store  *store.Store
	Client *http.Client
}

// NewMovies creates a movie service.
func NewMovies(cfg *config.Config, st *store.Store) *Movies {
	return &Movies{Cfg: cfg, Store: st, Client: &http.Client{Timeout: 20 * time.Second}}
}

// Root returns /data/movies/<user>.
func (s *Movies) Root(u *store.User) string {
	return filepath.Join(s.Cfg.DataDir, "movies", u.Username)
}

var (
	reEpisode = regexp.MustCompile(`(?i)s(\d{1,2})[\s._-]*e(\d{1,3})`)
	reYear    = regexp.MustCompile(`(19\d{2}|20\d{2})`)
	reBracket = regexp.MustCompile(`[\[\(\{][^\]\)\}]*[\]\)\}]`)
	reSpaces  = regexp.MustCompile(`\s+`)
)

var sceneTags = map[string]bool{
	"1080p": true, "2160p": true, "720p": true, "480p": true, "4k": true, "8k": true,
	"bluray": true, "blu-ray": true, "brrip": true, "bdrip": true, "webrip": true,
	"web-dl": true, "webdl": true, "hdtv": true, "dvdrip": true, "remux": true,
	"x264": true, "x265": true, "h264": true, "h265": true, "hevc": true, "av1": true,
	"aac": true, "ac3": true, "dts": true, "truehd": true, "atmos": true, "flac": true,
	"10bit": true, "8bit": true, "hdr": true, "hdr10": true, "dv": true, "dolby": true,
	"proper": true, "repack": true, "extended": true, "unrated": true, "remastered": true,
	"multi": true, "chs": true, "cht": true, "eng": true, "gb": true, "big5": true,
	"国语": true, "中字": true, "双语": true, "特效": true, "字幕": true,
}

// GuessMetadata parses a scene-style filename.
func GuessMetadata(fileName string) (title string, year, season, episode int, kind string) {
	base := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	base = strings.ReplaceAll(base, "_", " ")
	base = strings.ReplaceAll(base, ".", " ")
	raw := base
	if m := reEpisode.FindStringSubmatch(raw); m != nil {
		season, _ = strconv.Atoi(m[1])
		episode, _ = strconv.Atoi(m[2])
		kind = "series"
		raw = reEpisode.ReplaceAllString(raw, " ")
	}
	if m := reYear.FindStringSubmatch(raw); m != nil {
		year, _ = strconv.Atoi(m[1])
		if idx := strings.Index(raw, m[1]); idx > 0 {
			raw = raw[:idx]
		}
	}
	raw = strings.TrimRight(raw, " ._-[({")
	clean := reBracket.ReplaceAllString(raw, " ")
	var keep []string
	for _, token := range strings.Fields(clean) {
		low := strings.ToLower(strings.Trim(token, "-"))
		if sceneTags[low] {
			continue
		}
		if strings.Contains(low, "x26") || strings.Contains(low, "h26") {
			continue
		}
		keep = append(keep, token)
	}
	title = strings.TrimSpace(reSpaces.ReplaceAllString(strings.Join(keep, " "), " "))
	title = strings.Trim(title, "-–— ")
	if kind == "" {
		kind = "movie"
	}
	return title, year, season, episode, kind
}

// Scan walks the movie library and indexes new files.
func (s *Movies) Scan(ctx context.Context, u *store.User, report func(int, string)) (int, error) {
	root := s.Root(u)
	if err := os.MkdirAll(root, 0o700); err != nil {
		return 0, err
	}
	var paths []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if files.IsVideo(d.Name()) {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	added := 0
	for i, path := range paths {
		if ctx.Err() != nil {
			return added, ctx.Err()
		}
		if _, err := s.IndexFile(ctx, u, path); err == nil {
			added++
		}
		if i%10 == 0 || i == len(paths)-1 {
			pct := 0
			if len(paths) > 0 {
				pct = int(float64(i+1) / float64(len(paths)) * 100)
			}
			report(pct, fmt.Sprintf("已扫描 %d/%d", i+1, len(paths)))
		}
	}
	return added, nil
}

// IndexFile adds or refreshes one movie file.
func (s *Movies) IndexFile(ctx context.Context, u *store.User, abs string) (*store.Movie, error) {
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !files.IsVideo(abs) {
		return nil, fmt.Errorf("不是视频文件")
	}
	if existing, err := s.Store.GetMovieByPath(u.ID, abs); err == nil {
		tech := probeVideo(ctx, abs)
		_ = s.Store.UpdateMovieTechnical(existing.ID, tech.Runtime, tech.Resolution, tech.VideoCodec, tech.AudioCodec)
		existing.SizeBytes = info.Size()
		existing.RuntimeSeconds = tech.Runtime
		return existing, nil
	}
	title, year, season, episode, kind := GuessMetadata(filepath.Base(abs))
	lower := strings.ToLower(abs)
	switch {
	case strings.Contains(lower, "/anime/") || strings.Contains(lower, "动漫") || strings.Contains(lower, "/番剧/"):
		kind = "anime"
	case strings.Contains(lower, "/documentary/") || strings.Contains(lower, "/纪录片/"):
		kind = "documentary"
	}
	tech := probeVideo(ctx, abs)
	m := &store.Movie{
		OwnerID: u.ID, Path: abs, FileName: filepath.Base(abs), SizeBytes: info.Size(),
		Title: title, Year: year, Kind: kind, Season: season, Episode: episode,
		RuntimeSeconds: tech.Runtime, Resolution: tech.Resolution,
		VideoCodec: tech.VideoCodec, AudioCodec: tech.AudioCodec, CreatedAt: store.Now(),
	}
	if err := s.Store.UpsertMovie(m); err != nil {
		return nil, err
	}
	if saved, err := s.Store.GetMovieByPath(u.ID, abs); err == nil {
		return saved, nil
	}
	return m, nil
}

type ffprobeResult struct {
	Runtime    int
	Resolution string
	VideoCodec string
	AudioCodec string
}

// probeVideo reads technical metadata with ffprobe when installed.
func probeVideo(ctx context.Context, abs string) ffprobeResult {
	var res ffprobeResult
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		return res
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, ffprobe, "-v", "quiet", "-print_format", "json",
		"-show_format", "-show_streams", abs)
	out, err := cmd.Output()
	if err != nil {
		return res
	}
	var doc struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if json.Unmarshal(out, &doc) != nil {
		return res
	}
	if d, err := strconv.ParseFloat(doc.Format.Duration, 64); err == nil {
		res.Runtime = int(d)
	}
	for _, st := range doc.Streams {
		switch st.CodecType {
		case "video":
			if res.VideoCodec == "" {
				res.VideoCodec = st.CodecName
				res.Resolution = fmt.Sprintf("%dx%d", st.Width, st.Height)
			}
		case "audio":
			if res.AudioCodec == "" {
				res.AudioCodec = st.CodecName
			}
		}
	}
	return res
}

// StreamPath validates that a movie belongs to the user and returns its path.
func (s *Movies) StreamPath(u *store.User, id int64) (string, *store.Movie, error) {
	m, err := s.Store.GetMovie(id, u.ID)
	if err != nil {
		return "", nil, err
	}
	if m.Path == "" {
		return "", m, fmt.Errorf("该条目未关联媒体文件")
	}
	root := s.Root(u)
	if !strings.HasPrefix(m.Path, root+string(os.PathSeparator)) {
		return "", nil, fmt.Errorf("非法媒体路径")
	}
	if _, err := os.Stat(m.Path); err != nil {
		return "", nil, fmt.Errorf("媒体文件不存在")
	}
	return m.Path, m, nil
}

// Poster serves a local poster image for a movie.
func (s *Movies) Poster(id, ownerID int64, size int) ([]byte, error) {
	m, err := s.Store.GetMovie(id, ownerID)
	if err != nil {
		return nil, err
	}
	if m.Poster == "" {
		return nil, fmt.Errorf("没有海报")
	}
	if _, err := os.Stat(m.Poster); err != nil {
		return nil, err
	}
	return thumb.Cached(filepath.Join(s.Cfg.CacheDir, "posters"), m.Poster, size)
}

// Decorate fills public URLs on a movie.
func (s *Movies) Decorate(m *store.Movie) {
	m.StreamURL = fmt.Sprintf("/api/v1/movies/%d/stream", m.ID)
	if m.Poster != "" {
		m.PosterURL = fmt.Sprintf("/api/v1/movies/%d/poster?size=400", m.ID)
	}
	if m.Path != "" {
		if info, err := os.Stat(m.Path); err == nil && !info.IsDir() {
			m.Playable = true
		}
	}
}

// Subtitle is a sidecar subtitle file.
type Subtitle struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Subtitles lists sidecar subtitle files next to the movie.
func (s *Movies) Subtitles(u *store.User, id int64) ([]Subtitle, error) {
	m, err := s.Store.GetMovie(id, u.ID)
	if err != nil {
		return nil, err
	}
	if m.Path == "" {
		return []Subtitle{}, nil
	}
	base := strings.TrimSuffix(m.Path, filepath.Ext(m.Path))
	var out []Subtitle
	for _, ext := range []string{".vtt", ".srt", ".ass", ".ssa"} {
		path := base + ext
		if _, err := os.Stat(path); err != nil {
			continue
		}
		out = append(out, Subtitle{
			Name: filepath.Base(path),
			URL:  fmt.Sprintf("/api/v1/movies/%d/subtitles/%s", id, url.PathEscape(filepath.Base(path))),
		})
	}
	return out, nil
}

// SubtitleVTT returns a subtitle converted to WebVTT.
func (s *Movies) SubtitleVTT(u *store.User, id int64, name string) ([]byte, string, error) {
	m, err := s.Store.GetMovie(id, u.ID)
	if err != nil {
		return nil, "", err
	}
	name = filepath.Base(name)
	if m.Path == "" {
		return nil, "", fmt.Errorf("该条目未关联媒体文件")
	}
	base := strings.TrimSuffix(m.Path, filepath.Ext(m.Path))
	path := base + filepath.Ext(name)
	if filepath.Base(path) != name {
		return nil, "", fmt.Errorf("非法字幕文件")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".vtt":
		return data, "text/vtt; charset=utf-8", nil
	case ".srt":
		return srtToVTT(data), "text/vtt; charset=utf-8", nil
	default:
		return data, "text/plain; charset=utf-8", nil
	}
}

// srtToVTT converts SubRip text to WebVTT.
func srtToVTT(data []byte) []byte {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	// Drop numeric counter lines preceding timestamps.
	lines := strings.Split(text, "\n")
	var out []string
	for _, line := range lines {
		if regexp.MustCompile(`^\d+$`).MatchString(strings.TrimSpace(line)) {
			continue
		}
		if strings.Contains(line, "-->") {
			line = strings.ReplaceAll(line, ",", ".")
		}
		out = append(out, line)
	}
	return []byte("WEBVTT\n\n" + strings.Join(out, "\n"))
}

// ---------- TMDB ----------

// TMDB is a minimal TMDB v3 client.
type TMDB struct {
	APIKey   string
	Language string
	client   *http.Client
}

// NewTMDB builds a client from settings.
func NewTMDB(apiKey, language string) *TMDB {
	if language == "" {
		language = "zh-CN"
	}
	return &TMDB{APIKey: apiKey, Language: language, client: &http.Client{Timeout: 20 * time.Second}}
}

type tmdbSearchResult struct {
	ID           int64   `json:"id"`
	Title        string  `json:"title"`
	Name         string  `json:"name"`
	OriginalTitle string `json:"original_title"`
	OriginalName  string `json:"original_name"`
	Overview     string  `json:"overview"`
	ReleaseDate  string  `json:"release_date"`
	FirstAirDate string  `json:"first_air_date"`
	PosterPath   string  `json:"poster_path"`
	BackdropPath string  `json:"backdrop_path"`
	VoteAverage  float64 `json:"vote_average"`
	Popularity   float64 `json:"popularity"`
}

type tmdbDetails struct {
	tmdbSearchResult
	Runtime  int `json:"runtime"`
	Genres   []struct{ Name string `json:"name"` } `json:"genres"`
	Credits  struct {
		Cast []struct {
			Name      string `json:"name"`
			Character string `json:"character"`
		} `json:"cast"`
	} `json:"credits"`
}

// Search queries TMDB for a title.
func (t *TMDB) Search(ctx context.Context, kind, title string, year int) (*tmdbSearchResult, error) {
	if t.APIKey == "" {
		return nil, fmt.Errorf("未配置 TMDB API Key")
	}
	mediaType := "movie"
	if kind == "series" || kind == "anime" {
		mediaType = "tv"
	}
	q := url.Values{}
	q.Set("api_key", t.APIKey)
	q.Set("query", title)
	q.Set("language", t.Language)
	if year > 0 {
		if mediaType == "movie" {
			q.Set("year", strconv.Itoa(year))
		} else {
			q.Set("first_air_date_year", strconv.Itoa(year))
		}
	}
	endpoint := fmt.Sprintf("https://api.themoviedb.org/3/search/%s?%s", mediaType, q.Encode())
	var doc struct {
		Results []tmdbSearchResult `json:"results"`
	}
	if err := t.getJSON(ctx, endpoint, &doc); err != nil {
		return nil, err
	}
	if len(doc.Results) == 0 {
		return nil, fmt.Errorf("TMDB 未找到匹配结果")
	}
	return &doc.Results[0], nil
}

// Details fetches full metadata including credits.
func (t *TMDB) Details(ctx context.Context, kind string, id int64) (*tmdbDetails, error) {
	mediaType := "movie"
	if kind == "series" || kind == "anime" {
		mediaType = "tv"
	}
	q := url.Values{}
	q.Set("api_key", t.APIKey)
	q.Set("language", t.Language)
	q.Set("append_to_response", "credits")
	endpoint := fmt.Sprintf("https://api.themoviedb.org/3/%s/%d?%s", mediaType, id, q.Encode())
	var doc tmdbDetails
	if err := t.getJSON(ctx, endpoint, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

func (t *TMDB) getJSON(ctx context.Context, endpoint string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("TMDB HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

// DownloadPoster saves a TMDB image into the poster cache.
func (t *TMDB) DownloadPoster(ctx context.Context, imagePath, destDir, prefix string) (string, error) {
	if imagePath == "" {
		return "", fmt.Errorf("没有海报")
	}
	if err := os.MkdirAll(destDir, 0o750); err != nil {
		return "", err
	}
	endpoint := "https://image.tmdb.org/t/p/w500" + imagePath
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("海报下载失败 HTTP %d", resp.StatusCode)
	}
	dest := filepath.Join(destDir, prefix+".jpg")
	f, err := os.Create(dest)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, io.LimitReader(resp.Body, 8<<20)); err != nil {
		f.Close()
		os.Remove(dest)
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return dest, nil
}

// ScrapeOne fetches TMDB metadata for a single movie.
func (s *Movies) ScrapeOne(ctx context.Context, u *store.User, m *store.Movie, tmdb *TMDB) error {
	title := m.Title
	if title == "" {
		title, _, _, _, _ = GuessMetadata(m.FileName)
	}
	if title == "" {
		return fmt.Errorf("无法推断片名")
	}
	hit, err := tmdb.Search(ctx, m.Kind, title, m.Year)
	if err != nil {
		return err
	}
	details, err := tmdb.Details(ctx, m.Kind, hit.ID)
	if err != nil {
		return err
	}
	displayTitle := details.Title
	if displayTitle == "" {
		displayTitle = details.Name
	}
	if displayTitle != "" {
		m.Title = displayTitle
	}
	m.OriginalTitle = details.OriginalTitle
	if m.OriginalTitle == "" {
		m.OriginalTitle = details.OriginalName
	}
	if d := details.ReleaseDate; d != "" {
		if y, err := strconv.Atoi(d[:4]); err == nil {
			m.Year = y
		}
	}
	if d := details.FirstAirDate; d != "" {
		if y, err := strconv.Atoi(d[:4]); err == nil {
			m.Year = y
		}
	}
	m.Overview = details.Overview
	m.Rating = details.VoteAverage
	m.TMDBID = details.ID
	if details.Runtime > 0 {
		m.RuntimeSeconds = details.Runtime * 60
	}
	var genres []string
	for _, g := range details.Genres {
		genres = append(genres, g.Name)
	}
	m.Genres = strings.Join(genres, ",")
	var cast []string
	for i, c := range details.Credits.Cast {
		if i >= 8 {
			break
		}
		cast = append(cast, c.Name)
	}
	m.Cast = strings.Join(cast, ",")
	if details.PosterPath != "" {
		dest, err := tmdb.DownloadPoster(ctx, details.PosterPath,
			filepath.Join(s.Cfg.CacheDir, "posters"), fmt.Sprintf("tmdb-%d", details.ID))
		if err == nil {
			m.Poster = dest
		}
	}
	if details.BackdropPath != "" {
		dest, err := tmdb.DownloadPoster(ctx, details.BackdropPath,
			filepath.Join(s.Cfg.CacheDir, "posters"), fmt.Sprintf("tmdb-bd-%d", details.ID))
		if err == nil {
			m.Backdrop = dest
		}
	}
	m.Scraped = true
	return s.Store.UpdateMovieMeta(m)
}

// ScrapeAll scrapes metadata for the given IDs (or all unscraped items).
func (s *Movies) ScrapeAll(ctx context.Context, u *store.User, ids []int64, tmdb *TMDB,
	report func(int, string)) (int, int, error) {

	var targets []store.Movie
	if len(ids) > 0 {
		for _, id := range ids {
			m, err := s.Store.GetMovie(id, u.ID)
			if err == nil {
				targets = append(targets, *m)
			}
		}
	} else {
		list, err := s.Store.UnscrapedMovies(u.ID, 200)
		if err != nil {
			return 0, 0, err
		}
		targets = list
	}
	ok, failed := 0, 0
	for i := range targets {
		if ctx.Err() != nil {
			return ok, failed, ctx.Err()
		}
		m := &targets[i]
		if err := s.ScrapeOne(ctx, u, m, tmdb); err != nil {
			failed++
		} else {
			ok++
		}
		report(int(float64(i+1)/float64(len(targets))*100), fmt.Sprintf("刮削 %d/%d", i+1, len(targets)))
		select {
		case <-ctx.Done():
			return ok, failed, ctx.Err()
		case <-time.After(260 * time.Millisecond):
		}
	}
	return ok, failed, nil
}
