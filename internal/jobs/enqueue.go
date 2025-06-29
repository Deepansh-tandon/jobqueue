package jobs
import(
	
	"context"
	"github.com/google/uuid"
	"github.com/go-redis/redis/v8"
	"time"
	"encoding/json"
)

func EnqueueJob(rdb *redis.Client, job Job) error {
	if job.MaxRetries == 0 {
		job.MaxRetries = 3
	}
	job.ID = uuid.New().String()
	job.CreatedAt = time.Now()

	data, err := json.Marshal(job)
	if err != nil {
		return err
	}

	// For now, use a simple queue name since heuristics router is not implemented
	queueName := "queue:priority:1"
	return rdb.LPush(context.Background(), queueName, data).Err()
eturn "queue:priority:3"
	default:
		return "queue:default"
	}
}urn "queue:priority:3"
	default:
		return "queue:default"
	}
turn "queue:priority:3"
	default:
		return "queue:default"
	}
}urn "queue:priority:3"
	default:
		return "queue:default"
	}
}