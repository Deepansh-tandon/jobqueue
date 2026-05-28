package jobs

import (
	"testing"

	"jobqueue/internal/config"

	"github.com/stretchr/testify/assert"
)

func TestValidateSubmit(t *testing.T) {
	validPayload := map[string]interface{}{"to": "a@b.com"}

	assert.NoError(t, ValidateSubmit("proj-1", config.JobTypeSendEmail, validPayload))
	assert.ErrorIs(t, ValidateSubmit("", config.JobTypeSendEmail, validPayload), ErrProjectIDRequired)
	assert.ErrorIs(t, ValidateSubmit("proj-1", "", validPayload), ErrJobTypeRequired)
	assert.ErrorIs(t, ValidateSubmit("proj-1", "email", validPayload), ErrUnknownJobType)
	assert.ErrorIs(t, ValidateSubmit("proj-1", config.JobTypeSendEmail, nil), ErrPayloadRequired)
}
