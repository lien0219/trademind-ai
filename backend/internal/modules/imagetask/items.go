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
	tenantID, scoped, err := imageTaskTenantScope(ctx)
	if err != nil {
		return nil, err
	}
	if scoped {
		var n int64
		taskQuery := s.DB.WithContext(ctx).Model(&ImageTask{}).Where("id = ?", taskID)
		taskQuery = scopeImageTaskQuery(taskQuery, tenantID, true)
		if err := taskQuery.Count(&n).Error; err != nil {
			return nil, err
		}
		if n == 0 {
			return nil, gorm.ErrRecordNotFound
		}
	}
	var items []ImageTaskItem
	query := s.DB.WithContext(ctx).Model(&ImageTaskItem{}).Where("task_id = ?", taskID)
	query = scopeImageTaskItemQuery(query, tenantID, scoped)
	if err := query.Order("created_at ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// DeleteTaskItem removes a task item row (does not delete stored files).
func (s *Service) DeleteTaskItem(ctx context.Context, taskID, itemID uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("imagetask: no db")
	}
	tenantID, scoped, err := imageTaskTenantScope(ctx)
	if err != nil {
		return err
	}
	query := s.DB.WithContext(ctx).Model(&ImageTaskItem{}).Where("id = ? AND task_id = ?", itemID, taskID)
	query = scopeImageTaskItemQuery(query, tenantID, scoped)
	res := query.Delete(&ImageTaskItem{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
