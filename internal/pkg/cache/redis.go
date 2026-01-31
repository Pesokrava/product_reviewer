package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/Pesokrava/product_reviewer/internal/config"
)

// NewRedisClient creates a new Redis client
func NewRedisClient(cfg *config.Config) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.GetRedisAddr(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return client, nil
}

// WaitForRedis waits for Redis to become available with retries
func WaitForRedis(cfg *config.Config, maxRetries int, retryDelay time.Duration) (*redis.Client, error) {
	var client *redis.Client
	var err error

	for i := 0; i < maxRetries; i++ {
		client, err = NewRedisClient(cfg)
		if err == nil {
			return client, nil
		}

		if i < maxRetries-1 {
			time.Sleep(retryDelay)
		}
	}

	return nil, fmt.Errorf("failed to connect to Redis after %d retries: %w", maxRetries, err)
}
