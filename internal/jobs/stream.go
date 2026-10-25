package jobs

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-redis/redis/v8"
	"jobqueue/internal/config"
	"jobqueue/internal/heuristics"
)

func EnsureConsumerGroups(ctx context.Context, rdb *redis.Client, group string) error {
	for _, stream := range []string{config.StreamHigh, config.StreamDefault} {
		err := rdb.XGroupCreateMkStream(ctx, stream, group, "0").Err()
		if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
			return fmt.Errorf("xgroup create %s: %w", stream, err)
		}
	}
	return nil
}

// EnqueueJobID appends a job ID to the Redis stream. Postgres is the source of truth.
// Retry = new XADD after ACK of the previous delivery (do not leave the same message in PEL).
func EnqueueJobID(rdb *redis.Client, jobType, jobID string) error {
	return EnqueueJobIDCtx(context.Background(), rdb, jobType, jobID)
}

func EnqueueJobIDCtx(ctx context.Context, rdb *redis.Client, jobType, jobID string) error {
	stream := heuristics.GetQueue(jobType)
	return rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: map[string]interface{}{"job_id": jobID},
	}).Err()
}

func StreamDepth(ctx context.Context, rdb *redis.Client, stream string) (int64, error) {
	return rdb.XLen(ctx, stream).Result()
}
