package shares

import (
	"strings"
	"testing"

	"privacyos/backend/internal/config"
	"privacyos/backend/internal/store"
)

func testService(t *testing.T) *Service {
	t.Helper()
	cfg := &config.Config{
		Dev: true, DataDir: t.TempDir(), ConfigDir: t.TempDir(), LogDir: t.TempDir(),
		CacheDir: t.TempDir(), DBPath: t.TempDir() + "/test.db",
		Edition: "normal", AdminUser: "padmin", AdminPassword: "privacyos-test",
	}
	st, err := store.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return New(cfg, st)
}

func TestRenderSamba(t *testing.T) {
	s := testService(t)
	shares := []store.NetworkShare{
		{Name: "users", Path: "/data/users", Comment: "用户文件", Protocols: []string{"smb"}, ValidUsers: "@users"},
		{Name: "photos", Path: "/data/photos", Comment: "照片", Protocols: []string{"smb"}, ReadOnly: true},
		{Name: "nfs-only", Path: "/data/nfs", Protocols: []string{"nfs"}},
	}
	out := s.RenderSamba(shares, false)
	for _, want := range []string{"[global]", "[users]", "path = /data/users", "valid users = @users",
		"[photos]", "read only = yes", "server min protocol = SMB2_10", "smb encrypt = desired"} {
		if !strings.Contains(out, want) {
			t.Errorf("smb.conf missing %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "[nfs-only]") {
		t.Error("NFS-only share must not appear in smb.conf")
	}
	if strings.Contains(out, "hide unreadable") {
		t.Error("normal edition should not force hide unreadable")
	}
	pro := s.RenderSamba(shares, true)
	if !strings.Contains(pro, "hide unreadable = yes") {
		t.Error("pro edition should hide unreadable entries")
	}
}

func TestRenderExports(t *testing.T) {
	s := testService(t)
	shares := []store.NetworkShare{
		{Name: "users", Path: "/data/users", Protocols: []string{"nfs", "smb"}},
		{Name: "photos", Path: "/data/photos", Protocols: []string{"smb"}},
	}
	out := s.RenderExports(shares, []string{"192.168.1.0/24"})
	if !strings.Contains(out, "/data/users 192.168.1.0/24(rw") {
		t.Fatalf("unexpected exports:\n%s", out)
	}
	if strings.Contains(out, "/data/photos") {
		t.Fatal("SMB-only share must not be exported via NFS")
	}
	ro := s.RenderExports([]store.NetworkShare{{Name: "x", Path: "/data/x", Protocols: []string{"nfs"}, ReadOnly: true}}, nil)
	if !strings.Contains(ro, "(ro,") {
		t.Fatalf("expected read-only export: %s", ro)
	}
	for _, c := range []string{"192.168.0.0/16", "10.0.0.0/8"} {
		if !strings.Contains(ro, c) {
			t.Errorf("default client %s missing", c)
		}
	}
}
