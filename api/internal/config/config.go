// Package config loads runtime configuration from environment variables.
package config

import "os"

type Config struct {
	HTTPAddr    string
	DatabaseURL string
	Env         string
}

func FromEnv() Config {
	return Config{
		HTTPAddr:    getenv("HTTP_ADDR", ":8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		Env:         getenv("APP_ENV", "development"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
