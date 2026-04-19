package queue

import (
	"fmt"

	"github.com/redis/go-redis/v9"
)

// compile-time check that adapters implement Queue
var (
	_ Queue = (*RedisAdapter)(nil)
	_ Queue = (*MemoryAdapter)(nil)
)

// NewQueue creates a Queue instance based on the provided config.
func NewQueue(cfg Config) (Queue, error) {
	switch cfg.Backend {
	case "redis", "":
		if cfg.RedisURL == "" {
			return nil, &QueueError{
				Code:    "CONFIG_ERROR",
				Message: "RedisURL is required for redis backend",
			}
		}
		opts, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			return nil, &QueueError{
				Code:    "CONFIG_ERROR",
				Message: "invalid Redis URL",
				Cause:   err,
			}
		}
		client := redis.NewClient(opts)
		redisOpts := []RedisOption{}
		if cfg.PollInterval > 0 {
			redisOpts = append(redisOpts, WithPollInterval(cfg.PollInterval))
		}
		if cfg.VisibilityTimeout > 0 {
			redisOpts = append(redisOpts, WithVisibilityTimeout(cfg.VisibilityTimeout))
		}
		return NewRedisAdapter(client, redisOpts...), nil
	case "memory":
		return NewMemoryAdapter(), nil
	default:
		return nil, &QueueError{
			Code:    "CONFIG_ERROR",
			Message: fmt.Sprintf("unknown queue backend: %s", cfg.Backend),
		}
	}
}
