package auth

import (
	"crypto/hmac"
	"strconv"
	"strings"
	"time"
)

// InviteTTL is how long an invitation link stays usable.
const InviteTTL = 7 * 24 * time.Hour

// Invite is the decoded payload of an invitation token. ProjectID is 0 for
// tokens minted before projects existed; those still activate the account but
// do not name a project.
type Invite struct {
	UserID    int64
	ProjectID int64
}

// IssueInvite returns a signed token granting `userID` one chance to set a
// password, optionally naming the project they were invited to. It reuses the
// session secret, so it cannot be forged client-side.
func (m *Manager) IssueInvite(userID, projectID int64, now time.Time) string {
	payload := "invite:" + strconv.FormatInt(userID, 10) + ":" +
		strconv.FormatInt(projectID, 10) + ":" + strconv.FormatInt(now.Unix(), 10)
	return payload + ":" + sign(m.secret, payload)
}

// ParseInvite validates the signature and age of an invitation token.
//
// Two shapes are accepted: the current `invite:user:project:issued:sig` and the
// older `invite:user:issued:sig`, so links already sitting in inboxes keep
// working across the deploy.
func (m *Manager) ParseInvite(token string, now time.Time) (Invite, bool) {
	parts := strings.Split(token, ":")
	if len(parts) < 4 || parts[0] != "invite" {
		return Invite{}, false
	}

	var userID, projectID, issued, sig string
	switch len(parts) {
	case 4: // legacy: invite:user:issued:sig
		userID, issued, sig = parts[1], parts[2], parts[3]
	case 5: // invite:user:project:issued:sig
		userID, projectID, issued, sig = parts[1], parts[2], parts[3], parts[4]
	default:
		return Invite{}, false
	}
	if userID == "" || issued == "" || sig == "" {
		return Invite{}, false
	}

	// Rebuild the exact payload that was signed so the length fixes the format.
	payload := "invite:" + userID + ":" + issued
	if projectID != "" {
		payload = "invite:" + userID + ":" + projectID + ":" + issued
	}
	if !hmac.Equal([]byte(sig), []byte(sign(m.secret, payload))) {
		return Invite{}, false
	}

	id, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || id <= 0 {
		return Invite{}, false
	}
	var project int64
	if projectID != "" {
		project, err = strconv.ParseInt(projectID, 10, 64)
		if err != nil || project < 0 {
			return Invite{}, false
		}
	}
	unix, err := strconv.ParseInt(issued, 10, 64)
	if err != nil {
		return Invite{}, false
	}
	issuedAt := time.Unix(unix, 0)
	if issuedAt.After(now.Add(5 * time.Minute)) {
		return Invite{}, false
	}
	if now.Sub(issuedAt) > InviteTTL {
		return Invite{}, false
	}
	return Invite{UserID: id, ProjectID: project}, true
}
