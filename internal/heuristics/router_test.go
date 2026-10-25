package heuristics

import (
	"testing"

	"jobqueue/internal/config"

	"github.com/stretchr/testify/assert"
)

func TestGetQueue(t *testing.T) {
	assert.Equal(t, config.StreamHigh, GetQueue(config.JobTypeSendEmail))
	assert.Equal(t, config.StreamDefault, GetQueue(config.JobTypeGenerateReceipt))
	assert.Equal(t, config.StreamDefault, GetQueue(config.JobTypeEcho))
	assert.Equal(t, config.StreamDefault, GetQueue(config.JobTypeAlwaysFail))
	assert.Equal(t, config.StreamDefault, GetQueue("unknown_type"))
}
