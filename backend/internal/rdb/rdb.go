package rdb

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/trademind-ai/trademind/backend/internal/config"
)

// Client wraps the go-redis client for cache and future queue workers.
type Client struct {
	*redis.Client
}

// Open dials Redis (required for full health checks when available).
func Open(cfg *config.Config) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	r := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := r.Ping(ctx).Err(); err != nil {
		_ = r.Close()
		return nil, err
	}
	return &Client{r}, nil
}

// Close closes the Redis connection.
func (c *Client) Close() error {
	if c == nil || c.Client == nil {
		return nil
	}
	return c.Client.Close()
}
