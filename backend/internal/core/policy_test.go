package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"privacyos/backend/internal/config"
	"privacyos/backend/internal/store"
)

func testApp(t *testing.T, edition string) *App {
	t.Helper()
	base := t.TempDir()
	cfg := &config.Config{
		Version: "test", Edition: edition, Dev: true,
		DataDir: filepath.Join(base, "data"), ConfigDir: filepath.Join(base, "etc"),
		LogDir: filepath.Join(base, "log"), CacheDir: filepath.Join(base, "cache"),
		DBPath: filepath.Join(base, "etc", "privacyos.db"),
		AdminUser: "padmin", AdminPassword: "privacyos-test", Secret: []byte("test-secret"),
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
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	app, err := New(cfg, st)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	return app
}

func TestBackoffDelay(t *testing.T) {
	cases := []struct {
		failures, from, base, max int
		want                      time.Duration
	}{
		{1, 3, 3, 300, 0},
		{2, 3, 3, 300, 0},
		{3, 3, 3, 300, 3 * time.Second},
		{4, 3, 3, 300, 6 * time.Second},
		{5, 3, 3, 300, 12 * time.Second},
		{6, 3, 3, 300, 24 * time.Second},
		{7, 3, 3, 300, 48 * time.Second},
		{9, 3, 3, 300, 192 * time.Second},
		{10, 3, 3, 300, 300 * time.Second}, // capped
		{100, 3, 3, 300, 300 * time.Second},
		{3, 3, 0, 0, 0},
	}
	for _, c := range cases {
		got := BackoffDelay(c.failures, c.from, c.base, c.max)
		if got != c.want {
			t.Errorf("BackoffDelay(%d,%d,%d,%d) = %v, want %v",
				c.failures, c.from, c.base, c.max, got, c.want)
		}
	}
}

func TestLoginGateAndFailureFlow(t *testing.T) {
	app := testApp(t, "normal")
	u, err := app.Store.GetUserByUsername("padmin")
	if err != nil {
		t.Fatal(err)
	}
	gate := app.LoginGateFor(u.Username)
	if !gate.Allowed || gate.Failures != 0 {
		t.Fatalf("initial gate: %+v", gate)
	}
	// Three failures trigger the exponential delay.
	for i := 0; i < 3; i++ {
		app.RecordLoginFailure(u, "127.0.0.1")
	}
	gate = app.LoginGateFor(u.Username)
	if gate.Allowed {
		t.Fatalf("expected delay gate, got %+v", gate)
	}
	if gate.RetryAfter <= 0 {
		t.Fatalf("expected retry_after, got %+v", gate)
	}
	// Fill up to the lockout threshold.
	settings := app.LoadSettings()
	for i := 3; i < settings.Security.MaxAttempts; i++ {
		app.RecordLoginFailure(u, "127.0.0.1")
	}
	reloaded, _ := app.Store.GetUserByUsername(u.Username)
	if reloaded.LockedUntil == "" {
		t.Fatal("account should be locked")
	}
	gate = app.LoginGateFor(u.Username)
	if gate.Allowed || !gate.Locked {
		t.Fatalf("expected locked gate, got %+v", gate)
	}
	// Unlocking clears the state.
	if err := app.Store.Unlock(u.ID); err != nil {
		t.Fatal(err)
	}
	_ = app.Store.ClearLoginAttempts(u.Username)
	gate = app.LoginGateFor(u.Username)
	if !gate.Allowed {
		t.Fatalf("unlock failed: %+v", gate)
	}
}

func TestDestroyPolicyTriggersInProOnly(t *testing.T) {
	normal := testApp(t, "normal")
	u, _ := normal.Store.GetUserByUsername("padmin")
	out := normal.RecordLoginFailure(u, "127.0.0.1")
	if out.DestroyTriggered {
		t.Fatal("normal edition must never trigger destruction")
	}

	pro := testApp(t, "pro")
	st := pro.LoadSettings()
	st.Destroy.Enabled = true
	st.Destroy.Threshold = 3
	st.Destroy.Method = "delete"
	if err := pro.Store.SetSetting("destroy", st.Destroy); err != nil {
		t.Fatal(err)
	}
	victim := &store.User{Username: "target", DisplayName: "Target", PasswordHash: "x",
		Role: "user", Status: "active", Permissions: store.DefaultPermissions("user")}
	if err := pro.Store.CreateUser(victim); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		out = pro.RecordLoginFailure(victim, "127.0.0.1")
	}
	if !out.DestroyTriggered {
		t.Fatalf("expected destruction trigger, got %+v", out)
	}
	// Wait for the async job to finish.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		jobs := pro.Jobs.List(10)
		done := false
		for _, j := range jobs {
			if j.Status != "running" {
				done = true
			}
		}
		if done {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestAutoDestroyProtectsLastAdmin(t *testing.T) {
	app := testApp(t, "pro")
	st := app.LoadSettings()
	st.Destroy.Enabled = true
	st.Destroy.Threshold = 3
	st.Destroy.Method = "delete"
	if err := app.Store.SetSetting("destroy", st.Destroy); err != nil {
		t.Fatal(err)
	}
	admin, _ := app.Store.GetUserByUsername("padmin")
	var out LoginOutcome
	for i := 0; i < 3; i++ {
		out = app.RecordLoginFailure(admin, "127.0.0.1")
	}
	if out.DestroyTriggered {
		t.Fatal("auto destruction must not target the last administrator")
	}
	reloaded, _ := app.Store.GetUserByUsername("padmin")
	if reloaded.Status != "active" {
		t.Fatalf("admin should remain active, got %q", reloaded.Status)
	}
}

func TestExecuteDestroyRemovesData(t *testing.T) {
	app := testApp(t, "pro")
	username := "victim"
	u := &store.User{
		Username: username, DisplayName: "Victim", PasswordHash: "x",
		Role: "user", Status: "active", Permissions: store.DefaultPermissions("user"),
	}
	if err := app.Store.CreateUser(u); err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(app.Cfg.DataDir, "users", username)
	photos := filepath.Join(app.Cfg.DataDir, "photos", username)
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(photos, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "secret.txt"), []byte("top secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(photos, "photo.jpg"), []byte("jpegdata"), 0o600); err != nil {
		t.Fatal(err)
	}
	settings := app.LoadSettings().Destroy
	settings.Method = "overwrite"
	settings.Silent = true
	err := app.ExecuteDestroy(context.Background(), username, settings, "unit-test", func(int, string) {})
	if err != nil {
		t.Fatalf("destroy: %v", err)
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Fatal("home directory should be removed")
	}
	if _, err := os.Stat(photos); !os.IsNotExist(err) {
		t.Fatal("photos directory should be removed")
	}
	reloaded, _ := app.Store.GetUserByUsername(username)
	if reloaded.Status != "disabled" {
		t.Fatalf("account should be disabled, got %q", reloaded.Status)
	}
	events, _ := app.Store.ListDestroyEvents(10)
	if len(events) == 0 || events[0].Status != "completed" {
		t.Fatalf("expected completed destroy event, got %+v", events)
	}
}

func TestVaultDevLifecycle(t *testing.T) {
	app := testApp(t, "pro")
	u := &store.User{Username: "vaultuser", DisplayName: "Vault", PasswordHash: "x",
		Role: "user", Status: "active", Permissions: store.DefaultPermissions("user")}
	if err := app.Store.CreateUser(u); err != nil {
		t.Fatal(err)
	}
	if err := app.CreateVault(context.Background(), u, 1<<30); err != nil {
		t.Fatalf("create vault: %v", err)
	}
	if !app.VaultExists("vaultuser") {
		t.Fatal("vault should exist")
	}
	info := app.VaultInfo("vaultuser")
	if !info.Unlocked {
		t.Fatal("dev vault should be unlocked")
	}
	if _, err := app.DestroyVault(context.Background(), u); err != nil {
		t.Fatalf("destroy vault: %v", err)
	}
	if app.VaultExists("vaultuser") {
		t.Fatal("vault should be gone")
	}
}

func TestJanitorPurgesExpiredUsers(t *testing.T) {
	app := testApp(t, "pro")
	u := &store.User{Username: "expired", DisplayName: "Expired", PasswordHash: "x",
		Role: "user", Status: "active", Permissions: store.DefaultPermissions("user")}
	if err := app.Store.CreateUser(u); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	if err := app.Store.MarkPendingDeletion(u.ID, past); err != nil {
		t.Fatal(err)
	}
	app.RunJanitor()
	if _, err := app.Store.GetUserByUsername("expired"); err == nil {
		t.Fatal("expired account should be purged")
	}
}
