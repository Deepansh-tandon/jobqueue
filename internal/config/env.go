package config

import (
	"os"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application.
type Config struct {
	PostgresDSN string
	RedisURL    string
	Port        string
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	// In a local environment, load .env file.
	// In production (e.g., Docker), environment variables are usually passed directly.
	_ = godotenv.Load()

	dsn := os.Getenv("POSTGRES_DSN")
	redisURL := os.Getenv("REDIS_URL")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return &Config{
		PostgresDSN: dsn,
		RedisURL:    redisURL,
		Port:        port,
	}, nil
}