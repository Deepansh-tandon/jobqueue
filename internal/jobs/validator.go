package jobs

import (
	"errors"
	"strings"

	"jobqueue/internal/config"
)

var (
	ErrProjectIDRequired = errors.New("project_id is required")
	ErrJobTypeRequired   = errors.New("type is required")
	ErrUnknownJobType    = errors.New("unknown job type")
	ErrPayloadRequired   = errors.New("payload is required")
)

// ValidateSubmit checks fields required for POST /api/v1/job/submit.
func ValidateSubmit(projectID, jobType string, payload map[string]interface{}) error {
	if strings.TrimSpace(projectID) == "" {
		return ErrProjectIDRequired
	}
	if strings.TrimSpace(jobType) == "" {
		return ErrJobTypeRequired
	}
	if _, ok := config.AllowedJobTypes[jobType]; !ok {
		return ErrUnknownJobType
	}
	if payload == nil {
		return ErrPayloadRequired
	}
	return nil
}
