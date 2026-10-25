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

	ctx := context.Background()
	stream := config.StreamHigh
	require.NoError(t, rdb.Del(ctx, stream).Err())
	require.NoError(t, EnsureConsumerGroups(ctx, rdb, config.StreamGroup))

	jobID := "test-job-id-123"
	require.NoError(t, EnqueueJobID(rdb, config.JobTypeSendEmail, jobID))

	length, err := rdb.XLen(ctx, stream).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), length)

	msgs, err := rdb.XRange(ctx, stream, "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	assert.Equal(t, jobID, msgs[0].Values["job_id"])
}
