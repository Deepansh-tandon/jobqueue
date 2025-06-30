package workers

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"jobqueue/internal/heuristics"
	"jobqueue/internal/models"
	"jobqueue/internal/monitoring"
)

type Reaper struct {
	db          *gorm.DB
	rdb         *redis.Client
	metrics     *monitoring.Metrics
	logger      *zap.Logger
	interval    time.Duration
	maxStuckAge time.Duration
}

func NewReaper(db *gorm.DB, rdb *redis.Client, metrics *monitoring.Metrics, logger *zap.Logger) *Reaper {
	return &Reaper{
		db:          db,
		rdb:         rdb,
		metrics:     metrics,
		logger:      logger.With(zap.String("component", "reaper")),
		interval:    5 * time.Minute,
		maxStuckAge: 1 * time.Hour,
	}
}

func (r *Reaper) Run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	r.logger.Info("reaper started")

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("reaper stopped")
			return
		case <-ticker.C:
			r.reapStuckJobs(ctx)
		}
	}
}

func (r *Reaper) reapStuckJobs(ctx context.Context) {
	r.logger.Info("reaping stuck jobs")
	var stuckJobs []models.Job

	stuckTime := time.Now().Add(-r.maxStuckAge)
	if err := r.db.Where("status = ? AND updated_at < ?", models.StatusRunning, stuckTime).Find(&stuckJobs).Error; err != nil {
		r.logger.Error("failed to query for stuck jobs", zap.Error(err))
		return
	}

	if len(stuckJobs) == 0 {
		r.logger.Info("no stuck jobs found")
		return
	}

	r.logger.Warn("found stuck jobs", zap.Int("count", len(stuckJobs)))

	for _, job := range stuckJobs {
		tx := r.db.Begin()
		job.Status = models.StatusQueued
		if err := tx.Save(&job).Error; err != nil {
			tx.Rollback()
			r.logger.Error("failed to update job status for reaping", zap.Error(err), zap.String("job_id", job.ID))
			continue
		}

		queueName := heuristics.GetPriorityQueue(job.Type)
		if err := r.rdb.LPush(ctx, queueName, job.ID).Err(); err != nil {
			tx.Rollback()
			r.logger.Error("failed to re-enqueue reaped job", zap.Error(err), zap.String("job_id", job.ID))
			continue
		}

		if err := tx.Commit().Error; err != nil {
			r.logger.Error("failed to commit reap transaction", zap.Error(err), zap.String("job_id", job.ID))
			continue
		}
		r.metrics.JobsReapedTotal.WithLabelValues(queueName).Inc()
		r.logger.Info("reaped and re-queued job", zap.String("job_id", job.ID))
	}
} 