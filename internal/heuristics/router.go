package heuristics

import "jobqueue/internal/config"

// GetQueue returns the Redis list name workers listen on for the given job type.
func GetQueue(jobType string) string {
	switch jobType {
	case config.JobTypeSendEmail:
		return config.QueueHigh
	default:
		return config.QueueDefault
	}
}

// GetPriorityQueue is deprecated; use GetQueue.
func GetPriorityQueue(jobType string) string {
	return GetQueue(jobType)
}
