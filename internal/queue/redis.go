package queue

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/Deepansh-tandon/internal/config"
)

var Ctx = context.Background()

func RedisClient(cfg *config.Config) *redis.Client{
	opts,err:= redis.ParseURL(cfg.Redis.URI)
	if err != nil {
		fmt.Println(err)
		opts=&redis.Options{Addr:cfg.RedisUrl}
	}
	client:= redis.NewClient(opts)
	if err:= client.Ping(Ctx).Err();err!=nil{
		panic("redis ping failed"+ err.Error())
	}
	return client
}