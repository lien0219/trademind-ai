package imagetask

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ListTaskItems returns all items for a task.
func (s *Service) ListTaskItems(ctx context.Context, taskID uuid.UUID) ([]ImageTaskItem, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("imagetask: no db")
	}
	var items []ImageTaskItem
	if err := s.DB.WithContext(ctx).Where("task_id = ?", taskID).Order("created_at ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// DeleteTaskItem removes a task item row (does not delete stored files).
func (s *Service) DeleteTaskItem(ctx context.Context, taskID, itemID uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("imagetask: no db")
	}
	res := s.DB.WithContext(ctx).Delete(&ImageTaskItem{}, "id = ? AND task_id = ?", itemID, taskID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
