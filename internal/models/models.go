package models

import "time"

const (
	StatusQueued    = "queued"
	StatusScheduled = "scheduled"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusDiscarded = "discarded"
)

const (
	EventQueued    = "queued"
	EventRunning   = "running"
	EventRetry     = "retry"
	EventCompleted = "completed"
	EventFailed    = "failed"
	EventRequeued  = "requeued"
	EventClaimed   = "claimed"
)

type User struct {
	ID        string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	Email     string `gorm:"unique;not null"`
	Password  string `gorm:"not null"`
	APIKey    string `gorm:"unique;not null"`
	CreatedAt time.Time
	Projects  []Project
}

type Project struct {
	ID        string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	Name      string `gorm:"not null"`
	UserID    string `gorm:"type:uuid;not null"`
	CreatedAt time.Time
	Jobs      []Job
}

type Job struct {
	ID             string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	Type           string `gorm:"not null"`
	Payload        string `gorm:"type:jsonb;not null"`
	Status         string `gorm:"not null;default:'queued'"`
	ExecuteAt      time.Time `gorm:"index"`
	Duration       int64
	ProjectID      string `gorm:"type:uuid;not null;uniqueIndex:idx_project_idempotency"`
	IdempotencyKey string `gorm:"not null;uniqueIndex:idx_project_idempotency"`
	RequestHash    string `gorm:"not null"`
	MaxRetries     int    `gorm:"not null;default:3"`
	RetryCount     int    `gorm:"not null;default:0"`
	LastError      string `gorm:"type:text"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type JobEvent struct {
	ID        string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	JobID     string `gorm:"type:uuid;not null;index"`
	Type      string `gorm:"not null"`
	Payload   string `gorm:"type:jsonb"`
	CreatedAt time.Time
}

type FailureIncident struct {
	Fingerprint     string `gorm:"primaryKey"`
	JobType         string `gorm:"not null"`
	ErrorClass      string `gorm:"not null"`
	PayloadKeys     string `gorm:"type:jsonb;not null"`
	Diagnosis       string `gorm:"type:text"`
	OccurrenceCount int    `gorm:"not null;default:1"`
	FirstSeenAt     time.Time
	LastSeenAt      time.Time
}

type FailureEvent struct {
	ID          string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	JobID       string `gorm:"type:uuid;not null;index"`
	Fingerprint string `gorm:"not null;index"`
	ErrorRaw    string `gorm:"type:text"`
	RetryCount  int
	DurationMs  int64
	CreatedAt   time.Time
}
