package collect

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// QueueMessage is JSON-serialized for Redis list LPUSH (producer → worker).
type QueueMessage struct {
	TaskID    string `json:"taskId"`
	Source    string `json:"source"`
	URL       string `json:"url"`
	CreatedBy string `json:"createdBy,omitempty"`
	RequestID string `json:"requestId"`
}

func collectQueueName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "collect:tasks"
	}
	return name
}

func collectProcessingQueueName(name string) string {
	return collectQueueName(name) + ":processing"
}

func (s *Service) enqueueTask(ctx context.Context, taskID uuid.UUID, source, sourceURL string, createdBy *uuid.UUID, requestID string) error {
	if s == nil || s.Redis == nil || s.Redis.Client == nil {
		return ErrRedisQueueUnavailable
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := s.Redis.Ping(ctx).Err(); err != nil {
		return ErrRedisQueueUnavailable
	}
	payload, err := marshalQueueMessage(taskID, source, sourceURL, createdBy, requestID)
	if err != nil {
		return err
	}
	if err := s.Redis.LPush(ctx, collectQueueName(s.QueueName), payload).Err(); err != nil {
		return ErrRedisQueueUnavailable
	}
	return nil
}

func marshalQueueMessage(taskID uuid.UUID, source, sourceURL string, createdBy *uuid.UUID, requestID string) ([]byte, error) {
	var cb string
	if createdBy != nil {
		cb = createdBy.String()
	}
	payload, err := json.Marshal(QueueMessage{
		TaskID:    taskID.String(),
		Source:    source,
		URL:       sourceURL,
		CreatedBy: cb,
		RequestID: requestID,
	})
	if err != nil {
		return nil, fmt.Errorf("collect: marshal queue message: %w", err)
	}
	return payload, nil
}

func (s *Service) reserveTask(ctx context.Context, queueName string, timeout time.Duration) (string, error) {
	if s == nil || s.Redis == nil || s.Redis.Client == nil {
		return "", ErrRedisQueueUnavailable
	}
	payload, err := s.Redis.BRPopLPush(ctx, collectQueueName(queueName), collectProcessingQueueName(queueName), timeout).Result()
	if err != nil {
		return "", err
	}
	return payload, nil
}

func (s *Service) ackReservedTask(ctx context.Context, queueName, payload string) error {
	if s == nil || s.Redis == nil || s.Redis.Client == nil {
		return ErrRedisQueueUnavailable
	}
	return s.Redis.LRem(ctx, collectProcessingQueueName(queueName), 1, payload).Err()
}

func (s *Service) requeueReservedTask(ctx context.Context, queueName, payload string) error {
	if s == nil || s.Redis == nil || s.Redis.Client == nil {
		return ErrRedisQueueUnavailable
	}
	_, err := s.Redis.Eval(ctx, `
local removed = redis.call('lrem', KEYS[2], 1, ARGV[1])
if removed > 0 then
  return redis.call('lpush', KEYS[1], ARGV[1])
end
return 0
`, []string{collectQueueName(queueName), collectProcessingQueueName(queueName)}, payload).Result()
	return err
}
