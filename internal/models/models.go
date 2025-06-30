package models

import "time"

const (
	StatusQueued    = "queued"
	StatusScheduled = "scheduled"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

type User struct {
    ID        string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
    Email     string    `gorm:"unique;not null"`
    Password  string    `gorm:"not null"`
    APIKey    string    `gorm:"unique;not null"`
    CreatedAt time.Time
    Projects  []Project
}

type Project struct {
    ID        string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
    Name      string    `gorm:"not null"`
    UserID    string    `gorm:"type:uuid;not null"`
    CreatedAt time.Time
    Jobs      []Job
}

type Job struct {
    ID         string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
    Type       string    `gorm:"not null"`
    Payload    string    `gorm:"type:jsonb;not null"`
    Status     string    `gorm:"not null;default:'queued'"`
    ExecuteAt  time.Time `gorm:"index"`
    Duration   int64     // in milliseconds
    ProjectID  string    `gorm:"type:uuid;not null"`
    MaxRetries int       `gorm:"not null;default:3"`
    RetryCount int       `gorm:"not null;default:0"`
    CreatedAt  time.Time
    UpdatedAt  time.Time
}
