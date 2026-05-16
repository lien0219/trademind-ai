package collect

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/rdb"
)

// Service orchestrates collect tasks and persists results via product drafts.
type Service struct {
	DB                      *gorm.DB
	Products                *product.Service
	OpLog                   *operationlog.Service
	Client                  *CollectorClient
	Redis                   *rdb.Client
	QueueName               string
	QueueEnabled            bool
	BatchMaxURLs            int
	CollectorTimeoutSeconds int

	AutoRetryEnabled  bool
	MaxAutoRetries    int
	RetryBaseDelaySec int
	RetryMaxDelaySec  int
}

func clampCollectPage(page, ps int) (int, int) {
	if page < 1 {
		page = 1
	}
	if ps < 1 {
		ps = 20
	}
	if ps > 100 {
		ps = 100
	}
	return page, ps
}

func truncateRunes(s string, max int) string {
	if max <= 0 || s == "" {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max])
}

type normalizedProduct struct {
	Source            string            `json:"source"`
	SourceURL         string            `json:"sourceUrl"`
	Title             string            `json:"title"`
	Currency          string            `json:"currency"`
	MainImages        []string          `json:"mainImages"`
	DescriptionImages []string          `json:"descriptionImages"`
	Attributes        json.RawMessage   `json:"attributes"`
	SKUs              []json.RawMessage `json:"skus"`
	Raw               json.RawMessage   `json:"raw"`
}

func parseNormalized(b json.RawMessage) (*normalizedProduct, error) {
	var n normalizedProduct
	if err := json.Unmarshal(b, &n); err != nil {
		return nil, err
	}
	return &n, nil
}

func (n *normalizedProduct) importParams(fullJSON json.RawMessage) product.ImportDraftParams {
	if n == nil {
		return product.ImportDraftParams{FullNormalizedJSON: fullJSON}
	}
	var skus []product.ImportSKUParams
	for _, raw := range n.SKUs {
		line, err := product.BuildImportSKU(raw)
		if err != nil {
			continue
		}
		skus = append(skus, line)
	}
	return product.ImportDraftParams{
		Source:             strings.TrimSpace(n.Source),
		SourceURL:          strings.TrimSpace(n.SourceURL),
		Title:              strings.TrimSpace(n.Title),
		Currency:           strings.TrimSpace(n.Currency),
		MainImages:         n.MainImages,
		DescriptionImages:  n.DescriptionImages,
		SKUs:               skus,
		FullNormalizedJSON: fullJSON,
	}
}

func (s *Service) failTask(ctx context.Context, task *CollectTask, fromStatus, msg string, payload map[string]any) {
	if s == nil || s.DB == nil || task == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	fs := strings.TrimSpace(fromStatus)
	if fs == "" {
		fs = StatusRunning
	}
	msg = truncateRunes(strings.TrimSpace(msg), 8000)
	fin := time.Now().UTC()
	tid := task.ID
	_ = s.DB.WithContext(ctx).Model(&CollectTask{}).
		Where("id = ?", tid).
		Updates(map[string]interface{}{
			"status":            StatusFailed,
			"error_message":     msg,
			"finished_at":       &fin,
			"next_retry_at":     nil,
			"retry_enqueued_at": nil,
			"updated_at":        fin,
		}).Error

	s.RecordTaskEvent(ctx, task, TaskEventInput{
		EventType:    EventTaskFailed,
		FromStatus:   fs,
		ToStatus:     StatusFailed,
		Message:      "collect job failed",
		ErrorMessage: msg,
		PayloadMap:   payload,
		MaxRetries:   s.effectiveMaxRetries(task),
		RetryCount:   task.RetryCount,
	})

	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: task.CreatedBy,
			Action:      "collect.task.failed",
			Resource:    "collect_task",
			ResourceID:  tid.String(),
			Status:      "failed",
			Message:     truncateRunes(msg, 2000),
		})
	}
	if task.BatchID != nil {
		s.reconcileCollectBatch(ctx, task.BatchID)
	}
}

func (s *Service) failTaskRetryExhausted(ctx context.Context, task *CollectTask, msg string, payload map[string]any) {
	if s == nil || s.DB == nil || task == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	msg = truncateRunes(strings.TrimSpace(msg), 8000)
	fin := time.Now().UTC()
	tid := task.ID
	_ = s.DB.WithContext(ctx).Model(&CollectTask{}).
		Where("id = ?", tid).
		Updates(map[string]interface{}{
			"status":            StatusFailed,
			"error_message":     msg,
			"finished_at":       &fin,
			"next_retry_at":     nil,
			"retry_enqueued_at": nil,
			"updated_at":        fin,
		}).Error

	s.RecordTaskEvent(ctx, task, TaskEventInput{
		EventType:    EventTaskRetryExhausted,
		FromStatus:   StatusRunning,
		ToStatus:     StatusFailed,
		Message:      "auto-retry exhausted",
		ErrorMessage: msg,
		RetryCount:   task.RetryCount,
		MaxRetries:   s.effectiveMaxRetries(task),
		PayloadMap:   payload,
	})

	if s.OpLog != nil {
		logMsg := fmt.Sprintf("taskId=%s retryCount=%d", tid.String(), task.RetryCount)
		if task.BatchID != nil {
			logMsg += fmt.Sprintf(" batchId=%s", task.BatchID.String())
		}
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: task.CreatedBy,
			Action:      "collect.task.retry_exhausted",
			Resource:    "collect_task",
			ResourceID:  tid.String(),
			Status:      "failed",
			Message:     truncateRunes(logMsg+" "+msg, 2000),
		})
	}
	if task.BatchID != nil {
		s.reconcileCollectBatch(ctx, task.BatchID)
	}
}

// RunCollectJob executes one task from the queue (Collector → product draft).
func (s *Service) RunCollectJob(parent context.Context, taskID uuid.UUID) {
	if s == nil || s.DB == nil || s.Client == nil || s.Products == nil {
		return
	}
	ctx := parent
	if ctx == nil {
		ctx = context.Background()
	}

	var task CollectTask
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return
	}
	prevStatus := task.Status
	started := time.Now().UTC()
	res := s.DB.WithContext(ctx).Model(&CollectTask{}).
		Where("id = ? AND status IN ?", taskID, []string{StatusPending, StatusRetrying}).
		Updates(map[string]interface{}{
			"status":            StatusRunning,
			"started_at":        &started,
			"error_message":     "",
			"finished_at":       nil,
			"retry_enqueued_at": nil,
			"updated_at":        started,
		})
	if res.Error != nil {
		s.failTask(ctx, &task, prevStatus, res.Error.Error(), nil)
		return
	}
	if res.RowsAffected == 0 {
		return
	}

	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return
	}
	s.RecordTaskEvent(ctx, &task, TaskEventInput{
		EventType:  EventTaskRunning,
		FromStatus: prevStatus,
		ToStatus:   StatusRunning,
		Message:    "worker claimed task",
	})
	s.reconcileCollectBatch(ctx, task.BatchID)

	outcome, err := s.Client.Collect(ctx, task.Source, task.SourceURL)
	if err != nil {
		s.handleCollectJobError(ctx, &task, err)
		return
	}

	norm, err := parseNormalized(outcome.ProductJSON)
	if err != nil {
		s.handleCollectJobError(ctx, &task, fmt.Errorf("parse normalized product: %w", err))
		return
	}

	params := norm.importParams(outcome.ProductJSON)
	created, err := s.Products.ImportDraftWithContext(ctx, task.CreatedBy, params)
	if err != nil {
		s.handleCollectJobError(ctx, &task, err)
		return
	}

	fin := time.Now().UTC()
	rawJSON := datatypes.JSON(outcome.ProductJSON)
	pid := created.ID
	if err := s.DB.WithContext(ctx).Model(&CollectTask{}).
		Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"status":            StatusSuccess,
			"result_product_id": pid,
			"raw_result":        rawJSON,
			"error_message":     "",
			"finished_at":       &fin,
			"next_retry_at":     nil,
			"retry_enqueued_at": nil,
			"retry_count":       0,
			"updated_at":        fin,
		}).Error; err != nil {
		s.failTask(ctx, &task, StatusRunning, err.Error(), nil)
		return
	}
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return
	}
	s.reconcileCollectBatch(ctx, task.BatchID)

	s.RecordTaskEvent(ctx, &task, TaskEventInput{
		EventType:  EventTaskSuccess,
		FromStatus: StatusRunning,
		ToStatus:   StatusSuccess,
		Message:    "draft imported from collector response",
		RetryCount: task.RetryCount,
		MaxRetries: task.MaxRetries,
		PayloadMap: map[string]any{"productId": pid.String()},
	})

	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: task.CreatedBy,
			Action:      "collect.task.success",
			Resource:    "collect_task",
			ResourceID:  taskID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("product_id=%s", pid.String()),
		})
	}
}

func requestIDFromGin(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if v, ok := c.Get(ctxkey.TraceID); ok {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// CreateTaskAsync validates input, persists a pending task, enqueues, returns immediately.
func (s *Service) CreateTaskAsync(c *gin.Context, body CreateTaskBody, adminID *uuid.UUID) (TaskDTO, error) {
	var zero TaskDTO
	if s == nil || s.DB == nil {
		return zero, fmt.Errorf("collect: no db")
	}
	if !s.QueueEnabled {
		return zero, ErrCollectQueueDisabled
	}
	source := strings.TrimSpace(body.Source)
	url := strings.TrimSpace(body.URL)
	if source == "" || url == "" {
		return zero, fmt.Errorf("source and url are required")
	}
	if !looksLikeCollectURL(url) {
		return zero, ErrCollectURLNeedsHTTPScheme
	}
	if err := s.ValidateSourceForCollect(c.Request.Context(), source, false); err != nil {
		return zero, err
	}

	task := &CollectTask{
		Source:     source,
		SourceURL:  url,
		Status:     StatusPending,
		MaxRetries: s.defaultMaxRetriesForNewTask(),
		CreatedBy:  adminID,
	}
	if err := s.DB.WithContext(c.Request.Context()).Create(task).Error; err != nil {
		return zero, err
	}

	s.RecordTaskEvent(c.Request.Context(), task, TaskEventInput{
		EventType:  EventTaskCreated,
		ToStatus:   StatusPending,
		Message:    "collect task persisted",
		MaxRetries: task.MaxRetries,
	})

	reqID := requestIDFromGin(c)
	if err := s.enqueueTask(c.Request.Context(), task.ID, task.Source, task.SourceURL, adminID, reqID); err != nil {
		_ = s.DB.WithContext(c.Request.Context()).Where("task_id = ?", task.ID).Delete(&CollectTaskEvent{}).Error
		_ = s.DB.WithContext(c.Request.Context()).Unscoped().Where("id = ?", task.ID).Delete(&CollectTask{}).Error
		return zero, err
	}

	s.RecordTaskEvent(c.Request.Context(), task, TaskEventInput{
		EventType:  EventTaskEnqueued,
		ToStatus:   StatusPending,
		Message:    "queued to Redis",
		MaxRetries: task.MaxRetries,
		PayloadMap: func() map[string]any {
			if strings.TrimSpace(reqID) != "" {
				return map[string]any{"requestId": reqID}
			}
			return nil
		}(),
	})

	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			Action:     "collect.task.create",
			Resource:   "collect_task",
			ResourceID: task.ID.String(),
			Status:     "success",
			Message:    "task submitted to queue",
		})
	}
	return s.GetDTO(c, task.ID)
}

// RetryAsync re-queues a failed task.
func (s *Service) RetryAsync(c *gin.Context, id uuid.UUID, adminID *uuid.UUID) (TaskDTO, error) {
	var zero TaskDTO
	if s == nil || s.DB == nil {
		return zero, fmt.Errorf("collect: no db")
	}
	if !s.QueueEnabled {
		return zero, ErrCollectQueueDisabled
	}

	var task CollectTask
	if err := s.DB.WithContext(c.Request.Context()).First(&task, "id = ?", id).Error; err != nil {
		return zero, err
	}
	if task.Status != StatusFailed {
		return zero, fmt.Errorf("only failed tasks can be retried")
	}

	retryAt := time.Now().UTC()
	if err := s.DB.WithContext(c.Request.Context()).Model(&CollectTask{}).
		Where("id = ?", id).
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
		}).Error; err != nil {
		return zero, err
	}

	if err := s.DB.WithContext(c.Request.Context()).First(&task, "id = ?", id).Error; err != nil {
		return zero, err
	}

	reqID := requestIDFromGin(c)
	if err := s.enqueueTask(c.Request.Context(), task.ID, task.Source, task.SourceURL, task.CreatedBy, reqID); err != nil {
		fin := time.Now().UTC()
		_ = s.DB.WithContext(c.Request.Context()).Model(&CollectTask{}).
			Where("id = ?", id).
			Updates(map[string]interface{}{
				"status":        StatusFailed,
				"error_message": ErrRedisQueueUnavailable.Error(),
				"finished_at":   &fin,
				"updated_at":    fin,
			}).Error
		var bumped CollectTask
		if er := s.DB.WithContext(c.Request.Context()).First(&bumped, "id = ?", id).Error; er == nil {
			s.RecordTaskEvent(c.Request.Context(), &bumped, TaskEventInput{
				EventType:    EventTaskFailed,
				FromStatus:   StatusRetrying,
				ToStatus:     StatusFailed,
				Message:      "enqueue after manual retry failed",
				ErrorMessage: ErrRedisQueueUnavailable.Error(),
			})
		}
		s.reconcileCollectBatch(c.Request.Context(), task.BatchID)
		return zero, err
	}

	s.RecordTaskEvent(c.Request.Context(), &task, TaskEventInput{
		EventType:  EventTaskManualRetry,
		FromStatus: StatusFailed,
		ToStatus:   StatusRetrying,
		Message:    "manual retry re-queued",
		RetryCount: task.RetryCount,
		MaxRetries: task.MaxRetries,
	})

	s.reconcileCollectBatch(c.Request.Context(), task.BatchID)

	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "collect.task.retry",
			Resource:    "collect_task",
			ResourceID:  id.String(),
			Status:      "success",
			Message:     "task re-queued",
		})
	}
	return s.GetDTO(c, id)
}

// GetDTO returns one task by id.
func (s *Service) GetDTO(c *gin.Context, id uuid.UUID) (TaskDTO, error) {
	var zero TaskDTO
	if s == nil || s.DB == nil {
		return zero, fmt.Errorf("collect: no db")
	}
	var t CollectTask
	if err := s.DB.WithContext(c.Request.Context()).First(&t, "id = ?", id).Error; err != nil {
		return zero, err
	}
	return taskToDTO(&t), nil
}

// List paginates tasks with filters.
func (s *Service) List(c *gin.Context, q ListQuery) (*ListResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("collect: no db")
	}
	page, ps := clampCollectPage(q.Page, q.PageSize)

	tx := s.DB.WithContext(c.Request.Context()).Model(&CollectTask{})
	if v := strings.TrimSpace(q.Status); v != "" {
		tx = tx.Where("status = ?", v)
	}
	if v := strings.TrimSpace(q.Source); v != "" {
		tx = tx.Where("source = ?", v)
	}
	if bid := strings.TrimSpace(q.BatchID); bid != "" {
		tx = tx.Where("batch_id = ?", bid)
	}
	if v := strings.TrimSpace(q.Keyword); v != "" {
		pat := "%" + strings.ToLower(v) + "%"
		tx = tx.Where("LOWER(source_url) LIKE ?", pat)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (page - 1) * ps
	var rows []CollectTask
	if err := tx.Order("created_at DESC").Offset(offset).Limit(ps).Find(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]TaskDTO, 0, len(rows))
	for i := range rows {
		items = append(items, taskToDTO(&rows[i]))
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
