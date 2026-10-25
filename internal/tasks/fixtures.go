package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"jobqueue/internal/models"
)

type EchoProcessor struct{}

func (EchoProcessor) Process(ctx context.Context, job models.Job) error {
	return nil
}

type AlwaysFailProcessor struct{}

func (AlwaysFailProcessor) Process(ctx context.Context, job models.Job) error {
	return fmt.Errorf("always_fail: poison job")
}

type FailUntilNProcessor struct{}

type failUntilPayload struct {
	FailTimes int `json:"fail_times"`
}

func (FailUntilNProcessor) Process(ctx context.Context, job models.Job) error {
	var p failUntilPayload
	_ = json.Unmarshal([]byte(job.Payload), &p)
	if p.FailTimes <= 0 {
		p.FailTimes = 2
	}
	if job.RetryCount < p.FailTimes {
		return fmt.Errorf("fail_until_n: attempt %d of %d", job.RetryCount+1, p.FailTimes)
	}
	return nil
}

type SleepProcessor struct{}

type sleepPayload struct {
	MS int `json:"ms"`
}

func (SleepProcessor) Process(ctx context.Context, job models.Job) error {
	var p sleepPayload
	_ = json.Unmarshal([]byte(job.Payload), &p)
	if p.MS <= 0 {
		p.MS = 100
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(time.Duration(p.MS) * time.Millisecond):
		return nil
	}
}
