package handlers

// SQL display label for a user row (optional table alias).
const (
	userLabelSQL  = `COALESCE(NULLIF(display_name, ''), username)`
	userLabelSQLM = `COALESCE(NULLIF(m.display_name, ''), m.username)`
)