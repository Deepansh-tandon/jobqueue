package workers

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"jobqueue/internal/jobs"
	"jobqueue/internal/monitoring"
)

type AutoScaler struct {
	pool         *Pool
	rdb          *redis.Client
	metrics      *monitoring.Metrics
	logger       *zap.Logger
	interval     time.Duration
	scaleUp      int64
	scaleDown    int64
	scaleUpInc   int
	scaleDownInc int
}

func NewAutoScaler(pool *Pool, rdb *redis.Client, metrics *monitoring.Metrics, logger *zap.Logger) *AutoScaler {
	return &AutoScaler{
		pool:         pool,
		rdb:          rdb,
		metrics:      metrics,
		logger:       logger,
		interval:     5 * time.Second,
		scaleUp:      20,
		scaleDown:    5,
		scaleUpInc:   2,
		scaleDownInc: 1,
	}
}

func (as *AutoScaler) Run(ctx context.Context) {
	ticker := time.NewTicker(as.interval)
	defer ticker.Stop()
	as.logger.Info("autoscaler started")
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			numWorkers, stream := as.pool.GetStats()
			length, err := jobs.StreamDepth(ctx, as.rdb, stream)
			if err != nil {
				as.logger.Error("failed to get stream length", zap.Error(err))
				continue
			}
			as.metrics.QueueLength.WithLabelValues(stream).Set(float64(length))
			if length > as.scaleUp {
				as.pool.ScaleUp(as.scaleUpInc)
			} else if length < as.scaleDown {
				as.pool.ScaleDown(as.scaleDownInc)
			}
			_ = numWorkers
		}
	}
}
