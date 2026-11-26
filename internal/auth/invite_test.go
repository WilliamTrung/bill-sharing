package auth

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

func newManager() *Manager {
	return New("test-secret-at-least-16-chars", 24*time.Hour)
}

func TestInviteRoundTrip(t *testing.T) {
	m := newManager()
	now := time.Now()

	token := m.IssueInvite(42, 7, now)
	inv, ok := m.ParseInvite(token, now.Add(time.Hour))
	if !ok || inv.UserID != 42 || inv.ProjectID != 7 {
		t.Fatalf("ParseInvite = (%+v, %v)", inv, ok)
	}
}

// A token minted before projects existed must still work, with no project.
func TestInviteLegacyFormat(t *testing.T) {
	m := newManager()
	now := time.Now()
	payload := "invite:9:" + strconv.FormatInt(now.Unix(), 10)
	legacy := payload + ":" + sign(m.secret, payload)

	inv, ok := m.ParseInvite(legacy, now)
	if !ok || inv.UserID != 9 || inv.ProjectID != 0 {
		t.Fatalf("legacy invite = (%+v, %v)", inv, ok)
	}
}

func TestInviteExpires(t *testing.T) {
	m := newManager()
	now := time.Now()

	token := m.IssueInvite(7, 1, now)
	if _, ok := m.ParseInvite(token, now.Add(InviteTTL+time.Minute)); ok {
		t.Fatal("expired invite accepted")
	}
}

func TestInviteRejectsSessionToken(t *testing.T) {
	m := newManager()
	now := time.Now()

	// A session token must not be usable as an invitation.
	session := m.Issue(5, now)
	if _, ok := m.ParseInvite(session, now); ok {
		t.Fatal("session token accepted as invite")
	}
}

func TestInviteRejectsTampering(t *testing.T) {
	m := newManager()
	now := time.Now()

	token := m.IssueInvite(1, 3, now)
	parts := strings.Split(token, ":")
	parts[1] = "2" // aim at another user, keep the old signature
	if _, ok := m.ParseInvite(strings.Join(parts, ":"), now); ok {
		t.Fatal("tampered invite accepted")
	}

	// Likewise, swapping only the project must invalidate the signature.
	token = m.IssueInvite(1, 3, now)
	parts = strings.Split(token, ":")
	parts[2] = "4"
	if _, ok := m.ParseInvite(strings.Join(parts, ":"), now); ok {
		t.Fatal("tampered project accepted")
	}
}

func TestInviteRejectsForeignSecret(t *testing.T) {
	now := time.Now()
	token := New("another-secret-16-chars-min", time.Hour).IssueInvite(3, 1, now)
	if _, ok := newManager().ParseInvite(token, now); ok {
		t.Fatal("invite signed with a different secret accepted")
	}
}
