// Package thumb generates and caches image/video thumbnails.
package thumb

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rwcarlsen/goexif/exif"
	xdraw "golang.org/x/image/draw"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/webp"
)

// ErrUnsupported is returned for files that cannot be thumbnailed.
var ErrUnsupported = errors.New("不支持生成缩略图")

var ffmpegPath = sync.OnceValue(func() string {
	p, err := exec.LookPath("ffmpeg")
	if err != nil {
		return ""
	}
	return p
})

// Cached returns a cached JPEG thumbnail, generating it when necessary.
func Cached(cacheDir, absPath string, size int) ([]byte, error) {
	if size <= 0 || size > 2048 {
		size = 400
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}
	key := cacheKey(absPath, info.ModTime().UnixNano(), info.Size(), int64(size))
	sub := filepath.Join(cacheDir, key[:2])
	cachePath := filepath.Join(sub, key+".jpg")
	if data, err := os.ReadFile(cachePath); err == nil {
		return data, nil
	}
	data, err := Generate(absPath, size)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(sub, 0o750); err != nil {
		return data, nil
	}
	tmp := cachePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o640); err == nil {
		_ = os.Rename(tmp, cachePath)
	}
	return data, nil
}

// Generate produces a JPEG thumbnail for an image or video.
func Generate(absPath string, size int) ([]byte, error) {
	ext := strings.ToLower(filepath.Ext(absPath))
	if isVideo(ext) {
		return videoFrame(absPath, size)
	}
	if isImage(ext) {
		if ext == ".heic" || ext == ".avif" {
			return nil, ErrUnsupported
		}
		return imageThumb(absPath, size)
	}
	return nil, ErrUnsupported
}

func imageThumb(path string, size int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	if orientation := readOrientation(path); orientation > 1 {
		img = applyOrientation(img, orientation)
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return nil, ErrUnsupported
	}
	scale := float64(size) / float64(max(w, h))
	if scale > 1 {
		scale = 1
	}
	nw, nh := int(float64(w)*scale), int(float64(h)*scale)
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, b, xdraw.Src, nil)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 84}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func videoFrame(path string, size int) ([]byte, error) {
	ff := ffmpegPath()
	if ff == "" {
		return nil, ErrUnsupported
	}
	scale := fmt.Sprintf("scale='min(%d,iw)':-2", size)
	cmd := exec.Command(ff, "-v", "error", "-ss", "00:00:01", "-i", path,
		"-frames:v", "1", "-vf", scale, "-f", "image2", "-vcodec", "mjpeg", "pipe:1")
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// Short clips may not have a frame at 1s; retry at 0.
		cmd2 := exec.Command(ff, "-v", "error", "-i", path, "-frames:v", "1",
			"-vf", scale, "-f", "image2", "-vcodec", "mjpeg", "pipe:1")
		out.Reset()
		cmd2.Stdout = &out
		cmd2.Stderr = &stderr
		if err2 := cmd2.Run(); err2 != nil {
			return nil, fmt.Errorf("ffmpeg: %v", err2)
		}
	}
	if out.Len() == 0 {
		return nil, ErrUnsupported
	}
	return out.Bytes(), nil
}

func readOrientation(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 1
	}
	defer f.Close()
	x, err := exif.Decode(f)
	if err != nil {
		return 1
	}
	tag, err := x.Get(exif.Orientation)
	if err != nil {
		return 1
	}
	v, err := tag.Int(0)
	if err != nil {
		return 1
	}
	return v
}

// applyOrientation rotates/flips an image according to the EXIF orientation tag.
func applyOrientation(src image.Image, orientation int) image.Image {
	b := src.Bounds()
	var dst *image.RGBA
	switch orientation {
	case 3:
		dst = image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		for y := 0; y < b.Dy(); y++ {
			for x := 0; x < b.Dx(); x++ {
				dst.Set(b.Dx()-1-x, b.Dy()-1-y, src.At(b.Min.X+x, b.Min.Y+y))
			}
		}
	case 6:
		dst = image.NewRGBA(image.Rect(0, 0, b.Dy(), b.Dx()))
		for y := 0; y < b.Dy(); y++ {
			for x := 0; x < b.Dx(); x++ {
				dst.Set(b.Dy()-1-y, x, src.At(b.Min.X+x, b.Min.Y+y))
			}
		}
	case 8:
		dst = image.NewRGBA(image.Rect(0, 0, b.Dy(), b.Dx()))
		for y := 0; y < b.Dy(); y++ {
			for x := 0; x < b.Dx(); x++ {
				dst.Set(y, b.Dx()-1-x, src.At(b.Min.X+x, b.Min.Y+y))
			}
		}
	default:
		return src
	}
	return dst
}

func cacheKey(parts ...any) string {
	h := sha1.New()
	for _, p := range parts {
		fmt.Fprintf(h, "%v|", p)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func isImage(ext string) bool {
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".tif", ".tiff", ".heic", ".avif":
		return true
	}
	return false
}

func isVideo(ext string) bool {
	switch ext {
	case ".mp4", ".mkv", ".avi", ".mov", ".m4v", ".webm", ".ts", ".flv", ".wmv":
		return true
	}
	return false
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
