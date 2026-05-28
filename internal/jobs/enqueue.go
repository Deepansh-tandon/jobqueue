package jobs

import (
	"context"

	"github.com/go-redis/redis/v8"
	"jobqueue/internal/heuristics"
)

// EnqueueJobID pushes a job ID onto the Redis queue workers consume.
// The job row must already exist in PostgreSQL with status queued.
func EnqueueJobID(rdb *redis.Client, jobType, jobID string) error {
	queueName := heuristics.GetQueue(jobType)
	return rdb.LPush(context.Background(), queueName, jobID).Err()
}
