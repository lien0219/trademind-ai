package collect

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
)

func (s *Service) effectiveMaxRetries(task *CollectTask) int {
	if task != nil && task.MaxRetries > 0 {
		return task.MaxRetries
	}
	if s != nil && s.MaxAutoRetries > 0 {
		return s.MaxAutoRetries
	}
	return 3
}

func (s *Service) defaultMaxRetriesForNewTask() int {
	return s.effectiveMaxRetries(nil)
}

func (s *Service) effectiveRetryBaseSec() int {
	if s != nil && s.RetryBaseDelaySec > 0 {
		return s.RetryBaseDelaySec
	}
	return 30
}

func (s *Service) effectiveRetryMaxSec() int {
	if s != nil && s.RetryMaxDelaySec > 0 {
		return s.RetryMaxDelaySec
	}
	return 600
}

// collectRetryDelaySeconds returns wait before enqueueing the attempt numbered retryCount (1-based after a failure).
func collectRetryDelaySeconds(retryCount, baseSec, capSec int) int {
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

func collectErrNonRetryable(err error) bool {
	if err == nil {
		return false
	}
	var rej *CollectorRejectedError
	if errors.As(err, &rej) && rej != nil {
		switch strings.ToUpper(strings.TrimSpace(rej.Code)) {
		case "INVALID_URL", "INVALID_REQUEST", "PROVIDER_NOT_FOUND":
			return true
		default:
			return false
		}
	}
	return false
}

func collectorRejectExtras(err error) map[string]any {
	if err == nil {
		return nil
	}
	var rej *CollectorRejectedError
	if errors.As(err, &rej) && rej != nil {
		code := strings.TrimSpace(rej.Code)
		if code != "" {
			return map[string]any{"collectorCode": code}
		}
	}
	return nil
}

func (s *Service) scheduleAutoRetry(ctx context.Context, task *CollectTask, msg string, extras map[string]any) {
	if s == nil || s.DB == nil || task == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	now := time.Now().UTC()
	newRC := task.RetryCount + 1
	delaySec := collectRetryDelaySeconds(newRC, s.effectiveRetryBaseSec(), s.effectiveRetryMaxSec())
	next := now.Add(time.Duration(delaySec) * time.Second)
	tid := task.ID
	maxR := s.effectiveMaxRetries(task)

	payload := map[string]any{"nextDelaySeconds": delaySec}
	for k, v := range extras {
		payload[k] = v
	}

	_ = s.DB.WithContext(ctx).Model(&CollectTask{}).
		Where("id = ?", tid).
		Updates(map[string]interface{}{
			"status":            StatusRetrying,
			"retry_count":       newRC,
			"next_retry_at":     &next,
			"error_message":     truncateRunes(strings.TrimSpace(msg), 8000),
			"finished_at":       nil,
			"retry_enqueued_at": nil,
			"updated_at":        now,
		}).Error

	if task.BatchID != nil {
		s.reconcileCollectBatch(ctx, task.BatchID)
	}

	var cur CollectTask
	if err := s.DB.WithContext(ctx).First(&cur, "id = ?", tid).Error; err == nil {
		s.RecordTaskEvent(ctx, &cur, TaskEventInput{
			EventType:    EventTaskAutoRetryScheduled,
			FromStatus:   StatusRunning,
			ToStatus:     StatusRetrying,
			Message:      "scheduled automatic retry backoff",
			ErrorMessage: truncateRunes(strings.TrimSpace(msg), 8000),
			RetryCount:   cur.RetryCount,
			MaxRetries:   maxR,
			NextRetryAt:  cur.NextRetryAt,
			PayloadMap:   payload,
		})
	}

	if s.OpLog != nil {
		msgLog := fmt.Sprintf("taskId=%s retryCount=%d nextRetryAt=%s", tid.String(), newRC, next.Format(time.RFC3339))
		if task.BatchID != nil {
			msgLog += fmt.Sprintf(" batchId=%s", task.BatchID.String())
		}
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: task.CreatedBy,
			Action:      "collect.task.auto_retry_scheduled",
			Resource:    "collect_task",
			ResourceID:  tid.String(),
			Status:      "success",
			Message:     truncateRunes(msgLog, 2000),
		})
	}
}

func (s *Service) handleCollectJobError(ctx context.Context, task *CollectTask, jobErr error) {
	if s == nil || task == nil || jobErr == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	msg := jobErr.Error()
	var rej *CollectorRejectedError
	if errors.As(jobErr, &rej) && rej != nil && strings.TrimSpace(rej.Message) != "" {
		msg = rej.Message
	}
	msg = truncateRunes(strings.TrimSpace(msg), 8000)

	if !s.AutoRetryEnabled || collectErrNonRetryable(jobErr) {
		s.failTask(ctx, task, StatusRunning, msg, collectorRejectExtras(jobErr))
		return
	}

	maxR := s.effectiveMaxRetries(task)
	extras := collectorRejectExtras(jobErr)
	if task.RetryCount >= maxR {
		s.failTaskRetryExhausted(ctx, task, msg, extras)
		return
	}
	s.scheduleAutoRetry(ctx, task, msg, extras)
}

// StartRetryScheduler periodically moves due retrying tasks back onto the Redis list.
func StartRetryScheduler(ctx context.Context, wg *sync.WaitGroup, log *slog.Logger, svc *Service, interval time.Duration) {
	if svc == nil || !svc.QueueEnabled || svc.Redis == nil || svc.Redis.Client == nil {
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
				svc.flushDueRetries(ctx, log)
			}
		}
	}()
}

func (s *Service) flushDueRetries(ctx context.Context, log *slog.Logger) {
	if s == nil || s.DB == nil || !s.AutoRetryEnabled {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	now := time.Now().UTC()
	var due []CollectTask
	if err := s.DB.WithContext(ctx).
		Where("status = ? AND next_retry_at IS NOT NULL AND next_retry_at <= ?", StatusRetrying, now).
		Order("next_retry_at ASC").
		Limit(50).
		Find(&due).Error; err != nil {
		return
	}
	for i := range due {
		s.tryEnqueueScheduledRetry(ctx, log, &due[i], now)
	}
}

func (s *Service) tryEnqueueScheduledRetry(ctx context.Context, log *slog.Logger, task *CollectTask, now time.Time) {
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
	mark := now.UTC()

	res := s.DB.WithContext(ctx).Model(&CollectTask{}).
		Where("id = ? AND status = ? AND next_retry_at IS NOT NULL AND next_retry_at <= ?", tid, StatusRetrying, now).
		Updates(map[string]interface{}{
			"next_retry_at":     nil,
			"retry_enqueued_at": &mark,
			"updated_at":        mark,
		})
	if res.Error != nil || res.RowsAffected == 0 {
		return
	}

	if err := s.enqueueTask(ctx, tid, task.Source, task.SourceURL, task.CreatedBy, "auto-retry-scheduler"); err != nil {
		if log != nil {
			log.Warn("collect_auto_retry_enqueue_failed", "taskId", tid.String(), "error", err)
		}
		due := *task.NextRetryAt
		_ = s.DB.WithContext(ctx).Model(&CollectTask{}).
			Where("id = ?", tid).
			Updates(map[string]interface{}{
				"next_retry_at":     &due,
				"retry_enqueued_at": nil,
				"updated_at":        time.Now().UTC(),
			}).Error
		return
	}

	var cur CollectTask
	if err := s.DB.WithContext(ctx).First(&cur, "id = ?", tid).Error; err == nil {
		s.RecordTaskEvent(ctx, &cur, TaskEventInput{
			EventType:  EventTaskAutoRetryEnqueued,
			FromStatus: StatusRetrying,
			ToStatus:   StatusRetrying,
			Message:    "due automatic retry pushed to Redis",
			RetryCount: cur.RetryCount,
			MaxRetries: s.effectiveMaxRetries(&cur),
			PayloadMap: map[string]any{"requestId": "auto-retry-scheduler"},
		})
	}

	if s.OpLog != nil {
		msgLog := fmt.Sprintf("taskId=%s retryCount=%d", tid.String(), task.RetryCount)
		if task.BatchID != nil {
			msgLog += fmt.Sprintf(" batchId=%s", task.BatchID.String())
		}
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: task.CreatedBy,
			Action:      "collect.task.auto_retry_enqueued",
			Resource:    "collect_task",
			ResourceID:  tid.String(),
			Status:      "success",
			Message:     truncateRunes(msgLog, 2000),
		})
	}
}
