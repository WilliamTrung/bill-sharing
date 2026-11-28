package accounts

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/WilliamTrung/bill-sharing/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUsernameTaken      = errors.New("username is already taken")
	ErrEmailTaken         = errors.New("email is already in use")
)

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func verifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// EnsureSeed creates the seed account once. If a row with the seed username or
// email already exists it is left untouched, so a password changed in the UI is
// never reverted on the next startup.
func EnsureSeed(ctx context.Context, pool *pgxpool.Pool, username, email, password string) (string, error) {
	username = strings.TrimSpace(username)
	email = strings.TrimSpace(email)

	var exists bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM users
			WHERE lower(username) = lower($1) OR lower(email) = lower($2)
		)`, username, email).Scan(&exists)
	if err != nil {
		return "", fmt.Errorf("look up seed user: %w", err)
	}
	if exists {
		return "exists", nil
	}

	hash, err := HashPassword(password)
	if err != nil {
		return "", fmt.Errorf("hash seed password: %w", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO users (username, email, password_hash, display_name) VALUES ($1, $2, $3, $1)`,
		username, email, hash); err != nil {
		return "", fmt.Errorf("create seed user: %w", err)
	}
	return "created", nil
}

const userColumns = `id, username, email, display_name, phone, notes, avatar_color, is_active,
	(password_hash IS NOT NULL) AS has_password, is_public, language, created_at, updated_at`

// userDest returns the scan destinations for userColumns. Every query that
// selects userColumns must scan through this, so adding a column can never
// leave one call site with a mismatched destination list.
func userDest(u *models.User) []any {
	return []any{
		&u.ID, &u.Username, &u.Email, &u.DisplayName, &u.Phone, &u.Notes,
		&u.AvatarColor, &u.IsActive, &u.HasPassword, &u.IsPublic, &u.Language,
		&u.CreatedAt, &u.UpdatedAt,
	}
}

func scanUser(row pgx.Row) (models.User, error) {
	var user models.User
	err := row.Scan(userDest(&user)...)
	return user, err
}

func GetUser(ctx context.Context, pool *pgxpool.Pool, userID int64) (models.User, error) {
	return scanUser(pool.QueryRow(ctx, `
		SELECT `+userColumns+` FROM users WHERE id = $1`, userID))
}

func UpdateProfile(ctx context.Context, pool *pgxpool.Pool, userID int64, displayName, username, email, language, phone, notes string, isPublic bool) (models.User, error) {
	displayName = strings.TrimSpace(displayName)
	username = strings.TrimSpace(username)
	email = strings.TrimSpace(email)
	phone = strings.TrimSpace(phone)
	notes = strings.TrimSpace(notes)

	tx, err := pool.Begin(ctx)
	if err != nil {
		return models.User{}, err
	}
	defer tx.Rollback(ctx)

	var conflictUsername, conflictEmail bool
	err = tx.QueryRow(ctx, `
		SELECT
			EXISTS (SELECT 1 FROM users WHERE lower(username) = lower($1) AND id <> $3),
			EXISTS (SELECT 1 FROM users WHERE lower(email) = lower($2) AND id <> $3)`,
		username, email, userID,
	).Scan(&conflictUsername, &conflictEmail)
	if err != nil {
		return models.User{}, fmt.Errorf("check profile conflicts: %w", err)
	}
	if conflictUsername {
		return models.User{}, ErrUsernameTaken
	}
	if conflictEmail {
		return models.User{}, ErrEmailTaken
	}

	user, err := scanUser(tx.QueryRow(ctx, `
		UPDATE users SET display_name = $1, username = $2, email = $3, language = $4,
		                 is_public = $5, phone = $6, notes = $7, updated_at = now()
		WHERE id = $8
		RETURNING `+userColumns,
		displayName, username, email, language, isPublic, phone, notes, userID,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return models.User{}, err
	}
	if err != nil {
		return models.User{}, fmt.Errorf("update profile: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return models.User{}, err
	}
	return user, nil
}

func ChangePassword(ctx context.Context, pool *pgxpool.Pool, userID int64, current, next string) error {
	var hash string
	err := pool.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1`, userID).Scan(&hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidCredentials
	}
	if err != nil {
		return fmt.Errorf("load password hash: %w", err)
	}
	if hash == "" || !verifyPassword(hash, current) {
		return ErrInvalidCredentials
	}

	newHash, err := HashPassword(next)
	if err != nil {
		return fmt.Errorf("hash new password: %w", err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE users SET password_hash = $1, updated_at = now() WHERE id = $2`, newHash, userID); err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	return nil
}

// Invitee returns the account behind an invitation token, but only while it has
// no password yet — an accepted or already-active account cannot be re-claimed.
func Invitee(ctx context.Context, pool *pgxpool.Pool, userID int64) (models.User, error) {
	user, err := scanUser(pool.QueryRow(ctx, `
		SELECT `+userColumns+` FROM users WHERE id = $1`, userID))
	if err != nil {
		return models.User{}, err
	}
	if user.HasPassword || !user.IsActive {
		return models.User{}, ErrInvalidCredentials
	}
	return user, nil
}

// Activate sets the first password for an invited account.
func Activate(ctx context.Context, pool *pgxpool.Pool, userID int64, password string) (models.User, error) {
	hash, err := HashPassword(password)
	if err != nil {
		return models.User{}, fmt.Errorf("hash password: %w", err)
	}

	var user models.User
	err = pool.QueryRow(ctx, `
		UPDATE users SET password_hash = $1, updated_at = now()
		WHERE id = $2 AND password_hash IS NULL AND is_active
		RETURNING `+userColumns, hash, userID,
	).Scan(userDest(&user)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.User{}, ErrInvalidCredentials
	}
	if err != nil {
		return models.User{}, fmt.Errorf("activate account: %w", err)
	}
	return user, nil
}

func Authenticate(ctx context.Context, pool *pgxpool.Pool, identifier, password string) (models.User, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" || password == "" {
		return models.User{}, ErrInvalidCredentials
	}

	var user models.User
	var hash *string
	err := pool.QueryRow(ctx, `
		SELECT id, username, email, password_hash, is_active, created_at, updated_at
		FROM users
		WHERE lower(username) = lower($1) OR lower(email) = lower($1)
		ORDER BY (lower(email) = lower($1)) DESC
		LIMIT 1`, identifier,
	).Scan(&user.ID, &user.Username, &user.Email, &hash, &user.IsActive, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.User{}, ErrInvalidCredentials
	}
	if err != nil {
		return models.User{}, fmt.Errorf("load user: %w", err)
	}
	// A password-less account (a member who never activated) has a NULL hash; it
	// must read as bad credentials, not a server error.
	if hash == nil || *hash == "" || !verifyPassword(*hash, password) || !user.IsActive {
		return models.User{}, ErrInvalidCredentials
	}
	return user, nil
}
