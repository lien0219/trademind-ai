package inventory

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

func (s *Service) inventoryRateDeferKey(taskID uuid.UUID) string {
	return fmt.Sprintf("inv:sync:defer:%s", taskID.String())
}

func (s *Service) inventoryRateDeferCount(ctx context.Context, taskID uuid.UUID) int {
	if s == nil || s.Redis == nil || s.Redis.Client == nil {
		return 0
	}
	n, err := s.Redis.Get(ctx, s.inventoryRateDeferKey(taskID)).Int()
	if err != nil {
		return 0
	}
	return n
}

func (s *Service) inventoryRateDeferIncr(ctx context.Context, taskID uuid.UUID) {
	if s == nil || s.Redis == nil || s.Redis.Client == nil {
		return
	}
	key := s.inventoryRateDeferKey(taskID)
	_ = s.Redis.Incr(ctx, key).Err()
	_ = s.Redis.Expire(ctx, key, 30*time.Minute).Err()
}

func (s *Service) markInventoryRateDeferFailed(ctx context.Context, taskID uuid.UUID, msg string) error {
	if s == nil || s.DB == nil {
		return nil
	}
	fin := time.Now().UTC()
	return s.DB.WithContext(ctx).Model(&InventorySyncTask{}).Where("id = ? AND status = ?", taskID, StatusPending).
		Updates(map[string]any{
			"status":        StatusFailed,
			"error_message": msg,
			"finished_at":   &fin,
			"updated_at":    fin,
		}).Error
}
