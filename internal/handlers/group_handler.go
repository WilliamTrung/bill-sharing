package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/WilliamTrung/bill-sharing/internal/models"
	"github.com/WilliamTrung/bill-sharing/internal/realtime"
	"github.com/jackc/pgx/v5"
)

type groupMemberInput struct {
	MemberID      int64   `json:"member_id"`
	InternalRatio float64 `json:"internal_ratio"`
}

type groupInput struct {
	Name              string             `json:"name"`
	InternalSplitType string             `json:"internal_split_type"`
	IsActive          *bool              `json:"is_active"`
	Members           []groupMemberInput `json:"members"`
}

func (s *Server) handleListGroups(w http.ResponseWriter, r *http.Request) {
	pc, _ := projectFrom(r.Context())
	ctx := r.Context()
	rows, err := s.db.Query(ctx, `
		SELECT id, name, internal_split_type, is_active, leader_user_id, created_at, updated_at
		FROM member_groups
		WHERE project_id = $1
		ORDER BY is_active DESC, name ASC`, pc.ProjectID)
	if err != nil {
		s.logger.Error("list groups", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not load groups")
		return
	}
	defer rows.Close()

	groups := make([]models.Group, 0)
	for rows.Next() {
		var g models.Group
		if err := rows.Scan(&g.ID, &g.Name, &g.InternalSplitType, &g.IsActive,
			&g.LeaderUserID, &g.CreatedAt, &g.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "Could not load groups")
			return
		}
		g.Members = []models.GroupMember{}
		groups = append(groups, g)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load groups")
		return
	}

	memberRows, err := s.db.Query(ctx, `
		SELECT gm.group_id, gm.member_id, `+userLabelSQLM+`, gm.internal_ratio
		FROM group_members gm
		JOIN member_groups g ON g.id = gm.group_id AND g.project_id = $1
		JOIN users m ON m.id = gm.member_id
		ORDER BY `+userLabelSQLM+` ASC`, pc.ProjectID)
	if err != nil {
		s.logger.Error("list group members", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not load groups")
		return
	}
	defer memberRows.Close()

	byGroup := make(map[int64][]models.GroupMember)
	for memberRows.Next() {
		var groupID int64
		var gm models.GroupMember
		if err := memberRows.Scan(&groupID, &gm.MemberID, &gm.Name, &gm.InternalRatio); err != nil {
			writeError(w, http.StatusInternalServerError, "Could not load groups")
			return
		}
		byGroup[groupID] = append(byGroup[groupID], gm)
	}
	for i := range groups {
		if members, ok := byGroup[groups[i].ID]; ok {
			groups[i].Members = members
		}
	}

	writeJSON(w, http.StatusOK, groups)
}

func (s *Server) handleCreateGroup(w http.ResponseWriter, r *http.Request) {
	pc, _ := projectFrom(r.Context())

	var in groupInput
	if err := readJSON(w, r, &in); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	name, splitType, err := validateGroupFields(in.Name, in.InternalSplitType)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	members, err := s.validateGroupMembers(r.Context(), pc.ProjectID, in.Members, splitType)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Within a project every member is equal, so a group may be created on
	// anyone's behalf. The creator leads it when they are in it, which is what
	// lets them keep editing it.
	var leaderUserID *int64
	userID := pc.UserID
	if containsMember(members, userID) {
		leaderUserID = &userID
	}

	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}

	ctx := r.Context()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not create group")
		return
	}
	defer tx.Rollback(ctx)

	var groupID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO member_groups (project_id, name, internal_split_type, is_active, leader_user_id)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		pc.ProjectID, name, splitType, active, leaderUserID,
	).Scan(&groupID); err != nil {
		s.logger.Error("create group", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not create group")
		return
	}

	for _, member := range members {
		if _, err := tx.Exec(ctx, `
			INSERT INTO group_members (group_id, member_id, internal_ratio)
			VALUES ($1, $2, $3)`, groupID, member.MemberID, member.InternalRatio); err != nil {
			s.logger.Error("insert group member", "error", err)
			writeError(w, http.StatusInternalServerError, "Could not create group")
			return
		}
	}

	if err := tx.Commit(ctx); err != nil {
		writeError(w, http.StatusInternalServerError, "Could not create group")
		return
	}

	group, err := s.loadGroup(ctx, pc.ProjectID, groupID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load group")
		return
	}

	// Every member should learn they were added and refresh their participant list.
	s.notifyGroupMembers(ctx, pc.ProjectID, group, realtime.TypeGroupCreated)

	writeJSON(w, http.StatusCreated, group)
}

func (s *Server) handleGetGroup(w http.ResponseWriter, r *http.Request) {
	pc, _ := projectFrom(r.Context())
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	group, err := s.loadGroup(r.Context(), pc.ProjectID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Group not found")
		return
	}
	if err != nil {
		s.logger.Error("load group", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not load group")
		return
	}
	writeJSON(w, http.StatusOK, group)
}

func (s *Server) handleUpdateGroup(w http.ResponseWriter, r *http.Request) {
	pc, _ := projectFrom(r.Context())
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}

	var in groupInput
	if err := readJSON(w, r, &in); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx := r.Context()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not update group")
		return
	}
	defer tx.Rollback(ctx)

	var current models.Group
	var leaderUserID *int64
	if err := tx.QueryRow(ctx, `
		SELECT id, name, internal_split_type, is_active, leader_user_id
		FROM member_groups WHERE id = $1 AND project_id = $2 FOR UPDATE`, id, pc.ProjectID,
	).Scan(&current.ID, &current.Name, &current.InternalSplitType, &current.IsActive, &leaderUserID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "Group not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Could not update group")
		return
	}

	// Inside a project any member may reshape a group; the leader field only
	// decides who can edit it when set, and a global admin always can.
	if !pc.IsAdmin && leaderUserID != nil && !userLeadsGroup(pc.UserID, leaderUserID) {
		writeError(w, http.StatusForbidden, "Only the group leader can edit this group")
		return
	}

	name := current.Name
	if strings.TrimSpace(in.Name) != "" {
		name = strings.TrimSpace(in.Name)
	}
	splitType := current.InternalSplitType
	if strings.TrimSpace(in.InternalSplitType) != "" {
		splitType = strings.ToUpper(strings.TrimSpace(in.InternalSplitType))
	}
	if _, _, err := validateGroupFields(name, splitType); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	active := current.IsActive
	if in.IsActive != nil {
		active = *in.IsActive
	}

	if _, err := tx.Exec(ctx, `
		UPDATE member_groups
		SET name = $1, internal_split_type = $2, is_active = $3, updated_at = now()
		WHERE id = $4 AND project_id = $5`, name, splitType, active, id, pc.ProjectID); err != nil {
		s.logger.Error("update group", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not update group")
		return
	}

	if in.Members != nil {
		members, err := s.validateGroupMembers(ctx, pc.ProjectID, in.Members, splitType)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if _, err := tx.Exec(ctx, `DELETE FROM group_members WHERE group_id = $1`, id); err != nil {
			writeError(w, http.StatusInternalServerError, "Could not update group")
			return
		}
		for _, member := range members {
			if _, err := tx.Exec(ctx, `
				INSERT INTO group_members (group_id, member_id, internal_ratio)
				VALUES ($1, $2, $3)`, id, member.MemberID, member.InternalRatio); err != nil {
				s.logger.Error("insert group member", "error", err)
				writeError(w, http.StatusInternalServerError, "Could not update group")
				return
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		writeError(w, http.StatusInternalServerError, "Could not update group")
		return
	}

	group, err := s.loadGroup(ctx, pc.ProjectID, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load group")
		return
	}

	s.notifyGroupMembers(ctx, pc.ProjectID, group, realtime.TypeGroupUpdated)

	writeJSON(w, http.StatusOK, group)
}

func (s *Server) loadGroup(ctx context.Context, projectID, id int64) (models.Group, error) {
	var g models.Group
	if err := s.db.QueryRow(ctx, `
		SELECT id, name, internal_split_type, is_active, leader_user_id, created_at, updated_at
		FROM member_groups WHERE id = $1 AND project_id = $2`, id, projectID,
	).Scan(&g.ID, &g.Name, &g.InternalSplitType, &g.IsActive, &g.LeaderUserID, &g.CreatedAt, &g.UpdatedAt); err != nil {
		return models.Group{}, err
	}

	rows, err := s.db.Query(ctx, `
		SELECT gm.member_id, `+userLabelSQLM+`, gm.internal_ratio
		FROM group_members gm
		JOIN users m ON m.id = gm.member_id
		WHERE gm.group_id = $1
		ORDER BY `+userLabelSQLM+` ASC`, id)
	if err != nil {
		return models.Group{}, err
	}
	defer rows.Close()

	g.Members = []models.GroupMember{}
	for rows.Next() {
		var gm models.GroupMember
		if err := rows.Scan(&gm.MemberID, &gm.Name, &gm.InternalRatio); err != nil {
			return models.Group{}, err
		}
		g.Members = append(g.Members, gm)
	}
	return g, rows.Err()
}

func containsMember(members []groupMemberInput, id int64) bool {
	for _, member := range members {
		if member.MemberID == id {
			return true
		}
	}
	return false
}

func validateGroupFields(name, splitType string) (string, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", "", errors.New("Group name is required")
	}
	if len([]rune(name)) > 120 {
		return "", "", errors.New("Group name must be 120 characters or fewer")
	}
	splitType = strings.ToUpper(strings.TrimSpace(splitType))
	if splitType == "" {
		splitType = "EVEN"
	}
	if splitType != "EVEN" && splitType != "RATIO" {
		return "", "", errors.New("Group split type must be EVEN or RATIO")
	}
	return name, splitType, nil
}

func (s *Server) validateGroupMembers(ctx context.Context, projectID int64, inputs []groupMemberInput, splitType string) ([]groupMemberInput, error) {
	if len(inputs) < 2 {
		return nil, errors.New("A group needs at least two members")
	}
	seen := make(map[int64]bool, len(inputs))
	ids := make([]int64, 0, len(inputs))
	for _, member := range inputs {
		if member.MemberID <= 0 {
			return nil, errors.New("Invalid member in group")
		}
		if seen[member.MemberID] {
			return nil, errors.New("A member can only appear once in a group")
		}
		seen[member.MemberID] = true
		ids = append(ids, member.MemberID)
	}

	// The project join is what stops a caller from pulling another tenant's
	// users into a group; an unknown id falls through to "does not exist".
	rows, err := s.db.Query(ctx, `
		SELECT u.id, `+userLabelSQL+` AS name, u.is_active
		FROM users u
		JOIN project_members pm ON pm.user_id = u.id AND pm.project_id = $2
		WHERE u.id = ANY($1)`, ids, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	found := make(map[int64]bool, len(ids))
	names := make(map[int64]string, len(ids))
	for rows.Next() {
		var id int64
		var name string
		var active bool
		if err := rows.Scan(&id, &name, &active); err != nil {
			return nil, err
		}
		if !active {
			return nil, fmt.Errorf("%s is inactive and cannot be added to a group", name)
		}
		found[id] = true
		names[id] = name
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, id := range ids {
		if !found[id] {
			return nil, fmt.Errorf("member #%d does not exist", id)
		}
	}

	result := make([]groupMemberInput, 0, len(inputs))
	for _, member := range inputs {
		ratio := 1.0
		if splitType == "RATIO" {
			ratio = member.InternalRatio
			if ratio <= 0 || ratio > 1_000_000 {
				return nil, fmt.Errorf("Ratio for %s must be greater than zero", names[member.MemberID])
			}
		}
		result = append(result, groupMemberInput{MemberID: member.MemberID, InternalRatio: ratio})
	}
	return result, nil
}
