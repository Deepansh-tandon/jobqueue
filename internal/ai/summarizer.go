package ai

import (
    "context"
    "fmt"
    "jobqueue/internal/queue"
    "log"
)

// SummarizeFailure builds a summary string; replace with real API call.
func SummarizeFailure(jobID, jobType string, payload map[string]interface{}, retries int) string {
    prompt := fmt.Sprintf(
        "Job %s of type '%s' failed after %d retries. Payload: %v. Why?",
        jobID, jobType, retries, payload,
    )
    // TODO: call real LLM client here
    summary := "LLM says: " + prompt
    return summary
}

// Optionally, store or log the summary alongside the DLQ entry
func HandleDLQWithAI(jobID, jobType string, payload map[string]interface{}, retries int) {
    summary := SummarizeFailure(jobID, jobType, payload, retries)
    log.Printf("🧠 AI Summary for job %s: %s\n", jobID, summary)
    // e.g., push summary into a Redis hash or database for later inspection
    rdb := queue.NewRedisClient()
    key := fmt.Sprintf("dlq_summary:%s", jobID)
    if err := rdb.Set(context.Background(), key, summary, 0).Err(); err != nil {
        log.Println("❌ Failed to save AI summary:", err)
    }
}
