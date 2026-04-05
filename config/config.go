package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration.
type Config struct {
	Port        string
	JWTSecret   string
	JWTExpiry   time.Duration
	DatabaseURL string
	RedisURL    string
	TurnTimeout time.Duration
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		Port:        envOrDefault("PORT", "8080"),
		JWTSecret:   envOrDefault("JWT_SECRET", "change-me-in-production"),
		JWTExpiry:   envDurationOrDefault("JWT_EXPIRY", 24*time.Hour),
		DatabaseURL: envOrDefault("DATABASE_URL", "postgres://cardgames:cardgames@localhost:5432/cardgames?sslmode=disable"),
		RedisURL:    envOrDefault("REDIS_URL", "redis://localhost:6379/0"),
		TurnTimeout: envDurationOrDefault("TURN_TIMEOUT", 20*time.Second),
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envDurationOrDefault(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if secs, err := strconv.Atoi(v); err == nil {
			return time.Duration(secs) * time.Second
		}
	}
	return fallback
}
