package collect

import (
	"context"
	"encoding/json"
	"fmt"

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
	var cb string
	if createdBy != nil {
		cb = createdBy.String()
	}
	msg := QueueMessage{
		TaskID:    taskID.String(),
		Source:    source,
		URL:       sourceURL,
		CreatedBy: cb,
		RequestID: requestID,
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("collect: marshal queue message: %w", err)
	}
	name := s.QueueName
	if name == "" {
		name = "collect:tasks"
	}
	if err := s.Redis.LPush(ctx, name, payload).Err(); err != nil {
		return ErrRedisQueueUnavailable
	}
	return nil
}
