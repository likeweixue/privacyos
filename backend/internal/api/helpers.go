package api

import (
	"crypto/hmac"
	"encoding/json"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// nowPlusHours returns an RFC3339 timestamp h hours from now.
func nowPlusHours(h int) string {
	return time.Now().Add(time.Duration(h) * time.Hour).UTC().Format(time.RFC3339)
}

// hashSharePassword hashes a share password.
func hashSharePassword(password string) (string, error) {
	if len(password) < 4 {
		return "", fmt.Errorf("分享密码至少需要 4 个字符")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	return string(hash), err
}

// asciiFallback strips non-ASCII characters for Content-Disposition.
func asciiFallback(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r >= 32 && r < 127 && r != '"' && r != '\\' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "download"
	}
	return b.String()
}

// urlEncode escapes a filename for RFC 5987.
func urlEncode(s string) string {
	return strings.ReplaceAll(url.PathEscape(s), "+", "%20")
}

const (
	shareCookie = "pos_share"
	keepSecret  = "__keep__"
)

// makeShareGrant signs a short-lived download grant for a share token.
func (s *Server) makeShareGrant(token string) string {
	exp := strconv.FormatInt(time.Now().Add(2*time.Hour).Unix(), 10)
	mac := hmac.New(sha256.New, s.Cfg.Secret)
	mac.Write([]byte(token + "|" + exp))
	return exp + "." + hex.EncodeToString(mac.Sum(nil))
}

// checkShareGrant validates a download grant cookie.
func (s *Server) checkShareGrant(c *gin.Context, token string) bool {
	value, err := c.Cookie(shareCookie)
	if err != nil {
		return false
	}
	parts := strings.SplitN(value, ".", 2)
	if len(parts) != 2 {
		return false
	}
	exp, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return false
	}
	mac := hmac.New(sha256.New, s.Cfg.Secret)
	mac.Write([]byte(token + "|" + parts[0]))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(parts[1]))
}

// mergeJSON deep-merges a JSON patch into a settings struct. Values equal to
// keepSecret are ignored so masked secrets can be round-tripped by the UI.
func mergeJSON(target any, patch map[string]any) error {
	base, err := json.Marshal(target)
	if err != nil {
		return err
	}
	var merged map[string]any
	if err := json.Unmarshal(base, &merged); err != nil {
		return err
	}
	deepMerge(merged, patch)
	out, err := json.Marshal(merged)
	if err != nil {
		return err
	}
	return json.Unmarshal(out, target)
}

func deepMerge(dst, src map[string]any) {
	for k, v := range src {
		if s, ok := v.(string); ok && s == keepSecret {
			continue
		}
		if sub, ok := v.(map[string]any); ok {
			if existing, ok := dst[k].(map[string]any); ok {
				deepMerge(existing, sub)
				continue
			}
		}
		dst[k] = v
	}
}
