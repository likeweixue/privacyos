// Package files implements the PrivacyOS file manager: sandboxed path
// resolution, directory listing, upload/download, trash, quotas and sharing.
package files

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"privacyos/backend/internal/config"
	"privacyos/backend/internal/store"
)

// Errors returned by the service.
var (
	ErrInvalidPath = errors.New("非法路径")
	ErrNotFound    = errors.New("文件不存在")
	ErrQuota       = errors.New("存储配额不足")
	ErrConflict    = errors.New("目标已存在")
)

// Scope identifies the directory tree a request operates on.
type Scope struct {
	User   *store.User
	SeeAll bool // Normal-edition administrator browsing every home
}

// Service provides file operations for a scope.
type Service struct {
	Cfg   *config.Config
	Store *store.Store

	mu    sync.Mutex
	cache map[int64]usageCache
}

type usageCache struct {
	bytes int64
	files int64
	at    time.Time
}

// Entry is a directory item returned to the UI.
type Entry struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	IsDir     bool   `json:"is_dir"`
	Size      int64  `json:"size"`
	Mode      string `json:"mode"`
	Owner     string `json:"owner"`
	Modified  string `json:"modified"`
	Type      string `json:"type"`
	Mime      string `json:"mime,omitempty"`
	Thumbnail bool   `json:"thumbnail"`
	Children  int    `json:"children,omitempty"`
}

// Listing is a directory listing with usage information.
type Listing struct {
	Path      string   `json:"path"`
	Parent    string   `json:"parent"`
	Entries   []Entry  `json:"entries"`
	Total     int      `json:"total"`
	Truncated bool     `json:"truncated"`
	Usage     Usage    `json:"usage"`
}

// Usage reports quota consumption for a user.
type Usage struct {
	UsedBytes  int64 `json:"used_bytes"`
	QuotaBytes int64 `json:"quota_bytes"`
	TrashBytes int64 `json:"trash_bytes"`
	FileCount  int64 `json:"file_count"`
}

// New creates a file service.
func New(cfg *config.Config, st *store.Store) *Service {
	return &Service{Cfg: cfg, Store: st, cache: map[int64]usageCache{}}
}

// UsersRoot returns /data/users.
func (s *Service) UsersRoot() string { return filepath.Join(s.Cfg.DataDir, "users") }

// Home returns the home directory of a user.
func (s *Service) Home(u *store.User) string {
	return filepath.Join(s.UsersRoot(), u.Username)
}

// RootFor returns the sandbox root for the scope.
func (s *Service) RootFor(sc Scope) (string, error) {
	if sc.User == nil {
		return "", ErrInvalidPath
	}
	if sc.SeeAll {
		if sc.User.Role != "admin" {
			return "", ErrInvalidPath
		}
		return s.UsersRoot(), nil
	}
	return s.Home(sc.User), nil
}

// SecureJoin resolves rel under root, rejecting traversal and symlink escapes.
func SecureJoin(root, rel string) (string, error) {
	rel = strings.TrimPrefix(strings.TrimSpace(rel), "/")
	rel = strings.ReplaceAll(rel, "\\", "/")
	if rel == "" || rel == "." {
		return root, nil
	}
	for _, part := range strings.Split(rel, "/") {
		if part == ".." {
			return "", ErrInvalidPath
		}
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." {
		return root, nil
	}
	abs := filepath.Join(root, clean)
	if !within(root, abs) {
		return "", ErrInvalidPath
	}
	// Guard against symlinks pointing outside the sandbox.
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		realRoot, rerr := filepath.EvalSymlinks(root)
		if rerr == nil && !within(realRoot, resolved) {
			return "", ErrInvalidPath
		}
	} else if os.IsNotExist(err) {
		if resolvedParent, perr := filepath.EvalSymlinks(filepath.Dir(abs)); perr == nil {
			realRoot, rerr := filepath.EvalSymlinks(root)
			if rerr == nil && !within(realRoot, resolvedParent) {
				return "", ErrInvalidPath
			}
		}
	}
	return abs, nil
}

// within reports whether path is root or inside root.
func within(root, path string) bool {
	if path == root {
		return true
	}
	return strings.HasPrefix(path, root+string(os.PathSeparator))
}

// Resolve maps a scope-relative path to an absolute path inside the sandbox.
func (s *Service) Resolve(sc Scope, rel string) (string, error) {
	root, err := s.RootFor(sc)
	if err != nil {
		return "", err
	}
	return SecureJoin(root, rel)
}

// Relative converts an absolute path to a scope-relative slash path.
func (s *Service) Relative(sc Scope, abs string) string {
	root, err := s.RootFor(sc)
	if err != nil {
		return abs
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return abs
	}
	if rel == "." {
		return ""
	}
	return filepath.ToSlash(rel)
}

// List returns a directory listing.
func (s *Service) List(sc Scope, rel string) (*Listing, error) {
	abs, err := s.Resolve(sc, rel)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, ErrNotFound
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("不是目录")
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(entries))
	for _, de := range entries {
		if len(out) >= 5000 {
			break
		}
		e, err := s.entryFor(sc, abs, de)
		if err != nil {
			continue
		}
		out = append(out, *e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	usage, _ := s.Usage(sc.User)
	return &Listing{
		Path:      s.Relative(sc, abs),
		Parent:    parentPath(s.Relative(sc, abs)),
		Entries:   out,
		Total:     len(out),
		Truncated: len(entries) > len(out),
		Usage:     usage,
	}, nil
}

func parentPath(rel string) string {
	if rel == "" {
		return ""
	}
	idx := strings.LastIndex(rel, "/")
	if idx < 0 {
		return ""
	}
	return rel[:idx]
}

// entryFor builds an Entry from a directory entry.
func (s *Service) entryFor(sc Scope, dir string, de os.DirEntry) (*Entry, error) {
	abs := filepath.Join(dir, de.Name())
	info, err := de.Info()
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		if target, err := os.Stat(abs); err == nil {
			info = target
		}
	}
	e := &Entry{
		Name:     de.Name(),
		Path:     s.Relative(sc, abs),
		IsDir:    info.IsDir(),
		Size:     info.Size(),
		Mode:     info.Mode().String(),
		Owner:    ownerName(info),
		Modified: info.ModTime().UTC().Format(time.RFC3339),
		Type:     classify(info, de.Name()),
	}
	if e.IsDir {
		e.Size = 0
		e.Mime = ""
	}
	if !e.IsDir {
		e.Mime = mimeFor(de.Name())
		e.Thumbnail = e.Type == "image" || e.Type == "video"
	}
	return e, nil
}

// Stat returns metadata for one path.
func (s *Service) Stat(sc Scope, rel string) (*Entry, error) {
	abs, err := s.Resolve(sc, rel)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, ErrNotFound
	}
	e := &Entry{
		Name:     filepath.Base(abs),
		Path:     s.Relative(sc, abs),
		IsDir:    info.IsDir(),
		Size:     info.Size(),
		Mode:     info.Mode().String(),
		Owner:    ownerName(info),
		Modified: info.ModTime().UTC().Format(time.RFC3339),
		Type:     classify(info, filepath.Base(abs)),
	}
	if e.IsDir {
		e.Size = 0
		e.Path = s.Relative(sc, abs)
	} else {
		e.Mime = mimeFor(abs)
	}
	return e, nil
}

// Mkdir creates a directory.
func (s *Service) Mkdir(sc Scope, rel, name string) (*Entry, error) {
	name = sanitizeName(name)
	if name == "" {
		return nil, ErrInvalidPath
	}
	parent, err := s.Resolve(sc, rel)
	if err != nil {
		return nil, err
	}
	if st, err := os.Stat(parent); err != nil || !st.IsDir() {
		return nil, ErrNotFound
	}
	abs := filepath.Join(parent, name)
	if !within(parent, abs) {
		return nil, ErrInvalidPath
	}
	if _, err := os.Stat(abs); err == nil {
		return nil, ErrConflict
	}
	if err := os.Mkdir(abs, 0o700); err != nil {
		return nil, err
	}
	s.applyOwner(sc.User, abs)
	return s.Stat(sc, s.Relative(sc, abs))
}

// Rename renames a file or directory in place.
func (s *Service) Rename(sc Scope, rel, newName string) error {
	newName = sanitizeName(newName)
	if newName == "" {
		return ErrInvalidPath
	}
	abs, err := s.Resolve(sc, rel)
	if err != nil {
		return err
	}
	root, _ := s.RootFor(sc)
	if abs == root {
		return ErrInvalidPath
	}
	if _, err := os.Stat(abs); err != nil {
		return ErrNotFound
	}
	dest := filepath.Join(filepath.Dir(abs), newName)
	if !within(root, dest) {
		return ErrInvalidPath
	}
	if _, err := os.Stat(dest); err == nil {
		return ErrConflict
	}
	return os.Rename(abs, dest)
}

// Move moves paths into a destination directory.
func (s *Service) Move(sc Scope, rels []string, destRel string) error {
	dest, err := s.Resolve(sc, destRel)
	if err != nil {
		return err
	}
	st, err := os.Stat(dest)
	if err != nil || !st.IsDir() {
		return ErrNotFound
	}
	root, _ := s.RootFor(sc)
	for _, rel := range rels {
		src, err := s.Resolve(sc, rel)
		if err != nil {
			return err
		}
		if src == root {
			return ErrInvalidPath
		}
		if !within(root, src) || !within(root, dest) {
			return ErrInvalidPath
		}
		if within(src, dest) {
			return fmt.Errorf("不能移动到自身子目录")
		}
		target := filepath.Join(dest, filepath.Base(src))
		if _, err := os.Stat(target); err == nil {
			return fmt.Errorf("%s 已存在", filepath.Base(src))
		}
		if err := os.Rename(src, target); err != nil {
			return err
		}
	}
	return nil
}

// Copy duplicates paths into a destination directory.
func (s *Service) Copy(sc Scope, rels []string, destRel string) error {
	dest, err := s.Resolve(sc, destRel)
	if err != nil {
		return err
	}
	root, _ := s.RootFor(sc)
	for _, rel := range rels {
		src, err := s.Resolve(sc, rel)
		if err != nil {
			return err
		}
		if !within(root, src) || !within(root, dest) {
			return ErrInvalidPath
		}
		target := filepath.Join(dest, filepath.Base(src))
		if within(src, dest) {
			return fmt.Errorf("不能复制到自身子目录")
		}
		if err := CopyTree(src, target); err != nil {
			return err
		}
		s.applyOwner(sc.User, target)
	}
	s.Invalidate(sc.User.ID)
	return nil
}

// DeleteOptions controls soft delete behaviour.
type DeleteOptions struct {
	RetentionDays int
}

// Delete moves paths to the trash and returns the created records.
func (s *Service) Delete(sc Scope, rels []string, opts DeleteOptions) ([]store.TrashItem, error) {
	root, _ := s.RootFor(sc)
	var items []store.TrashItem
	for _, rel := range rels {
		abs, err := s.Resolve(sc, rel)
		if err != nil {
			return items, err
		}
		if abs == root {
			return items, ErrInvalidPath
		}
		info, err := os.Lstat(abs)
		if err != nil {
			return items, fmt.Errorf("%s: 文件不存在", rel)
		}
		id := store.NewID()
		trashDir := filepath.Join(s.Cfg.DataDir, "trash", sc.User.Username, id)
		if err := os.MkdirAll(trashDir, 0o700); err != nil {
			return items, err
		}
		trashPath := filepath.Join(trashDir, filepath.Base(abs))
		if err := movePath(abs, trashPath); err != nil {
			return items, err
		}
		item := store.TrashItem{
			ID: id, OwnerID: sc.User.ID, OriginalPath: abs, TrashPath: trashPath,
			Name: filepath.Base(abs), IsDir: info.IsDir(), SizeBytes: pathSize(trashPath),
		}
		if opts.RetentionDays > 0 {
			item.ExpiresAt = time.Now().AddDate(0, 0, opts.RetentionDays).UTC().Format(time.RFC3339)
		}
		if err := s.Store.AddTrashItem(&item); err != nil {
			return items, err
		}
		items = append(items, item)
	}
	s.Invalidate(sc.User.ID)
	return items, nil
}

// ListTrash returns the trash for a user.
func (s *Service) ListTrash(u *store.User) ([]store.TrashItem, error) {
	return s.Store.ListTrash(u.ID)
}

// RestoreTrash returns trash items to their original location.
func (s *Service) RestoreTrash(u *store.User, ids []string) error {
	trashRoot := filepath.Join(s.Cfg.DataDir, "trash", u.Username)
	for _, id := range ids {
		item, err := s.Store.GetTrashItem(id)
		if err != nil || item.OwnerID != u.ID {
			return fmt.Errorf("回收站条目不存在")
		}
		if !within(trashRoot, item.TrashPath) {
			return ErrInvalidPath
		}
		if _, err := os.Stat(item.TrashPath); err != nil {
			_ = s.Store.DeleteTrashItem(id)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(item.OriginalPath), 0o700); err != nil {
			return err
		}
		dest := item.OriginalPath
		if _, err := os.Stat(dest); err == nil {
			dest = uniquePath(dest)
		}
		if err := os.Rename(item.TrashPath, dest); err != nil {
			return err
		}
		_ = os.Remove(filepath.Dir(item.TrashPath))
		if err := s.Store.DeleteTrashItem(id); err != nil {
			return err
		}
	}
	s.Invalidate(u.ID)
	return nil
}

// EmptyTrash permanently deletes trash entries.
func (s *Service) EmptyTrash(u *store.User, ids []string) error {
	trashRoot := filepath.Join(s.Cfg.DataDir, "trash", u.Username)
	items, err := s.Store.ListTrash(u.ID)
	if err != nil {
		return err
	}
	wanted := map[string]bool{}
	for _, id := range ids {
		wanted[id] = true
	}
	for _, item := range items {
		if len(ids) > 0 && !wanted[item.ID] {
			continue
		}
		if within(trashRoot, item.TrashPath) {
			_ = os.RemoveAll(item.TrashPath)
			_ = os.Remove(filepath.Dir(item.TrashPath))
		}
		_ = s.Store.DeleteTrashItem(item.ID)
	}
	s.Invalidate(u.ID)
	return nil
}

// SaveUpload streams an upload into dir under the scope.
func (s *Service) SaveUpload(sc Scope, dirRel, filename string, r io.Reader, size int64) (*Entry, error) {
	filename = sanitizeName(filename)
	if filename == "" {
		return nil, ErrInvalidPath
	}
	dir, err := s.Resolve(sc, dirRel)
	if err != nil {
		return nil, err
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return nil, ErrNotFound
	}
	usage, _ := s.Usage(sc.User)
	if sc.User.QuotaBytes > 0 && usage.UsedBytes+size > sc.User.QuotaBytes {
		return nil, ErrQuota
	}
	target := filepath.Join(dir, filename)
	if _, err := os.Stat(target); err == nil {
		target = uniquePath(target)
	}
	tmp := filepath.Join(dir, ".privacyos-upload-"+store.RandomToken(6))
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		return nil, err
	}
	written, copyErr := io.Copy(f, r)
	syncErr := f.Sync()
	closeErr := f.Close()
	if copyErr != nil || syncErr != nil || closeErr != nil {
		_ = os.Remove(tmp)
		if copyErr != nil {
			return nil, copyErr
		}
		return nil, errors.New("写入文件失败")
	}
	if sc.User.QuotaBytes > 0 && usage.UsedBytes+written > sc.User.QuotaBytes {
		_ = os.Remove(tmp)
		return nil, ErrQuota
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return nil, err
	}
	s.applyOwner(sc.User, target)
	s.Invalidate(sc.User.ID)
	return s.Stat(sc, s.Relative(sc, target))
}

// Usage returns cached quota usage for a user.
func (s *Service) Usage(u *store.User) (Usage, error) {
	if u == nil {
		return Usage{}, nil
	}
	s.mu.Lock()
	c, ok := s.cache[u.ID]
	s.mu.Unlock()
	if ok && time.Since(c.at) < 30*time.Second {
		trash, _ := s.Store.TrashSize(u.ID)
		return Usage{UsedBytes: c.bytes, QuotaBytes: u.QuotaBytes, TrashBytes: trash, FileCount: c.files}, nil
	}
	var total int64
	var count int64
	for _, root := range []string{
		s.Home(u),
		filepath.Join(s.Cfg.DataDir, "photos", u.Username),
		filepath.Join(s.Cfg.DataDir, "movies", u.Username),
	} {
		walkSize(root, &total, &count)
	}
	s.mu.Lock()
	s.cache[u.ID] = usageCache{bytes: total, files: count, at: time.Now()}
	s.mu.Unlock()
	trash, _ := s.Store.TrashSize(u.ID)
	return Usage{UsedBytes: total, QuotaBytes: u.QuotaBytes, TrashBytes: trash, FileCount: count}, nil
}

// Invalidate clears the usage cache for a user.
func (s *Service) Invalidate(userID int64) {
	s.mu.Lock()
	delete(s.cache, userID)
	s.mu.Unlock()
}

// Search walks the scope looking for names matching query.
func (s *Service) Search(sc Scope, query string, limit int) ([]Entry, error) {
	if strings.TrimSpace(query) == "" {
		return []Entry{}, nil
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	root, err := s.RootFor(sc)
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(query)
	out := []Entry{}
	depthBase := strings.Count(root, string(os.PathSeparator))
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || len(out) >= limit {
			return filepath.SkipAll
		}
		if strings.Count(path, string(os.PathSeparator))-depthBase > 8 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if path == root {
			return nil
		}
		name := strings.ToLower(d.Name())
		if strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.Contains(name, q) {
			if e, err := s.entryFor(sc, filepath.Dir(path), d); err == nil {
				out = append(out, *e)
			}
		}
		return nil
	})
	return out, err
}

// applyOwner chowns a path to the matching OS account when running as root.
func (s *Service) applyOwner(u *store.User, path string) {
	if u == nil || os.Geteuid() != 0 {
		return
	}
	usr, err := user.Lookup(u.Username)
	if err != nil {
		return
	}
	uid, err1 := strconv.Atoi(usr.Uid)
	gid, err2 := strconv.Atoi(usr.Gid)
	if err1 != nil || err2 != nil {
		return
	}
	_ = filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err == nil {
			_ = os.Lchown(p, uid, gid)
		}
		return nil
	})
}

// ---------- helpers ----------

func sanitizeName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "\x00", "")
	if name == "." || name == ".." || name == "" {
		return ""
	}
	return name
}

func movePath(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := CopyTree(src, dst); err != nil {
		return err
	}
	return os.RemoveAll(src)
}

func CopyTree(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil // never follow symlinks when copying
	}
	if info.IsDir() {
		if err := os.MkdirAll(dst, info.Mode().Perm()|0o700); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, de := range entries {
			if err := CopyTree(filepath.Join(src, de.Name()), filepath.Join(dst, de.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func uniquePath(path string) string {
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(filepath.Base(path), ext)
	dir := filepath.Dir(path)
	for i := 1; i < 1000; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
	return filepath.Join(dir, base+"-"+store.RandomToken(3)+ext)
}

func pathSize(path string) int64 {
	var total int64
	info, err := os.Lstat(path)
	if err != nil {
		return 0
	}
	if !info.IsDir() {
		return info.Size()
	}
	walkSize(path, &total, nil)
	return total
}

func walkSize(root string, total *int64, count *int64) {
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			*total += info.Size()
			if count != nil {
				*count++
			}
		}
		return nil
	})
}

func ownerName(info os.FileInfo) string {
	if st, ok := info.Sys().(*statT); ok {
		if u, err := user.LookupId(strconv.Itoa(int(st.Uid))); err == nil {
			return u.Username
		}
	}
	return ""
}

func mimeFor(name string) string {
	if mt := mime.TypeByExtension(strings.ToLower(filepath.Ext(name))); mt != "" {
		if idx := strings.Index(mt, ";"); idx > 0 {
			return mt[:idx]
		}
		return mt
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".md", ".txt", ".log", ".conf", ".yaml", ".yml", ".ini", ".sh", ".go", ".py", ".js", ".ts", ".json", ".xml", ".csv":
		return "text/plain"
	case ".mkv":
		return "video/x-matroska"
	case ".m4v":
		return "video/mp4"
	case ".flac":
		return "audio/flac"
	}
	return "application/octet-stream"
}

var imageExts = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".bmp": true, ".tif": true, ".tiff": true, ".heic": true, ".avif": true}
var videoExts = map[string]bool{".mp4": true, ".mkv": true, ".avi": true, ".mov": true, ".m4v": true, ".webm": true, ".ts": true, ".flv": true, ".wmv": true}
var audioExts = map[string]bool{".mp3": true, ".flac": true, ".wav": true, ".aac": true, ".m4a": true, ".ogg": true, ".opus": true}
var archiveExts = map[string]bool{".zip": true, ".tar": true, ".gz": true, ".tgz": true, ".bz2": true, ".xz": true, ".7z": true, ".rar": true}
var docExts = map[string]bool{".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true, ".ppt": true, ".pptx": true, ".txt": true, ".md": true, ".csv": true}

func classify(info os.FileInfo, name string) string {
	if info.IsDir() {
		return "folder"
	}
	ext := strings.ToLower(filepath.Ext(name))
	switch {
	case imageExts[ext]:
		return "image"
	case videoExts[ext]:
		return "video"
	case audioExts[ext]:
		return "audio"
	case archiveExts[ext]:
		return "archive"
	case docExts[ext]:
		return "document"
	}
	return "file"
}

// IsImage reports whether a filename looks like an image.
func IsImage(name string) bool { return imageExts[strings.ToLower(filepath.Ext(name))] }

// IsVideo reports whether a filename looks like a video.
func IsVideo(name string) bool { return videoExts[strings.ToLower(filepath.Ext(name))] }

// IsArchive reports whether a filename looks like an archive.
func IsArchive(name string) bool { return archiveExts[strings.ToLower(filepath.Ext(name))] }
