package workers

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"jobqueue/internal/ai"
	"jobqueue/internal/models"
	"jobqueue/internal/monitoring"
	"jobqueue/internal/tasks"
)

const (
	dlqKey = "queue:dlq"
)

type Worker struct {
	id      int
	queue   string
	db      *gorm.DB
	rdb     *redis.Client
	ai      *ai.AI
	metrics *monitoring.Metrics
	logger  *zap.Logger
}

func NewWorker(id int, queue string, db *gorm.DB, rdb *redis.Client, ai *ai.AI, metrics *monitoring.Metrics, logger *zap.Logger) *Worker {
	return &Worker{
		id:      id,
		queue:   queue,
		db:      db,
		rdb:     rdb,
		ai:      ai,
		metrics: metrics,
		logger:  logger.With(zap.Int("worker_id", id), zap.String("queue", queue)),
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
			result, err := w.rdb.BRPop(ctx, 5*time.Second, w.queue).Result()
			if err != nil {
				if errors.Is(err, redis.Nil) {
					continue // Timeout, no job received.
				}
				w.logger.Error("failed to pop job from queue", zap.Error(err))
				time.Sleep(1 * time.Second)
				continue
			}

			if len(result) < 2 {
				continue
			}
			w.processJob(ctx, result[1])
		}
	}
}

func (w *Worker) processJob(ctx context.Context, jobID string) {
	w.logger.Info("processing job", zap.String("job_id", jobID))
	startTime := time.Now()

	tx := w.db.Begin()
	if tx.Error != nil {
		w.logger.Error("failed to begin transaction", zap.Error(tx.Error))
		return
	}
	defer tx.Rollback() // Rollback is ignored if tx is committed

	var job models.Job
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&job, "id = ?", jobID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			w.logger.Warn("job not found in db, maybe deleted", zap.String("job_id", jobID))
			return
		}
		w.logger.Error("failed to get job from db", zap.Error(err))
		return
	}

	if job.Status != models.StatusQueued {
		w.logger.Warn("job was already processed", zap.String("job_id", jobID), zap.String("status", job.Status))
		return // Idempotency check
	}

	job.Status = models.StatusRunning
	if err := tx.Save(&job).Error; err != nil {
		w.logger.Error("failed to update job status to running", zap.Error(err))
		return
	}
	if err := tx.Commit().Error; err != nil {
		w.logger.Error("failed to commit transaction", zap.Error(err))
		return
	}

	processingErr := w.executeTask(ctx, job)

	duration := time.Since(startTime).Milliseconds()
	if processingErr != nil {
		w.logger.Warn("job execution failed", zap.Error(processingErr), zap.String("job_id", jobID))
		w.metrics.JobFailuresTotal.WithLabelValues(w.queue, job.Type).Inc()
		w.handleFailure(ctx, job, duration)
	} else {
		w.logger.Info("job executed successfully", zap.String("job_id", jobID))
		if err := w.db.Model(&job).Updates(models.Job{Status: models.StatusCompleted, Duration: duration}).Error; err != nil {
			w.logger.Error("failed to update job to completed", zap.Error(err))
		}
		w.metrics.JobsProcessedTotal.WithLabelValues(w.queue, models.StatusCompleted).Inc()
	}
	w.metrics.JobDurationSeconds.WithLabelValues(w.queue, job.Type).Observe(float64(duration) / 1000)
}

// executeTask finds the correct processor and executes the job.
func (w *Worker) executeTask(ctx context.Context, job models.Job) error {
	processor, err := tasks.Get(job.Type)
	if err != nil {
		// This is a permanent failure, as the job type is unknown.
		w.logger.Error("no processor for job type", zap.String("job_type", job.Type))
		return err
	}
	return processor.Process(ctx, job)
}

func (w *Worker) handleFailure(ctx context.Context, job models.Job, duration int64) {
	job.RetryCount++
	job.Duration = duration

	if job.RetryCount > job.MaxRetries {
		w.logger.Warn("job failed permanently, moving to DLQ", zap.String("job_id", job.ID))
		job.Status = models.StatusFailed
		if err := w.db.Save(&job).Error; err != nil {
			w.logger.Error("failed to update job status to failed", zap.Error(err))
		}
		w.metrics.JobsProcessedTotal.WithLabelValues(w.queue, models.StatusFailed).Inc()

		jobData, _ := json.Marshal(job)
		if err := w.rdb.LPush(ctx, dlqKey, jobData).Err(); err != nil {
			w.logger.Error("failed to push job to DLQ", zap.Error(err))
		}

		var payload map[string]interface{}
		_ = json.Unmarshal([]byte(job.Payload), &payload)
		go w.ai.HandleDLQWithAI(ctx, job.ID, job.Type, payload, job.RetryCount)
	} else {
		w.logger.Info("retrying job", zap.String("job_id", job.ID), zap.Int("retry_count", job.RetryCount))
		job.Status = models.StatusQueued
		if err := w.db.Save(&job).Error; err != nil {
			w.logger.Error("failed to update job status for retry", zap.Error(err))
			return
		}

		if err := w.rdb.LPush(ctx, w.queue, job.ID).Err(); err != nil {
			w.logger.Error("failed to re-enqueue job for retry", zap.Error(err))
			// If this fails, the job is now in a failed state in the DB but not in a queue.
			// A separate recovery process (reaper) would be needed for a truly robust system.
		}
	}
}