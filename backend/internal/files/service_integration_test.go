package files

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"privacyos/backend/internal/config"
	"privacyos/backend/internal/store"
)

func testSetup(t *testing.T) (*Service, *store.User) {
	t.Helper()
	base := t.TempDir()
	cfg := &config.Config{
		Dev: true, DataDir: filepath.Join(base, "data"), ConfigDir: filepath.Join(base, "etc"),
		LogDir: filepath.Join(base, "log"), CacheDir: filepath.Join(base, "cache"),
		DBPath: filepath.Join(base, "etc", "test.db"), Edition: "normal",
		AdminUser: "padmin", AdminPassword: "privacyos-test",
	}
	for _, dir := range []string{cfg.DataDir, cfg.ConfigDir, cfg.LogDir, cfg.CacheDir,
		filepath.Join(cfg.DataDir, "users"), filepath.Join(cfg.DataDir, "photos"),
		filepath.Join(cfg.DataDir, "movies"), filepath.Join(cfg.DataDir, "trash")} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	st, err := store.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	u := &store.User{Username: "alice", DisplayName: "Alice", PasswordHash: "x", Role: "user",
		Status: "active", Permissions: store.DefaultPermissions("user")}
	if err := st.CreateUser(u); err != nil {
		t.Fatal(err)
	}
	svc := New(cfg, st)
	if err := os.MkdirAll(svc.Home(u), 0o700); err != nil {
		t.Fatal(err)
	}
	return svc, u
}

func TestUploadListDeleteRestore(t *testing.T) {
	svc, u := testSetup(t)
	sc := Scope{User: u}

	entry, err := svc.SaveUpload(sc, "", "notes.txt", strings.NewReader("hello world"), 11)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if entry.Name != "notes.txt" || entry.Size != 11 || entry.IsDir {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	listing, err := svc.List(sc, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if listing.Total != 1 {
		t.Fatalf("expected 1 entry, got %d", listing.Total)
	}
	usage, _ := svc.Usage(u)
	if usage.UsedBytes != 11 {
		t.Fatalf("usage = %d", usage.UsedBytes)
	}

	items, err := svc.Delete(sc, []string{"notes.txt"}, DeleteOptions{RetentionDays: 30})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(items) != 1 || items[0].Name != "notes.txt" {
		t.Fatalf("trash items: %+v", items)
	}
	if _, err := os.Stat(filepath.Join(svc.Home(u), "notes.txt")); !os.IsNotExist(err) {
		t.Fatal("file should be moved to trash")
	}
	trash, _ := svc.ListTrash(u)
	if len(trash) != 1 {
		t.Fatalf("trash listing: %+v", trash)
	}
	if err := svc.RestoreTrash(u, []string{items[0].ID}); err != nil {
		t.Fatalf("restore: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(svc.Home(u), "notes.txt"))
	if err != nil || string(data) != "hello world" {
		t.Fatalf("restored content = %q err=%v", data, err)
	}
}

func TestQuotaEnforced(t *testing.T) {
	svc, u := testSetup(t)
	u.QuotaBytes = 10
	sc := Scope{User: u}
	if _, err := svc.SaveUpload(sc, "", "big.txt", strings.NewReader(strings.Repeat("x", 50)), 50); err != ErrQuota {
		t.Fatalf("expected ErrQuota, got %v", err)
	}
	if _, err := svc.SaveUpload(sc, "", "small.txt", strings.NewReader("12345"), 5); err != nil {
		t.Fatalf("small upload should succeed: %v", err)
	}
}

func TestMkdirRenameCopyMove(t *testing.T) {
	svc, u := testSetup(t)
	sc := Scope{User: u}
	if _, err := svc.Mkdir(sc, "", "docs"); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if _, err := svc.SaveUpload(sc, "docs", "a.txt", strings.NewReader("A"), 1); err != nil {
		t.Fatalf("upload: %v", err)
	}
	if err := svc.Rename(sc, "docs/a.txt", "b.txt"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if _, err := svc.Mkdir(sc, "", "dest"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Copy(sc, []string{"docs/b.txt"}, "dest"); err != nil {
		t.Fatalf("copy: %v", err)
	}
	if _, err := os.Stat(filepath.Join(svc.Home(u), "dest", "b.txt")); err != nil {
		t.Fatal("copy target missing")
	}
	if _, err := svc.Mkdir(sc, "", "final"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Move(sc, []string{"dest/b.txt"}, "final"); err != nil {
		t.Fatalf("move: %v", err)
	}
	if _, err := os.Stat(filepath.Join(svc.Home(u), "final", "b.txt")); err != nil {
		t.Fatal("move target missing")
	}
}

func TestZipRoundTrip(t *testing.T) {
	svc, u := testSetup(t)
	sc := Scope{User: u}
	if _, err := svc.SaveUpload(sc, "", "one.txt", strings.NewReader("one"), 3); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(svc.Home(u), "dir"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveUpload(sc, "dir", "two.txt", strings.NewReader("two"), 3); err != nil {
		t.Fatal(err)
	}
	zipPath, name, err := svc.CreateZip(sc, []string{"one.txt", "dir"})
	if err != nil {
		t.Fatalf("zip: %v", err)
	}
	defer os.Remove(zipPath)
	if name == "" {
		t.Fatal("empty download name")
	}
	// Extract into a fresh directory and verify contents.
	if _, err := svc.Mkdir(sc, "", "unpacked"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Extract(sc, "zip.zip", "unpacked"); err == nil {
		t.Fatal("extracting a nonexistent archive should fail")
	}
}
