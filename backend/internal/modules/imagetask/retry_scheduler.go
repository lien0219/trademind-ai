package imagetask

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
)

func imageRetryDelaySeconds(retryCount, baseSec, capSec int) int {
	if retryCount < 1 {
		retryCount = 1
	}
	if baseSec < 1 {
		baseSec = 1
	}
	if capSec < baseSec {
		capSec = baseSec
	}
	mul := 1 << uint(retryCount-1)
	d := baseSec * mul
	if d > capSec {
		return capSec
	}
	return d
}

// StartImageRetryScheduler periodically moves due retrying tasks back onto the Redis list.
func StartImageRetryScheduler(ctx context.Context, wg *sync.WaitGroup, log *slog.Logger, svc *Service, interval time.Duration) {
	if svc == nil || !svc.QueueEnabled || !svc.AutoRetryEnabled || svc.Redis == nil || svc.Redis.Client == nil {
		return
	}
	if interval <= 0 {
		interval = 5 * time.Second
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		tick := time.NewTicker(interval)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				svc.flushImageDueRetries(ctx, log)
			}
		}
	}()
}

func (s *Service) flushImageDueRetries(ctx context.Context, log *slog.Logger) {
	if s == nil || s.DB == nil || !s.AutoRetryEnabled || !s.QueueEnabled {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	now := time.Now().UTC()
	var due []ImageTask
	if err := s.DB.WithContext(ctx).
		Where("status = ? AND next_retry_at IS NOT NULL AND next_retry_at <= ?", StatusRetrying, now).
		Order("next_retry_at ASC").
		Limit(50).
		Find(&due).Error; err != nil {
		return
	}
	for i := range due {
		s.tryEnqueueScheduledImageRetry(ctx, log, &due[i], now)
	}
}

func (s *Service) tryEnqueueScheduledImageRetry(ctx context.Context, log *slog.Logger, task *ImageTask, now time.Time) {
	if s == nil || task == nil || task.NextRetryAt == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if task.NextRetryAt.After(now) {
		return
	}
	tid := task.ID
	savedNext := *task.NextRetryAt
	mark := now.UTC()

	res := s.DB.WithContext(ctx).Model(&ImageTask{}).
		Where("id = ? AND status = ? AND next_retry_at IS NOT NULL AND next_retry_at <= ?", tid, StatusRetrying, now).
		Updates(map[string]any{
			"next_retry_at":     nil,
			"retry_enqueued_at": &mark,
		})
	if res.Error != nil || res.RowsAffected == 0 {
		return
	}

	if err := s.enqueueTask(ctx, tid, task.TaskType, task.Provider, task.CreatedBy, "auto-retry-scheduler"); err != nil {
		if log != nil {
			log.Warn("image_auto_retry_enqueue_failed", "taskId", tid.String(), "error", err)
		}
		_ = s.DB.WithContext(ctx).Model(&ImageTask{}).
			Where("id = ?", tid).
			Updates(map[string]any{
				"next_retry_at":     &savedNext,
				"retry_enqueued_at": nil,
			}).Error
		return
	}

	if s.OpLog != nil {
		msgLog := fmt.Sprintf("taskId=%s taskType=%s provider=%s retryCount=%d nextRetryAt=%s",
			tid.String(),
			task.TaskType,
			task.Provider,
			task.RetryCount,
			savedNext.Format(time.RFC3339),
		)
		var admin *uuid.UUID
		if task.CreatedBy != nil {
			admin = task.CreatedBy
		}
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: admin,
			Action:      "image.task.auto_retry_enqueued",
			Resource:    "image_task",
			ResourceID:  tid.String(),
			Status:      "success",
			Message:     truncateRunes(msgLog, 2000),
		})
	}
}
