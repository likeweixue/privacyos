package files

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"privacyos/backend/internal/store"
	"privacyos/backend/internal/thumb"
)

// CreateZip archives the selected paths into a temporary zip file.
func (s *Service) CreateZip(sc Scope, rels []string) (string, string, error) {
	if len(rels) == 0 {
		return "", "", fmt.Errorf("未选择文件")
	}
	if err := os.MkdirAll(filepath.Join(s.Cfg.CacheDir, "tmp"), 0o750); err != nil {
		return "", "", err
	}
	name := "privacyos-export-" + store.RandomToken(4) + ".zip"
	tmp := filepath.Join(s.Cfg.CacheDir, "tmp", name)
	f, err := os.Create(tmp)
	if err != nil {
		return "", "", err
	}
	zw := zip.NewWriter(f)
	var first string
	for _, rel := range rels {
		abs, err := s.Resolve(sc, rel)
		if err != nil {
			zw.Close()
			f.Close()
			os.Remove(tmp)
			return "", "", err
		}
		base := filepath.Base(abs)
		if first == "" {
			first = base
		}
		if err := addToZip(zw, abs, base); err != nil {
			zw.Close()
			f.Close()
			os.Remove(tmp)
			return "", "", err
		}
	}
	if err := zw.Close(); err != nil {
		f.Close()
		return "", "", err
	}
	if err := f.Close(); err != nil {
		return "", "", err
	}
	downloadName := first + ".zip"
	if len(rels) > 1 {
		downloadName = "privacyos-export.zip"
	}
	return tmp, downloadName, nil
}

func addToZip(zw *zip.Writer, abs, entryName string) error {
	info, err := os.Lstat(abs)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	if info.IsDir() {
		header := &zip.FileHeader{Name: entryName + "/", Method: zip.Store}
		header.SetMode(info.Mode())
		if _, err := zw.CreateHeader(header); err != nil {
			return err
		}
		entries, err := os.ReadDir(abs)
		if err != nil {
			return err
		}
		for _, de := range entries {
			if err := addToZip(zw, filepath.Join(abs, de.Name()), entryName+"/"+de.Name()); err != nil {
				return err
			}
		}
		return nil
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = entryName
	header.Method = zip.Deflate
	w, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	src, err := os.Open(abs)
	if err != nil {
		return err
	}
	defer src.Close()
	_, err = io.Copy(w, src)
	return err
}

// Extract unpacks a zip/tar archive inside the scope.
func (s *Service) Extract(sc Scope, rel, destRel string) (string, error) {
	abs, err := s.Resolve(sc, rel)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(abs); err != nil {
		return "", ErrNotFound
	}
	base := filepath.Base(abs)
	for _, suffix := range []string{".tar.gz", ".tgz", ".zip", ".tar", ".gz", ".bz2", ".xz", ".7z", ".rar"} {
		if strings.HasSuffix(strings.ToLower(base), suffix) {
			base = base[:len(base)-len(suffix)]
			break
		}
	}
	destDir := destRel
	if destDir == "" {
		destDir = parentPath(s.Relative(sc, abs))
	}
	destAbs, err := s.Resolve(sc, destDir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(destAbs, 0o700); err != nil {
		return "", err
	}
	target := filepath.Join(destAbs, base)
	if _, err := os.Stat(target); err == nil {
		target = uniquePath(target)
	}
	if err := os.MkdirAll(target, 0o700); err != nil {
		return "", err
	}

	lower := strings.ToLower(abs)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		err = extractZip(abs, target)
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		err = extractTarGz(abs, target)
	case strings.HasSuffix(lower, ".tar"):
		err = extractTar(abs, target)
	default:
		return "", fmt.Errorf("暂不支持该压缩格式，请使用 zip 或 tar.gz")
	}
	if err != nil {
		_ = os.RemoveAll(target)
		return "", err
	}
	s.applyOwner(sc.User, target)
	s.Invalidate(sc.User.ID)
	return s.Relative(sc, target), nil
}

func extractZip(src, dest string) error {
	zr, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, f := range zr.File {
		target, err := safeArchivePath(dest, f.Name)
		if err != nil {
			return err
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o700); err != nil {
				return err
			}
			continue
		}
		if f.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode().Perm()|0o600)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func extractTarGz(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	return extractTarReader(gz, dest)
}

func extractTar(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	return extractTarReader(f, dest)
}

func extractTarReader(r io.Reader, dest string) error {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target, err := safeArchivePath(dest, hdr.Name)
		if err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o700); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		default:
			// skip devices, symlinks and other special entries
		}
	}
}

func safeArchivePath(dest, name string) (string, error) {
	name = strings.ReplaceAll(name, "\\", "/")
	name = strings.TrimPrefix(name, "/")
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "." || strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return "", fmt.Errorf("压缩包包含非法路径: %s", name)
	}
	target := filepath.Join(dest, clean)
	if !within(dest, target) {
		return "", fmt.Errorf("压缩包包含非法路径: %s", name)
	}
	return target, nil
}

// Thumbnail returns a cached thumbnail for a file in the scope.
func (s *Service) Thumbnail(sc Scope, rel string, size int) ([]byte, error) {
	abs, err := s.Resolve(sc, rel)
	if err != nil {
		return nil, err
	}
	return thumb.Cached(filepath.Join(s.Cfg.CacheDir, "thumbs"), abs, size)
}

// ZipAbsolute archives absolute paths (used for public directory shares).
func (s *Service) ZipAbsolute(paths []string, downloadName string) (string, error) {
	if len(paths) == 0 {
		return "", fmt.Errorf("没有可打包的路径")
	}
	if err := os.MkdirAll(filepath.Join(s.Cfg.CacheDir, "tmp"), 0o750); err != nil {
		return "", err
	}
	tmp := filepath.Join(s.Cfg.CacheDir, "tmp", "share-"+store.RandomToken(6)+".zip")
	f, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	zw := zip.NewWriter(f)
	for _, abs := range paths {
		if !filepath.IsAbs(abs) {
			zw.Close()
			f.Close()
			os.Remove(tmp)
			return "", fmt.Errorf("非法路径")
		}
		if err := addToZip(zw, abs, filepath.Base(abs)); err != nil {
			zw.Close()
			f.Close()
			os.Remove(tmp)
			return "", err
		}
	}
	if err := zw.Close(); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return tmp, nil
}
