package jobs

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"jobqueue/internal/models"
)

func AppendEvent(tx *gorm.DB, jobID, eventType string, payload map[string]interface{}) error {
	raw := "{}"
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		raw = string(b)
	}
	ev := models.JobEvent{
		ID:        uuid.NewString(),
		JobID:     jobID,
		Type:      eventType,
		Payload:   raw,
		CreatedAt: time.Now(),
	}
	return tx.Create(&ev).Error
}
