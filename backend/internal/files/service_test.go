package files

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSecureJoinRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	cases := []string{"../etc/passwd", "a/../../etc/passwd", `..\windows`, "ok/../../x"}
	for _, rel := range cases {
		if _, err := SecureJoin(root, rel); err == nil {
			t.Fatalf("SecureJoin(%q) should fail", rel)
		}
	}
	got, err := SecureJoin(root, "/docs/2024/report.pdf")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(root, "docs", "2024", "report.pdf")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSecureJoinBlocksSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skip("symlinks unavailable")
	}
	if _, err := SecureJoin(root, "link/secret.txt"); err == nil {
		t.Fatal("symlink escape should be rejected")
	}
}

func TestSanitizeName(t *testing.T) {
	cases := map[string]string{
		"hello.txt":    "hello.txt",
		"../evil":      ".._evil",
		"a/b":          "a_b",
		"..":           "",
		"":             "",
		"  spaced.txt": "spaced.txt",
	}
	for in, want := range cases {
		if got := sanitizeName(in); got != want {
			t.Errorf("sanitizeName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestClassify(t *testing.T) {
	dir, err := os.MkdirTemp("", "classify")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	info, _ := os.Stat(dir)
	if got := classify(info, "photos"); got != "folder" {
		t.Errorf("folder classification = %q", got)
	}
	if !IsImage("a.JPG") || !IsVideo("b.mkv") || !IsArchive("c.tar.gz") {
		t.Error("extension helpers returned unexpected values")
	}
	if IsImage("notes.txt") {
		t.Error("txt should not be an image")
	}
}

func TestPathHelpers(t *testing.T) {
	if within("/data/users/a", "/data/users/ab") {
		t.Error("prefix without separator must not count as inside")
	}
	if !within("/data/users/a", "/data/users/a/x") {
		t.Error("child should be inside")
	}
	if got := parentPath("a/b/c"); got != "a/b" {
		t.Errorf("parentPath = %q", got)
	}
	if got := parentPath(""); got != "" {
		t.Errorf("parentPath empty = %q", got)
	}
	if got := mimeFor("movie.mkv"); got != "video/x-matroska" {
		t.Errorf("mimeFor mkv = %q", got)
	}
}

func TestUniquePath(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(first, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := uniquePath(first)
	if !strings.Contains(got, "(1)") {
		t.Fatalf("expected deduplicated name, got %q", got)
	}
	if _, err := os.Stat(got); !os.IsNotExist(err) {
		t.Fatal("uniquePath must return a free name")
	}
}
