// Package auth implements password hashing, JWT sessions and TOTP 2FA.
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"

	"privacyos/backend/internal/store"
)

// Errors returned by the auth package.
var (
	ErrInvalidToken = errors.New("无效的登录凭证")
	ErrExpiredToken = errors.New("登录已过期")
)

// Manager issues and validates tokens.
type Manager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// New creates a token manager.
func New(secret []byte) *Manager {
	return &Manager{secret: secret, accessTTL: 12 * time.Hour, refreshTTL: 7 * 24 * time.Hour}
}

// Claims is the JWT payload.
type Claims struct {
	UserID   int64  `json:"uid"`
	Username string `json:"usr"`
	Role     string `json:"role"`
	Type     string `json:"typ"` // access | refresh
	jwt.RegisteredClaims
}

// TokenPair is returned after a successful login.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// Issue creates a new access/refresh pair bound to a session ID.
func (m *Manager) Issue(u *store.User, sessionID string) (*TokenPair, error) {
	now := time.Now()
	accessExp := now.Add(m.accessTTL)
	refreshExp := now.Add(m.refreshTTL)

	access := Claims{
		UserID: u.ID, Username: u.Username, Role: u.Role, Type: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ID: sessionID, Subject: fmt.Sprint(u.ID), Issuer: "privacyos",
			IssuedAt: jwt.NewNumericDate(now), NotBefore: jwt.NewNumericDate(now.Add(-30 * time.Second)),
			ExpiresAt: jwt.NewNumericDate(accessExp),
		},
	}
	refresh := Claims{
		UserID: u.ID, Username: u.Username, Role: u.Role, Type: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			ID: sessionID, Subject: fmt.Sprint(u.ID), Issuer: "privacyos",
			IssuedAt: jwt.NewNumericDate(now), NotBefore: jwt.NewNumericDate(now.Add(-30 * time.Second)),
			ExpiresAt: jwt.NewNumericDate(refreshExp),
		},
	}
	accessStr, err := jwt.NewWithClaims(jwt.SigningMethodHS256, access).SignedString(m.secret)
	if err != nil {
		return nil, err
	}
	refreshStr, err := jwt.NewWithClaims(jwt.SigningMethodHS256, refresh).SignedString(m.secret)
	if err != nil {
		return nil, err
	}
	return &TokenPair{AccessToken: accessStr, RefreshToken: refreshStr, ExpiresIn: int64(m.accessTTL.Seconds())}, nil
}

// Parse validates a token and returns its claims.
func (m *Manager) Parse(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return m.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired(), jwt.WithIssuer("privacyos"))
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}
	if !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// SessionExpiry returns the refresh-token expiry stored on a session.
func (m *Manager) SessionExpiry() string {
	return time.Now().Add(m.refreshTTL).UTC().Format(time.RFC3339)
}

// ---------- passwords ----------

// HashPassword returns a bcrypt hash.
func HashPassword(password string) (string, error) {
	if len(password) < 8 {
		return "", fmt.Errorf("密码至少需要 8 个字符")
	}
	if len(password) > 72 {
		return "", fmt.Errorf("密码不能超过 72 个字符")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword checks a password against a hash in constant time.
func VerifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// dummyHash is used to equalize timing for unknown accounts.
var dummyHash = "$2a$12$C6UzMDM.H6dfI/f/IKcEeO7Z0V9Wl0nF4QZ7K9pQ8s4rM2wJ6n1Ee"

// BurnCompare performs a dummy bcrypt comparison.
func BurnCompare(password string) {
	_ = bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte(password))
}

// ---------- TOTP ----------

// NewTOTP generates a new TOTP secret for an account.
func NewTOTP(username string) (secret, otpauthURL string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "PrivacyOS", AccountName: username})
	if err != nil {
		return "", "", err
	}
	return key.Secret(), key.URL(), nil
}

// VerifyTOTP validates a 6-digit code with a one-step clock skew.
func VerifyTOTP(secret, code string) bool {
	if secret == "" {
		return false
	}
	valid, err := totp.ValidateCustom(code, secret, time.Now().UTC(), totp.ValidateOpts{
		Period: 30, Skew: 1, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1,
	})
	return err == nil && valid
}
