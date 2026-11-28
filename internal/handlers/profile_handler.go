package handlers

import (
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/WilliamTrung/bill-sharing/internal/accounts"
)

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.auth.RequestUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	user, err := accounts.GetUser(r.Context(), s.db, userID)
	if err != nil {
		s.logger.Error("load profile", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not load profile")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.auth.RequestUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var body struct {
		DisplayName string `json:"display_name"`
		Username    string `json:"username"`
		Email       string `json:"email"`
		Language    string `json:"language"`
		Phone       string `json:"phone"`
		Notes       string `json:"notes"`
		IsPublic    *bool  `json:"is_public"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	displayName := strings.TrimSpace(body.DisplayName)
	username := strings.TrimSpace(body.Username)
	email := strings.TrimSpace(body.Email)
	language := strings.ToLower(strings.TrimSpace(body.Language))
	phone := strings.TrimSpace(body.Phone)
	notes := strings.TrimSpace(body.Notes)

	// Keep the stored value when the client did not send the toggle.
	isPublic := false
	if body.IsPublic != nil {
		isPublic = *body.IsPublic
	} else if current, err := accounts.GetUser(r.Context(), s.db, userID); err == nil {
		isPublic = current.IsPublic
	}

	switch {
	case utf8.RuneCountInString(displayName) < 1:
		writeError(w, http.StatusBadRequest, "Display name is required")
		return
	case utf8.RuneCountInString(displayName) > 120:
		writeError(w, http.StatusBadRequest, "Display name must be 120 characters or fewer")
		return
	case utf8.RuneCountInString(username) < 3:
		writeError(w, http.StatusBadRequest, "Username must be at least 3 characters")
		return
	case utf8.RuneCountInString(username) > 60:
		writeError(w, http.StatusBadRequest, "Username must be 60 characters or fewer")
		return
	case utf8.RuneCountInString(phone) > 40:
		writeError(w, http.StatusBadRequest, "Phone must be 40 characters or fewer")
		return
	case utf8.RuneCountInString(notes) > 2000:
		writeError(w, http.StatusBadRequest, "Notes must be 2000 characters or fewer")
		return
	}
	if strings.ContainsAny(username, " \t\r\n@") {
		writeError(w, http.StatusBadRequest, "Username cannot contain spaces or @")
		return
	}
	if err := validateEmail(email); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if email == "" {
		writeError(w, http.StatusBadRequest, "Email is required")
		return
	}

	if language == "" {
		language = "en"
	}
	if language != "en" && language != "vi" {
		writeError(w, http.StatusBadRequest, "Language must be en or vi")
		return
	}

	user, err := accounts.UpdateProfile(r.Context(), s.db, userID, displayName, username, email, language, phone, notes, isPublic)
	switch {
	case errors.Is(err, accounts.ErrUsernameTaken):
		writeError(w, http.StatusConflict, "Username is already taken")
		return
	case errors.Is(err, accounts.ErrEmailTaken):
		writeError(w, http.StatusConflict, "Email is already in use")
		return
	case err != nil:
		s.logger.Error("update profile", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not update profile")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.auth.RequestUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	switch {
	case body.NewPassword == "":
		writeError(w, http.StatusBadRequest, "New password is required")
		return
	case utf8.RuneCountInString(body.NewPassword) < 8:
		writeError(w, http.StatusBadRequest, "New password must be at least 8 characters")
		return
	case len(body.NewPassword) > 72:
		writeError(w, http.StatusBadRequest, "New password must be 72 bytes or fewer")
		return
	case body.NewPassword == body.CurrentPassword:
		writeError(w, http.StatusBadRequest, "New password must be different from the current one")
		return
	}

	err := accounts.ChangePassword(r.Context(), s.db, userID, body.CurrentPassword, body.NewPassword)
	if errors.Is(err, accounts.ErrInvalidCredentials) {
		writeError(w, http.StatusUnauthorized, "Current password is incorrect")
		return
	}
	if err != nil {
		s.logger.Error("change password", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not change password")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
