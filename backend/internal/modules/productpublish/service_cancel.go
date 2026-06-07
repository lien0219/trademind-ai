package productpublish

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
)

// CancelTask marks a pending/running publish task as cancelled.
func (s *Service) CancelTask(c *gin.Context, taskID uuid.UUID, adminID *uuid.UUID) (*TaskDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("productpublish: no db")
	}
	var task ProductPublishTask
	if err := s.DB.WithContext(c.Request.Context()).First(&task, "id = ?", taskID).Error; err != nil {
		return nil, err
	}
	st := strings.TrimSpace(task.Status)
	if st != TaskPending && st != TaskRunning {
		return nil, fmt.Errorf("only pending or running tasks can be cancelled")
	}
	fin := time.Now().UTC()
	if err := s.DB.WithContext(c.Request.Context()).Model(&ProductPublishTask{}).Where("id = ?", taskID).
		Updates(map[string]any{
			"status":         TaskCancelled,
			"publish_status": TaskCancelled,
			"finished_at":    &fin,
			"locked_by":      nil,
			"locked_until":   nil,
			"updated_at":     fin,
		}).Error; err != nil {
		return nil, err
	}
	if snap, ok := parseDouyinDraftSnapshot(task.Input); ok {
		_ = s.DB.WithContext(c.Request.Context()).Model(&ProductPublication{}).Where("id = ?", snap.PublicationID).
			Updates(map[string]any{"status": TaskCancelled, "publish_status": TaskCancelled, "updated_at": fin}).Error
	} else if rid, ok := snapshotPublicationFromTask(&task); ok {
		_ = s.DB.WithContext(c.Request.Context()).Model(&ProductPublication{}).Where("id = ?", rid).
			Updates(map[string]any{"status": TaskCancelled, "publish_status": TaskCancelled, "updated_at": fin}).Error
	}
	action := "product.publish.cancel"
	if task.Platform == "douyin_shop" {
		action = "douyin.product.publish_task.cancel"
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      action,
			Resource:    "product_publish_task",
			ResourceID:  taskID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("taskId=%s platform=%s", taskID, task.Platform),
		})
	}
	out, err := s.GetDTO(c.Request.Context(), taskID)
	return &out, err
}
