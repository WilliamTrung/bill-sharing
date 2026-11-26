package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/WilliamTrung/bill-sharing/internal/models"
	"github.com/jackc/pgx/v5"
)

// projectContextKey is the unexported key type that keeps the project context
// from colliding with any other value stored on a request context.
type projectContextKey struct{}

// ProjectContext is the tenancy state resolved for one request. Every scoped
// handler reads it from the request context instead of trusting the client.
type ProjectContext struct {
	ProjectID int64
	UserID    int64
	Role      string
	IsAdmin   bool // global super-admin, allowed to cross boundaries
}

// IsOwner reports whether the caller may rename, remove members or delete.
func (p ProjectContext) IsOwner() bool { return p.Role == "owner" }

// CanManage reports whether the caller may administer the project: its owner, or
// a global super-admin.
func (p ProjectContext) CanManage() bool { return p.IsOwner() || p.IsAdmin }

// withProject stores the resolved context on the request.
func withProject(r *http.Request, pc ProjectContext) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), projectContextKey{}, pc))
}

// projectFrom retrieves the resolved context. The second result is false only if
// a handler was wired without requireProject, which would be a routing bug.
func projectFrom(ctx context.Context) (ProjectContext, bool) {
	pc, ok := ctx.Value(projectContextKey{}).(ProjectContext)
	return pc, ok
}

// requireProject resolves the caller's membership in the project named in the
// path and rejects the request otherwise.
//
// A caller who is not a member gets 404, not 403: 403 would confirm that the
// project exists and let someone enumerate ids. Authentication is checked first
// so an anonymous request still gets 401 rather than a misleading 404.
func (s *Server) requireProject(next http.Handler) http.Handler {
	return s.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := s.auth.RequestUser(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "Authentication required")
			return
		}
		projectID, ok := pathID(w, r, "projectID")
		if !ok {
			return
		}

		pc, err := s.resolveProject(r, projectID, userID)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "Project not found")
			return
		}
		if err != nil {
			s.logger.Error("resolve project", "project_id", projectID, "error", err)
			writeError(w, http.StatusInternalServerError, "Could not load project")
			return
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), projectContextKey{}, pc)))
	}))
}

// requireProjectManager wraps requireProject and additionally demands owner
// rights (or global admin). Used for actions that change who belongs to the
// project or delete it.
func (s *Server) requireProjectManager(next http.Handler) http.Handler {
	return s.requireProject(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pc, _ := projectFrom(r.Context())
		if !pc.CanManage() {
			writeError(w, http.StatusForbidden, "Only the project owner can do this")
			return
		}
		next.ServeHTTP(w, r)
	}))
}

// resolveProject loads the caller's membership row for a project.
func (s *Server) resolveProject(r *http.Request, projectID, userID int64) (ProjectContext, error) {
	pc := ProjectContext{ProjectID: projectID, UserID: userID, IsAdmin: s.requestIsAdmin(r)}

	err := s.db.QueryRow(r.Context(), `
		SELECT pm.role
		FROM project_members pm
		JOIN projects p ON p.id = pm.project_id
		WHERE pm.project_id = $1 AND pm.user_id = $2 AND p.is_active`,
		projectID, userID,
	).Scan(&pc.Role)
	return pc, err
}

// authUserID is the signed-in user for a request that ran through requireAuth.
// The route only reaches a handler after the cookie was validated, so a missing
// user here means the handler was wired without authentication: return 0, which
// matches no row and therefore authorizes nothing.
func (s *Server) authUserID(r *http.Request) int64 {
	userID, _ := s.auth.RequestUser(r)
	return userID
}

// loadProject fills a Project row and the caller's role in it.
func (s *Server) loadProject(ctx context.Context, projectID int64) (models.Project, error) {
	var p models.Project
	err := s.db.QueryRow(ctx, `
		SELECT p.id, p.name, p.owner_user_id, p.is_active, p.public_slug, p.public_enabled,
		       p.created_at, p.updated_at,
		       (SELECT COUNT(*) FROM project_members pm WHERE pm.project_id = p.id)
		FROM projects p WHERE p.id = $1`, projectID,
	).Scan(&p.ID, &p.Name, &p.OwnerUserID, &p.IsActive, &p.PublicSlug, &p.PublicEnabled,
		&p.CreatedAt, &p.UpdatedAt, &p.MemberCount)
	return p, err
}

// listProjectsFor returns every project the user belongs to, oldest first, with
// their role. The public slug is deliberately omitted: it is a share credential
// and only the owner needs it, so listProjectDetail adds it.
func (s *Server) listProjectsFor(ctx context.Context, userID int64) ([]models.Project, error) {
	rows, err := s.db.Query(ctx, `
		SELECT p.id, p.name, p.owner_user_id, pm.role, p.is_active, p.public_enabled,
		       p.created_at, p.updated_at,
		       (SELECT COUNT(*) FROM project_members m WHERE m.project_id = p.id)
		FROM projects p
		JOIN project_members pm ON pm.project_id = p.id
		WHERE pm.user_id = $1 AND p.is_active
		ORDER BY p.id ASC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	projects := make([]models.Project, 0)
	for rows.Next() {
		var p models.Project
		if err := rows.Scan(&p.ID, &p.Name, &p.OwnerUserID, &p.Role, &p.IsActive,
			&p.PublicEnabled, &p.CreatedAt, &p.UpdatedAt, &p.MemberCount); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}
