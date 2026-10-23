// Package media indexes photos and movies, extracts metadata and generates
// thumbnails/posters.
package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rwcarlsen/goexif/exif"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/webp"

	"privacyos/backend/internal/config"
	"privacyos/backend/internal/files"
	"privacyos/backend/internal/store"
	"privacyos/backend/internal/thumb"
)

// Photos indexes and serves the per-user photo library.
type Photos struct {
	Cfg   *config.Config
	Store *store.Store
}

// NewPhotos creates a photo service.
func NewPhotos(cfg *config.Config, st *store.Store) *Photos {
	return &Photos{Cfg: cfg, Store: st}
}

// Root returns /data/photos/<user>.
func (s *Photos) Root(u *store.User) string {
	return filepath.Join(s.Cfg.DataDir, "photos", u.Username)
}

// TimelineGroup groups photos by year and month.
type TimelineGroup struct {
	Year   int            `json:"year"`
	Count  int            `json:"count"`
	Months []MonthGroup   `json:"months"`
}

// MonthGroup is a month bucket.
type MonthGroup struct {
	Month int              `json:"month"`
	Count int              `json:"count"`
	Days  []store.DateCount `json:"days"`
}

// Timeline aggregates a user's photos by date.
func (s *Photos) Timeline(u *store.User) ([]TimelineGroup, error) {
	days, err := s.Store.PhotoTimeline(u.ID)
	if err != nil {
		return nil, err
	}
	type monthKey struct {
		year, month int
	}
	months := map[monthKey]*MonthGroup{}
	years := map[int]*TimelineGroup{}
	for _, d := range days {
		t, err := time.Parse("2006-01-02", d.Date)
		if err != nil || t.Year() < 1970 {
			continue
		}
		yk := t.Year()
		if years[yk] == nil {
			years[yk] = &TimelineGroup{Year: yk}
		}
		years[yk].Count += d.Count
		mk := monthKey{yk, int(t.Month())}
		if months[mk] == nil {
			months[mk] = &MonthGroup{Month: int(t.Month())}
		}
		mg := months[mk]
		mg.Count += d.Count
		mg.Days = append(mg.Days, d)
	}
	for mk, mg := range months {
		years[mk.year].Months = append(years[mk.year].Months, *mg)
	}
	out := make([]TimelineGroup, 0, len(years))
	for yk := range years {
		out = append(out, *years[yk])
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Year > out[j].Year })
	for i := range out {
		sort.Slice(out[i].Months, func(a, b int) bool { return out[i].Months[a].Month > out[i].Months[b].Month })
	}
	return out, nil
}

// Scan walks the photo library and indexes images.
func (s *Photos) Scan(ctx context.Context, u *store.User, report func(int, string)) (int, error) {
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
		if files.IsImage(d.Name()) {
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
		if _, err := s.IndexFile(u, path); err == nil {
			added++
		}
		if i%20 == 0 || i == len(paths)-1 {
			pct := 0
			if len(paths) > 0 {
				pct = int(float64(i+1) / float64(len(paths)) * 100)
			}
			report(pct, fmt.Sprintf("已索引 %d/%d", i+1, len(paths)))
		}
	}
	return added, nil
}

// IndexFile reads metadata for one image and stores it.
func (s *Photos) IndexFile(u *store.User, abs string) (*store.Photo, error) {
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !files.IsImage(abs) {
		return nil, fmt.Errorf("不是图片")
	}
	meta := readPhotoMeta(abs, info.ModTime())
	sum, _ := fileSHA256(abs)
	p := &store.Photo{
		OwnerID: u.ID, Path: abs, FileName: filepath.Base(abs), SizeBytes: info.Size(),
		Width: meta.Width, Height: meta.Height, TakenAt: meta.TakenAt, Camera: meta.Camera,
		Lens: meta.Lens, GPSLat: meta.Lat, GPSLng: meta.Lng, Orientation: meta.Orientation,
		SHA256: sum, CreatedAt: store.Now(),
	}
	if err := s.Store.UpsertPhoto(p); err != nil {
		return nil, err
	}
	// Reload to obtain the generated ID.
	if saved, err := s.Store.GetPhotoByPath(u.ID, abs); err == nil {
		return saved, nil
	}
	return p, nil
}

// SaveUpload stores an uploaded image in the date-based archive.
func (s *Photos) SaveUpload(u *store.User, filename string, r io.Reader, now time.Time) (*store.Photo, error) {
	tmpDir := filepath.Join(s.Cfg.CacheDir, "tmp")
	if err := os.MkdirAll(tmpDir, 0o700); err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp(tmpDir, "photo-*")
	if err != nil {
		return nil, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	meta := readPhotoMeta(tmpName, now)
	taken := now
	if meta.TakenAt != "" {
		if t, err := time.Parse(time.RFC3339, meta.TakenAt); err == nil {
			taken = t
		}
	}
	dir := filepath.Join(s.Root(u), taken.Format("2006"), taken.Format("01"), taken.Format("02"))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	name := sanitizePhotoName(filename)
	target := filepath.Join(dir, name)
	if _, err := os.Stat(target); err == nil {
		base := strings.TrimSuffix(name, filepath.Ext(name))
		target = filepath.Join(dir, fmt.Sprintf("%s_%s%s", base, store.RandomToken(3), filepath.Ext(name)))
	}
	if err := os.Rename(tmpName, target); err != nil {
		// Cross-device fallback.
		if err := copyFile(tmpName, target); err != nil {
			return nil, err
		}
	}
	return s.IndexFile(u, target)
}

// Delete removes a photo from disk and the index.
func (s *Photos) Delete(u *store.User, id int64) error {
	p, err := s.Store.GetPhoto(id, u.ID)
	if err != nil {
		return err
	}
	root := s.Root(u)
	if strings.HasPrefix(p.Path, root+string(os.PathSeparator)) {
		if err := os.Remove(p.Path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return s.Store.DeletePhotoRecord(id)
}

// Thumb returns a JPEG thumbnail for a photo.
func (s *Photos) Thumb(p *store.Photo, size int) ([]byte, error) {
	if _, err := os.Stat(p.Path); err != nil {
		return nil, err
	}
	return thumb.Cached(filepath.Join(s.Cfg.CacheDir, "thumbs"), p.Path, size)
}

// Decorate fills public URLs on a photo.
func Decorate(p *store.Photo) {
	p.FileURL = fmt.Sprintf("/api/v1/photos/%d/file", p.ID)
	p.ThumbURL = fmt.Sprintf("/api/v1/photos/%d/thumb?size=400", p.ID)
}

type photoMeta struct {
	Width, Height int
	TakenAt       string
	Camera, Lens  string
	Lat, Lng      float64
	Orientation   int
}

// readPhotoMeta extracts EXIF data with a graceful fallback to file times.
func readPhotoMeta(abs string, modTime time.Time) photoMeta {
	meta := photoMeta{Orientation: 1, TakenAt: modTime.UTC().Format(time.RFC3339)}
	if f, err := os.Open(abs); err == nil {
		if cfg, _, derr := image.DecodeConfig(f); derr == nil {
			meta.Width, meta.Height = cfg.Width, cfg.Height
		}
		f.Close()
	}
	f, err := os.Open(abs)
	if err != nil {
		return meta
	}
	defer f.Close()
	x, err := exif.Decode(f)
	if err != nil {
		return meta
	}
	if t, err := x.DateTime(); err == nil && !t.IsZero() {
		meta.TakenAt = t.UTC().Format(time.RFC3339)
	}
	if v, err := x.Get(exif.Model); err == nil {
		if s, err := v.StringVal(); err == nil {
			meta.Camera = strings.Trim(s, "\x00 ")
		}
	}
	if v, err := x.Get(exif.LensModel); err == nil {
		if s, err := v.StringVal(); err == nil {
			meta.Lens = strings.Trim(s, "\x00 ")
		}
	}
	if v, err := x.Get(exif.Orientation); err == nil {
		if n, err := v.Int(0); err == nil && n >= 1 && n <= 8 {
			meta.Orientation = n
		}
	}
	if lat, lng, err := x.LatLong(); err == nil {
		meta.Lat, meta.Lng = lat, lng
	}
	return meta
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func sanitizePhotoName(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" || name == "." || name == ".." {
		name = "IMG_" + time.Now().Format("20060102_150405") + ".jpg"
	}
	return name
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
