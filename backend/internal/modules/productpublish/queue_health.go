package productpublish

import (
	"context"

	"github.com/trademind-ai/trademind/backend/internal/rdb"
)

type ProductPublishQueueHealthBlock struct {
	Enabled        bool   `json:"enabled"`
	Name           string `json:"name"`
	RedisAvailable bool   `json:"redisAvailable"`
	Depth          int64  `json:"depth"`
	WorkerEnabled  bool   `json:"workerEnabled"`
	WorkerRunning  bool   `json:"workerRunning"`
	Concurrency    int    `json:"concurrency"`
}

func BuildProductPublishQueueHealthBlock(ctx context.Context, redis *rdb.Client, queueEnabled bool, queueName string, workerConcurrency int) ProductPublishQueueHealthBlock {
	if ctx == nil {
		ctx = context.Background()
	}
	out := ProductPublishQueueHealthBlock{
		Enabled:       queueEnabled,
		Name:          queueName,
		WorkerEnabled: queueEnabled,
		WorkerRunning: ProductPublishWorkersRunning(),
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
