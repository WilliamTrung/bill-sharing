package handlers

import (
	"context"
	"errors"
	"fmt"
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

func (s *Server) handleListMembers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := s.db.Query(ctx, `
		SELECT id, `+userLabelSQL+`, phone, email, notes, avatar_color, is_active,
		       (password_hash IS NOT NULL) AS has_password, created_at, updated_at
		FROM users
		ORDER BY is_active DESC, `+userLabelSQL+` ASC`)
	if err != nil {
		s.logger.Error("list members", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not load members")
		return
	}
	defer rows.Close()

	members := make([]models.Member, 0)
	for rows.Next() {
		var m models.Member
		if err := rows.Scan(&m.ID, &m.Name, &m.Phone, &m.Email, &m.Notes, &m.AvatarColor,
			&m.IsActive, &m.HasPassword, &m.CreatedAt, &m.UpdatedAt); err != nil {
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
		JOIN member_groups g ON g.id = gm.group_id
		ORDER BY g.name ASC`)
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

	writeJSON(w, http.StatusOK, members)
}

func (s *Server) handleCreateMember(w http.ResponseWriter, r *http.Request) {
	var in memberInput
	if err := readJSON(w, r, &in); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	name, err := validateMemberFields(in.Name, in.Phone, in.Email, in.Notes)
	if err != nil {
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

	email := strings.TrimSpace(in.Email)
	if email != "" {
		var taken bool
		if err := s.db.QueryRow(r.Context(), `
			SELECT EXISTS(SELECT 1 FROM users WHERE lower(email) = lower($1))`, email,
		).Scan(&taken); err != nil {
			s.logger.Error("check member email", "error", err)
			writeError(w, http.StatusInternalServerError, "Could not create member")
			return
		}
		if taken {
			writeError(w, http.StatusBadRequest, "Email is already in use")
			return
		}
	}

	username, err := accounts.AllocUsername(r.Context(), s.db, accounts.UsernameBase(name, email))
	if err != nil {
		s.logger.Error("alloc member username", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not create member")
		return
	}
	if email == "" {
		email = username + "@users.invalid"
	}

	var m models.Member
	err = s.db.QueryRow(r.Context(), `
		INSERT INTO users (username, email, password_hash, display_name, phone, notes, avatar_color, is_active)
		VALUES ($1, $2, NULL, $3, $4, $5, $6, $7)
		RETURNING id, `+userLabelSQL+`, phone, email, notes, avatar_color, is_active,
		          (password_hash IS NOT NULL) AS has_password, created_at, updated_at`,
		username, email, name, strings.TrimSpace(in.Phone),
		strings.TrimSpace(in.Notes), color, active,
	).Scan(&m.ID, &m.Name, &m.Phone, &m.Email, &m.Notes, &m.AvatarColor,
		&m.IsActive, &m.HasPassword, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		s.logger.Error("create member", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not create member")
		return
	}
	m.Groups = []models.MemberGroupRef{}

	if s.mailer.Enabled() && deliverableEmail(m.Email) {
		s.sendInvite(m.Name, m.Email, m.ID)
	}

	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) handleGetMember(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	member, err := s.loadMember(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Member not found")
		return
	}
	if err != nil {
		s.logger.Error("get member", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not load member")
		return
	}
	writeJSON(w, http.StatusOK, member)
}

func (s *Server) handleUpdateMember(w http.ResponseWriter, r *http.Request) {
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
	existing, err := s.loadMember(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Member not found")
		return
	}
	if err != nil {
		s.logger.Error("load member", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not update member")
		return
	}

	name := existing.Name
	phone := existing.Phone
	email := existing.Email
	notes := existing.Notes
	color := existing.AvatarColor
	active := existing.IsActive

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

	if _, err := validateMemberFields(name, phone, email, notes); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if color == "" {
		color = avatarColor(name)
	}

	sets := make([]string, 0, 7)
	args := make([]any, 0, 7)
	add := func(column string, value any) {
		args = append(args, value)
		sets = append(sets, fmt.Sprintf("%s = $%d", column, len(args)))
	}
	if in.Name != nil {
		add("display_name", name)
	}
	if in.Phone != nil {
		add("phone", phone)
	}
	if in.Email != nil {
		add("email", email)
	}
	if in.Notes != nil {
		add("notes", notes)
	}
	if in.AvatarColor != nil {
		add("avatar_color", color)
	}
	if in.IsActive != nil {
		add("is_active", active)
	}

	if len(sets) > 0 {
		sets = append(sets, "updated_at = now()")
		args = append(args, id)
		query := fmt.Sprintf("UPDATE users SET %s WHERE id = $%d", strings.Join(sets, ", "), len(args))
		if _, err := s.db.Exec(ctx, query, args...); err != nil {
			s.logger.Error("update member", "error", err)
			writeError(w, http.StatusInternalServerError, "Could not update member")
			return
		}
	}

	member, err := s.loadMember(ctx, id)
	if err != nil {
		s.logger.Error("load member", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not load member")
		return
	}
	writeJSON(w, http.StatusOK, member)
}

func (s *Server) loadMember(ctx context.Context, id int64) (models.Member, error) {
	var m models.Member
	err := s.db.QueryRow(ctx, `
		SELECT id, `+userLabelSQL+`, phone, email, notes, avatar_color, is_active,
		       (password_hash IS NOT NULL) AS has_password, created_at, updated_at
		FROM users WHERE id = $1`, id,
	).Scan(&m.ID, &m.Name, &m.Phone, &m.Email, &m.Notes, &m.AvatarColor,
		&m.IsActive, &m.HasPassword, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return models.Member{}, err
	}
	m.Groups = []models.MemberGroupRef{}

	rows, err := s.db.Query(ctx, `
		SELECT g.id, g.name
		FROM group_members gm
		JOIN member_groups g ON g.id = gm.group_id
		WHERE gm.member_id = $1
		ORDER BY g.name`, id)
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

func validateMemberFields(name, phone, email, notes string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("Name is required")
	}
	if len([]rune(name)) > 120 {
		return "", errors.New("Name must be 120 characters or fewer")
	}
	if len([]rune(strings.TrimSpace(phone))) > 40 {
		return "", errors.New("Phone must be 40 characters or fewer")
	}
	if err := validateEmail(strings.TrimSpace(email)); err != nil {
		return "", err
	}
	if len([]rune(strings.TrimSpace(notes))) > 2000 {
		return "", errors.New("Notes must be 2000 characters or fewer")
	}
	return name, nil
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
