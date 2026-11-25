package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/WilliamTrung/bill-sharing/internal/accounts"
	"github.com/WilliamTrung/bill-sharing/internal/auth"
)

func readLoginBody(r *http.Request) (identifier, password string, err error) {
	var body struct {
		Identifier string `json:"identifier"`
		Username   string `json:"username"`
		Email      string `json:"email"`
		Password   string `json:"password"`
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := decoder.Decode(&body); err != nil {
		return "", "", fmt.Errorf("invalid JSON body: %w", err)
	}
	if decoder.More() {
		return "", "", errors.New("invalid JSON body: unexpected trailing data")
	}
	identifier = strings.TrimSpace(body.Identifier)
	if identifier == "" {
		identifier = strings.TrimSpace(body.Username)
	}
	if identifier == "" {
		identifier = strings.TrimSpace(body.Email)
	}
	return identifier, body.Password, nil
}

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
	identifier, password, err := readLoginBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if identifier == "" || password == "" {
		writeError(w, http.StatusBadRequest, "Username or email and password are required")
		return
	}

	key := clientIP(r)
	if !s.limiter.allowed(key) {
		writeError(w, http.StatusTooManyRequests, "Too many attempts. Try again later.")
		return
	}

	user, err := accounts.Authenticate(r.Context(), s.db, identifier, password)
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
	userID, ok := s.auth.RequestUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	language, err := accounts.SessionLanguage(r.Context(), s.db, userID)
	if err != nil {
		s.logger.Error("load session language", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not load session")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "language": language})
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
