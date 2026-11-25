package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port           string
	DatabaseURL    string
	AdminPassword  string
	SessionSecret  string
	UploadDir      string
	WebDir         string
	CookieSecure   bool
	MaxUploadBytes int64
	SessionTTL     time.Duration
	Currency       string
}

func Load() (Config, error) {
	cfg := Config{
		Port:           getenv("PORT", "8080"),
		DatabaseURL:    strings.TrimSpace(os.Getenv("DATABASE_URL")),
		AdminPassword:  os.Getenv("ADMIN_PASSWORD"),
		SessionSecret:  os.Getenv("SESSION_SECRET"),
		UploadDir:      getenv("UPLOAD_DIR", "./uploads"),
		WebDir:         getenv("WEB_DIR", "./web"),
		CookieSecure:   getBool("COOKIE_SECURE", true),
		MaxUploadBytes: getInt64("MAX_UPLOAD_BYTES", 25<<20),
		SessionTTL:     30 * 24 * time.Hour,
		Currency:       getenv("CURRENCY", "$"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if len(cfg.AdminPassword) < 8 {
		return Config{}, errors.New("ADMIN_PASSWORD is required and must be at least 8 characters")
	}
	if len(cfg.SessionSecret) < 16 {
		return Config{}, errors.New("SESSION_SECRET is required and must be at least 16 characters")
	}
	if cfg.MaxUploadBytes <= 0 {
		return Config{}, errors.New("MAX_UPLOAD_BYTES must be positive")
	}
	if _, err := strconv.Atoi(cfg.Port); err != nil {
		return Config{}, fmt.Errorf("PORT must be a number: %w", err)
	}

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func getBool(key string, fallback bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return parsed
}

func getInt64(key string, fallback int64) int64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}
