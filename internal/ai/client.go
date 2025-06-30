package ai

import "github.com/go-redis/redis/v8"

type AI struct {
	rdb *redis.Client
	// Potentially an LLM client would go here in a real app
}

func New(rdb *redis.Client) *AI {
	return &AI{rdb: rdb}
}