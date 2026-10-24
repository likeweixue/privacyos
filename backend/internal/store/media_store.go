package store

import (
	"database/sql"
	"errors"
	"strings"
)

const photoColumns = `id, owner_id, path, file_name, size_bytes, width, height, taken_at, camera, lens,
	gps_lat, gps_lng, orientation, thumb_path, sha256, favorite, deleted_at, created_at`

func scanPhoto(sc rowScanner) (*Photo, error) {
	var p Photo
	var taken, deleted sql.NullString
	var lat, lng sql.NullFloat64
	var favorite int
	if err := sc.Scan(&p.ID, &p.OwnerID, &p.Path, &p.FileName, &p.SizeBytes, &p.Width, &p.Height,
		&taken, &p.Camera, &p.Lens, &lat, &lng, &p.Orientation, &p.ThumbPath, &p.SHA256,
		&favorite, &deleted, &p.CreatedAt); err != nil {
		return nil, err
	}
	p.TakenAt = taken.String
	p.GPSLat = lat.Float64
	p.GPSLng = lng.Float64
	p.Favorite = favorite != 0
	p.DeletedAt = deleted.String
	return &p, nil
}

// UpsertPhoto inserts or refreshes an indexed photo.
func (s *Store) UpsertPhoto(p *Photo) error {
	if p.CreatedAt == "" {
		p.CreatedAt = Now()
	}
	var lat, lng any
	if p.GPSLat != 0 || p.GPSLng != 0 {
		lat, lng = p.GPSLat, p.GPSLng
	}
	_, err := s.db.Exec(`INSERT INTO photos (owner_id, path, file_name, size_bytes, width, height,
		taken_at, camera, lens, gps_lat, gps_lng, orientation, thumb_path, sha256, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(owner_id, path) DO UPDATE SET file_name=excluded.file_name, size_bytes=excluded.size_bytes,
			width=excluded.width, height=excluded.height, taken_at=excluded.taken_at, camera=excluded.camera,
			lens=excluded.lens, gps_lat=excluded.gps_lat, gps_lng=excluded.gps_lng,
			orientation=excluded.orientation, thumb_path=excluded.thumb_path, sha256=excluded.sha256`,
		p.OwnerID, p.Path, p.FileName, p.SizeBytes, p.Width, p.Height, nullIfEmpty(p.TakenAt),
		p.Camera, p.Lens, lat, lng, p.Orientation, p.ThumbPath, p.SHA256, p.CreatedAt)
	return err
}

// GetPhoto returns one photo, optionally scoped to an owner (0 = any).
func (s *Store) GetPhoto(id, ownerID int64) (*Photo, error) {
	q := `SELECT ` + photoColumns + ` FROM photos WHERE id=?`
	args := []any{id}
	if ownerID > 0 {
		q += ` AND owner_id=?`
		args = append(args, ownerID)
	}
	p, err := scanPhoto(s.db.QueryRow(q, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

// GetPhotoByPath returns one photo by owner and absolute path.
func (s *Store) GetPhotoByPath(ownerID int64, path string) (*Photo, error) {
	p, err := scanPhoto(s.db.QueryRow(`SELECT `+photoColumns+` FROM photos WHERE owner_id=? AND path=?`, ownerID, path))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

// ListPhotos returns a filtered page of photos and the total count.
func (s *Store) ListPhotos(ownerID int64, view, search, from, to string, page, pageSize int) ([]Photo, int, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 500 {
		pageSize = 60
	}
	var where []string
	var args []any
	if ownerID > 0 {
		where = append(where, "owner_id=?")
		args = append(args, ownerID)
	}
	if view == "deleted" {
		where = append(where, "deleted_at IS NOT NULL")
	} else {
		where = append(where, "deleted_at IS NULL")
		if view == "favorites" {
			where = append(where, "favorite=1")
		}
	}
	if search != "" {
		where = append(where, "file_name LIKE ? ESCAPE '\\'")
		args = append(args, like(search))
	}
	if from != "" {
		where = append(where, "COALESCE(taken_at, created_at) >= ?")
		args = append(args, from)
	}
	if to != "" {
		where = append(where, "COALESCE(taken_at, created_at) <= ?")
		args = append(args, to)
	}
	clause := ""
	if len(where) > 0 {
		clause = " WHERE " + strings.Join(where, " AND ")
	}
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM photos`+clause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, pageSize, (page-1)*pageSize)
	order := "COALESCE(taken_at, created_at) DESC"
	if view == "recent" {
		order = "created_at DESC"
	}
	rows, err := s.db.Query(`SELECT `+photoColumns+` FROM photos`+clause+`
		ORDER BY `+order+`, id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Photo{}
	for rows.Next() {
		p, err := scanPhoto(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *p)
	}
	return out, total, rows.Err()
}

// DeletePhotoRecord removes a photo index record.
func (s *Store) DeletePhotoRecord(id int64) error {
	_, err := s.db.Exec(`DELETE FROM photos WHERE id=?`, id)
	return err
}

// CountPhotos counts indexed photos (ownerID 0 = all).
func (s *Store) CountPhotos(ownerID int64) (int, error) {
	q := `SELECT COUNT(*) FROM photos WHERE deleted_at IS NULL`
	args := []any{}
	if ownerID > 0 {
		q += ` AND owner_id=?`
		args = append(args, ownerID)
	}
	var n int
	err := s.db.QueryRow(q, args...).Scan(&n)
	return n, err
}

// DateCount is an aggregation bucket for the timeline view.
type DateCount struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// PhotoTimeline returns per-day counts for a user.
func (s *Store) PhotoTimeline(ownerID int64) ([]DateCount, error) {
	rows, err := s.db.Query(`SELECT substr(COALESCE(taken_at, created_at), 1, 10) AS d, COUNT(*)
		FROM photos WHERE owner_id=? AND deleted_at IS NULL GROUP BY d ORDER BY d DESC`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DateCount{}
	for rows.Next() {
		var dc DateCount
		if err := rows.Scan(&dc.Date, &dc.Count); err != nil {
			return nil, err
		}
		out = append(out, dc)
	}
	return out, rows.Err()
}

// PhotosWithGPS returns geo-tagged photos for the map view.
func (s *Store) PhotosWithGPS(ownerID int64) ([]Photo, error) {
	rows, err := s.db.Query(`SELECT `+photoColumns+` FROM photos
		WHERE owner_id=? AND deleted_at IS NULL AND gps_lat IS NOT NULL AND gps_lng IS NOT NULL
		ORDER BY COALESCE(taken_at, created_at) DESC LIMIT 500`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Photo{}
	for rows.Next() {
		p, err := scanPhoto(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// ---------- albums ----------

// CreateAlbum inserts an album.
func (s *Store) CreateAlbum(a *Album) error {
	a.CreatedAt = Now()
	res, err := s.db.Exec(`INSERT INTO albums (owner_id, name, description, created_at) VALUES (?,?,?,?)`,
		a.OwnerID, a.Name, a.Description, a.CreatedAt)
	if err != nil {
		return err
	}
	a.ID, _ = res.LastInsertId()
	return nil
}

// ListAlbums returns a user's albums with photo counts.
func (s *Store) ListAlbums(ownerID int64) ([]Album, error) {
	rows, err := s.db.Query(`SELECT a.id, a.owner_id, a.name, a.description, COALESCE(a.cover_photo,0), a.created_at,
		(SELECT COUNT(*) FROM album_photos ap WHERE ap.album_id = a.id)
		FROM albums a WHERE a.owner_id=? ORDER BY a.id DESC`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Album{}
	for rows.Next() {
		var a Album
		if err := rows.Scan(&a.ID, &a.OwnerID, &a.Name, &a.Description, &a.CoverPhoto, &a.CreatedAt, &a.PhotoCount); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetAlbum returns one album scoped to its owner.
func (s *Store) GetAlbum(id, ownerID int64) (*Album, error) {
	var a Album
	err := s.db.QueryRow(`SELECT id, owner_id, name, description, COALESCE(cover_photo,0), created_at,
		(SELECT COUNT(*) FROM album_photos ap WHERE ap.album_id = albums.id)
		FROM albums WHERE id=? AND owner_id=?`, id, ownerID).
		Scan(&a.ID, &a.OwnerID, &a.Name, &a.Description, &a.CoverPhoto, &a.CreatedAt, &a.PhotoCount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// DeleteAlbum removes an album.
func (s *Store) DeleteAlbum(id, ownerID int64) error {
	_, err := s.db.Exec(`DELETE FROM albums WHERE id=? AND owner_id=?`, id, ownerID)
	return err
}

// AddPhotosToAlbum links photos to an album owned by the user.
func (s *Store) AddPhotosToAlbum(albumID, ownerID int64, photoIDs []int64) error {
	for _, pid := range photoIDs {
		_, err := s.db.Exec(`INSERT OR IGNORE INTO album_photos (album_id, photo_id, added_at)
			SELECT ?, id, ? FROM photos WHERE id=? AND owner_id=?`, albumID, Now(), pid, ownerID)
		if err != nil {
			return err
		}
	}
	_, err := s.db.Exec(`UPDATE albums SET cover_photo = COALESCE(cover_photo,
		(SELECT photo_id FROM album_photos WHERE album_id=? LIMIT 1)) WHERE id=?`, albumID, albumID)
	return err
}

// RemovePhotoFromAlbum unlinks a photo.
func (s *Store) RemovePhotoFromAlbum(albumID, ownerID, photoID int64) error {
	_, err := s.db.Exec(`DELETE FROM album_photos WHERE album_id=? AND photo_id=?
		AND album_id IN (SELECT id FROM albums WHERE id=? AND owner_id=?)`,
		albumID, photoID, albumID, ownerID)
	return err
}

// AlbumPhotos returns the photos in an album.
func (s *Store) AlbumPhotos(albumID, ownerID int64) ([]Photo, error) {
	rows, err := s.db.Query(`SELECT `+photoColumns+` FROM photos
		WHERE id IN (SELECT photo_id FROM album_photos WHERE album_id=?)
		AND owner_id=? AND deleted_at IS NULL ORDER BY COALESCE(taken_at, created_at) DESC`, albumID, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Photo{}
	for rows.Next() {
		p, err := scanPhoto(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// ---------- movies ----------

const movieColumns = `id, owner_id, path, file_name, size_bytes, title, original_title, year, kind,
	season, episode, overview, poster, backdrop, rating, runtime_seconds, genres, cast_list,
	tmdb_id, resolution, video_codec, audio_codec, scraped, created_at`

func scanMovie(sc rowScanner) (*Movie, error) {
	var m Movie
	err := sc.Scan(&m.ID, &m.OwnerID, &m.Path, &m.FileName, &m.SizeBytes, &m.Title, &m.OriginalTitle,
		&m.Year, &m.Kind, &m.Season, &m.Episode, &m.Overview, &m.Poster, &m.Backdrop, &m.Rating,
		&m.RuntimeSeconds, &m.Genres, &m.Cast, &m.TMDBID, &m.Resolution, &m.VideoCodec,
		&m.AudioCodec, &m.Scraped, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// UpsertMovie inserts or refreshes an indexed movie file.
func (s *Store) UpsertMovie(m *Movie) error {
	if m.CreatedAt == "" {
		m.CreatedAt = Now()
	}
	_, err := s.db.Exec(`INSERT INTO movies (owner_id, path, file_name, size_bytes, title, original_title,
		year, kind, season, episode, overview, poster, backdrop, rating, runtime_seconds, genres,
		cast_list, tmdb_id, resolution, video_codec, audio_codec, scraped, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(owner_id, path) DO UPDATE SET file_name=excluded.file_name, size_bytes=excluded.size_bytes,
			title=excluded.title, original_title=excluded.original_title, year=excluded.year, kind=excluded.kind,
			season=excluded.season, episode=excluded.episode, overview=excluded.overview,
			poster=excluded.poster, backdrop=excluded.backdrop, rating=excluded.rating,
			runtime_seconds=excluded.runtime_seconds, genres=excluded.genres, cast_list=excluded.cast_list,
			tmdb_id=excluded.tmdb_id, resolution=excluded.resolution, video_codec=excluded.video_codec,
			audio_codec=excluded.audio_codec, scraped=excluded.scraped`,
		m.OwnerID, m.Path, m.FileName, m.SizeBytes, m.Title, m.OriginalTitle, m.Year, m.Kind,
		m.Season, m.Episode, m.Overview, m.Poster, m.Backdrop, m.Rating, m.RuntimeSeconds, m.Genres,
		m.Cast, m.TMDBID, m.Resolution, m.VideoCodec, m.AudioCodec, m.Scraped, m.CreatedAt)
	return err
}

// MovieFilter selects movies for the library view.
type MovieFilter struct {
	OwnerID  int64
	Query    string
	Kind     string
	Year     int
	Genre    string
	Sort     string
	Page     int
	PageSize int
}

// ListMovies returns a filtered page of movies and the total count.
func (s *Store) ListMovies(f MovieFilter) ([]Movie, int, error) {
	if f.Page <= 0 {
		f.Page = 1
	}
	if f.PageSize <= 0 || f.PageSize > 200 {
		f.PageSize = 40
	}
	var where []string
	var args []any
	if f.OwnerID > 0 {
		where = append(where, "owner_id=?")
		args = append(args, f.OwnerID)
	}
	if f.Query != "" {
		where = append(where, "(title LIKE ? ESCAPE '\\' OR original_title LIKE ? ESCAPE '\\' OR file_name LIKE ? ESCAPE '\\')")
		args = append(args, like(f.Query), like(f.Query), like(f.Query))
	}
	if f.Kind != "" && f.Kind != "all" {
		where = append(where, "kind=?")
		args = append(args, f.Kind)
	}
	if f.Year > 0 {
		where = append(where, "year=?")
		args = append(args, f.Year)
	}
	if f.Genre != "" {
		where = append(where, "genres LIKE ? ESCAPE '\\'")
		args = append(args, like(f.Genre))
	}
	clause := ""
	if len(where) > 0 {
		clause = " WHERE " + strings.Join(where, " AND ")
	}
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM movies`+clause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	order := "title COLLATE NOCASE ASC"
	switch f.Sort {
	case "newest":
		order = "year DESC, id DESC"
	case "rating":
		order = "rating DESC, title ASC"
	case "recent":
		order = "id DESC"
	}
	args = append(args, f.PageSize, (f.Page-1)*f.PageSize)
	rows, err := s.db.Query(`SELECT `+movieColumns+` FROM movies`+clause+` ORDER BY `+order+` LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Movie{}
	for rows.Next() {
		m, err := scanMovie(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *m)
	}
	return out, total, rows.Err()
}

// GetMovie returns one movie scoped to an owner (0 = any).
func (s *Store) GetMovie(id, ownerID int64) (*Movie, error) {
	q := `SELECT ` + movieColumns + ` FROM movies WHERE id=?`
	args := []any{id}
	if ownerID > 0 {
		q += ` AND owner_id=?`
		args = append(args, ownerID)
	}
	m, err := scanMovie(s.db.QueryRow(q, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return m, err
}

// GetMovieByPath returns a movie by owner and absolute path.
func (s *Store) GetMovieByPath(ownerID int64, path string) (*Movie, error) {
	m, err := scanMovie(s.db.QueryRow(`SELECT `+movieColumns+` FROM movies WHERE owner_id=? AND path=?`, ownerID, path))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return m, err
}

// UpdateMovieMeta updates scraped metadata fields.
func (s *Store) UpdateMovieMeta(m *Movie) error {
	_, err := s.db.Exec(`UPDATE movies SET title=?, original_title=?, year=?, kind=?, season=?, episode=?,
		overview=?, poster=?, backdrop=?, rating=?, runtime_seconds=?, genres=?, cast_list=?,
		tmdb_id=?, resolution=?, video_codec=?, audio_codec=?, scraped=? WHERE id=? AND owner_id=?`,
		m.Title, m.OriginalTitle, m.Year, m.Kind, m.Season, m.Episode, m.Overview, m.Poster,
		m.Backdrop, m.Rating, m.RuntimeSeconds, m.Genres, m.Cast, m.TMDBID, m.Resolution,
		m.VideoCodec, m.AudioCodec, m.Scraped, m.ID, m.OwnerID)
	return err
}

// UpdateMovieTechnical stores ffprobe-derived stream info.
func (s *Store) UpdateMovieTechnical(id int64, runtime int, resolution, videoCodec, audioCodec string) error {
	_, err := s.db.Exec(`UPDATE movies SET runtime_seconds=?, resolution=?, video_codec=?, audio_codec=? WHERE id=?`,
		runtime, resolution, videoCodec, audioCodec, id)
	return err
}

// DeleteMovieRecord removes a movie row.
func (s *Store) DeleteMovieRecord(id int64) error {
	_, err := s.db.Exec(`DELETE FROM movies WHERE id=?`, id)
	return err
}

// CountMovies counts indexed titles for an owner (0 = all).
func (s *Store) CountMovies(ownerID int64) (int, error) {
	q := `SELECT COUNT(*) FROM movies`
	args := []any{}
	if ownerID > 0 {
		q += ` WHERE owner_id=?`
		args = append(args, ownerID)
	}
	var n int
	err := s.db.QueryRow(q, args...).Scan(&n)
	return n, err
}

// UnscrapedMovies returns titles without metadata.
func (s *Store) UnscrapedMovies(ownerID int64, limit int) ([]Movie, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	q := `SELECT ` + movieColumns + ` FROM movies WHERE scraped=0`
	args := []any{}
	if ownerID > 0 {
		q += ` AND owner_id=?`
		args = append(args, ownerID)
	}
	q += ` ORDER BY id ASC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Movie{}
	for rows.Next() {
		m, err := scanMovie(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	return out, rows.Err()
}

// DistinctYears returns the release years present in the library.
func (s *Store) DistinctYears(ownerID int64) ([]int, error) {
	q := `SELECT DISTINCT year FROM movies WHERE year > 0`
	args := []any{}
	if ownerID > 0 {
		q += ` AND owner_id=?`
		args = append(args, ownerID)
	}
	q += ` ORDER BY year DESC`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []int{}
	for rows.Next() {
		var y int
		if err := rows.Scan(&y); err != nil {
			return nil, err
		}
		out = append(out, y)
	}
	return out, rows.Err()
}

// CountMoviesByKind returns counts grouped by media kind.
func (s *Store) CountMoviesByKind(ownerID int64) (map[string]int, error) {
	q := `SELECT kind, COUNT(*) FROM movies`
	args := []any{}
	if ownerID > 0 {
		q += ` WHERE owner_id=?`
		args = append(args, ownerID)
	}
	q += ` GROUP BY kind`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var k string
		var n int
		if err := rows.Scan(&k, &n); err != nil {
			return nil, err
		}
		out[k] = n
	}
	return out, rows.Err()
}

// SetPhotoFavorite toggles the favorite flag for a photo.
func (s *Store) SetPhotoFavorite(id, ownerID int64, favorite bool) error {
	res, err := s.db.Exec(`UPDATE photos SET favorite=? WHERE id=? AND owner_id=?`,
		boolInt(favorite), id, ownerID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// SoftDeletePhoto marks a photo as deleted without removing the file.
func (s *Store) SoftDeletePhoto(id, ownerID int64) error {
	res, err := s.db.Exec(`UPDATE photos SET deleted_at=? WHERE id=? AND owner_id=? AND deleted_at IS NULL`,
		Now(), id, ownerID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// RestorePhoto clears the soft-delete marker.
func (s *Store) RestorePhoto(id, ownerID int64) error {
	res, err := s.db.Exec(`UPDATE photos SET deleted_at=NULL WHERE id=? AND owner_id=?`, id, ownerID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DuplicateGroups returns groups of photos sharing a non-empty SHA-256.
func (s *Store) DuplicateGroups(ownerID int64) ([]DuplicateGroup, error) {
	rows, err := s.db.Query(`SELECT `+photoColumns+` FROM photos
		WHERE owner_id=? AND deleted_at IS NULL AND sha256 != ''
		AND sha256 IN (
			SELECT sha256 FROM photos WHERE owner_id=? AND deleted_at IS NULL AND sha256 != ''
			GROUP BY sha256 HAVING COUNT(*) > 1
		)
		ORDER BY sha256, width*height DESC, size_bytes DESC`, ownerID, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	groups := []DuplicateGroup{}
	index := map[string]int{}
	for rows.Next() {
		p, err := scanPhoto(rows)
		if err != nil {
			return nil, err
		}
		gi, ok := index[p.SHA256]
		if !ok {
			gi = len(groups)
			index[p.SHA256] = gi
			groups = append(groups, DuplicateGroup{SHA256: p.SHA256})
		}
		groups[gi].Photos = append(groups[gi].Photos, *p)
	}
	return groups, rows.Err()
}

// MergeDuplicateGroups soft-deletes lower-quality duplicates, keeping the
// largest resolution (then largest file) in each group. Returns removed count.
func (s *Store) MergeDuplicateGroups(ownerID int64) (int, error) {
	groups, err := s.DuplicateGroups(ownerID)
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, g := range groups {
		if len(g.Photos) < 2 {
			continue
		}
		for _, p := range g.Photos[1:] {
			if err := s.SoftDeletePhoto(p.ID, ownerID); err == nil {
				removed++
			}
		}
	}
	return removed, nil
}

// ListPhotoSources returns the caller's extra photo source folders.
func (s *Store) ListPhotoSources(ownerID int64) ([]PhotoSource, error) {
	rows, err := s.db.Query(`SELECT id, owner_id, path, created_at FROM photo_sources
		WHERE owner_id=? ORDER BY id`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PhotoSource{}
	for rows.Next() {
		var src PhotoSource
		if err := rows.Scan(&src.ID, &src.OwnerID, &src.Path, &src.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, src)
	}
	return out, rows.Err()
}

// AddPhotoSource adds a source folder if not already present.
func (s *Store) AddPhotoSource(ownerID int64, path string) (*PhotoSource, error) {
	src := &PhotoSource{OwnerID: ownerID, Path: path, CreatedAt: Now()}
	res, err := s.db.Exec(`INSERT INTO photo_sources (owner_id, path, created_at) VALUES (?,?,?)
		ON CONFLICT(owner_id, path) DO NOTHING`, ownerID, path, src.CreatedAt)
	if err != nil {
		return nil, err
	}
	if src.ID, _ = res.LastInsertId(); src.ID == 0 {
		// Existing row; fetch its id.
		if err := s.db.QueryRow(`SELECT id, created_at FROM photo_sources WHERE owner_id=? AND path=?`,
			ownerID, path).Scan(&src.ID, &src.CreatedAt); err != nil {
			return nil, err
		}
	}
	return src, nil
}

// DeletePhotoSource removes a source folder.
func (s *Store) DeletePhotoSource(id, ownerID int64) error {
	_, err := s.db.Exec(`DELETE FROM photo_sources WHERE id=? AND owner_id=?`, id, ownerID)
	return err
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
