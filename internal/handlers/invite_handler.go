package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/WilliamTrung/bill-sharing/internal/accounts"
	"github.com/WilliamTrung/bill-sharing/internal/auth"
	"github.com/jackc/pgx/v5"
)

// handleSendInvite re-issues an invitation email for an account that has not set
// a password yet.
func (s *Server) handleSendInvite(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}

	invitee, err := accounts.Invitee(r.Context(), s.db, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Member not found")
		return
	}
	if errors.Is(err, accounts.ErrInvalidCredentials) {
		writeError(w, http.StatusBadRequest, "This member already has a password")
		return
	}
	if err != nil {
		s.logger.Error("load invitee", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not send invite")
		return
	}
	if !s.mailer.Enabled() {
		writeError(w, http.StatusBadRequest, "Email is not configured")
		return
	}
	if !deliverableEmail(invitee.Email) {
		writeError(w, http.StatusBadRequest, "This member has no email address")
		return
	}

	s.sendInvite(invitee.DisplayName, invitee.Email, invitee.ID)
	writeJSON(w, http.StatusOK, map[string]bool{"sent": true})
}

// handlePublicInvite describes an invitation so the activation page can greet the
// invitee by name before they choose a password.
func (s *Server) handlePublicInvite(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.inviteUser(w, r)
	if !ok {
		return
	}
	invitee, err := accounts.Invitee(r.Context(), s.db, userID)
	if errors.Is(err, accounts.ErrInvalidCredentials) {
		writeError(w, http.StatusGone, "This invitation has already been used")
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Invitation not found")
		return
	}
	if err != nil {
		s.logger.Error("load invite", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not load invitation")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"display_name": invitee.DisplayName,
		"username":     invitee.Username,
		"email":        invitee.Email,
	})
}

// handleAcceptInvite sets the first password and signs the invitee in.
func (s *Server) handleAcceptInvite(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.inviteUser(w, r)
	if !ok {
		return
	}

	var body struct {
		Password string `json:"password"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	switch {
	case utf8.RuneCountInString(body.Password) < 8:
		writeError(w, http.StatusBadRequest, "Password must be at least 8 characters")
		return
	case len(body.Password) > 72:
		writeError(w, http.StatusBadRequest, "Password must be 72 bytes or fewer")
		return
	}

	user, err := accounts.Activate(r.Context(), s.db, userID, body.Password)
	if errors.Is(err, accounts.ErrInvalidCredentials) {
		writeError(w, http.StatusGone, "This invitation has already been used")
		return
	}
	if err != nil {
		s.logger.Error("accept invite", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not activate account")
		return
	}

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
	})
}

// inviteUser resolves and verifies the `token` query parameter.
func (s *Server) inviteUser(w http.ResponseWriter, r *http.Request) (int64, bool) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		writeError(w, http.StatusBadRequest, "Missing invitation token")
		return 0, false
	}
	userID, ok := s.auth.ParseInvite(token, time.Now())
	if !ok {
		writeError(w, http.StatusBadRequest, "This invitation link is invalid or expired")
		return 0, false
	}
	return userID, true
}

// sendInvite mails a fresh activation link without blocking the request path.
func (s *Server) sendInvite(name, email string, userID int64) {
	link := fmt.Sprintf("%s/activate.html?token=%s", s.cfg.PublicBaseURL, s.auth.IssueInvite(userID, time.Now()))
	subject := "Activate your Bill Sharing account"
	body := fmt.Sprintf(
		"Hi %s,\n\nYou were added to Bill Sharing.\n\nSet your password to sign in:\n%s\n\nThis link expires in 7 days.\n",
		name, link,
	)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		done := make(chan error, 1)
		go func() { done <- s.mailer.Send(email, subject, body) }()
		select {
		case err := <-done:
			if err != nil {
				s.logger.Warn("invite email failed", "to", email, "error", err)
			}
		case <-ctx.Done():
			s.logger.Warn("invite email timed out", "to", email)
		}
	}()
}

// deliverableEmail filters out the placeholder addresses generated for members
// created without one.
func deliverableEmail(email string) bool {
	email = strings.TrimSpace(email)
	if email == "" || strings.HasSuffix(email, "@users.invalid") {
		return false
	}
	return strings.Contains(email, "@")
}
