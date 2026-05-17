package productpublish

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

type productPublishQueueMsg struct {
	TaskID string `json:"taskId"`
}

func (s *Service) enqueue(ctx context.Context, taskID uuid.UUID) error {
	if s.Redis == nil || s.Redis.Client == nil {
		return ErrRedisQueueUnavailable
	}
	raw, err := json.Marshal(productPublishQueueMsg{TaskID: taskID.String()})
	if err != nil {
		return err
	}
	return s.Redis.LPush(ctx, s.normalizedQueueName(), string(raw)).Err()
}
