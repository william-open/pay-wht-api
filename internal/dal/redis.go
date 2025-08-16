package dal

import (
	"context"
	"log"
	"time"

	"wht-order-api/internal/config"

	"github.com/go-redis/redis/v8"
)

var RedisClient *redis.Client
var RedisCtx = context.Background()

func InitRedis() {
	c := config.C.Redis
	RedisClient = redis.NewClient(&redis.Options{
		Addr:     c.Addr,
		Password: c.Password,
		DB:       c.DB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := RedisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("redis ping failed: %v", err)
	}
}
