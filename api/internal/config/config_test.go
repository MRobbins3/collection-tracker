package config

import (
	"testing"
)

func TestFromEnv(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want Config
	}{
		{
			name: "defaults when env is empty",
			env: map[string]string{
				"HTTP_ADDR": "", "DATABASE_URL": "", "APP_ENV": "",
				"CORS_ORIGINS": "", "WEB_BASE_URL": "", "SESSION_SECRET": "",
				"GOOGLE_OAUTH_CLIENT_ID": "", "GOOGLE_OAUTH_CLIENT_SECRET": "",
				"GOOGLE_OAUTH_REDIRECT_URL": "", "SEED_DEV_DATA": "", "DEV_AUTH_ENABLED": "",
			},
			want: Config{
				HTTPAddr:          ":8080",
				Env:               "development",
				CORSOrigins:       []string{"http://localhost:3000"},
				WebBaseURL:        "http://localhost:3000",
				GoogleRedirectURL: "http://localhost:8080/auth/google/callback",
			},
		},
		{
			name: "overrides from env",
			env: map[string]string{
				"HTTP_ADDR":                  ":9090",
				"DATABASE_URL":               "postgres://example/db",
				"APP_ENV":                    "production",
				"CORS_ORIGINS":               "https://app.example.com,https://admin.example.com",
				"WEB_BASE_URL":               "https://app.example.com",
				"SESSION_SECRET":             "seekrit",
				"GOOGLE_OAUTH_CLIENT_ID":     "cid",
				"GOOGLE_OAUTH_CLIENT_SECRET": "csec",
				"GOOGLE_OAUTH_REDIRECT_URL":  "https://api.example.com/auth/google/callback",
				"SEED_DEV_DATA":              "true",
				"DEV_AUTH_ENABLED":           "true",
			},
			want: Config{
				HTTPAddr:           ":9090",
				DatabaseURL:        "postgres://example/db",
				Env:                "production",
				CORSOrigins:        []string{"https://app.example.com", "https://admin.example.com"},
				WebBaseURL:         "https://app.example.com",
				SessionSecret:      "seekrit",
				GoogleClientID:     "cid",
				GoogleClientSecret: "csec",
				GoogleRedirectURL:  "https://api.example.com/auth/google/callback",
				SeedDevData:        true,
				DevAuthEnabled:     true,
			},
		},
		{
			name: "partial override falls back on missing",
			env: map[string]string{
				"HTTP_ADDR": ":7070", "DATABASE_URL": "", "APP_ENV": "",
				"CORS_ORIGINS": "", "WEB_BASE_URL": "", "SESSION_SECRET": "",
				"GOOGLE_OAUTH_CLIENT_ID": "", "GOOGLE_OAUTH_CLIENT_SECRET": "",
				"GOOGLE_OAUTH_REDIRECT_URL": "", "SEED_DEV_DATA": "", "DEV_AUTH_ENABLED": "",
			},
			want: Config{
				HTTPAddr:          ":7070",
				Env:               "development",
				CORSOrigins:       []string{"http://localhost:3000"},
				WebBaseURL:        "http://localhost:3000",
				GoogleRedirectURL: "http://localhost:8080/auth/google/callback",
			},
		},
		{
			name: "unparsable SEED_DEV_DATA and DEV_AUTH_ENABLED are false",
			env: map[string]string{
				"HTTP_ADDR": "", "DATABASE_URL": "", "APP_ENV": "",
				"CORS_ORIGINS": "", "WEB_BASE_URL": "", "SESSION_SECRET": "",
				"GOOGLE_OAUTH_CLIENT_ID": "", "GOOGLE_OAUTH_CLIENT_SECRET": "",
				"GOOGLE_OAUTH_REDIRECT_URL": "", "SEED_DEV_DATA": "yes please", "DEV_AUTH_ENABLED": "on",
			},
			want: Config{
				HTTPAddr:          ":8080",
				Env:               "development",
				CORSOrigins:       []string{"http://localhost:3000"},
				WebBaseURL:        "http://localhost:3000",
				GoogleRedirectURL: "http://localhost:8080/auth/google/callback",
				Warnings: []string{
					`SEED_DEV_DATA="yes please" is not a boolean (use true or false); treating it as false`,
					`DEV_AUTH_ENABLED="on" is not a boolean (use true or false); treating it as false`,
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			got := FromEnv()
			if got.HTTPAddr != tc.want.HTTPAddr ||
				got.DatabaseURL != tc.want.DatabaseURL ||
				got.Env != tc.want.Env ||
				!equalStrings(got.CORSOrigins, tc.want.CORSOrigins) ||
				got.WebBaseURL != tc.want.WebBaseURL ||
				got.SessionSecret != tc.want.SessionSecret ||
				got.GoogleClientID != tc.want.GoogleClientID ||
				got.GoogleClientSecret != tc.want.GoogleClientSecret ||
				got.GoogleRedirectURL != tc.want.GoogleRedirectURL ||
				got.SeedDevData != tc.want.SeedDevData ||
				got.DevAuthEnabled != tc.want.DevAuthEnabled ||
				!equalStrings(got.Warnings, tc.want.Warnings) {
				t.Fatalf("FromEnv() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// Dev-only features (dev seeds, dev auth) are allowlisted: only the exact
// values "development" and "test" permit them.
func TestDevOnlyAllowed(t *testing.T) {
	cases := map[string]bool{
		"development":  true,
		"test":         true,
		"":             false,
		"production":   false,
		"prod":         false,
		"staging":      false,
		"Development":  false,
		" development": false,
		"test ":        false,
	}
	for env, want := range cases {
		t.Run("env="+env, func(t *testing.T) {
			if got := DevOnlyAllowed(env); got != want {
				t.Fatalf("DevOnlyAllowed(%q) = %v, want %v", env, got, want)
			}
		})
	}
}

func TestValidateRefusesDevAuthOutsideAllowlist(t *testing.T) {
	cases := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"dev auth off in production is fine", Config{Env: "production"}, false},
		{"dev auth on in development", Config{Env: "development", DevAuthEnabled: true}, false},
		{"dev auth on in test", Config{Env: "test", DevAuthEnabled: true}, false},
		{"dev auth on in production refuses", Config{Env: "production", DevAuthEnabled: true}, true},
		{"dev auth on in staging refuses", Config{Env: "staging", DevAuthEnabled: true}, true},
		{"dev auth on with empty env refuses", Config{Env: "", DevAuthEnabled: true}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.wantErr && err == nil {
				t.Fatal("Validate() = nil, want an error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
		})
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
