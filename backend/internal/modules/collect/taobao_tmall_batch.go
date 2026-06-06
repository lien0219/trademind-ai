package collect

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/collectdomain"
)

var (
	ErrTaobaoTmallBatchLoginRequired  = errors.New("当前淘宝/天猫采集需要登录，请先打开淘宝/天猫采集浏览器完成登录后再开始批量采集")
	ErrTaobaoTmallBatchVerifyRequired = errors.New("当前淘宝/天猫页面需要安全验证，请在采集浏览器中完成验证后重试")
)

type taobaoTmallURLFilterResult struct {
	Valid   []string
	Skipped []string
}

func filterTaobaoTmallBatchURLs(urls []string) taobaoTmallURLFilterResult {
	seen := make(map[string]string, len(urls))
	var skipped []string
	for _, raw := range urls {
		u := strings.TrimSpace(raw)
		if u == "" {
			continue
		}
		if !looksLikeCollectURL(u) {
			skipped = append(skipped, u)
			continue
		}
		switch collectdomain.ClassifyTaobaoTmallURL(u) {
		case "product_detail":
			key := strings.ToLower(u)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = u
		default:
			skipped = append(skipped, u)
		}
	}
	valid := make([]string, 0, len(seen))
	for _, u := range seen {
		valid = append(valid, u)
	}
	return taobaoTmallURLFilterResult{Valid: valid, Skipped: skipped}
}

func (s *Service) batchMaxURLsForSource(ctx context.Context, source string) int {
	if isTaobaoTmallCollectSource(source) {
		return s.taobaoTmallBatchMaxItems(ctx)
	}
	return s.batchMaxURLs()
}

func (s *Service) ensureTaobaoTmallBatchReady(ctx context.Context, firstValidURL string) error {
	if s == nil || s.Client == nil {
		return fmt.Errorf("collect: collector client unavailable")
	}
	contextURL, settingsTestURL := s.ResolveTaobaoTmallAuthCheckInputs(ctx, firstValidURL)
	checkURL := strings.TrimSpace(contextURL)
	if checkURL == "" {
		checkURL = strings.TrimSpace(settingsTestURL)
	}
	out, err := s.Client.CheckTaobaoTmallLogin(ctx, checkURL, settingsTestURL)
	if err != nil {
		return fmt.Errorf("collector login check failed: %w", err)
	}
	if out == nil {
		return fmt.Errorf("collector login check returned empty")
	}
	st := strings.TrimSpace(out.Status)
	if st == "" && out.LoggedIn {
		st = "logged_in"
	}
	if out.NeedVerification || isTaobaoTmallAuthStatusVerifyRequired(st) {
		if s.OpLog != nil {
			_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
				Action:     "collect.taobao_tmall.batch.verify_blocked",
				Resource:   "collector_provider",
				ResourceID: "taobao_tmall",
				Status:     "failed",
				Message:    "verify_required before batch start",
			})
		}
		return ErrTaobaoTmallBatchVerifyRequired
	}
	if !out.LoggedIn && !isTaobaoTmallAuthStatusLoggedIn(st) {
		if s.OpLog != nil {
			_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
				Action:     "collect.taobao_tmall.batch.login_blocked",
				Resource:   "collector_provider",
				ResourceID: "taobao_tmall",
				Status:     "failed",
				Message:    "login_required before batch start",
			})
		}
		return ErrTaobaoTmallBatchLoginRequired
	}
	return nil
}

func collectorCodeFromTaskPayload(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	if v, ok := payload["collectorCode"].(string); ok {
		return strings.ToUpper(strings.TrimSpace(v))
	}
	if v, ok := payload["collectorErrorCode"].(string); ok {
		return strings.ToUpper(strings.TrimSpace(v))
	}
	return ""
}

func (s *Service) maybePauseTaobaoTmallBatchOnAuthFailure(ctx context.Context, task *CollectTask, payload map[string]any) {
	if s == nil || s.DB == nil || task == nil || task.BatchID == nil {
		return
	}
	if !isTaobaoTmallCollectSource(task.Source) {
		return
	}
	code := collectorCodeFromTaskPayload(payload)
	if code == "" {
		code = normalizeCollectorErrorCode("", task.ErrorMessage)
	}
	switch code {
	case "LOGIN_REQUIRED":
		if !s.taobaoTmallBatchPauseOnLogin(ctx) {
			return
		}
		s.cancelRemainingBatchTasks(ctx, *task.BatchID, "batch paused: login required")
		if s.OpLog != nil {
			_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
				AdminUserID: task.CreatedBy,
				Action:      "collect.taobao_tmall.batch.pause_login",
				Resource:    "collect_batch",
				ResourceID:  task.BatchID.String(),
				Status:      "success",
				Message:     fmt.Sprintf("triggerTaskId=%s", task.ID.String()),
			})
		}
	case "VERIFY_REQUIRED", "PAGE_BLOCKED_OR_VERIFY_REQUIRED":
		if !s.taobaoTmallBatchPauseOnVerify(ctx) {
			return
		}
		s.cancelRemainingBatchTasks(ctx, *task.BatchID, "batch paused: verification required")
		if s.OpLog != nil {
			_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
				AdminUserID: task.CreatedBy,
				Action:      "collect.taobao_tmall.batch.pause_verify",
				Resource:    "collect_batch",
				ResourceID:  task.BatchID.String(),
				Status:      "success",
				Message:     fmt.Sprintf("triggerTaskId=%s", task.ID.String()),
			})
		}
	}
}

func (s *Service) cancelRemainingBatchTasks(ctx context.Context, batchID uuid.UUID, reason string) {
	if s == nil || s.DB == nil {
		return
	}
	now := time.Now().UTC()
	msg := truncateRunes(strings.TrimSpace(reason), 500)
	var pending []CollectTask
	if err := s.DB.WithContext(ctx).
		Where("batch_id = ? AND status IN ?", batchID, []string{StatusPending, StatusRetrying}).
		Find(&pending).Error; err != nil {
		return
	}
	for i := range pending {
		t := pending[i]
		up := s.DB.WithContext(ctx).Model(&CollectTask{}).
			Where("id = ? AND status IN ?", t.ID, []string{StatusPending, StatusRetrying}).
			Updates(map[string]interface{}{
				"status":            StatusCancelled,
				"error_message":     msg,
				"finished_at":       &now,
				"next_retry_at":     nil,
				"retry_enqueued_at": nil,
				"updated_at":        now,
			})
		if up.Error != nil || up.RowsAffected == 0 {
			continue
		}
		s.RecordTaskEvent(ctx, &t, TaskEventInput{
			EventType:    EventTaskCancelled,
			FromStatus:   t.Status,
			ToStatus:     StatusCancelled,
			Message:      msg,
			ErrorMessage: msg,
		})
	}
	s.reconcileCollectBatch(ctx, &batchID)
}

func (s *Service) maybeLogTaobaoTmallBatchTerminal(ctx context.Context, batchID uuid.UUID, prevStatus, newStatus string) {
	if s == nil || s.OpLog == nil {
		return
	}
	if prevStatus == newStatus {
		return
	}
	if newStatus == BatchStatusRunning {
		return
	}
	var batch CollectBatch
	if err := s.DB.WithContext(ctx).First(&batch, "id = ?", batchID).Error; err != nil {
		return
	}
	if !isTaobaoTmallCollectSource(batch.Source) {
		return
	}
	_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
		AdminUserID: batch.CreatedBy,
		Action:      "collect.taobao_tmall.batch.complete",
		Resource:    "collect_batch",
		ResourceID:  batchID.String(),
		Status:      newStatus,
		Message: fmt.Sprintf(
			"total=%d success=%d failed=%d cancelled=%d status=%s",
			batch.TotalCount, batch.SuccessCount, batch.FailedCount, batch.CancelledCount, newStatus,
		),
	})
}

func (s *Service) reconcileCollectBatchWithTerminalLog(ctx context.Context, batchID *uuid.UUID) {
	if s == nil || s.DB == nil || batchID == nil {
		return
	}
	var before CollectBatch
	_ = s.DB.WithContext(ctx).First(&before, "id = ?", *batchID).Error
	prev := strings.TrimSpace(before.Status)
	s.reconcileCollectBatch(ctx, batchID)
	var after CollectBatch
	if err := s.DB.WithContext(ctx).First(&after, "id = ?", *batchID).Error; err != nil {
		return
	}
	s.maybeLogTaobaoTmallBatchTerminal(ctx, *batchID, prev, strings.TrimSpace(after.Status))
}

func (s *Service) logTaobaoTmallBatchSkippedURLs(ctx context.Context, adminID *uuid.UUID, skipped []string) {
	if s == nil || s.OpLog == nil || len(skipped) == 0 {
		return
	}
	_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
		AdminUserID: adminID,
		Action:      "collect.taobao_tmall.batch.skip_invalid_url",
		Resource:    "collect_batch",
		ResourceID:  "taobao_tmall",
		Status:      "success",
		Message:     fmt.Sprintf("skipped=%d sample=%s", len(skipped), truncateRunes(strings.Join(skipped, "; "), 500)),
	})
}
