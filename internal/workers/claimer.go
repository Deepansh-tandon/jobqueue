package workers

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"jobqueue/internal/ai"
	"jobqueue/internal/config"
	"jobqueue/internal/jobs"
	"jobqueue/internal/models"
	"jobqueue/internal/monitoring"
)

// Claimer reclaims idle PEL entries (lease expiry) and re-XADDs queued jobs stuck without a stream delivery.
type Claimer struct {
	db      *gorm.DB
	rdb     *redis.Client
	ai      *ai.AI
	metrics *monitoring.Metrics
	logger  *zap.Logger
	group   string
	lease   time.Duration
	consumer string
}

func NewClaimer(db *gorm.DB, rdb *redis.Client, ai *ai.AI, metrics *monitoring.Metrics, logger *zap.Logger, group string, lease time.Duration) *Claimer {
	return &Claimer{
		db:       db,
		rdb:      rdb,
		ai:       ai,
		metrics:  metrics,
		logger:   logger.With(zap.String("component", "claimer")),
		group:    group,
		lease:    lease,
		consumer: "claimer",
	}
}

func (c *Claimer) Run(ctx context.Context) {
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	c.logger.Info("claimer started", zap.Duration("lease", c.lease))
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			c.claimStream(ctx, config.StreamHigh)
			c.claimStream(ctx, config.StreamDefault)
			c.sweepOrphanQueued(ctx)
			c.resetStuckRunning(ctx)
		}
	}
}

func (c *Claimer) claimStream(ctx context.Context, stream string) {
	start := "0-0"
	for {
		msgs, next, err := c.rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
			Stream:   stream,
			Group:    c.group,
			Consumer: c.consumer,
			MinIdle:  c.lease,
			Start:    start,
			Count:    20,
		}).Result()
		if err != nil {
			if err != redis.Nil {
				c.logger.Debug("xautoclaim", zap.Error(err), zap.String("stream", stream))
			}
			return
		}
		for _, msg := range msgs {
			jobID, _ := msg.Values["job_id"].(string)
			if jobID == "" {
				_ = c.rdb.XAck(ctx, stream, c.group, msg.ID).Err()
				continue
			}
			_ = jobs.AppendEvent(c.db, jobID, models.EventClaimed, map[string]interface{}{"stream_id": msg.ID, "stream": stream})
			w := NewWorker(0, stream, c.group, c.consumer, c.lease, c.db, c.rdb, c.ai, c.metrics, c.logger)
			w.processJob(ctx, jobID, msg.ID)
			c.metrics.JobsReapedTotal.WithLabelValues(stream).Inc()
		}
		if next == "0-0" || len(msgs) == 0 {
			return
		}
		start = next
	}
}

func (c *Claimer) sweepOrphanQueued(ctx context.Context) {
	cutoff := time.Now().Add(-15 * time.Second)
	var jobsRows []models.Job
	if err := c.db.Where("status = ? AND updated_at < ?", models.StatusQueued, cutoff).Limit(50).Find(&jobsRows).Error; err != nil {
		return
	}
	for _, job := range jobsRows {
		_ = jobs.EnqueueJobIDCtx(ctx, c.rdb, job.Type, job.ID)
	}
}

func (c *Claimer) resetStuckRunning(ctx context.Context) {
	cutoff := time.Now().Add(-2 * c.lease)
	var stuck []models.Job
	if err := c.db.Where("status = ? AND updated_at < ?", models.StatusRunning, cutoff).Find(&stuck).Error; err != nil {
		return
	}
	for _, job := range stuck {
		tx := c.db.Begin()
		job.Status = models.StatusQueued
		_ = tx.Save(&job).Error
		_ = jobs.AppendEvent(tx, job.ID, models.EventRetry, map[string]interface{}{"reason": "stuck_running"})
		if tx.Commit().Error != nil {
			tx.Rollback()
			continue
		}
		_ = jobs.EnqueueJobIDCtx(ctx, c.rdb, job.Type, job.ID)
	}
}
