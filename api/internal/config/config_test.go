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
			env:  map[string]string{"HTTP_ADDR": "", "DATABASE_URL": "", "APP_ENV": ""},
			want: Config{HTTPAddr: ":8080", DatabaseURL: "", Env: "development", CORSOrigins: []string{"http://localhost:3000"}},
		},
		{
			name: "overrides from env",
			env: map[string]string{
				"HTTP_ADDR":    ":9090",
				"DATABASE_URL": "postgres://example/db",
				"APP_ENV":      "production",
				"CORS_ORIGINS": "https://app.example.com,https://admin.example.com",
			},
			want: Config{
				HTTPAddr:    ":9090",
				DatabaseURL: "postgres://example/db",
				Env:         "production",
				CORSOrigins: []string{"https://app.example.com", "https://admin.example.com"},
			},
		},
		{
			name: "partial override falls back on missing",
			env:  map[string]string{"HTTP_ADDR": ":7070", "DATABASE_URL": "", "APP_ENV": "", "CORS_ORIGINS": ""},
			want: Config{HTTPAddr: ":7070", DatabaseURL: "", Env: "development", CORSOrigins: []string{"http://localhost:3000"}},
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
				!equalStrings(got.CORSOrigins, tc.want.CORSOrigins) {
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
