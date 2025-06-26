package config

import ( "fmt" "os")

type Config struct{
	RedisUrl string
	Port 	 string
	AiKey	string
}

func load()(*Config,error){
	c:=&Config{
		RedisUrl:os.Getenv("REDIS_URL"),
		Port:os.Getenv("PORT"),
		AiKey:os.Getenv("AI_KEY"),
	}
	if c.RedisUrl==""{
		return nil,fmt.Errorf("REDIS_URL is empty")
	}
	if c.Port==""{
		c.Port="8080"
	}
	return c,nil
}