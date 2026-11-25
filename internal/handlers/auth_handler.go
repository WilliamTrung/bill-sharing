package handlers

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/WilliamTrung/bill-sharing/internal/accounts"
	"github.com/WilliamTrung/bill-sharing/internal/auth"
)

type loginEntry struct {
	failures int
	resetAt  time.Time
}

type loginLimiter struct {
	mu      sync.Mutex
	entries map[string]*loginEntry
	max     int
	window  time.Duration
}

func newLoginLimiter(max int, window time.Duration) *loginLimiter {
	return &loginLimiter{entries: make(map[string]*loginEntry), max: max, window: window}
}

func (l *loginLimiter) allowed(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry, ok := l.entries[key]
	if !ok || time.Now().After(entry.resetAt) {
		return true
	}
	return entry.failures < l.max
}

func (l *loginLimiter) fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.entries) > 1000 {
		now := time.Now()
		for k, entry := range l.entries {
			if now.After(entry.resetAt) {
				delete(l.entries, k)
			}
		}
	}
	entry, ok := l.entries[key]
	if !ok || time.Now().After(entry.resetAt) {
		l.entries[key] = &loginEntry{failures: 1, resetAt: time.Now().Add(l.window)}
		return
	}
	entry.failures++
}

func (l *loginLimiter) reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Identifier string `json:"identifier"`
		Password   string `json:"password"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	key := clientIP(r)
	if !s.limiter.allowed(key) {
		writeError(w, http.StatusTooManyRequests, "Too many attempts. Try again later.")
		return
	}

	user, err := accounts.Authenticate(r.Context(), s.db, body.Identifier, body.Password)
	if err != nil {
		if errors.Is(err, accounts.ErrInvalidCredentials) {
			s.limiter.fail(key)
			writeError(w, http.StatusUnauthorized, "Incorrect username, email or password")
			return
		}
		s.logger.Error("authenticate", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not sign in")
		return
	}
	s.limiter.reset(key)

	http.SetCookie(w, &http.Cookie{
		Name:     auth.CookieName,
		Value:    s.auth.Issue(user.ID, time.Now()),
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(s.auth.TTL().Seconds()),
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"username": user.Username,
		"email":    user.Email,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"authenticated": true})
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"currency": s.cfg.Currency})
}

func clientIP(r *http.Request) string {
	if ip := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); ip != "" {
		return ip
	}
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		if ip := strings.TrimSpace(strings.Split(forwarded, ",")[0]); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
