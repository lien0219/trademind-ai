package collect

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
)

func (s *Service) batchMaxURLs() int {
	if s == nil || s.BatchMaxURLs <= 0 {
		return 50
	}
	return s.BatchMaxURLs
}

// normalizeDedupeURLs trims, drops empties, de-dupes case-insensitively, caps at max.
func normalizeDedupeURLs(urls []string, max int) ([]string, error) {
	seen := make(map[string]string, len(urls))
	for _, raw := range urls {
		u := strings.TrimSpace(raw)
		if u == "" {
			continue
		}
		if !looksLikeCollectURL(u) {
			return nil, ErrCollectURLsNeedHTTPScheme
		}
		key := strings.ToLower(u)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = u
	}
	out := make([]string, 0, len(seen))
	for _, u := range seen {
		out = append(out, u)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("urls is empty")
	}
	if len(out) > max {
		return nil, fmt.Errorf("too many urls (max %d)", max)
	}
	return out, nil
}

func (s *Service) reconcileCollectBatchTx(ctx context.Context, tx *gorm.DB, batchID uuid.UUID) error {
	if s == nil || tx == nil {
		return fmt.Errorf("collect: reconcile: no tx")
	}
	var batch CollectBatch
	if err := tx.WithContext(ctx).First(&batch, "id = ?", batchID).Error; err != nil {
		return err
	}

	var rows []struct {
		Status string
		N      int64
	}
	if err := tx.WithContext(ctx).Model(&CollectTask{}).
		Select("status, COUNT(*) AS n").
		Where("batch_id = ?", batchID).
		Group("status").
		Scan(&rows).Error; err != nil {
		return err
	}

	var p, retr, run, succ, fail, canc int64
	for _, r := range rows {
		switch strings.TrimSpace(r.Status) {
		case StatusPending:
			p += r.N
		case StatusRetrying:
			retr += r.N
		case StatusRunning:
			run += r.N
		case StatusSuccess:
			succ += r.N
		case StatusFailed:
			fail += r.N
		case StatusCancelled:
			canc += r.N
		}
	}

	pendingShown := int(p + retr)
	running := int(run)

	st := deriveBatchAggregateStatus(batch.TotalCount, pendingShown, running, int(succ), int(fail), int(canc))

	now := time.Now().UTC()
	updates := map[string]interface{}{
		"pending_count":   pendingShown,
		"running_count":   running,
		"success_count":   int(succ),
		"failed_count":    int(fail),
		"cancelled_count": int(canc),
		"status":          st,
		"updated_at":      now,
	}
	if st == BatchStatusRunning {
		updates["finished_at"] = nil
	} else {
		if batch.FinishedAt != nil {
			updates["finished_at"] = batch.FinishedAt
		} else {
			t := now
			updates["finished_at"] = &t
		}
	}

	return tx.WithContext(ctx).Model(&CollectBatch{}).
		Where("id = ?", batchID).
		Updates(updates).Error
}

func (s *Service) reconcileCollectBatch(ctx context.Context, batchID *uuid.UUID) {
	if s == nil || s.DB == nil || batchID == nil {
		return
	}
	_ = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return s.reconcileCollectBatchTx(ctx, tx, *batchID)
	})
}

func deriveBatchAggregateStatus(total, pendingCombined, running, success, failed, cancelled int) string {
	inFlight := pendingCombined > 0 || running > 0
	if inFlight {
		return BatchStatusRunning
	}
	t := total
	switch {
	case t <= 0:
		return BatchStatusSuccess
	case success == t:
		return BatchStatusSuccess
	case failed == t:
		return BatchStatusFailed
	case cancelled == t:
		return BatchStatusCancelled
	default:
		return BatchStatusPartialSuccess
	}
}

func (s *Service) rollbackBatchCreates(ctx context.Context, batchID uuid.UUID) {
	if s == nil || s.DB == nil {
		return
	}
	_ = s.DB.WithContext(ctx).Where("batch_id = ?", batchID).Delete(&CollectTaskEvent{}).Error
	_ = s.DB.WithContext(ctx).Where("batch_id = ?", batchID).Unscoped().Delete(&CollectTask{}).Error
	_ = s.DB.WithContext(ctx).Where("id = ?", batchID).Unscoped().Delete(&CollectBatch{}).Error
}

// CreateBatchAsync creates a batch, tasks, aggregates, enqueues — rolls back DB on enqueue failure.
func (s *Service) CreateBatchAsync(c *gin.Context, body CreateBatchBody, adminID *uuid.UUID) (CreateBatchResult, error) {
	var zero CreateBatchResult
	if s == nil || s.DB == nil {
		return zero, fmt.Errorf("collect: no db")
	}
	if !s.QueueEnabled {
		return zero, ErrCollectQueueDisabled
	}
	ctx := c.Request.Context()
	if err := s.redisPing(ctx); err != nil {
		return zero, err
	}

	source := strings.TrimSpace(body.Source)
	if source == "" {
		return zero, fmt.Errorf("source is required")
	}
	if err := s.ValidateSourceForCollect(ctx, source, true); err != nil {
		return zero, err
	}
	if isPinduoduoCollectSource(source) && !s.pinduoduoBatchEnabled(ctx) {
		return zero, fmt.Errorf("pinduoduo batch collect is disabled in settings")
	}
	urls, err := normalizeDedupeURLs(body.URLs, s.batchMaxURLs())
	if err != nil {
		return zero, err
	}

	var batch CollectBatch
	var taskIDs []uuid.UUID

	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		batch = CollectBatch{
			Source:         source,
			TotalCount:     len(urls),
			PendingCount:   0,
			RunningCount:   0,
			SuccessCount:   0,
			FailedCount:    0,
			CancelledCount: 0,
			Status:         BatchStatusRunning,
			CreatedBy:      adminID,
			FinishedAt:     nil,
		}
		if err := tx.Create(&batch).Error; err != nil {
			return err
		}
		bid := batch.ID
		taskIDs = make([]uuid.UUID, 0, len(urls))
		batchPolicy := s.batchPolicyForSource(ctx, source)
		maxRetries := s.defaultMaxRetriesForNewTask()
		if (strings.EqualFold(strings.TrimSpace(source), "1688") || isPinduoduoCollectSource(source)) &&
			batchPolicy.MaxRetries > 0 {
			maxRetries = batchPolicy.MaxRetries
		}
		for _, u := range urls {
			task := CollectTask{
				BatchID:    &bid,
				Source:     source,
				SourceURL:  u,
				Status:     StatusPending,
				MaxRetries: maxRetries,
				CreatedBy:  adminID,
			}
			if err := tx.Create(&task).Error; err != nil {
				return err
			}
			taskIDs = append(taskIDs, task.ID)
			s.RecordTaskEventWithDB(tx.WithContext(ctx), ctx, &task, TaskEventInput{
				EventType:  EventTaskCreated,
				ToStatus:   StatusPending,
				Message:    "bulk collect task persisted",
				MaxRetries: task.MaxRetries,
			})
		}
		return s.reconcileCollectBatchTx(ctx, tx, bid)
	})
	if err != nil {
		return zero, err
	}

	reqID := requestIDFromGin(c)
	for _, tid := range taskIDs {
		var t CollectTask
		if err := s.DB.WithContext(ctx).First(&t, "id = ?", tid).Error; err != nil {
			s.rollbackBatchCreates(ctx, batch.ID)
			return zero, fmt.Errorf("collect: load created task: %w", err)
		}
		if err := s.enqueueTask(ctx, t.ID, t.Source, t.SourceURL, t.CreatedBy, reqID); err != nil {
			s.rollbackBatchCreates(ctx, batch.ID)
			return zero, err
		}
		s.RecordTaskEvent(ctx, &t, TaskEventInput{
			EventType:  EventTaskEnqueued,
			ToStatus:   StatusPending,
			Message:    "queued to Redis",
			MaxRetries: t.MaxRetries,
			PayloadMap: func() map[string]any {
				if strings.TrimSpace(reqID) != "" {
					return map[string]any{"requestId": reqID}
				}
				return nil
			}(),
		})
	}

	if s.OpLog != nil {
		action := "collect.batch.create"
		if isPinduoduoCollectSource(source) {
			action = "collect.pinduoduo.batch"
		}
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			Action:     action,
			Resource:   "collect_batch",
			ResourceID: batch.ID.String(),
			Status:     "success",
			Message:    fmt.Sprintf("source=%s task_count=%d", source, len(taskIDs)),
		})
	}

	var fresh CollectBatch
	_ = s.DB.WithContext(ctx).First(&fresh, "id = ?", batch.ID)
	return CreateBatchResult{
		Batch:     batchToDTO(&fresh),
		TaskCount: len(taskIDs),
	}, nil
}

func (s *Service) redisPing(ctx context.Context) error {
	if s == nil || s.Redis == nil || s.Redis.Client == nil {
		return ErrRedisQueueUnavailable
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := s.Redis.Ping(ctx).Err(); err != nil {
		return ErrRedisQueueUnavailable
	}
	return nil
}

// ListBatches paginates batches with filters.
func (s *Service) ListBatches(c *gin.Context, q BatchListQuery) (*BatchListResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("collect: no db")
	}
	page, ps := clampCollectPage(q.Page, q.PageSize)
	tx := s.DB.WithContext(c.Request.Context()).Model(&CollectBatch{})

	if v := strings.TrimSpace(q.Status); v != "" {
		tx = tx.Where("status = ?", v)
	}
	if v := strings.TrimSpace(q.Source); v != "" {
		tx = tx.Where("source = ?", v)
	}
	if v := strings.TrimSpace(q.StartRFC); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return nil, fmt.Errorf("invalid start time (RFC3339)")
		}
		tx = tx.Where("created_at >= ?", t.UTC())
	}
	if v := strings.TrimSpace(q.EndRFC); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return nil, fmt.Errorf("invalid end time (RFC3339)")
		}
		tx = tx.Where("created_at <= ?", t.UTC())
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}
	offset := (page - 1) * ps
	var rows []CollectBatch
	if err := tx.Order("created_at DESC").Offset(offset).Limit(ps).Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]BatchDTO, 0, len(rows))
	for i := range rows {
		items = append(items, batchToDTO(&rows[i]))
	}
	pages := int(total) / ps
	if int(total)%ps != 0 {
		pages++
	}
	if pages == 0 && total > 0 {
		pages = 1
	}
	return &BatchListResult{
		Items:      items,
		Total:      total,
		Page:       page,
		PageSize:   ps,
		TotalPages: pages,
	}, nil
}

// GetBatchDTO returns one batch with derived failure stats.
func (s *Service) GetBatchDTO(c *gin.Context, id uuid.UUID) (BatchDTO, error) {
	var zero BatchDTO
	if s == nil || s.DB == nil {
		return zero, fmt.Errorf("collect: no db")
	}
	var b CollectBatch
	if err := s.DB.WithContext(c.Request.Context()).First(&b, "id = ?", id).Error; err != nil {
		return zero, err
	}
	stats := s.computeBatchStats(c.Request.Context(), id)
	return batchToDetailDTO(&b, stats), nil
}

// ListBatchTasks paginates tasks in a batch.
func (s *Service) ListBatchTasks(c *gin.Context, batchID uuid.UUID, q ListQuery) (*ListResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("collect: no db")
	}
	var exists CollectBatch
	if err := s.DB.WithContext(c.Request.Context()).First(&exists, "id = ?", batchID).Error; err != nil {
		return nil, err
	}
	page, ps := clampCollectPage(q.Page, q.PageSize)
	tx := s.DB.WithContext(c.Request.Context()).Model(&CollectTask{}).Where("batch_id = ?", batchID)
	if v := strings.TrimSpace(q.Status); v != "" {
		tx = tx.Where("status = ?", v)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}
	offset := (page - 1) * ps
	var rows []CollectTask
	if err := tx.Order("created_at ASC").Offset(offset).Limit(ps).Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]TaskDTO, 0, len(rows))
	for i := range rows {
		items = append(items, s.enrichTaskDTO(c.Request.Context(), &rows[i]))
	}
	pages := int(total) / ps
	if int(total)%ps != 0 {
		pages++
	}
	if pages == 0 && total > 0 {
		pages = 1
	}
	return &ListResult{
		Items:      items,
		Total:      total,
		Page:       page,
		PageSize:   ps,
		TotalPages: pages,
	}, nil
}

// RetryFailedBatchTasks re-queues all failed tasks in a batch (single aggregate log).
func (s *Service) RetryFailedBatchTasks(c *gin.Context, batchID uuid.UUID, adminID *uuid.UUID) (RetryBatchFailedResult, error) {
	var zero RetryBatchFailedResult
	if s == nil || s.DB == nil {
		return zero, fmt.Errorf("collect: no db")
	}
	if !s.QueueEnabled {
		return zero, ErrCollectQueueDisabled
	}
	ctx := c.Request.Context()

	var batch CollectBatch
	if err := s.DB.WithContext(ctx).First(&batch, "id = ?", batchID).Error; err != nil {
		return zero, err
	}
	if err := s.redisPing(ctx); err != nil {
		return zero, err
	}

	reqID := requestIDFromGin(c)
	var failed []CollectTask
	if err := s.DB.WithContext(ctx).
		Where("batch_id = ? AND status = ?", batchID, StatusFailed).
		Order("created_at ASC").
		Find(&failed).Error; err != nil {
		return zero, err
	}
	if len(failed) == 0 {
		return RetryBatchFailedResult{Retried: 0}, nil
	}

	retried := 0
	for i := range failed {
		task := failed[i]
		retryAt := time.Now().UTC()
		up := s.DB.WithContext(ctx).Model(&CollectTask{}).
			Where("id = ? AND status = ?", task.ID, StatusFailed).
			Updates(map[string]interface{}{
				"status":            StatusRetrying,
				"error_message":     "",
				"finished_at":       nil,
				"result_product_id": nil,
				"raw_result":        datatypes.JSON(nil),
				"retry_count":       0,
				"next_retry_at":     nil,
				"retry_enqueued_at": nil,
				"updated_at":        retryAt,
			})
		if up.Error != nil {
			return zero, up.Error
		}
		if up.RowsAffected == 0 {
			continue
		}
		if err := s.enqueueTask(ctx, task.ID, task.Source, task.SourceURL, task.CreatedBy, reqID); err != nil {
			fin := time.Now().UTC()
			_ = s.DB.WithContext(ctx).Model(&CollectTask{}).
				Where("id = ?", task.ID).
				Updates(map[string]interface{}{
					"status":        StatusFailed,
					"error_message": ErrRedisQueueUnavailable.Error(),
					"finished_at":   &fin,
					"updated_at":    fin,
				}).Error
			var bumped CollectTask
			if er := s.DB.WithContext(ctx).First(&bumped, "id = ?", task.ID).Error; er == nil {
				s.RecordTaskEvent(ctx, &bumped, TaskEventInput{
					EventType:    EventTaskFailed,
					FromStatus:   StatusRetrying,
					ToStatus:     StatusFailed,
					Message:      "enqueue after batch retry-failed failed",
					ErrorMessage: ErrRedisQueueUnavailable.Error(),
				})
			}
			return zero, err
		}
		var fresh CollectTask
		if err := s.DB.WithContext(ctx).First(&fresh, "id = ?", task.ID).Error; err != nil {
			return zero, err
		}
		s.RecordTaskEvent(ctx, &fresh, TaskEventInput{
			EventType:  EventTaskManualRetry,
			FromStatus: StatusFailed,
			ToStatus:   StatusRetrying,
			Message:    "batch retry-failed re-queued",
			RetryCount: fresh.RetryCount,
			MaxRetries: fresh.MaxRetries,
		})
		retried++
	}

	s.reconcileCollectBatch(ctx, &batchID)

	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "collect.batch.retry_failed",
			Resource:    "collect_batch",
			ResourceID:  batchID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("retried=%d", retried),
		})
	}

	return RetryBatchFailedResult{Retried: retried}, nil
}
