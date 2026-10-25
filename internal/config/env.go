package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	PostgresDSN string
	RedisURL    string
	Port        string
	GroqAPIKey  string
	GroqModel   string
	GroqBaseURL string
	JobLease    time.Duration
	StreamGroup string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	model := os.Getenv("GROQ_MODEL")
	if model == "" {
		model = "llama-3.1-8b-instant"
	}
	base := os.Getenv("GROQ_BASE_URL")
	if base == "" {
		base = "https://api.groq.com/openai/v1"
	}
	group := os.Getenv("STREAM_GROUP")
	if group == "" {
		group = StreamGroup
	}
	lease := DefaultLease
	if ms := os.Getenv("JOB_LEASE_MS"); ms != "" {
		if n, err := strconv.Atoi(ms); err == nil && n > 0 {
			lease = time.Duration(n) * time.Millisecond
		}
	}

	return &Config{
		PostgresDSN: os.Getenv("POSTGRES_DSN"),
		RedisURL:    os.Getenv("REDIS_URL"),
		Port:        port,
		GroqAPIKey:  firstNonEmpty(os.Getenv("GROQ_API_KEY"), os.Getenv("GROK_API_KEY")),
		GroqModel:   model,
		GroqBaseURL: base,
		JobLease:    lease,
		StreamGroup: group,
	}, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
