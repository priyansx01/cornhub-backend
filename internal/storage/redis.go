package storage

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/priyansx01/corn-hub-clone/internal/config"
)

func NewRedisClient(cfg config.RedisConfig) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis connect: %w", err)
	}

	return client, nil
}