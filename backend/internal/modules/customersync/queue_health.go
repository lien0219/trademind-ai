package customersync

import (
	"context"
	"github.com/trademind-ai/trademind/backend/internal/rdb"
)

// CustomerMessageSyncQueueHealthBlock mirrors Redis LIST metrics for /health.
type CustomerMessageSyncQueueHealthBlock struct {
	Enabled           bool   `json:"enabled"`
	QueueName         string `json:"queueName,omitempty"`
	RedisOk           bool   `json:"redisOk"`
	RedisAvailable    bool   `json:"redisAvailable"`
	LLen              int64  `json:"llen,omitempty"`
	WorkerRunning     bool   `json:"workerRunning"`
	WorkerConcurrency int    `json:"workerConcurrency,omitempty"`
}

// BuildCustomerMessageSyncQueueHealthBlock builds a small health payload.
func BuildCustomerMessageSyncQueueHealthBlock(ctx context.Context, redis *rdb.Client, queueEnabled bool, queueName string, workerConcurrency int) CustomerMessageSyncQueueHealthBlock {
	out := CustomerMessageSyncQueueHealthBlock{
		Enabled:           queueEnabled,
		QueueName:         queueName,
		WorkerRunning:     CustomerMessageSyncWorkersRunning(),
		WorkerConcurrency: workerConcurrency,
	}
	if !queueEnabled {
		return out
	}
	if redis == nil || redis.Client == nil {
		out.RedisOk = false
		out.RedisAvailable = false
		return out
	}
	if err := redis.Ping(ctx).Err(); err != nil {
		out.RedisOk = false
		out.RedisAvailable = false
		return out
	}
	out.RedisOk = true
	out.RedisAvailable = true
	if n, err := redis.LLen(ctx, queueName).Result(); err == nil {
		out.LLen = n
	} else {
		out.RedisAvailable = false
	}
	return out
}
