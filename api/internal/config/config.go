// Package config loads runtime configuration from environment variables.
package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	HTTPAddr    string
	DatabaseURL string
	Env         string
	CORSOrigins []string
	WebBaseURL  string

	// SeedDevData applies the dev seed set (fake users etc.) on startup.
	// Local/CI only; the seed runner refuses it when Env is "production".
	SeedDevData bool

	SessionSecret      string
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
}

func FromEnv() Config {
	return Config{
		HTTPAddr:           getenv("HTTP_ADDR", ":8080"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		Env:                getenv("APP_ENV", "development"),
		CORSOrigins:        splitCSV(getenv("CORS_ORIGINS", "http://localhost:3000")),
		WebBaseURL:         getenv("WEB_BASE_URL", "http://localhost:3000"),
		SeedDevData:        getbool("SEED_DEV_DATA"),
		SessionSecret:      os.Getenv("SESSION_SECRET"),
		GoogleClientID:     os.Getenv("GOOGLE_OAUTH_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET"),
		GoogleRedirectURL:  getenv("GOOGLE_OAUTH_REDIRECT_URL", "http://localhost:8080/auth/google/callback"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getbool parses key with strconv.ParseBool; unset or unparsable is false.
func getbool(key string) bool {
	b, err := strconv.ParseBool(os.Getenv(key))
	return err == nil && b
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
