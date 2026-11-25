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

var ErrInvalidCredentials = errors.New("invalid credentials")

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

func EnsureSeed(ctx context.Context, pool *pgxpool.Pool, username, email, password string) (string, error) {
	username = strings.TrimSpace(username)
	email = strings.TrimSpace(email)

	var existingID int64
	var existingHash string
	err := pool.QueryRow(ctx, `
		SELECT id, password_hash FROM users
		WHERE lower(username) = lower($1) OR lower(email) = lower($2)
		LIMIT 1`, username, email).Scan(&existingID, &existingHash)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("look up seed user: %w", err)
	}

	if errors.Is(err, pgx.ErrNoRows) {
		hash, err := HashPassword(password)
		if err != nil {
			return "", fmt.Errorf("hash seed password: %w", err)
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO users (username, email, password_hash) VALUES ($1, $2, $3)`,
			username, email, hash); err != nil {
			return "", fmt.Errorf("create seed user: %w", err)
		}
		return "created", nil
	}

	if verifyPassword(existingHash, password) {
		return "unchanged", nil
	}

	hash, err := HashPassword(password)
	if err != nil {
		return "", fmt.Errorf("hash seed password: %w", err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE users SET password_hash = $1, updated_at = now() WHERE id = $2`, hash, existingID); err != nil {
		return "", fmt.Errorf("update seed password: %w", err)
	}
	return "updated", nil
}

func Authenticate(ctx context.Context, pool *pgxpool.Pool, identifier, password string) (models.User, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" || password == "" {
		return models.User{}, ErrInvalidCredentials
	}

	var user models.User
	var hash string
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
	if !verifyPassword(hash, password) || !user.IsActive {
		return models.User{}, ErrInvalidCredentials
	}
	return user, nil
}
