package workers

import (
	"context"
	"errors"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"jobqueue/internal/ai"
	"jobqueue/internal/jobs"
	"jobqueue/internal/models"
	"jobqueue/internal/monitoring"
	"jobqueue/internal/tasks"
)

type Worker struct {
	id       int
	stream   string
	group    string
	consumer string
	lease    time.Duration
	db       *gorm.DB
	rdb      *redis.Client
	ai       *ai.AI
	metrics  *monitoring.Metrics
	logger   *zap.Logger
}

func NewWorker(id int, stream, group, consumer string, lease time.Duration, db *gorm.DB, rdb *redis.Client, ai *ai.AI, metrics *monitoring.Metrics, logger *zap.Logger) *Worker {
	return &Worker{
		id:       id,
		stream:   stream,
		group:    group,
		consumer: consumer,
		lease:    lease,
		db:       db,
		rdb:      rdb,
		ai:       ai,
		metrics:  metrics,
		logger:   logger.With(zap.Int("worker_id", id), zap.String("stream", stream), zap.String("consumer", consumer)),
	}
}

func (w *Worker) Loop(ctx context.Context) {
	w.logger.Info("worker loop started")
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("worker loop stopping")
			return
		default:
			res, err := w.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    w.group,
				Consumer: w.consumer,
				Streams:  []string{w.stream, ">"},
				Count:    1,
				Block:    5 * time.Second,
			}).Result()
			if err != nil {
				if errors.Is(err, redis.Nil) || errors.Is(err, context.Canceled) {
					continue
				}
				w.logger.Error("xreadgroup failed", zap.Error(err))
				time.Sleep(1 * time.Second)
				continue
			}
			for _, stream := range res {
				for _, msg := range stream.Messages {
					w.handleMessage(ctx, msg)
				}
			}
		}
	}
}

func (w *Worker) handleMessage(ctx context.Context, msg redis.XMessage) {
	jobID, _ := msg.Values["job_id"].(string)
	if jobID == "" {
		_ = w.rdb.XAck(ctx, w.stream, w.group, msg.ID).Err()
		return
	}
	hb, stopHB := w.startHeartbeat(ctx, msg.ID)
	defer stopHB()
	w.processJob(ctx, jobID, msg.ID)
	_ = hb
}

func (w *Worker) startHeartbeat(ctx context.Context, msgID string) (context.CancelFunc, func()) {
	hbCtx, cancel := context.WithCancel(ctx)
	interval := w.lease / 3
	if interval < time.Second {
		interval = time.Second
	}
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-hbCtx.Done():
				return
			case <-t.C:
				_ = w.rdb.XClaim(hbCtx, &redis.XClaimArgs{
					Stream:   w.stream,
					Group:    w.group,
					Consumer: w.consumer,
					MinIdle:  0,
					Messages: []string{msgID},
				}).Err()
			}
		}
	}()
	return cancel, cancel
}

func (w *Worker) processJob(ctx context.Context, jobID, msgID string) {
	w.logger.Info("processing job", zap.String("job_id", jobID), zap.String("msg_id", msgID))
	startTime := time.Now()

	tx := w.db.Begin()
	if tx.Error != nil {
		w.logger.Error("failed to begin transaction", zap.Error(tx.Error))
		return
	}

	var job models.Job
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&job, "id = ?", jobID).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			_ = w.rdb.XAck(ctx, w.stream, w.group, msgID).Err()
			return
		}
		w.logger.Error("failed to get job from db", zap.Error(err))
		return
	}

	if job.Status != models.StatusQueued {
		tx.Rollback()
		_ = jobs.AppendEvent(w.db, job.ID, models.EventClaimed, map[string]interface{}{"reason": "idempotent_skip", "status": job.Status})
		_ = w.rdb.XAck(ctx, w.stream, w.group, msgID).Err()
		return
	}

	job.Status = models.StatusRunning
	if err := tx.Save(&job).Error; err != nil {
		tx.Rollback()
		return
	}
	_ = jobs.AppendEvent(tx, job.ID, models.EventRunning, map[string]interface{}{"stream_id": msgID, "consumer": w.consumer})
	if err := tx.Commit().Error; err != nil {
		return
	}

	processingErr := w.executeTask(ctx, job)
	duration := time.Since(startTime).Milliseconds()

	if processingErr != nil {
		w.metrics.JobFailuresTotal.WithLabelValues(w.stream, job.Type).Inc()
		w.handleFailure(ctx, job, duration, processingErr.Error(), msgID)
	} else {
		tx := w.db.Begin()
		_ = tx.Model(&job).Updates(models.Job{Status: models.StatusCompleted, Duration: duration, LastError: ""}).Error
		_ = jobs.AppendEvent(tx, job.ID, models.EventCompleted, map[string]interface{}{"duration_ms": duration})
		if err := tx.Commit().Error; err != nil {
			w.logger.Error("failed to complete job", zap.Error(err))
			return
		}
		_ = w.rdb.XAck(ctx, w.stream, w.group, msgID).Err()
		w.metrics.JobsProcessedTotal.WithLabelValues(w.stream, models.StatusCompleted).Inc()
	}
	w.metrics.JobDurationSeconds.WithLabelValues(w.stream, job.Type).Observe(float64(duration) / 1000)
}

func (w *Worker) executeTask(ctx context.Context, job models.Job) error {
	processor, err := tasks.Get(job.Type)
	if err != nil {
		return err
	}
	return processor.Process(ctx, job)
}

func (w *Worker) handleFailure(ctx context.Context, job models.Job, duration int64, errRaw, msgID string) {
	job.RetryCount++
	job.Duration = duration
	job.LastError = errRaw

	if job.RetryCount > job.MaxRetries {
		tx := w.db.Begin()
		job.Status = models.StatusFailed
		_ = tx.Save(&job).Error
		_ = jobs.AppendEvent(tx, job.ID, models.EventFailed, map[string]interface{}{"error": errRaw})
		if err := tx.Commit().Error; err != nil {
			w.logger.Error("failed to mark failed", zap.Error(err))
			return
		}
		diag := jobs.Diagnoser(jobs.NopDiagnoser{})
		if w.ai != nil && w.ai.Diagnoser != nil {
			diag = w.ai.Diagnoser
		}
		_ = jobs.RecordPermanentFailure(ctx, w.db, diag, job, errRaw, duration)
		_ = w.rdb.XAck(ctx, w.stream, w.group, msgID).Err()
		w.metrics.JobsProcessedTotal.WithLabelValues(w.stream, models.StatusFailed).Inc()
		return
	}

	tx := w.db.Begin()
	job.Status = models.StatusQueued
	_ = tx.Save(&job).Error
	_ = jobs.AppendEvent(tx, job.ID, models.EventRetry, map[string]interface{}{"retry_count": job.RetryCount, "error": errRaw})
	if err := tx.Commit().Error; err != nil {
		return
	}
	if err := jobs.EnqueueJobIDCtx(ctx, w.rdb, job.Type, job.ID); err != nil {
		w.logger.Error("failed to re-enqueue retry", zap.Error(err))
		return
	}
	_ = w.rdb.XAck(ctx, w.stream, w.group, msgID).Err()
}
