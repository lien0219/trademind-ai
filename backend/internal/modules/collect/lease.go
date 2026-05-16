package collect

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *Service) collectLeaseTTL() time.Duration {
	sec := 600
	if s != nil && s.TaskLeaseTimeoutSeconds > 0 {
		sec = s.TaskLeaseTimeoutSeconds
	}
	colSec := 60
	if s != nil && s.CollectorTimeoutSeconds > 0 {
		colSec = s.CollectorTimeoutSeconds
	}
	if sec < colSec+60 {
		sec = colSec + 60
	}
	return time.Duration(sec) * time.Second
}

// tryClaimCollectTask atomically moves pending/retrying (due) to running with a lease.
func (s *Service) tryClaimCollectTask(ctx context.Context, taskID uuid.UUID, workerID string, lease time.Duration) (*CollectTask, bool) {
	if s == nil || s.DB == nil {
		return nil, false
	}
	now := time.Now().UTC()
	until := now.Add(lease)

	res := s.DB.WithContext(ctx).Model(&CollectTask{}).
		Where(`id = ? AND (status = ? OR (status = ? AND next_retry_at IS NULL)) AND (locked_by IS NULL OR locked_until < ?)`,
			taskID, StatusPending, StatusRetrying, now).
		Updates(map[string]interface{}{
			"status":            StatusRunning,
			"locked_by":         workerID,
			"locked_until":      &until,
			"lock_version":      gorm.Expr("lock_version + 1"),
			"started_at":        gorm.Expr("COALESCE(started_at, ?)", now),
			"error_message":     "",
			"finished_at":       nil,
			"retry_enqueued_at": nil,
			"updated_at":        now,
		})
	if res.Error != nil || res.RowsAffected == 0 {
		return nil, false
	}
	var task CollectTask
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return nil, false
	}
	return &task, true
}

func (s *Service) startCollectLeaseRenewal(ctx context.Context, taskID uuid.UUID, workerID string, leaseTTL time.Duration) (stop func()) {
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
				_ = s.DB.WithContext(context.Background()).Model(&CollectTask{}).
					Where("id = ? AND status = ? AND locked_by = ?", taskID, StatusRunning, workerID).
					Updates(map[string]interface{}{
						"locked_until": &until,
						"updated_at":   time.Now().UTC(),
					}).Error
			}
		}
	}()
	return cancel
}

// RecoverLeaseExpired is invoked by the task reaper when locked_until passes.
func (s *Service) RecoverLeaseExpired(ctx context.Context, taskID uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("collect: no db")
	}
	now := time.Now().UTC()
	var task CollectTask
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return err
	}
	if task.Status != StatusRunning || task.LockedUntil == nil || !task.LockedUntil.Before(now) {
		return nil
	}
	_ = s.DB.WithContext(ctx).Model(&CollectTask{}).Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"locked_by":    nil,
			"locked_until": nil,
			"updated_at":   now,
		}).Error
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return err
	}
	s.RecordTaskEvent(ctx, &task, TaskEventInput{
		EventType:    EventWorkerLeaseExpired,
		FromStatus:   StatusRunning,
		Message:      "worker lease expired",
		ErrorMessage: "worker lease expired",
		RetryCount:   task.RetryCount,
		MaxRetries:   s.effectiveMaxRetries(&task),
	})
	s.handleCollectJobError(ctx, &task, fmt.Errorf("worker lease expired"))
	return nil
}

// RecoverLegacyRunning handles historical running rows without lease metadata.
func (s *Service) RecoverLegacyRunning(ctx context.Context, taskID uuid.UUID, legacyCutoff time.Time) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("collect: no db")
	}
	var task CollectTask
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
	_ = s.DB.WithContext(ctx).Model(&CollectTask{}).Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"locked_by":    nil,
			"locked_until": nil,
			"updated_at":   now,
		}).Error
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return err
	}
	s.RecordTaskEvent(ctx, &task, TaskEventInput{
		EventType:    EventWorkerLeaseRecovered,
		FromStatus:   StatusRunning,
		Message:      "legacy running task recovered (no lease)",
		ErrorMessage: "legacy running task recovered",
		RetryCount:   task.RetryCount,
		MaxRetries:   s.effectiveMaxRetries(&task),
	})
	s.handleCollectJobError(ctx, &task, fmt.Errorf("legacy running task recovered"))
	return nil
}

func (s *Service) handleCollectPanic(parent context.Context, taskID uuid.UUID, workerID string, panicVal any) {
	if s == nil || s.DB == nil {
		return
	}
	ctx := parent
	if ctx == nil {
		ctx = context.Background()
	}
	var cur CollectTask
	if err := s.DB.WithContext(ctx).First(&cur, "id = ?", taskID).Error; err != nil {
		return
	}
	if cur.Status != StatusRunning || cur.LockedBy == nil || *cur.LockedBy != workerID {
		return
	}
	msg := truncateRunes(fmt.Sprintf("collect worker panic: %v", panicVal), 8000)
	s.failTask(ctx, &cur, StatusRunning, msg, map[string]any{"panic": true})
}

