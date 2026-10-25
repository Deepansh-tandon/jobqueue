package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"jobqueue/internal/models"
)

type Diagnoser interface {
	Diagnose(ctx context.Context, jobType, errorClass, errorRaw string, payloadKeys []string, retries int, durationMs int64) (string, error)
}

type NopDiagnoser struct{}

func (NopDiagnoser) Diagnose(context.Context, string, string, string, []string, int, int64) (string, error) {
	return "", errors.New("diagnoser disabled")
}

// RecordPermanentFailure writes failure_events + incidents. Calls diagnoser only for a new fingerprint.
func RecordPermanentFailure(ctx context.Context, db *gorm.DB, d Diagnoser, job models.Job, errRaw string, durationMs int64) error {
	fp, errClass, keys := Fingerprint(job.Type, errRaw, job.Payload)
	keysJSON, _ := json.Marshal(keys)
	now := time.Now()

	var inc models.FailureIncident
	err := db.Where("fingerprint = ?", fp).First(&inc).Error
	isNew := errors.Is(err, gorm.ErrRecordNotFound)
	if err != nil && !isNew {
		return err
	}

	if isNew {
		inc = models.FailureIncident{
			Fingerprint:     fp,
			JobType:         job.Type,
			ErrorClass:      errClass,
			PayloadKeys:     string(keysJSON),
			OccurrenceCount: 1,
			FirstSeenAt:     now,
			LastSeenAt:      now,
		}
		if err := db.Create(&inc).Error; err != nil {
			isNew = false
			if err := db.Model(&models.FailureIncident{}).Where("fingerprint = ?", fp).Updates(map[string]interface{}{
				"occurrence_count": gorm.Expr("occurrence_count + 1"),
				"last_seen_at":     now,
			}).Error; err != nil {
				return err
			}
		}
	} else {
		if err := db.Model(&models.FailureIncident{}).Where("fingerprint = ?", fp).Updates(map[string]interface{}{
			"occurrence_count": gorm.Expr("occurrence_count + 1"),
			"last_seen_at":     now,
		}).Error; err != nil {
			return err
		}
	}

	fe := models.FailureEvent{
		ID:          uuid.NewString(),
		JobID:       job.ID,
		Fingerprint: fp,
		ErrorRaw:    errRaw,
		RetryCount:  job.RetryCount,
		DurationMs:  durationMs,
		CreatedAt:   now,
	}
	if err := db.Create(&fe).Error; err != nil {
		return err
	}

	if isNew && d != nil {
		go func() {
			diag, err := d.Diagnose(context.Background(), job.Type, errClass, errRaw, keys, job.RetryCount, durationMs)
			if err != nil || diag == "" {
				return
			}
			db.Model(&models.FailureIncident{}).Where("fingerprint = ?", fp).
				Where("diagnosis = ? OR diagnosis IS NULL", "").
				Update("diagnosis", diag)
		}()
	}
	return nil
}
