package config

import "time"

const (
	StreamHigh    = "stream:high"
	StreamDefault = "stream:default"
	StreamGroup   = "workers"
)

const (
	JobTypeSendEmail       = "send_email"
	JobTypeGenerateReceipt = "generate_receipt"
	JobTypeSummarizeText   = "summarize_text"
	JobTypeEcho            = "echo"
	JobTypeFailUntilN      = "fail_until_n"
	JobTypeAlwaysFail      = "always_fail"
	JobTypeSleep           = "sleep"
)

var AllowedJobTypes = map[string]struct{}{
	JobTypeSendEmail:       {},
	JobTypeGenerateReceipt: {},
	JobTypeSummarizeText:   {},
	JobTypeEcho:            {},
	JobTypeFailUntilN:      {},
	JobTypeAlwaysFail:      {},
	JobTypeSleep:           {},
}

const DefaultLease = 30 * time.Second
