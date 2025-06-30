package heuristics

func GetPriorityQueue(jobType string) string {
    switch jobType {
    case "email":
        return "queue:priority:1"
    case "pdf":
        return "queue:priority:2"
    case "ai_summary":
        return "queue:priority:3"
    default:
        return "queue:priority:2"
    }
}