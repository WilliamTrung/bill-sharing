package handlers

import "testing"

func ptr(v int64) *int64 { return &v }

func TestUserLeadsGroup(t *testing.T) {
	tests := []struct {
		name     string
		userID   int64
		leaderID *int64
		want     bool
	}{
		{"leader edits own group", 7, ptr(7), true},
		{"member cannot edit another's group", 7, ptr(9), false},
		{"legacy group without a leader", 7, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := userLeadsGroup(tt.userID, tt.leaderID); got != tt.want {
				t.Fatalf("userLeadsGroup(%d, %v) = %v, want %v", tt.userID, tt.leaderID, got, tt.want)
			}
		})
	}
}

func TestContainsMember(t *testing.T) {
	members := []groupMemberInput{{MemberID: 1}, {MemberID: 4}}

	if !containsMember(members, 4) {
		t.Fatal("expected member 4 to be found")
	}
	if containsMember(members, 2) {
		t.Fatal("member 2 is not in the list")
	}
	if containsMember(nil, 1) {
		t.Fatal("an empty list contains no one")
	}
}
