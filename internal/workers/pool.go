package workers

import "log"

func StartWorkerPool(queueName string,count int){
	for i:=0;i<count;i++{
		go StartWorker(queueNameue)
	}
	log.Printf(" Launched %d workers for %s\n queue",count,queueName)
}