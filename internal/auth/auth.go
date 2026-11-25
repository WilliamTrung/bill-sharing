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

func (m *Manager) Issue(userID int64, now time.Time) string {
	payload := strconv.FormatInt(userID, 10) + "." + strconv.FormatInt(now.Unix(), 10)
	return payload + "." + sign(m.secret, payload)
}

func (m *Manager) Parse(token string, now time.Time) (int64, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return 0, false
	}
	userID, issued, sig := parts[0], parts[1], parts[2]
	if userID == "" || issued == "" || sig == "" {
		return 0, false
	}
	payload := userID + "." + issued
	if !hmac.Equal([]byte(sig), []byte(sign(m.secret, payload))) {
		return 0, false
	}

	id, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	unix, err := strconv.ParseInt(issued, 10, 64)
	if err != nil {
		return 0, false
	}
	issuedAt := time.Unix(unix, 0)
	if issuedAt.After(now.Add(5 * time.Minute)) {
		return 0, false
	}
	if now.Sub(issuedAt) > m.ttl {
		return 0, false
	}
	return id, true
}

func (m *Manager) RequestUser(r *http.Request) (int64, bool) {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return 0, false
	}
	return m.Parse(cookie.Value, time.Now())
}

func (m *Manager) RequestValid(r *http.Request) bool {
	_, ok := m.RequestUser(r)
	return ok
}

func (m *Manager) TTL() time.Duration {
	return m.ttl
}

func sign(secret []byte, payload string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
