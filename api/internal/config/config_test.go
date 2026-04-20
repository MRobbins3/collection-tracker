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
				"GOOGLE_OAUTH_REDIRECT_URL": "",
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
			},
		},
		{
			name: "partial override falls back on missing",
			env: map[string]string{
				"HTTP_ADDR": ":7070", "DATABASE_URL": "", "APP_ENV": "",
				"CORS_ORIGINS": "", "WEB_BASE_URL": "", "SESSION_SECRET": "",
				"GOOGLE_OAUTH_CLIENT_ID": "", "GOOGLE_OAUTH_CLIENT_SECRET": "",
				"GOOGLE_OAUTH_REDIRECT_URL": "",
			},
			want: Config{
				HTTPAddr:          ":7070",
				Env:               "development",
				CORSOrigins:       []string{"http://localhost:3000"},
				WebBaseURL:        "http://localhost:3000",
				GoogleRedirectURL: "http://localhost:8080/auth/google/callback",
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
				got.GoogleRedirectURL != tc.want.GoogleRedirectURL {
				t.Fatalf("FromEnv() = %+v, want %+v", got, tc.want)
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
