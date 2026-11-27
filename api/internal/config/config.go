// Package config loads runtime configuration from environment variables.
package config

import (
	"errors"
	"fmt"
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

	// DevAuthEnabled turns on the dev-only sign-in provider (/auth/dev/*),
	// which logs in as a seeded persona without Google. Local/CI only;
	// Validate refuses it unless Env is allowlisted (see DevOnlyAllowed).
	DevAuthEnabled bool

	SessionSecret      string
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string

	// Warnings lists env values that could not be parsed and fell back to a
	// default. main logs each one at startup.
	Warnings []string
}

func FromEnv() Config {
	var warnings []string
	cfg := Config{
		HTTPAddr:           getenv("HTTP_ADDR", ":8080"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		Env:                getenv("APP_ENV", "development"),
		CORSOrigins:        splitCSV(getenv("CORS_ORIGINS", "http://localhost:3000")),
		WebBaseURL:         getenv("WEB_BASE_URL", "http://localhost:3000"),
		SeedDevData:        getbool("SEED_DEV_DATA", &warnings),
		DevAuthEnabled:     getbool("DEV_AUTH_ENABLED", &warnings),
		SessionSecret:      os.Getenv("SESSION_SECRET"),
		GoogleClientID:     os.Getenv("GOOGLE_OAUTH_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET"),
		GoogleRedirectURL:  getenv("GOOGLE_OAUTH_REDIRECT_URL", "http://localhost:8080/auth/google/callback"),
	}
	cfg.Warnings = warnings
	return cfg
}

// DevOnlyAllowed reports whether env permits dev-only features (dev seed
// data, dev auth). It is an allowlist so a typo or an unexpected value
// (prod, staging, "Production", empty) can never enable them. FromEnv
// defaults an unset APP_ENV to development.
func DevOnlyAllowed(env string) bool {
	return env == "development" || env == "test"
}

// ErrDevAuthNotAllowed is returned by Validate when DEV_AUTH_ENABLED is set
// outside the development/test allowlist.
var ErrDevAuthNotAllowed = errors.New(`refusing to enable dev auth: APP_ENV must be "development" or "test"`)

// Validate rejects configurations the API must refuse to start with.
func (c Config) Validate() error {
	if c.DevAuthEnabled && !DevOnlyAllowed(c.Env) {
		return ErrDevAuthNotAllowed
	}
	return nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getbool parses key with strconv.ParseBool. Unset is false; unparsable is
// false plus a warning, so a typo like SEED_DEV_DATA=ture is visible.
func getbool(key string, warnings *[]string) bool {
	v := os.Getenv(key)
	if v == "" {
		return false
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		*warnings = append(*warnings, fmt.Sprintf("%s=%q is not a boolean (use true or false); treating it as false", key, v))
		return false
	}
	return b
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
