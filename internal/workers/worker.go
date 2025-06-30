package workers

import (
	"context"
	"encoding/json"
    "jobqueue/internal/jobs"
    "jobqueue/internal/queue"
    "log"
)

func StartWorker(queueName string){
	rdb:=queue.NewRedisClient
	for{
		result,err:=rdb.BRPop(context.Background(),0,queueName).Result()
		if err!=nil{
			log.Println(err)
			continue
		}
		if len(result)<2{
			continue
		}

		var job jobs.Job
		if err:= json.Unmarshal([]byte(result[1]),&job);err!=nil{
			log.Println(err)
			continue
		}

		func(){
			defer func(){
				if rec:= recover(); rec!=nil{
					log.Printf("Job %s (Type: %s) panicked: %v\n", job.ID, job.Type, rec)
					handleFailure(job,queueName)
				}
			}()
			job.Process(job)
		}()
	}
}

func handleFailure(job jobs.Job,queueName string){
	rdb:=queue.NewRedisClient()
	job.RetryCount++
	if job.RetryCount > job.MaxRetries {
		  log.Printf("Job %s failed permanently; moving to DLQ\n", job.ID)
        data, _ := json.Marshal(job)
        rdb.LPush(context.Background(), "queue:dlq", data)
	} else {
        log.Printf("Retrying job %s (%d/%d)\n", job.ID, job.RetryCount, job.MaxRetries)
        data, _ := json.Marshal(job)
        rdb.LPush(context.Background(), queueName, data)
    }
}