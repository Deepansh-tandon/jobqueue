package config

// Redis queue names consumed by worker pools (cmd/server/main.go).
const (
	QueueHigh    = "queue:high"
	QueueDefault = "queue:default"
)

// Canonical job types — must match tasks.Register in cmd/server/main.go.
const (
	JobTypeSendEmail        = "send_email"
	JobTypeGenerateReceipt  = "generate_receipt"
	JobTypeSummarizeText    = "summarize_text"
)

// AllowedJobTypes lists types accepted by the submit API.
var AllowedJobTypes = map[string]struct{}{
	JobTypeSendEmail:       {},
	JobTypeGenerateReceipt: {},
	JobTypeSummarizeText:   {},
}
