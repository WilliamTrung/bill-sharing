package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/WilliamTrung/bill-sharing/internal/accounts"
	"github.com/WilliamTrung/bill-sharing/internal/models"
	"github.com/WilliamTrung/bill-sharing/internal/realtime"
	"github.com/jackc/pgx/v5"
)

// newPublicSlug mints the unguessable token behind a project's share link.
func newPublicSlug() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

type projectInput struct {
	Name string `json:"name"`
}

type projectUpdateInput struct {
	Name          *string `json:"name"`
	PublicEnabled *bool   `json:"public_enabled"`
}

type addMemberInput struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

// handleListProjects returns every project the caller belongs to. This is the
// only project-scoped read that is itself unscoped, because the left pane needs
// it before a project is selected.
func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.auth.RequestUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	projects, err := s.listProjectsFor(r.Context(), userID)
	if err != nil {
		s.logger.Error("list projects", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not load projects")
		return
	}
	writeJSON(w, http.StatusOK, projects)
}

// handleCreateProject makes the caller the owner and sole member.
func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.auth.RequestUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var in projectInput
	if err := readJSON(w, r, &in); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	name, err := validateProjectName(in.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	slug, err := newPublicSlug()
	if err != nil {
		s.logger.Error("generate project slug", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not create project")
		return
	}

	ctx := r.Context()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		s.logger.Error("begin project", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not create project")
		return
	}
	defer tx.Rollback(ctx)

	var projectID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO projects (name, public_slug, owner_user_id)
		VALUES ($1, $2, $3) RETURNING id`, name, slug, userID,
	).Scan(&projectID)
	if err != nil {
		// The unique (owner, lower(name)) index is the only expected conflict.
		s.logger.Error("create project", "error", err)
		writeError(w, http.StatusConflict, "You already have a project with that name")
		return
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO project_members (project_id, user_id, role) VALUES ($1, $2, 'owner')`,
		projectID, userID); err != nil {
		s.logger.Error("create project membership", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not create project")
		return
	}
	if err := tx.Commit(ctx); err != nil {
		s.logger.Error("commit project", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not create project")
		return
	}

	project, err := s.loadProject(ctx, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load project")
		return
	}
	project.Role = "owner"
	writeJSON(w, http.StatusCreated, project)
}

// handleGetProject is the scoped overview: the project, the caller's role and
// the share link for an owner.
func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	pc, _ := projectFrom(r.Context())
	project, err := s.loadProject(r.Context(), pc.ProjectID)
	if err != nil {
		s.logger.Error("get project", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not load project")
		return
	}
	project.Role = pc.Role
	// The slug is the share credential; only someone who can manage the project
	// needs it.
	if !pc.CanManage() {
		project.PublicSlug = ""
	}
	writeJSON(w, http.StatusOK, project)
}

func (s *Server) handleUpdateProject(w http.ResponseWriter, r *http.Request) {
	pc, _ := projectFrom(r.Context())

	var in projectUpdateInput
	if err := readJSON(w, r, &in); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	sets := make([]string, 0, 3)
	args := make([]any, 0, 3)
	if in.Name != nil {
		name, err := validateProjectName(*in.Name)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		args = append(args, name)
		sets = append(sets, fmt.Sprintf("name = $%d", len(args)))
	}
	if in.PublicEnabled != nil {
		args = append(args, *in.PublicEnabled)
		sets = append(sets, fmt.Sprintf("public_enabled = $%d", len(args)))
	}
	if len(sets) > 0 {
		sets = append(sets, "updated_at = now()")
		args = append(args, pc.ProjectID)
		query := fmt.Sprintf("UPDATE projects SET %s WHERE id = $%d", strings.Join(sets, ", "), len(args))
		if _, err := s.db.Exec(r.Context(), query, args...); err != nil {
			s.logger.Error("update project", "error", err)
			writeError(w, http.StatusConflict, "Could not update project")
			return
		}
	}

	project, err := s.loadProject(r.Context(), pc.ProjectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load project")
		return
	}
	project.Role = pc.Role
	writeJSON(w, http.StatusOK, project)
}

// handleDeleteProject removes a project and everything in it. Bills, groups and
// memberships cascade; uploads are removed from disk explicitly.
func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	pc, _ := projectFrom(r.Context())

	ctx := r.Context()
	// Collect bill ids first: after the delete the cascade is gone, and the
	// upload directories are keyed by bill id.
	rows, err := s.db.Query(ctx, `SELECT id FROM bills WHERE project_id = $1`, pc.ProjectID)
	if err != nil {
		s.logger.Error("list project bills", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not delete project")
		return
	}
	billIDs := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			writeError(w, http.StatusInternalServerError, "Could not delete project")
			return
		}
		billIDs = append(billIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "Could not delete project")
		return
	}

	if _, err := s.db.Exec(ctx, `DELETE FROM projects WHERE id = $1`, pc.ProjectID); err != nil {
		s.logger.Error("delete project", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not delete project")
		return
	}
	for _, id := range billIDs {
		s.removeBillUploads(id)
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleListProjectMembers returns the roster with roles.
func (s *Server) handleListProjectMembers(w http.ResponseWriter, r *http.Request) {
	pc, _ := projectFrom(r.Context())
	members, err := s.loadProjectMembers(r.Context(), pc.ProjectID)
	if err != nil {
		s.logger.Error("list project members", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not load members")
		return
	}
	writeJSON(w, http.StatusOK, members)
}

// loadProjectMembers is the roster query used by the membership screen.
func (s *Server) loadProjectMembers(ctx context.Context, projectID int64) ([]models.ProjectMember, error) {
	rows, err := s.db.Query(ctx, `
		SELECT u.id, `+memberLabel("u")+`, u.email, pm.role, u.is_active,
		       (u.password_hash IS NOT NULL) AS has_password
		FROM project_members pm
		JOIN users u ON u.id = pm.user_id
		WHERE pm.project_id = $1
		ORDER BY (pm.role = 'owner') DESC, u.is_active DESC, `+memberLabel("u")+` ASC`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list project members: %w", err)
	}
	defer rows.Close()

	members := make([]models.ProjectMember, 0)
	for rows.Next() {
		var m models.ProjectMember
		if err := rows.Scan(&m.UserID, &m.Name, &m.Email, &m.Role, &m.IsActive, &m.HasPassword); err != nil {
			return nil, fmt.Errorf("scan project member: %w", err)
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

// handleAddProjectMember adds someone by email, creating the account if that
// address is unknown. Any member may do this; only the name and address are
// taken from the body, never an id, so an existing account cannot be hijacked.
func (s *Server) handleAddProjectMember(w http.ResponseWriter, r *http.Request) {
	pc, _ := projectFrom(r.Context())

	var in addMemberInput
	if err := readJSON(w, r, &in); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	email := strings.ToLower(strings.TrimSpace(in.Email))
	if err := validateEmail(email); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if email == "" {
		writeError(w, http.StatusBadRequest, "Email is required")
		return
	}
	// A placeholder address can never receive mail, so it cannot be an invite.
	if strings.HasSuffix(email, "@users.invalid") {
		writeError(w, http.StatusBadRequest, "Enter a real email address")
		return
	}

	// The name in the body is this project's nickname. It may be omitted, in
	// which case the membership inherits the account's global display name; a
	// brand-new account still gets a display name so invites/session have a label.
	projectName := strings.TrimSpace(in.Name)
	bootstrapName := projectName
	if bootstrapName == "" {
		if at := strings.Index(email, "@"); at > 0 {
			bootstrapName = email[:at]
		}
	}
	if len([]rune(projectName)) > 120 {
		writeError(w, http.StatusBadRequest, "Name must be 120 characters or fewer")
		return
	}

	ctx := r.Context()
	userID, created, err := s.findOrCreateUser(ctx, bootstrapName, email)
	if err != nil {
		s.logger.Error("find or create user", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not add member")
		return
	}

	// Ensure membership. A conflict means they were already in the project.
	tag, err := s.db.Exec(ctx, `
		INSERT INTO project_members (project_id, user_id, role, name) VALUES ($1, $2, 'member', $3)
		ON CONFLICT (project_id, user_id) DO NOTHING`, pc.ProjectID, userID, projectName)
	if err != nil {
		s.logger.Error("add project member", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not add member")
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusConflict, "That person is already in this project")
		return
	}

	project, _ := s.loadProject(ctx, pc.ProjectID)
	// Tell everyone already in the project, plus the new member's other tabs.
	members, err := s.loadProjectMembers(ctx, pc.ProjectID)
	if err == nil {
		ids := make([]int64, 0, len(members))
		for _, m := range members {
			ids = append(ids, m.UserID)
		}
		s.emitProject(ids, realtime.TypeProjectMemberAdded, project.ID, project.Name)
	}

	// Tell the new member by email. A brand-new or not-yet-activated account gets
	// an activation link; an existing active account gets a short "added" note.
	displayName := projectName
	if displayName == "" {
		displayName = bootstrapName
	}
	invited := s.notifyAddedToProject(ctx, userID, pc.ProjectID, displayName, email)

	writeJSON(w, http.StatusCreated, map[string]any{
		"user_id": userID,
		"created": created,
		"invited": invited,
		"name":    displayName,
		"email":   email,
		"role":    "member",
		"project": project.Name,
	})
}

// findOrCreateUser resolves an account by email, creating an unactivated one if
// the address is unknown. The second result reports whether it was created.
func (s *Server) findOrCreateUser(ctx context.Context, name, email string) (int64, bool, error) {
	var id int64
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE lower(email) = lower($1) LIMIT 1`, email).Scan(&id)
	if err == nil {
		return id, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, false, fmt.Errorf("lookup user: %w", err)
	}

	username, err := accounts.AllocUsername(ctx, s.db, accounts.UsernameBase(name, email))
	if err != nil {
		return 0, false, fmt.Errorf("alloc username: %w", err)
	}
	color := avatarColor(name)
	err = s.db.QueryRow(ctx, `
		INSERT INTO users (username, email, password_hash, display_name, avatar_color, is_active)
		VALUES ($1, $2, NULL, $3, $4, TRUE) RETURNING id`,
		username, email, name, color,
	).Scan(&id)
	if err != nil {
		return 0, false, fmt.Errorf("create user: %w", err)
	}
	return id, true, nil
}

// handleRemoveProjectMember drops someone from the project. Owner only, and the
// owner can never remove themselves: ownership would be left dangling.
func (s *Server) handleRemoveProjectMember(w http.ResponseWriter, r *http.Request) {
	pc, _ := projectFrom(r.Context())
	targetID, ok := pathID(w, r, "userID")
	if !ok {
		return
	}

	if targetID == pc.UserID {
		writeError(w, http.StatusBadRequest, "You cannot remove yourself from the project")
		return
	}

	var role string
	err := s.db.QueryRow(r.Context(), `
		SELECT role FROM project_members WHERE project_id = $1 AND user_id = $2`,
		pc.ProjectID, targetID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "That person is not in this project")
		return
	}
	if err != nil {
		s.logger.Error("load membership", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not remove member")
		return
	}
	if role == "owner" {
		writeError(w, http.StatusBadRequest, "The project owner cannot be removed")
		return
	}

	if _, err := s.db.Exec(r.Context(), `
		DELETE FROM project_members WHERE project_id = $1 AND user_id = $2`,
		pc.ProjectID, targetID); err != nil {
		s.logger.Error("remove member", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not remove member")
		return
	}

	project, _ := s.loadProject(r.Context(), pc.ProjectID)
	s.emitProject([]int64{targetID}, realtime.TypeProjectMemberRemoved, project.ID, project.Name)
	w.WriteHeader(http.StatusNoContent)
}

func validateProjectName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("Project name is required")
	}
	if len([]rune(name)) > 120 {
		return "", errors.New("Project name must be 120 characters or fewer")
	}
	return name, nil
}
