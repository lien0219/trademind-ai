package customersync

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"gorm.io/gorm"
)

func (s *Service) taskLeaseTTL() time.Duration {
	if s != nil && s.TaskTimeout > 0 {
		return s.TaskTimeout
	}
	return 120 * time.Second
}

func (s *Service) tryClaimTask(ctx context.Context, taskID uuid.UUID, workerID string, lease time.Duration) (*CustomerMessageSyncTask, bool, error) {
	if s == nil || s.DB == nil {
		return nil, false, fmt.Errorf("customersync: no db")
	}
	now := time.Now().UTC()
	until := now.Add(lease)
	res := s.DB.WithContext(ctx).Model(&CustomerMessageSyncTask{}).
		Where(`id = ? AND status = ? AND (locked_by IS NULL OR locked_until < ?)`, taskID, StatusPending, now).
		Updates(map[string]any{
			"status":       StatusRunning,
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
	var task CustomerMessageSyncTask
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return nil, false, err
	}
	return &task, true, nil
}

func (s *Service) startLeaseRenewal(ctx context.Context, taskID uuid.UUID, workerID string, leaseTTL time.Duration) (stop func()) {
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
				_ = s.DB.WithContext(context.Background()).Model(&CustomerMessageSyncTask{}).
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

func (s *Service) handlePanic(parent context.Context, taskID uuid.UUID, workerID string, panicVal any) {
	if s == nil || s.DB == nil {
		return
	}
	ctx := parent
	if ctx == nil {
		ctx = context.Background()
	}
	var cur CustomerMessageSyncTask
	if err := s.DB.WithContext(ctx).First(&cur, "id = ?", taskID).Error; err != nil {
		return
	}
	if cur.Status != StatusRunning || cur.LockedBy == nil || *cur.LockedBy != workerID {
		return
	}
	msg := fmt.Sprintf("customer message sync worker panic: %v", panicVal)
	fin := time.Now().UTC()
	_ = s.DB.WithContext(ctx).Model(&CustomerMessageSyncTask{}).Where("id = ?", taskID).
		Updates(map[string]any{
			"status":        StatusFailed,
			"error_message": msg,
			"finished_at":   &fin,
			"locked_by":     nil,
			"locked_until":  nil,
			"updated_at":    fin,
		}).Error
	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: cur.CreatedBy,
			Action:      "customer.message_sync.failed",
			Resource:    "customer_message_sync_task",
			ResourceID:  taskID.String(),
			Status:      "failed",
			Message:     fmt.Sprintf("taskId=%s panic recovery", taskID.String()),
		})
	}
}

// RecoverLeaseExpired marks task failed for human retry.
func (s *Service) RecoverLeaseExpired(ctx context.Context, taskID uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("customersync: no db")
	}
	now := time.Now().UTC()
	var task CustomerMessageSyncTask
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return err
	}
	if task.Status != StatusRunning || task.LockedUntil == nil || !task.LockedUntil.Before(now) {
		return nil
	}
	fin := now
	_ = s.DB.WithContext(ctx).Model(&CustomerMessageSyncTask{}).Where("id = ?", taskID).
		Updates(map[string]any{
			"status":        StatusFailed,
			"error_message": "worker lease expired",
			"finished_at":   &fin,
			"locked_by":     nil,
			"locked_until":  nil,
			"updated_at":    fin,
		}).Error
	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: task.CreatedBy,
			Action:      "customer.message_sync.lease_expired",
			Resource:    "customer_message_sync_task",
			ResourceID:  taskID.String(),
			Status:      "failed",
			Message:     fmt.Sprintf("taskId=%s shopId=%s", taskID.String(), task.ShopID.String()),
		})
	}
	return nil
}

// RecoverLegacyRunning fails stuck rows without lease metadata.
func (s *Service) RecoverLegacyRunning(ctx context.Context, taskID uuid.UUID, legacyCutoff time.Time) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("customersync: no db")
	}
	var task CustomerMessageSyncTask
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
	fin := time.Now().UTC()
	_ = s.DB.WithContext(ctx).Model(&CustomerMessageSyncTask{}).Where("id = ?", taskID).
		Updates(map[string]any{
			"status":        StatusFailed,
			"error_message": "legacy running task recovered (no lease)",
			"finished_at":   &fin,
			"locked_by":     nil,
			"locked_until":  nil,
			"updated_at":    fin,
		}).Error
	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: task.CreatedBy,
			Action:      "customer.message_sync.lease_expired",
			Resource:    "customer_message_sync_task",
			ResourceID:  taskID.String(),
			Status:      "failed",
			Message:     "legacy customer_message_sync_task recovered",
		})
	}
	return nil
}
