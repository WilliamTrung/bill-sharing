package handlers

import "fmt"

// userLabelSQL is the display label for an unaliased users row, for contexts
// without a project (session, notifications).
const userLabelSQL = `COALESCE(NULLIF(display_name, ''), username)`

// userLabel is the display label for a users row with the given alias, for
// contexts without a project.
func userLabel(alias string) string {
	return fmt.Sprintf(`COALESCE(NULLIF(%[1]s.display_name, ''), %[1]s.username)`, alias)
}

// memberLabel is the project-aware display label for a users row aliased
// userAlias. It requires project_members to be joined as "pm": the project
// nickname wins, then the global display name, then the username.
func memberLabel(userAlias string) string {
	return fmt.Sprintf(
		`COALESCE(NULLIF(pm.name, ''), NULLIF(%[1]s.display_name, ''), %[1]s.username)`,
		userAlias,
	)
}
