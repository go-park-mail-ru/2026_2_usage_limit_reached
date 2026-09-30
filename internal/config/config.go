// internal/config/config.go
package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	Addr          string
	JWTSecret     string
	TokenTTL      time.Duration
	AllowedOrigin string
}

func Load() (*Config, error) {
	port := getEnv("APP_PORT", "8080")

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	tokenTTLStr := getEnv("TOKEN_TTL", "24h")
	ttl, err := time.ParseDuration(tokenTTLStr)
	if err != nil {
		return nil, fmt.Errorf("invalid TOKEN_TTL: %w", err)
	}

	allowedOrigin := getEnv("ALLOWED_ORIGIN", "http://localhost:8080")

	return &Config{
		Addr:          ":" + port,
		JWTSecret:     secret,
		TokenTTL:      ttl,
		AllowedOrigin: allowedOrigin,
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
