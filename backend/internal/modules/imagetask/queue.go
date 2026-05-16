package imagetask

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/rdb"
)

// ImageQueueMessage is JSON-serialized for Redis list LPUSH (producer → worker).
// Keep payloads small: no API keys, no image bytes; worker loads rows by taskId.
type ImageQueueMessage struct {
	TaskID    string `json:"taskId"`
	TaskType  string `json:"taskType"`
	Provider  string `json:"provider"`
	CreatedBy string `json:"createdBy,omitempty"`
	RequestID string `json:"requestId,omitempty"`
}

func (s *Service) enqueueTask(ctx context.Context, taskID uuid.UUID, taskType, provider string, createdBy *uuid.UUID, requestID string) error {
	if s == nil || s.Redis == nil || s.Redis.Client == nil {
		return ErrImageQueueUnavailable
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := s.Redis.Ping(ctx).Err(); err != nil {
		return ErrImageQueueUnavailable
	}
	var cb string
	if createdBy != nil {
		cb = createdBy.String()
	}
	msg := ImageQueueMessage{
		TaskID:    taskID.String(),
		TaskType:  taskType,
		Provider:  provider,
		CreatedBy: cb,
		RequestID: requestID,
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("imagetask: marshal queue message: %w", err)
	}
	name := s.QueueName
	if name == "" {
		name = "image:tasks"
	}
	if err := s.Redis.LPush(ctx, name, payload).Err(); err != nil {
		return ErrImageQueueUnavailable
	}
	return nil
}

// RedisQueueDepth returns LLEN for the configured queue name (0 if Redis unavailable).
func RedisQueueDepth(ctx context.Context, redis *rdb.Client, queueName string) int64 {
	if redis == nil || redis.Client == nil || queueName == "" {
		return 0
	}
	if ctx == nil {
		ctx = context.Background()
	}
	n, err := redis.LLen(ctx, queueName).Result()
	if err != nil {
		return -1
	}
	return n
}
