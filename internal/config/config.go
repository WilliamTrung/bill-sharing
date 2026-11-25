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
	SeedUsername   string
	SeedEmail      string
	SeedPassword   string
	SessionSecret  string
	UploadDir      string
	WebDir         string
	CookieSecure   bool
	MaxUploadBytes int64
	SessionTTL     time.Duration
	Currency       string
	SMTPHost       string
	SMTPPort       int
	SMTPUser       string
	SMTPPassword   string
	SMTPFrom       string
	SMTPFromName   string
	PublicBaseURL  string
	AdminEmails    []string
}

func Load() (Config, error) {
	cfg := Config{
		Port:           getenv("PORT", "8080"),
		DatabaseURL:    strings.TrimSpace(os.Getenv("DATABASE_URL")),
		SeedUsername:   strings.TrimSpace(os.Getenv("SEED_USERNAME")),
		SeedEmail:      strings.TrimSpace(os.Getenv("SEED_EMAIL")),
		SeedPassword:   os.Getenv("SEED_PASSWORD"),
		SessionSecret:  os.Getenv("SESSION_SECRET"),
		UploadDir:      getenv("UPLOAD_DIR", "./uploads"),
		WebDir:         getenv("WEB_DIR", "./web"),
		CookieSecure:   getBool("COOKIE_SECURE", true),
		MaxUploadBytes: getInt64("MAX_UPLOAD_BYTES", 25<<20),
		SessionTTL:     30 * 24 * time.Hour,
		Currency:       getenv("CURRENCY", "$"),
		SMTPHost:       getenv("SMTP_HOST", "smtp.gmail.com"),
		SMTPPort:       int(getInt64("SMTP_PORT", 587)),
		SMTPUser:       strings.TrimSpace(os.Getenv("SMTP_USER")),
		SMTPPassword:   os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:       strings.TrimSpace(os.Getenv("SMTP_FROM")),
		SMTPFromName:   getenv("SMTP_FROM_NAME", "Share da Bill"),
		PublicBaseURL:  strings.TrimRight(getenv("PUBLIC_BASE_URL", "http://localhost:8080"), "/"),
	}

	// ADMIN_EMAILS lists who may edit members and send invites. The seed account is
	// always an admin so a misconfigured list can never lock everyone out.
	cfg.AdminEmails = parseEmailList(os.Getenv("ADMIN_EMAILS"))
	cfg.AdminEmails = appendUnique(cfg.AdminEmails, strings.ToLower(strings.TrimSpace(cfg.SeedEmail)))

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if cfg.SeedUsername == "" && cfg.SeedEmail != "" {
		cfg.SeedUsername = strings.SplitN(cfg.SeedEmail, "@", 2)[0]
	}
	if len(cfg.SeedUsername) < 3 {
		return Config{}, errors.New("SEED_USERNAME is required and must be at least 3 characters")
	}
	if !strings.Contains(cfg.SeedEmail, "@") {
		return Config{}, errors.New("SEED_EMAIL is required and must be a valid email")
	}
	if len(cfg.SeedPassword) < 8 {
		return Config{}, errors.New("SEED_PASSWORD is required and must be at least 8 characters")
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

// parseEmailList accepts either a JSON-style array (`["a@x","b@y"]`) or a plain
// comma/semicolon-separated list (`a@x,b@y`), so ADMIN_EMAILS is easy to set in
// shell env, compose, or a JSON-ish secret file.
func parseEmailList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	raw = strings.TrimSuffix(strings.TrimPrefix(raw, "["), "]")
	raw = strings.NewReplacer("\"", "", "'", "", "\n", ",", "\r", "").Replace(raw)

	out := make([]string, 0)
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t'
	}) {
		email := strings.ToLower(strings.TrimSpace(part))
		if email != "" {
			out = appendUnique(out, email)
		}
	}
	return out
}

func appendUnique(list []string, value string) []string {
	if value == "" {
		return list
	}
	for _, existing := range list {
		if existing == value {
			return list
		}
	}
	return append(list, value)
}

// IsAdminEmail reports whether an address may manage members and send invites.
func (c Config) IsAdminEmail(email string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return false
	}
	for _, admin := range c.AdminEmails {
		if admin == email {
			return true
		}
	}
	return false
}
