package productpublish

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func (s *Service) publishLeaseTTL() time.Duration {
	if s != nil && s.TaskTimeout > 0 {
		return s.TaskTimeout
	}
	return 180 * time.Second
}

func (s *Service) tryClaimProductPublishTask(ctx context.Context, taskID uuid.UUID, workerID string, lease time.Duration) (*ProductPublishTask, bool, error) {
	if s == nil || s.DB == nil {
		return nil, false, fmt.Errorf("productpublish: no db")
	}
	now := time.Now().UTC()
	until := now.Add(lease)
	res := s.DB.WithContext(ctx).Model(&ProductPublishTask{}).
		Where(`id = ? AND status = ? AND (locked_by IS NULL OR locked_until < ?)`, taskID, TaskPending, now).
		Updates(map[string]any{
			"status":       TaskRunning,
			"locked_by":    workerID,
			"locked_until": &until,
			"lock_version": gorm.Expr("lock_version + 1"),
			"started_at":   gorm.Expr("COALESCE(started_at, ?)", now),
			"updated_at":   now,
		})
	if res.Error != nil {
		return nil, false, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, false, nil
	}
	var task ProductPublishTask
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return nil, false, err
	}
	return &task, true, nil
}

func (s *Service) startPublishLeaseRenewal(ctx context.Context, taskID uuid.UUID, workerID string, leaseTTL time.Duration) func() {
	if s == nil || s.DB == nil {
		return func() {}
	}
	interval := leaseTTL / 3
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}
	runCtx, cancel := context.WithCancel(ctx)
	go func() {
		tick := time.NewTicker(interval)
		defer tick.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-tick.C:
				until := time.Now().UTC().Add(leaseTTL)
				_ = s.DB.WithContext(context.Background()).Model(&ProductPublishTask{}).
					Where("id = ? AND status = ? AND locked_by = ?", taskID, TaskRunning, workerID).
					Updates(map[string]any{
						"locked_until": &until,
						"updated_at":   time.Now().UTC(),
					}).Error
			}
		}
	}()
	return cancel
}

func (s *Service) RecoverLeaseExpired(ctx context.Context, taskID uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("productpublish: no db")
	}
	now := time.Now().UTC()
	var task ProductPublishTask
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return err
	}
	if task.Status != TaskRunning || task.LockedUntil == nil || !task.LockedUntil.Before(now) {
		return nil
	}
	fin := now
	msg := "worker lease expired"
	code := platformdouyin.CodeDouyinTaskStale
	recovery := platformdouyin.RecoveryStale
	if task.Platform == "douyin_shop" {
		out := platformdouyin.MarshalRecoveryOutput(nil, platformdouyin.TaskRecoveryMeta{
			RecoveryStatus: recovery,
			LastErrorCode:  code,
			UserMessage:    platformdouyin.UserMessageForRecovery(recovery),
			TechnicalCode:  code,
		})
		_ = s.DB.WithContext(ctx).Model(&ProductPublishTask{}).Where("id = ?", taskID).
			Updates(map[string]any{
				"status":        TaskFailed,
				"error_code":    code,
				"error_message": platformdouyin.UserMessageForRecovery(recovery),
				"retryable":     true,
				"finished_at":   &fin,
				"output":        datatypes.JSON(out),
				"locked_by":     nil,
				"locked_until":  nil,
				"updated_at":    fin,
			}).Error
	} else {
		_ = s.DB.WithContext(ctx).Model(&ProductPublishTask{}).Where("id = ?", taskID).
			Updates(map[string]any{
				"status":        TaskFailed,
				"error_message": msg,
				"finished_at":   &fin,
				"locked_by":     nil,
				"locked_until":  nil,
				"updated_at":    fin,
			}).Error
	}
	if rid, ok := snapshotPublicationFromTask(&task); ok {
		_ = s.DB.WithContext(ctx).Model(&ProductPublication{}).Where("id = ?", rid).
			Updates(map[string]any{
				"status":         StatusPubFailed,
				"publish_status": StatusPubFailed,
				"updated_at":     fin,
			}).Error
	}
	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: task.CreatedBy,
			Action:      "product.publish.failed",
			Resource:    "product_publish_task",
			ResourceID:  taskID.String(),
			Status:      "failed",
			Message:     fmt.Sprintf("taskId=%s reason=lease_expired shopId=%s", taskID.String(), task.ShopID.String()),
		})
	}
	return nil
}

func (s *Service) RecoverLegacyRunning(ctx context.Context, taskID uuid.UUID, legacyCutoff time.Time) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("productpublish: no db")
	}
	var task ProductPublishTask
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return err
	}
	if task.Status != TaskRunning {
		return nil
	}
	if task.LockedBy != nil && task.LockedUntil != nil {
		return nil
	}
	if !task.UpdatedAt.Before(legacyCutoff) {
		return nil
	}
	fin := time.Now().UTC()
	msg := "legacy running publish task recovered (no lease)"
	_ = s.DB.WithContext(ctx).Model(&ProductPublishTask{}).Where("id = ?", taskID).
		Updates(map[string]any{
			"status":        TaskFailed,
			"error_message": msg,
			"finished_at":   &fin,
			"locked_by":     nil,
			"locked_until":  nil,
			"updated_at":    fin,
		}).Error
	return nil
}
