package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/WilliamTrung/bill-sharing/internal/accounts"
	"github.com/WilliamTrung/bill-sharing/internal/models"
	"github.com/jackc/pgx/v5"
)

var avatarPalette = []string{
	"#2f6f5f", "#3b6ea5", "#8a5a44", "#6c5ce7",
	"#b5476b", "#2c7a7b", "#a16207", "#4f46e5",
}

type memberInput struct {
	Name        string `json:"name"`
	Phone       string `json:"phone"`
	Email       string `json:"email"`
	Notes       string `json:"notes"`
	AvatarColor string `json:"avatar_color"`
	IsActive    *bool  `json:"is_active"`
}

type memberUpdateInput struct {
	Name        *string `json:"name"`
	Phone       *string `json:"phone"`
	Email       *string `json:"email"`
	Notes       *string `json:"notes"`
	AvatarColor *string `json:"avatar_color"`
	IsActive    *bool   `json:"is_active"`
}

// redactMember hides contact details from anyone who is neither an admin nor the
// member themselves, unless that member chose to publish their profile. The name
// stays visible because it is how people pick participants.
func (s *Server) redactMember(viewerID int64, viewerIsAdmin bool, m *models.Member) {
	if viewerIsAdmin || m.ID == viewerID || m.IsPublic {
		return
	}
	m.Email = ""
	m.Phone = ""
	m.Notes = ""
	m.ProjectPhone = ""
	m.ProjectNotes = ""
	m.ProfilePhone = ""
	m.ProfileNotes = ""
}

func (s *Server) handleListMembers(w http.ResponseWriter, r *http.Request) {
	pc, _ := projectFrom(r.Context())
	ctx := r.Context()
	rows, err := s.db.Query(ctx, `
		SELECT u.id, `+memberLabel("u")+`,
		       COALESCE(NULLIF(pm.phone, ''), u.phone),
		       COALESCE(NULLIF(pm.notes, ''), u.notes),
		       u.email, u.avatar_color, u.is_active,
		       (u.password_hash IS NOT NULL) AS has_password, u.is_public,
		       pm.name, pm.phone, pm.notes,
		       `+userLabel("u")+`, u.phone, u.notes,
		       u.created_at, u.updated_at
		FROM users u
		JOIN project_members pm ON pm.user_id = u.id AND pm.project_id = $1
		ORDER BY u.is_active DESC, `+memberLabel("u")+` ASC`, pc.ProjectID)
	if err != nil {
		s.logger.Error("list members", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not load members")
		return
	}
	defer rows.Close()

	members := make([]models.Member, 0)
	for rows.Next() {
		var m models.Member
		if err := rows.Scan(&m.ID, &m.Name, &m.Phone, &m.Notes, &m.Email, &m.AvatarColor,
			&m.IsActive, &m.HasPassword, &m.IsPublic,
			&m.ProjectName, &m.ProjectPhone, &m.ProjectNotes,
			&m.ProfileName, &m.ProfilePhone, &m.ProfileNotes,
			&m.CreatedAt, &m.UpdatedAt); err != nil {
			s.logger.Error("scan member", "error", err)
			writeError(w, http.StatusInternalServerError, "Could not load members")
			return
		}
		m.Groups = []models.MemberGroupRef{}
		members = append(members, m)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load members")
		return
	}

	groupRows, err := s.db.Query(ctx, `
		SELECT gm.member_id, g.id, g.name
		FROM group_members gm
		JOIN member_groups g ON g.id = gm.group_id AND g.project_id = $1
		ORDER BY g.name ASC`, pc.ProjectID)
	if err != nil {
		s.logger.Error("list member groups", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not load members")
		return
	}
	defer groupRows.Close()

	byMember := make(map[int64][]models.MemberGroupRef)
	for groupRows.Next() {
		var memberID int64
		var ref models.MemberGroupRef
		if err := groupRows.Scan(&memberID, &ref.ID, &ref.Name); err != nil {
			writeError(w, http.StatusInternalServerError, "Could not load members")
			return
		}
		byMember[memberID] = append(byMember[memberID], ref)
	}
	for i := range members {
		if groups, ok := byMember[members[i].ID]; ok {
			members[i].Groups = groups
		}
	}

	viewerID, _ := s.auth.RequestUser(r)
	viewerIsAdmin := s.requestIsAdmin(r)
	for i := range members {
		s.redactMember(viewerID, viewerIsAdmin, &members[i])
	}

	writeJSON(w, http.StatusOK, members)
}

func (s *Server) handleCreateMember(w http.ResponseWriter, r *http.Request) {
	pc, _ := projectFrom(r.Context())

	var in memberInput
	if err := readJSON(w, r, &in); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	name := strings.TrimSpace(in.Name)
	phone := strings.TrimSpace(in.Phone)
	notes := strings.TrimSpace(in.Notes)
	email := strings.TrimSpace(in.Email)
	if err := validateMemberFields(name, phone, email, notes); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}
	color := strings.TrimSpace(in.AvatarColor)
	if color == "" {
		color = avatarColor(name)
	}

	ctx := r.Context()

	// Accounts are global: the same person can belong to several projects, and a
	// real address may already exist from another project. The account (proven by
	// email) is authoritative, so link it instead of creating a duplicate. The
	// name/phone/notes typed here become this project's overrides only.
	if email != "" {
		var existingID int64
		err := s.db.QueryRow(ctx, `
			SELECT id FROM users WHERE lower(email) = lower($1) LIMIT 1`, email).Scan(&existingID)
		switch {
		case err == nil:
			tag, err := s.db.Exec(ctx, `
				INSERT INTO project_members (project_id, user_id, role, name, phone, notes)
				VALUES ($1, $2, 'member', $3, $4, $5)
				ON CONFLICT (project_id, user_id) DO NOTHING`,
				pc.ProjectID, existingID, name, phone, notes)
			if err != nil {
				s.logger.Error("link existing member", "error", err)
				writeError(w, http.StatusInternalServerError, "Could not create member")
				return
			}
			if tag.RowsAffected() == 0 {
				writeError(w, http.StatusConflict, "That person is already in this project")
				return
			}
			m, err := s.loadMember(ctx, pc.ProjectID, existingID)
			if err != nil {
				s.logger.Error("load linked member", "error", err)
				writeError(w, http.StatusInternalServerError, "Could not create member")
				return
			}
			// The account already exists, but this may be the first time this
			// person hears they were added: notify (activation link if they never
			// set a password, otherwise a short "added" note).
			m.InviteSent = s.notifyAddedToProject(ctx, existingID, pc.ProjectID, m.Name, email)
			writeJSON(w, http.StatusCreated, m)
			return
		case !errors.Is(err, pgx.ErrNoRows):
			s.logger.Error("check member email", "error", err)
			writeError(w, http.StatusInternalServerError, "Could not create member")
			return
		}
	}

	username, err := accounts.AllocUsername(ctx, s.db, accounts.UsernameBase(name, email))
	if err != nil {
		s.logger.Error("alloc member username", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not create member")
		return
	}
	if email == "" {
		email = username + "@users.invalid"
	}
	// Give the new account a global label so invites and the session have a real
	// name. The per-project fields below still start exactly as typed.
	displayName := name
	if displayName == "" {
		displayName = username
	}

	var id int64
	err = s.db.QueryRow(ctx, `
		INSERT INTO users (username, email, password_hash, display_name, avatar_color, is_active)
		VALUES ($1, $2, NULL, $3, $4, $5)
		RETURNING id`,
		username, email, displayName, color, active,
	).Scan(&id)
	if err != nil {
		s.logger.Error("create member", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not create member")
		return
	}

	// The new account only exists for this project, so join it there with the
	// project-level values (empty means inherit the global profile).
	if _, err := s.db.Exec(ctx, `
		INSERT INTO project_members (project_id, user_id, role, name, phone, notes)
		VALUES ($1, $2, 'member', $3, $4, $5)
		ON CONFLICT (project_id, user_id) DO NOTHING`,
		pc.ProjectID, id, name, phone, notes); err != nil {
		s.logger.Error("create member membership", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not create member")
		return
	}

	// Only invite when the admin actually supplied a real address; members added
	// without one get a placeholder and simply can never sign in.
	var inviteSent bool
	if deliverableEmail(in.Email) {
		inviteSent = s.notifyAddedToProject(ctx, id, pc.ProjectID, displayName, email)
	}

	m, err := s.loadMember(ctx, pc.ProjectID, id)
	if err != nil {
		s.logger.Error("load created member", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not create member")
		return
	}
	m.InviteSent = inviteSent
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) handleGetMember(w http.ResponseWriter, r *http.Request) {
	pc, _ := projectFrom(r.Context())
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	member, err := s.loadMember(r.Context(), pc.ProjectID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Member not found")
		return
	}
	if err != nil {
		s.logger.Error("get member", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not load member")
		return
	}
	viewerID, _ := s.auth.RequestUser(r)
	s.redactMember(viewerID, s.requestIsAdmin(r), &member)
	writeJSON(w, http.StatusOK, member)
}

func (s *Server) handleUpdateMember(w http.ResponseWriter, r *http.Request) {
	pc, _ := projectFrom(r.Context())
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}

	var in memberUpdateInput
	if err := readJSON(w, r, &in); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx := r.Context()

	// Load the raw per-project overrides plus the global account fields, so an
	// omitted field keeps its current value and an explicit empty string clears
	// the override (inheriting the profile).
	var rawName, rawPhone, rawEmail, rawNotes, rawColor string
	var rawActive bool
	err := s.db.QueryRow(ctx, `
		SELECT pm.name, pm.phone, u.email, pm.notes, u.avatar_color, u.is_active
		FROM project_members pm
		JOIN users u ON u.id = pm.user_id
		WHERE pm.project_id = $1 AND pm.user_id = $2`, pc.ProjectID, id,
	).Scan(&rawName, &rawPhone, &rawEmail, &rawNotes, &rawColor, &rawActive)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Member not found")
		return
	}
	if err != nil {
		s.logger.Error("load member", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not update member")
		return
	}

	name := rawName
	phone := rawPhone
	email := rawEmail
	notes := rawNotes
	color := rawColor
	active := rawActive

	if in.Name != nil {
		name = strings.TrimSpace(*in.Name)
	}
	if in.Phone != nil {
		phone = strings.TrimSpace(*in.Phone)
	}
	if in.Email != nil {
		email = strings.TrimSpace(*in.Email)
	}
	if in.Notes != nil {
		notes = strings.TrimSpace(*in.Notes)
	}
	if in.AvatarColor != nil {
		color = strings.TrimSpace(*in.AvatarColor)
	}
	if in.IsActive != nil {
		active = *in.IsActive
	}

	if err := validateMemberFields(name, phone, email, notes); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if color == "" {
		color = avatarColor(name)
	}

	// Email is global (shared across projects), so changing it must not collide
	// with another account.
	if !strings.EqualFold(email, rawEmail) {
		var taken bool
		if err := s.db.QueryRow(ctx, `
			SELECT EXISTS(SELECT 1 FROM users WHERE lower(email) = lower($1) AND id <> $2)`,
			email, id).Scan(&taken); err != nil {
			s.logger.Error("check member email", "error", err)
			writeError(w, http.StatusInternalServerError, "Could not update member")
			return
		}
		if taken {
			writeError(w, http.StatusConflict, "Email is already in use")
			return
		}
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		s.logger.Error("begin update member", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not update member")
		return
	}
	defer tx.Rollback(ctx)

	// Name/phone/notes are project-scoped; email/avatar/active are account-wide.
	if _, err := tx.Exec(ctx, `
		UPDATE project_members SET name = $1, phone = $2, notes = $3
		WHERE project_id = $4 AND user_id = $5`,
		name, phone, notes, pc.ProjectID, id); err != nil {
		s.logger.Error("update member overrides", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not update member")
		return
	}
	if _, err := tx.Exec(ctx, `
		UPDATE users SET email = $1, avatar_color = $2, is_active = $3, updated_at = now()
		WHERE id = $4`,
		email, color, active, id); err != nil {
		s.logger.Error("update member account", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not update member")
		return
	}
	if err := tx.Commit(ctx); err != nil {
		s.logger.Error("commit update member", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not update member")
		return
	}

	member, err := s.loadMember(ctx, pc.ProjectID, id)
	if err != nil {
		s.logger.Error("load member", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not load member")
		return
	}
	writeJSON(w, http.StatusOK, member)
}

func (s *Server) loadMember(ctx context.Context, projectID, id int64) (models.Member, error) {
	var m models.Member
	err := s.db.QueryRow(ctx, `
		SELECT u.id, `+memberLabel("u")+`,
		       COALESCE(NULLIF(pm.phone, ''), u.phone),
		       COALESCE(NULLIF(pm.notes, ''), u.notes),
		       u.email, u.avatar_color, u.is_active,
		       (u.password_hash IS NOT NULL) AS has_password, u.is_public,
		       pm.name, pm.phone, pm.notes,
		       `+userLabel("u")+`, u.phone, u.notes,
		       u.created_at, u.updated_at
		FROM users u
		JOIN project_members pm ON pm.user_id = u.id AND pm.project_id = $2
		WHERE u.id = $1`, id, projectID,
	).Scan(&m.ID, &m.Name, &m.Phone, &m.Notes, &m.Email, &m.AvatarColor,
		&m.IsActive, &m.HasPassword, &m.IsPublic,
		&m.ProjectName, &m.ProjectPhone, &m.ProjectNotes,
		&m.ProfileName, &m.ProfilePhone, &m.ProfileNotes,
		&m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return models.Member{}, err
	}
	m.Groups = []models.MemberGroupRef{}

	rows, err := s.db.Query(ctx, `
		SELECT g.id, g.name
		FROM group_members gm
		JOIN member_groups g ON g.id = gm.group_id AND g.project_id = $2
		WHERE gm.member_id = $1
		ORDER BY g.name`, id, projectID)
	if err != nil {
		return models.Member{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var ref models.MemberGroupRef
		if err := rows.Scan(&ref.ID, &ref.Name); err != nil {
			return models.Member{}, err
		}
		m.Groups = append(m.Groups, ref)
	}
	return m, rows.Err()
}

func validateMemberFields(name, phone, email, notes string) error {
	if len([]rune(strings.TrimSpace(name))) > 120 {
		return errors.New("Name must be 120 characters or fewer")
	}
	if len([]rune(strings.TrimSpace(phone))) > 40 {
		return errors.New("Phone must be 40 characters or fewer")
	}
	if err := validateEmail(strings.TrimSpace(email)); err != nil {
		return err
	}
	if len([]rune(strings.TrimSpace(notes))) > 2000 {
		return errors.New("Notes must be 2000 characters or fewer")
	}
	return nil
}

func validateEmail(email string) error {
	if email == "" {
		return nil
	}
	if len([]rune(email)) > 200 {
		return errors.New("Email must be 200 characters or fewer")
	}
	at := strings.LastIndex(email, "@")
	if at <= 0 || at == len(email)-1 || !strings.Contains(email[at+1:], ".") {
		return errors.New("Email address looks invalid")
	}
	return nil
}

func avatarColor(name string) string {
	var hash uint32
	for _, r := range name {
		hash = hash*31 + uint32(r)
	}
	return avatarPalette[int(hash)%len(avatarPalette)]
}
