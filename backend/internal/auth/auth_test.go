package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"privacyos/backend/internal/store"
)

func testUser() *store.User {
	return &store.User{ID: 7, Username: "alice", Role: "user", Status: "active"}
}

func TestPasswordHashAndVerify(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !VerifyPassword(hash, "correct-horse-battery") {
		t.Fatal("expected password to verify")
	}
	if VerifyPassword(hash, "wrong-password") {
		t.Fatal("wrong password must not verify")
	}
	if _, err := HashPassword("short"); err == nil {
		t.Fatal("short password should be rejected")
	}
	if _, err := HashPassword(strings.Repeat("a", 100)); err == nil {
		t.Fatal("overlong password should be rejected")
	}
}

func TestTokenRoundTrip(t *testing.T) {
	m := New([]byte("0123456789abcdef0123456789abcdef"))
	pair, err := m.Issue(testUser(), "session-1")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	claims, err := m.Parse(pair.AccessToken)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.UserID != 7 || claims.Username != "alice" || claims.Type != "access" || claims.ID != "session-1" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	refresh, err := m.Parse(pair.RefreshToken)
	if err != nil || refresh.Type != "refresh" {
		t.Fatalf("refresh claims: %+v err=%v", refresh, err)
	}
	if _, err := m.Parse(pair.AccessToken + "x"); err == nil {
		t.Fatal("tampered token must fail")
	}
	other := New([]byte("ffffffffffffffffffffffffffffffff"))
	if _, err := other.Parse(pair.AccessToken); err == nil {
		t.Fatal("token signed with another key must fail")
	}
}

func TestTOTP(t *testing.T) {
	secret, url, err := NewTOTP("alice")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if secret == "" || url == "" {
		t.Fatal("empty totp material")
	}
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("code: %v", err)
	}
	if !VerifyTOTP(secret, code) {
		t.Fatal("valid code rejected")
	}
	if VerifyTOTP(secret, "000000") {
		t.Fatal("invalid code accepted")
	}
	if VerifyTOTP("", code) {
		t.Fatal("empty secret accepted")
	}
}
