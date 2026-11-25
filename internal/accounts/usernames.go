package accounts

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5/pgxpool"
)

func UsernameBase(name, email string) string {
	email = strings.TrimSpace(email)
	if email != "" {
		if at := strings.Index(email, "@"); at > 0 {
			if base := sanitizeUsername(email[:at]); base != "" {
				return base
			}
		}
	}
	if base := sanitizeUsername(name); base != "" {
		return base
	}
	return "member"
}

func sanitizeUsername(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	out := strings.Trim(b.String(), "._-")
	if len(out) > 40 {
		out = out[:40]
	}
	return out
}

func AllocUsername(ctx context.Context, pool *pgxpool.Pool, base string) (string, error) {
	base = sanitizeUsername(base)
	if base == "" {
		base = "member"
	}
	candidate := base
	for n := 0; n < 200; n++ {
		var exists bool
		if err := pool.QueryRow(ctx, `
			SELECT EXISTS(SELECT 1 FROM users WHERE lower(username) = lower($1))`, candidate,
		).Scan(&exists); err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s-%d", base, n+2)
	}
	return "", fmt.Errorf("could not allocate username for %q", base)
}