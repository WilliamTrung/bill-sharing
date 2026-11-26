package accounts

import (
	"strings"
	"testing"

	"github.com/WilliamTrung/bill-sharing/internal/models"
)

// countColumns counts the comma-separated SELECT list in userColumns. Parentheses
// are tracked so the `(password_hash IS NOT NULL) AS has_password` entry is one
// column, not two.
func countColumns(list string) int {
	flat := strings.Join(strings.Fields(list), " ")
	depth := 0
	count := 0
	for _, r := range flat {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				count++
			}
		}
	}
	if strings.TrimSpace(flat) != "" {
		count++
	}
	return count
}

// TestUserColumnsMatchScanDestinations guards the bug that broke account
// activation: userColumns grew to 13 fields while Activate scanned into 12,
// so the UPDATE committed the password and then failed to read the row back,
// returning a 500 and losing the invitee's session.
func TestUserColumnsMatchScanDestinations(t *testing.T) {
	var user models.User
	columns := countColumns(userColumns)
	dest := len(userDest(&user))
	if columns != dest {
		t.Fatalf("userColumns selects %d columns but userDest returns %d destinations", columns, dest)
	}
}

func TestCountColumns(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"id", 1},
		{"id, username", 2},
		{"(password_hash IS NOT NULL) AS has_password", 1},
		{"id, (a IS NOT NULL) AS b, c", 3},
		{"", 0},
	}
	for _, tc := range cases {
		if got := countColumns(tc.in); got != tc.want {
			t.Errorf("countColumns(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}
