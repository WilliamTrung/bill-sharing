package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const CookieName = "bs_session"

type Manager struct {
	secret []byte
	ttl    time.Duration
}

func New(secret string, ttl time.Duration) *Manager {
	return &Manager{secret: []byte(secret), ttl: ttl}
}

func (m *Manager) Issue(now time.Time) string {
	issued := strconv.FormatInt(now.Unix(), 10)
	return issued + "." + sign(m.secret, issued)
}

func (m *Manager) ValidToken(token string, now time.Time) bool {
	issued, sig, ok := strings.Cut(token, ".")
	if !ok || sig == "" || issued == "" {
		return false
	}
	if !hmac.Equal([]byte(sig), []byte(sign(m.secret, issued))) {
		return false
	}
	unix, err := strconv.ParseInt(issued, 10, 64)
	if err != nil {
		return false
	}
	issuedAt := time.Unix(unix, 0)
	if issuedAt.After(now.Add(5 * time.Minute)) {
		return false
	}
	return now.Sub(issuedAt) <= m.ttl
}

func (m *Manager) RequestValid(r *http.Request) bool {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return false
	}
	return m.ValidToken(cookie.Value, time.Now())
}

func (m *Manager) TTL() time.Duration {
	return m.ttl
}

func sign(secret []byte, payload string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
