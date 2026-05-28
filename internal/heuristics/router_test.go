package heuristics

import (
	"testing"

	"jobqueue/internal/config"

	"github.com/stretchr/testify/assert"
)

func TestGetQueue(t *testing.T) {
	assert.Equal(t, config.QueueHigh, GetQueue(config.JobTypeSendEmail))
	assert.Equal(t, config.QueueDefault, GetQueue(config.JobTypeGenerateReceipt))
	assert.Equal(t, config.QueueDefault, GetQueue(config.JobTypeSummarizeText))
	assert.Equal(t, config.QueueDefault, GetQueue("unknown_type"))
}
