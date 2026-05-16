package ordersync

import (
	"context"

	"github.com/trademind-ai/trademind/backend/internal/rdb"
)

// OrderSyncQueueHealthBlock mirrors Redis LIST metrics for /health.
type OrderSyncQueueHealthBlock struct {
	Enabled        bool   `json:"enabled"`
	Name           string `json:"name"`
	RedisAvailable bool   `json:"redisAvailable"`
	Depth          int64  `json:"depth"`
	WorkerEnabled  bool   `json:"workerEnabled"`
	WorkerRunning  bool   `json:"workerRunning"`
	Concurrency    int    `json:"concurrency"`
}

// BuildOrderSyncQueueHealthBlock builds a small health payload for order sync queue.
func BuildOrderSyncQueueHealthBlock(ctx context.Context, redis *rdb.Client, queueEnabled bool, queueName string, workerConcurrency int) OrderSyncQueueHealthBlock {
	if ctx == nil {
		ctx = context.Background()
	}
	out := OrderSyncQueueHealthBlock{
		Enabled:       queueEnabled,
		Name:          queueName,
		WorkerEnabled: queueEnabled,
		WorkerRunning: OrderSyncWorkersRunning(),
		Concurrency:   workerConcurrency,
	}
	if redis == nil || redis.Client == nil {
		return out
	}
	out.RedisAvailable = redis.Ping(ctx).Err() == nil
	if !queueEnabled || !out.RedisAvailable {
		return out
	}
	if n, err := redis.LLen(ctx, queueName).Result(); err == nil {
		out.Depth = n
	}
	return out
}
