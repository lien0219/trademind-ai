package imagetask

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"gorm.io/gorm"
)

func (s *Service) computeExecutionTimeout(ctx context.Context, task *ImageTask) time.Duration {
	if task == nil {
		return 60 * time.Second
	}
	timeout := imageOperationTimeout(ctx, s.Settings)
	if strings.EqualFold(strings.TrimSpace(task.Provider), "comfyui") {
		if b := s.comfyUIExecutionBudget(ctx); b > timeout {
			timeout = b
		}
	}
	if s != nil && s.TaskTimeoutMax > 0 && timeout > s.TaskTimeoutMax {
		timeout = s.TaskTimeoutMax
	}
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	return timeout
}

func (s *Service) tryClaimImageTask(ctx context.Context, taskID uuid.UUID, workerID string, lease time.Duration) (*ImageTask, bool, error) {
	if s == nil || s.DB == nil {
		return nil, false, fmt.Errorf("imagetask: no db")
	}
	now := time.Now().UTC()
	until := now.Add(lease)
	res := s.DB.WithContext(ctx).Model(&ImageTask{}).
		Where(`id = ? AND (status = ? OR (status = ? AND next_retry_at IS NULL)) AND (locked_by IS NULL OR locked_until < ?)`,
			taskID, StatusPending, StatusRetrying, now).
		Updates(map[string]any{
			"status":            StatusRunning,
			"locked_by":         workerID,
			"locked_until":      &until,
			"lock_version":      gorm.Expr("lock_version + 1"),
			"started_at":        gorm.Expr("COALESCE(started_at, ?)", now),
			"finished_at":       nil,
			"error_message":     "",
			"retry_enqueued_at": nil,
		})
	if res.Error != nil {
		return nil, false, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, false, nil
	}
	var task ImageTask
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return nil, false, err
	}
	return &task, true, nil
}

func (s *Service) startImageLeaseRenewal(ctx context.Context, taskID uuid.UUID, workerID string, leaseTTL time.Duration) (stop func()) {
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
				_ = s.DB.WithContext(context.Background()).Model(&ImageTask{}).
					Where("id = ? AND status = ? AND locked_by = ?", taskID, StatusRunning, workerID).
					Updates(map[string]any{
						"locked_until": &until,
						"updated_at":   time.Now().UTC(),
					}).Error
			}
		}
	}()
	return cancel
}

func (s *Service) handleImagePanic(parent context.Context, httpCtx *gin.Context, taskID uuid.UUID, workerID string, panicVal any) {
	if s == nil || s.DB == nil {
		return
	}
	ctx := parent
	if ctx == nil {
		ctx = context.Background()
	}
	var cur ImageTask
	if err := s.DB.WithContext(ctx).First(&cur, "id = ?", taskID).Error; err != nil {
		return
	}
	if cur.Status != StatusRunning || cur.LockedBy == nil || *cur.LockedBy != workerID {
		return
	}
	msg := fmt.Sprintf("image worker panic: %v", panicVal)
	_ = s.finalizeImageFailed(ctx, httpCtx, &cur, redactSensitiveErr(msg), false)
}

// RecoverLeaseExpired requeues or fails a stale running image task.
func (s *Service) RecoverLeaseExpired(ctx context.Context, taskID uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("imagetask: no db")
	}
	now := time.Now().UTC()
	var task ImageTask
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return err
	}
	if task.Status != StatusRunning || task.LockedUntil == nil || !task.LockedUntil.Before(now) {
		return nil
	}
	_ = s.DB.WithContext(ctx).Model(&ImageTask{}).Where("id = ?", taskID).
		Updates(map[string]any{
			"locked_by":    nil,
			"locked_until": nil,
			"updated_at":   now,
		}).Error
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return err
	}
	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: task.CreatedBy,
			Action:      "image.task.lease_expired",
			Resource:    "image_task",
			ResourceID:  taskID.String(),
			Status:      "failed",
			Message:     "worker lease expired (reaper)",
		})
	}
	return s.handleImageTaskFailure(ctx, nil, &task, ErrWorkerLeaseExpired)
}

// RecoverLegacyRunning clears stuck historical rows without lease metadata.
func (s *Service) RecoverLegacyRunning(ctx context.Context, taskID uuid.UUID, legacyCutoff time.Time) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("imagetask: no db")
	}
	var task ImageTask
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return err
	}
	if task.Status != StatusRunning {
		return nil
	}
	if task.LockedBy != nil && task.LockedUntil != nil {
		return nil
	}
	if !task.UpdatedAt.Before(legacyCutoff) {
		return nil
	}
	now := time.Now().UTC()
	_ = s.DB.WithContext(ctx).Model(&ImageTask{}).Where("id = ?", taskID).
		Updates(map[string]any{
			"locked_by":    nil,
			"locked_until": nil,
			"updated_at":   now,
		}).Error
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return err
	}
	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: task.CreatedBy,
			Action:      "image.task.lease_expired",
			Resource:    "image_task",
			ResourceID:  taskID.String(),
			Status:      "failed",
			Message:     "legacy running image task recovered",
		})
	}
	return s.handleImageTaskFailure(ctx, nil, &task, ErrWorkerLeaseExpired)
}
