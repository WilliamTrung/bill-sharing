package auth

import (
	"crypto/hmac"
	"strconv"
	"strings"
	"time"
)

// InviteTTL is how long an invitation link stays usable.
const InviteTTL = 7 * 24 * time.Hour

// IssueInvite returns a signed token granting `userID` one chance to set a
// password. It reuses the session secret, so it cannot be forged client-side.
func (m *Manager) IssueInvite(userID int64, now time.Time) string {
	payload := "invite:" + strconv.FormatInt(userID, 10) + ":" + strconv.FormatInt(now.Unix(), 10)
	return payload + ":" + sign(m.secret, payload)
}

// ParseInvite validates the signature and age of an invitation token.
func (m *Manager) ParseInvite(token string, now time.Time) (int64, bool) {
	parts := strings.Split(token, ":")
	if len(parts) != 4 || parts[0] != "invite" {
		return 0, false
	}
	userID, issued, sig := parts[1], parts[2], parts[3]
	if userID == "" || issued == "" || sig == "" {
		return 0, false
	}
	payload := "invite:" + userID + ":" + issued
	if !hmac.Equal([]byte(sig), []byte(sign(m.secret, payload))) {
		return 0, false
	}

	id, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	unix, err := strconv.ParseInt(issued, 10, 64)
	if err != nil {
		return 0, false
	}
	issuedAt := time.Unix(unix, 0)
	if issuedAt.After(now.Add(5 * time.Minute)) {
		return 0, false
	}
	if now.Sub(issuedAt) > InviteTTL {
		return 0, false
	}
	return id, true
}
