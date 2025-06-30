package queue

import (
	"context"

	"github.com/go-redis/redis/v8"
	"jobqueue/internal/config"
)

var Ctx = context.Background()

func NewRedisClient(cfg *config.Config) (*redis.Client, error) {
	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(opts)
	if err := client.Ping(Ctx).Err(); err != nil {
		return nil, err
	}
	return client, nil
}