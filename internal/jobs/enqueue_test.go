package jobs

import (
	"context"
	"os"
	"testing"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"jobqueue/internal/config"
)

func TestEnqueueJobID(t *testing.T) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		t.Skip("REDIS_URL not set")
	}

	opts, err := redis.ParseURL(redisURL)
	require.NoError(t, err)

	rdb := redis.NewClient(opts)
	defer rdb.Close()

	queueName := config.QueueHigh
	ctx := context.Background()
	require.NoError(t, rdb.Del(ctx, queueName).Err())

	jobID := "test-job-id-123"
	require.NoError(t, EnqueueJobID(rdb, config.JobTypeSendEmail, jobID))

	length, err := rdb.LLen(ctx, queueName).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), length)

	popped, err := rdb.RPop(ctx, queueName).Result()
	require.NoError(t, err)
	assert.Equal(t, jobID, popped)
}
