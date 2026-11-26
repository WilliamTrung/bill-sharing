package handlers

import (
	"fmt"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/WilliamTrung/bill-sharing/internal/accounts"
	"github.com/WilliamTrung/bill-sharing/internal/auth"
	"github.com/WilliamTrung/bill-sharing/internal/config"
	"github.com/WilliamTrung/bill-sharing/internal/mail"
	"github.com/WilliamTrung/bill-sharing/internal/realtime"
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
	hub     *realtime.Hub
}

func New(cfg config.Config, db *pgxpool.Pool, logger *slog.Logger, webFS fs.FS) *Server {
	s := &Server{
		cfg:  cfg,
		db:   db,
		auth: auth.New(cfg.SessionSecret, cfg.SessionTTL),
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
		hub:     realtime.NewHub(logger),
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
	s.mux.HandleFunc("GET /api/public/invite", s.handlePublicInvite)
	s.mux.HandleFunc("POST /api/public/invite/accept", s.handleAcceptInvite)

	// Public, but scoped to one project by its unguessable share slug. A project
	// that has not enabled sharing resolves to nothing, so these cannot be used
	// to enumerate other tenants.
	s.mux.HandleFunc("GET /api/public/p/{slug}", s.handlePublicProject)
	s.mux.HandleFunc("GET /api/public/p/{slug}/bills", s.handlePublicListBills)
	s.mux.HandleFunc("GET /api/public/p/{slug}/bills/{id}", s.handlePublicGetBill)

	// Attachments need a session and project membership: see handleUploadedFile.
	s.mux.Handle("GET /uploads/{billID}/{filename}", s.requireAuth(http.HandlerFunc(s.handleUploadedFile)))

	// Account-level routes. These are about the person, not a project.
	s.mux.Handle("POST /api/logout", s.requireAuth(http.HandlerFunc(s.handleLogout)))
	s.mux.Handle("GET /api/session", s.requireAuth(http.HandlerFunc(s.handleSession)))
	s.mux.Handle("GET /api/profile", s.requireAuth(http.HandlerFunc(s.handleProfile)))
	s.mux.Handle("PUT /api/profile", s.requireAuth(http.HandlerFunc(s.handleUpdateProfile)))
	s.mux.Handle("PUT /api/profile/password", s.requireAuth(http.HandlerFunc(s.handleChangePassword)))

	// The project collection: which projects exist for me, and creating one.
	s.mux.Handle("GET /api/projects", s.requireAuth(http.HandlerFunc(s.handleListProjects)))
	s.mux.Handle("POST /api/projects", s.requireAuth(http.HandlerFunc(s.handleCreateProject)))

	// Everything below is scoped to a project. requireProject resolves the
	// caller's membership from the path and rejects non-members with 404.
	scoped := func(pattern string, h http.HandlerFunc) {
		s.mux.Handle(pattern, s.requireProject(h))
	}
	managed := func(pattern string, h http.HandlerFunc) {
		s.mux.Handle(pattern, s.requireProjectManager(h))
	}

	scoped("GET /api/p/{projectID}", s.handleGetProject)
	managed("PUT /api/p/{projectID}", s.handleUpdateProject)
	managed("DELETE /api/p/{projectID}", s.handleDeleteProject)

	// Any member may add people; only the owner may remove them.
	scoped("GET /api/p/{projectID}/members", s.handleListProjectMembers)
	scoped("POST /api/p/{projectID}/members", s.handleAddProjectMember)
	managed("DELETE /api/p/{projectID}/members/{userID}", s.handleRemoveProjectMember)
	scoped("POST /api/p/{projectID}/members/{userID}/invite", s.handleSendInvite)

	// Member records (name, phone, email, notes) inside the project.
	scoped("GET /api/p/{projectID}/people", s.handleListMembers)
	scoped("POST /api/p/{projectID}/people", s.handleCreateMember)
	scoped("GET /api/p/{projectID}/people/{id}", s.handleGetMember)
	scoped("PUT /api/p/{projectID}/people/{id}", s.handleUpdateMember)

	scoped("GET /api/p/{projectID}/groups", s.handleListGroups)
	scoped("POST /api/p/{projectID}/groups", s.handleCreateGroup)
	scoped("GET /api/p/{projectID}/groups/{id}", s.handleGetGroup)
	scoped("PUT /api/p/{projectID}/groups/{id}", s.handleUpdateGroup)

	scoped("GET /api/p/{projectID}/bills", s.handleListBills)
	scoped("POST /api/p/{projectID}/bills", s.handleCreateBill)
	scoped("GET /api/p/{projectID}/bills/{id}", s.handleGetBill)
	scoped("PUT /api/p/{projectID}/bills/{id}/status", s.handleUpdateBillStatus)
	scoped("PUT /api/p/{projectID}/bills/{id}/shares/{shareID}/settled", s.handleUpdateShareSettled)
	scoped("DELETE /api/p/{projectID}/bills/{id}", s.handleDeleteBill)

	scoped("GET /api/p/{projectID}/balances", s.handleBalances)
	scoped("GET /api/p/{projectID}/settlements", s.handleSettlements)

	// Realtime event stream, established under an explicit project so a tab knows
	// which project's events it is listening for.
	scoped("GET /api/p/{projectID}/ws", s.handleWebSocket)

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
	case "projects":
		return "projects.html", true
	}
	return name, true
}

func staticPageRequiresAuth(file string) bool {
	if !strings.HasSuffix(file, ".html") {
		return false
	}
	switch file {
	case "index.html", "login.html", "bill-detail.html", "activate.html", "share.html":
		return false
	default:
		return true
	}
}

// requestIsAdmin resolves the caller's account server-side and checks it against
// ADMIN_EMAILS. Authorization is decided here, never by the browser.
func (s *Server) requestIsAdmin(r *http.Request) bool {
	userID, ok := s.auth.RequestUser(r)
	if !ok {
		return false
	}
	user, err := accounts.GetUser(r.Context(), s.db, userID)
	if err != nil {
		s.logger.Error("resolve admin for request", "error", err)
		return false
	}
	return s.cfg.IsAdminEmail(user.Email)
}

// userLeadsGroup is the pure half of the group-edit rule, kept separate so the
// permission check can be tested without a database.
func userLeadsGroup(userID int64, leaderUserID *int64) bool {
	return leaderUserID != nil && *leaderUserID == userID
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if redirect, ok := legacyStaticRedirect(r.URL.Path); ok {
		http.Redirect(w, r, redirect, http.StatusMovedPermanently)
		return
	}

	urlPath := r.URL.Path

	// A share link: /share/<slug> renders the anonymous read-only project view.
	if strings.HasPrefix(urlPath, "/share/") {
		s.servePage(w, r, "share.html")
		return
	}

	// Project-scoped pages: /p/<id>/dashboard.html and friends. The project id is
	// stripped before resolving the file, so one set of HTML files serves every
	// project and the browser learns the id from its own URL.
	projectID, rest, ok := splitProjectPath(urlPath)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if projectID > 0 {
		if !s.pageAllowsProject(w, r, projectID) {
			return
		}
		if rest == "" {
			http.Redirect(w, r, fmt.Sprintf("/p/%d/dashboard.html", projectID), http.StatusFound)
			return
		}
		urlPath = rest
	}

	name, ok := resolveStaticFile(urlPath)
	if !ok {
		http.NotFound(w, r)
		return
	}

	if staticPageRequiresAuth(name) && !s.auth.RequestValid(r) {
		http.Redirect(w, r, "/login.html", http.StatusFound)
		return
	}

	// A signed-in visitor landing on the bare home is sent into their first
	// project; with no project yet, the create-project screen is the destination.
	if name == "index.html" && s.auth.RequestValid(r) {
		http.Redirect(w, r, s.projectsHome(r), http.StatusFound)
		return
	}
	if name == "dashboard.html" && projectID == 0 && s.auth.RequestValid(r) {
		// Legacy /dashboard.html bookmark: move it under the user's project.
		http.Redirect(w, r, s.projectsHome(r), http.StatusFound)
		return
	}

	s.servePage(w, r, name)
}

// splitProjectPath peels a leading /p/<id>/ off a path. It returns projectID 0
// when the path is not project scoped, and ok=false when it looks project scoped
// but the id is not a positive integer.
func splitProjectPath(urlPath string) (int64, string, bool) {
	if !strings.HasPrefix(urlPath, "/p/") {
		return 0, urlPath, true
	}
	remainder := strings.TrimPrefix(urlPath, "/p/")
	slash := strings.Index(remainder, "/")
	idText := remainder
	rest := ""
	if slash >= 0 {
		idText = remainder[:slash]
		rest = remainder[slash+1:]
	}
	id, err := strconv.ParseInt(idText, 10, 64)
	if err != nil || id <= 0 {
		return 0, "", false
	}
	if rest != "" {
		rest = "/" + rest
	}
	return id, rest, true
}

// pageAllowsProject renders a project page only for a member of that project.
// An outsider is sent to their own first project rather than shown a 403, which
// would confirm the project exists.
func (s *Server) pageAllowsProject(w http.ResponseWriter, r *http.Request, projectID int64) bool {
	if !s.auth.RequestValid(r) {
		http.Redirect(w, r, "/login.html", http.StatusFound)
		return false
	}
	userID, _ := s.auth.RequestUser(r)
	var member bool
	err := s.db.QueryRow(r.Context(), `
		SELECT EXISTS(SELECT 1 FROM project_members WHERE project_id = $1 AND user_id = $2)`,
		projectID, userID).Scan(&member)
	if err != nil {
		s.logger.Error("check page membership", "project_id", projectID, "error", err)
		http.Redirect(w, r, s.projectsHome(r), http.StatusFound)
		return false
	}
	if !member {
		http.Redirect(w, r, s.projectsHome(r), http.StatusFound)
		return false
	}
	return true
}

// projectsHome is where a signed-in user belongs: their first project, or the
// project picker when they have none yet.
func (s *Server) projectsHome(r *http.Request) string {
	userID, ok := s.auth.RequestUser(r)
	if !ok {
		return "/login.html"
	}
	projects, err := s.listProjectsFor(r.Context(), userID)
	if err != nil || len(projects) == 0 {
		return "/projects"
	}
	return fmt.Sprintf("/p/%d/dashboard.html", projects[0].ID)
}

func (s *Server) servePage(w http.ResponseWriter, r *http.Request, name string) {
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

// Unwrap exposes the underlying ResponseWriter so http.ResponseController can
// reach the Hijacker a WebSocket upgrade needs. Without it, wrapping the
// connection for logging turns every upgrade into a 501.
func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
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
