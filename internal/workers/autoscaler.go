package workers

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"jobqueue/internal/monitoring"
)

type AutoScaler struct {
	pool        *Pool
	rdb         *redis.Client
	metrics     *monitoring.Metrics
	logger      *zap.Logger
	interval    time.Duration
	scaleUp     int64
	scaleDown   int64
	scaleUpInc  int
	scaleDownInc int
}

func NewAutoScaler(pool *Pool, rdb *redis.Client, metrics *monitoring.Metrics, logger *zap.Logger) *AutoScaler {
	return &AutoScaler{
		pool:        pool,
		rdb:         rdb,
		metrics:     metrics,
		logger:      logger,
		interval:    5 * time.Second,  // Check every 5 seconds
		scaleUp:     20,               // Scale up if queue length > 20
		scaleDown:   5,                // Scale down if queue length < 5
		scaleUpInc:  2,                // Add 2 workers at a time
		scaleDownInc: 1,                // Remove 1 worker at a time
	}
}

func (as *AutoScaler) Run(ctx context.Context) {
	ticker := time.NewTicker(as.interval)
	defer ticker.Stop()

	as.logger.Info("autoscaler started")

	for {
		select {
		case <-ctx.Done():
			as.logger.Info("autoscaler stopped")
			return
		case <-ticker.C:
			numWorkers, queueName := as.pool.GetStats()
			length, err := as.rdb.LLen(ctx, queueName).Result()
			if err != nil {
				as.logger.Error("failed to get queue length", zap.Error(err))
				continue
			}

			as.metrics.QueueLength.WithLabelValues(queueName).Set(float64(length))

			as.logger.Debug("checking queue length", zap.Int64("length", length), zap.Int("workers", numWorkers))

			if length > as.scaleUp {
				as.pool.ScaleUp(as.scaleUpInc)
			} else if length < as.scaleDown {
				as.pool.ScaleDown(as.scaleDownInc)
			}
		}
	}
} 