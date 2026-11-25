package auth

import (
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

	token := m.IssueInvite(42, now)
	id, ok := m.ParseInvite(token, now.Add(time.Hour))
	if !ok || id != 42 {
		t.Fatalf("ParseInvite = (%d, %v)", id, ok)
	}
}

func TestInviteExpires(t *testing.T) {
	m := newManager()
	now := time.Now()

	token := m.IssueInvite(7, now)
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

	token := m.IssueInvite(1, now)
	parts := strings.Split(token, ":")
	parts[1] = "2" // aim at another user, keep the old signature
	if _, ok := m.ParseInvite(strings.Join(parts, ":"), now); ok {
		t.Fatal("tampered invite accepted")
	}
}

func TestInviteRejectsForeignSecret(t *testing.T) {
	now := time.Now()
	token := New("another-secret-16-chars-min", time.Hour).IssueInvite(3, now)
	if _, ok := newManager().ParseInvite(token, now); ok {
		t.Fatal("invite signed with a different secret accepted")
	}
}
