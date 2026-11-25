package handlers

import (
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/WilliamTrung/bill-sharing/internal/auth"
	"github.com/WilliamTrung/bill-sharing/internal/config"
	"github.com/WilliamTrung/bill-sharing/internal/mail"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	cfg     config.Config
	db      *pgxpool.Pool
	auth    *auth.Manager
	mailer  mail.Gmail
	logger  *slog.Logger
	webFS   fs.FS
	mux     *http.ServeMux
	limiter *loginLimiter
}

func New(cfg config.Config, db *pgxpool.Pool, logger *slog.Logger, webFS fs.FS) *Server {
	s := &Server{
		cfg:     cfg,
		db:      db,
		auth:    auth.New(cfg.SessionSecret, cfg.SessionTTL),
		mailer: mail.Gmail{
			Host: cfg.SMTPHost,
			Port: cfg.SMTPPort,
			User: cfg.SMTPUser,
			Pass: cfg.SMTPPassword,
			From: cfg.SMTPFrom,
			Name: cfg.SMTPFromName,
		},
		logger:  logger,
		webFS:   webFS,
		mux:     http.NewServeMux(),
		limiter: newLoginLimiter(8, 10*time.Minute),
	}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.securityHeaders(s.logRequests(s.mux))
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealth)

	// Public: readable without a session.
	s.mux.HandleFunc("POST /api/login", s.handleLogin)
	s.mux.HandleFunc("GET /api/config", s.handleConfig)
	s.mux.HandleFunc("GET /api/public/bills", s.handlePublicListBills)
	s.mux.HandleFunc("GET /api/public/bills/{id}", s.handlePublicGetBill)
	s.mux.HandleFunc("GET /uploads/{billID}/{filename}", s.handleUploadedFile)

	// Authenticated: everything that mutates data or exposes member contact info.
	s.mux.Handle("POST /api/logout", s.requireAuth(http.HandlerFunc(s.handleLogout)))
	s.mux.Handle("GET /api/session", s.requireAuth(http.HandlerFunc(s.handleSession)))

	s.mux.Handle("GET /api/profile", s.requireAuth(http.HandlerFunc(s.handleProfile)))
	s.mux.Handle("PUT /api/profile", s.requireAuth(http.HandlerFunc(s.handleUpdateProfile)))
	s.mux.Handle("PUT /api/profile/password", s.requireAuth(http.HandlerFunc(s.handleChangePassword)))

	s.mux.Handle("GET /api/members", s.requireAuth(http.HandlerFunc(s.handleListMembers)))
	s.mux.Handle("POST /api/members", s.requireAuth(http.HandlerFunc(s.handleCreateMember)))
	s.mux.Handle("GET /api/members/{id}", s.requireAuth(http.HandlerFunc(s.handleGetMember)))
	s.mux.Handle("PUT /api/members/{id}", s.requireAuth(http.HandlerFunc(s.handleUpdateMember)))

	s.mux.Handle("GET /api/groups", s.requireAuth(http.HandlerFunc(s.handleListGroups)))
	s.mux.Handle("POST /api/groups", s.requireAuth(http.HandlerFunc(s.handleCreateGroup)))
	s.mux.Handle("GET /api/groups/{id}", s.requireAuth(http.HandlerFunc(s.handleGetGroup)))
	s.mux.Handle("PUT /api/groups/{id}", s.requireAuth(http.HandlerFunc(s.handleUpdateGroup)))

	s.mux.Handle("GET /api/bills", s.requireAuth(http.HandlerFunc(s.handleListBills)))
	s.mux.Handle("POST /api/bills", s.requireAuth(http.HandlerFunc(s.handleCreateBill)))
	s.mux.Handle("GET /api/bills/{id}", s.requireAuth(http.HandlerFunc(s.handleGetBill)))
	s.mux.Handle("PUT /api/bills/{id}/status", s.requireAuth(http.HandlerFunc(s.handleUpdateBillStatus)))
	s.mux.Handle("DELETE /api/bills/{id}", s.requireAuth(http.HandlerFunc(s.handleDeleteBill)))

	s.mux.Handle("GET /api/balances", s.requireAuth(http.HandlerFunc(s.handleBalances)))

	s.mux.HandleFunc("GET /", s.handleStatic)
}

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.auth.RequestValid(r) {
			writeError(w, http.StatusUnauthorized, "Authentication required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func legacyStaticRedirect(urlPath string) (string, bool) {
	p := strings.TrimSuffix(path.Clean("/"+urlPath), "/")
	switch p {
	case "/admin.html":
		return "/admin", true
	default:
		return "", false
	}
}

func resolveStaticFile(urlPath string) (string, bool) {
	name := strings.TrimPrefix(path.Clean("/"+urlPath), "/")
	if name == "" || name == "." {
		return "index.html", true
	}
	if strings.HasSuffix(name, "/") {
		return "", false
	}
	switch name {
	case "admin":
		return "admin/index.html", true
	}
	return name, true
}

func staticPageRequiresAuth(file string) bool {
	if !strings.HasSuffix(file, ".html") {
		return false
	}
	switch file {
	case "index.html", "login.html", "bill-detail.html":
		return false
	default:
		return true
	}
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if redirect, ok := legacyStaticRedirect(r.URL.Path); ok {
		http.Redirect(w, r, redirect, http.StatusMovedPermanently)
		return
	}

	name, ok := resolveStaticFile(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}

	if staticPageRequiresAuth(name) && !s.auth.RequestValid(r) {
		http.Redirect(w, r, "/login.html", http.StatusFound)
		return
	}

	data, err := fs.ReadFile(s.webFS, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	contentType := mime.TypeByExtension(filepath.Ext(name))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if strings.HasPrefix(contentType, "text/") || strings.HasPrefix(contentType, "application/javascript") {
		contentType += "; charset=utf-8"
	}
	w.Header().Set("Content-Type", contentType)
	if strings.HasSuffix(name, ".html") {
		w.Header().Set("Cache-Control", "no-store")
	} else if strings.HasSuffix(name, ".js") || strings.HasSuffix(name, ".css") {
		w.Header().Set("Cache-Control", "no-cache")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=3600")
	}
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(data)
	}
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline'; "+
				"script-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		s.logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.status,
			"duration_ms", time.Since(started).Milliseconds(),
		)
	})
}
