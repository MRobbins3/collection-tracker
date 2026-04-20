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
			want: Config{HTTPAddr: ":8080", DatabaseURL: "", Env: "development"},
		},
		{
			name: "overrides from env",
			env: map[string]string{
				"HTTP_ADDR":    ":9090",
				"DATABASE_URL": "postgres://example/db",
				"APP_ENV":      "production",
			},
			want: Config{HTTPAddr: ":9090", DatabaseURL: "postgres://example/db", Env: "production"},
		},
		{
			name: "partial override falls back on missing",
			env:  map[string]string{"HTTP_ADDR": ":7070", "DATABASE_URL": "", "APP_ENV": ""},
			want: Config{HTTPAddr: ":7070", DatabaseURL: "", Env: "development"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			got := FromEnv()
			if got != tc.want {
				t.Fatalf("FromEnv() = %+v, want %+v", got, tc.want)
			}
		})
	}
}
