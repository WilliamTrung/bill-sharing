package handlers

import (
	"context"
	"testing"
)

// These tests cover the pure parts of the tenancy rules. Anything that needs a
// database (the actual cross-tenant isolation of every endpoint) is exercised by
// the shell harness that boots the real server against two projects.

func TestSplitProjectPath(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantID    int64
		wantRest  string
		wantValid bool
	}{
		{"plain page is unscoped", "/dashboard.html", 0, "/dashboard.html", true},
		{"root is unscoped", "/", 0, "/", true},
		{"project root", "/p/3", 3, "", true},
		{"project root trailing slash", "/p/3/", 3, "", true},
		{"project page", "/p/12/dashboard.html", 12, "/dashboard.html", true},
		{"project nested page", "/p/5/admin/index.html", 5, "/admin/index.html", true},
		{"project assets stay absolute", "/js/app.js", 0, "/js/app.js", true},
		{"non-numeric id is invalid", "/p/abc/dashboard.html", 0, "", false},
		{"zero id is invalid", "/p/0/dashboard.html", 0, "", false},
		{"negative id is invalid", "/p/-4/dashboard.html", 0, "", false},
		{"empty id is invalid", "/p//dashboard.html", 0, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, rest, valid := splitProjectPath(tt.path)
			if valid != tt.wantValid {
				t.Fatalf("valid = %v, want %v", valid, tt.wantValid)
			}
			if !tt.wantValid {
				return
			}
			if id != tt.wantID || rest != tt.wantRest {
				t.Fatalf("splitProjectPath(%q) = (%d, %q), want (%d, %q)",
					tt.path, id, rest, tt.wantID, tt.wantRest)
			}
		})
	}
}

func TestProjectContextRoles(t *testing.T) {
	tests := []struct {
		name          string
		role          string
		isAdmin       bool
		wantIsOwner   bool
		wantCanManage bool
	}{
		{"member", "member", false, false, false},
		{"owner", "owner", false, true, true},
		// A global admin may manage any project, but does not literally own it.
		{"admin who is a member", "member", true, false, true},
		{"admin who owns", "owner", true, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := ProjectContext{ProjectID: 1, UserID: 2, Role: tt.role, IsAdmin: tt.isAdmin}
			if got := pc.IsOwner(); got != tt.wantIsOwner {
				t.Fatalf("IsOwner = %v, want %v", got, tt.wantIsOwner)
			}
			if got := pc.CanManage(); got != tt.wantCanManage {
				t.Fatalf("CanManage = %v, want %v", got, tt.wantCanManage)
			}
		})
	}
}

// A handler that forgets requireProject must not accidentally authorize: the
// context accessor returns zero values rather than a usable project.
func TestProjectFromMissingContext(t *testing.T) {
	pc, ok := projectFrom(context.Background())
	if ok {
		t.Fatal("projectFrom reported a context that was never set")
	}
	if pc.ProjectID != 0 || pc.UserID != 0 {
		t.Fatalf("missing context yielded %+v", pc)
	}
}
