package tasks

import (
	"context"
	"errors"
	"fmt"
	"jobqueue/internal/models"
)

// Processor defines the interface for any job type processor.
type Processor interface {
	Process(ctx context.Context, job models.Job) error
}

var processors = make(map[string]Processor)

// Register associates a job type with a processor.
func Register(jobType string, p Processor) {
	processors[jobType] = p
}

// Get returns the processor for a given job type.
func Get(jobType string) (Processor, error) {
	p, ok := processors[jobType]
	if !ok {
		return nil, fmt.Errorf("no processor registered for job type: %s", jobType)
	}
	return p, nil
}

// MockEmailSender is a placeholder for a real email service.
type MockEmailSender struct{}

func (s *MockEmailSender) Process(ctx context.Context, job models.Job) error {
	// In a real app, parse payload and send email via SMTP.
	fmt.Printf("SIMULATING: Sending email for job %s. Payload: %s\n", job.ID, job.Payload)
	if job.ID[0]%2 == 0 { // Simulate occasional error
		return errors.New("simulated SMTP connection error")
	}
	return nil
}

// MockSummarizer is a placeholder for a real summarization service.
type MockSummarizer struct{}

func (s *MockSummarizer) Process(ctx context.Context, job models.Job) error {
	fmt.Printf("SIMULATING: Summarizing text for job %s. Payload: %s\n", job.ID, job.Payload)
	return nil
} 