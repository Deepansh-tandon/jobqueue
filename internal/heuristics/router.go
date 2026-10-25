package heuristics

import "jobqueue/internal/config"

// GetQueue returns the Redis stream name workers consume for the given job type.
func GetQueue(jobType string) string {
	switch jobType {
	case config.JobTypeSendEmail:
		return config.StreamHigh
	default:
		return config.StreamDefault
	}
}

// GetPriorityQueue is deprecated; use GetQueue.
func GetPriorityQueue(jobType string) string {
	return GetQueue(jobType)
}
