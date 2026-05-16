package collect

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"strings"
	"time"
	"unicode/utf8"
)

// Collect task event_type values (timeline / troubleshooting).
const (
	EventTaskCreated            = "task.created"
	EventTaskEnqueued           = "task.enqueued"
	EventTaskRunning            = "task.running"
	EventTaskSuccess            = "task.success"
	EventTaskFailed             = "task.failed"
	EventTaskAutoRetryScheduled = "task.auto_retry_scheduled"
	EventTaskAutoRetryEnqueued  = "task.auto_retry_enqueued"
	EventTaskRetryExhausted     = "task.retry_exhausted"
	EventTaskManualRetry        = "task.manual_retry"
	EventTaskCancelled          = "task.cancelled"
	EventWorkerLeaseAcquired    = "worker.lease.acquired"
	EventWorkerLeaseExpired     = "worker.lease.expired"
	EventWorkerLeaseRecovered   = "worker.lease.recovered"
)

var blockedCollectEventPayloadKeys = map[string]struct{}{
	// HTTP / browser
	"cookie": {}, "set-cookie": {}, "authorization": {}, "auth": {},
	"token": {}, "access_token": {}, "refresh_token": {}, "secret": {},
	"api_key": {}, "apikey": {}, "password": {},
	// large / sensitive bodies
	"rawresult": {}, "raw_result": {}, "html": {}, "body": {},
}

// TaskEventInput binds one timeline row; only non-sensitive payload keys are allowed.
type TaskEventInput struct {
	EventType    string
	FromStatus   string
	ToStatus     string
	Message      string
	ErrorMessage string
	RetryCount   int
	MaxRetries   int
	NextRetryAt  *time.Time
	PayloadMap   map[string]any
}

func stringStatusPtr(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

func sanitizeCollectEventPayload(m map[string]any) datatypes.JSON {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		lk := strings.ToLower(strings.TrimSpace(k))
		if lk == "" {
			continue
		}
		if _, bad := blockedCollectEventPayloadKeys[lk]; bad {
			continue
		}
		switch x := v.(type) {
		case string:
			out[k] = truncateRunes(strings.TrimSpace(x), 2000)
		case bool, float64, int, int32, int64, uint, uint32, uint64:
			out[k] = x
		default:
			b, err := json.Marshal(x)
			if err != nil {
				out[k] = "[omitted]"
				continue
			}
			out[k] = json.RawMessage(b)
			if utf8.RuneCountInString(string(b)) > 2000 {
				out[k] = truncateRunes(string(b), 2000)
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil
	}
	return datatypes.JSON(b)
}

func (s *Service) RecordTaskEvent(ctx context.Context, task *CollectTask, in TaskEventInput) {
	if task == nil {
		return
	}
	db := s.DB
	if db == nil {
		return
	}
	_ = s.recordCollectTaskEvent(db.WithContext(ctx), ctx, task, in)
}

func (s *Service) RecordTaskEventWithDB(tx *gorm.DB, ctx context.Context, task *CollectTask, in TaskEventInput) {
	if tx == nil {
		s.RecordTaskEvent(ctx, task, in)
		return
	}
	_ = s.recordCollectTaskEvent(tx, ctx, task, in)
}

func (s *Service) recordCollectTaskEvent(db *gorm.DB, ctx context.Context, task *CollectTask, in TaskEventInput) error {
	if db == nil || task == nil || strings.TrimSpace(in.EventType) == "" {
		return nil
	}
	ev := CollectTaskEvent{
		TaskID:       task.ID,
		BatchID:      task.BatchID,
		EventType:    strings.TrimSpace(in.EventType),
		FromStatus:   stringStatusPtr(in.FromStatus),
		ToStatus:     stringStatusPtr(in.ToStatus),
		Message:      truncateRunes(strings.TrimSpace(in.Message), 4000),
		ErrorMessage: truncateRunes(strings.TrimSpace(in.ErrorMessage), 8000),
		RetryCount:   in.RetryCount,
		MaxRetries:   in.MaxRetries,
		NextRetryAt:  in.NextRetryAt,
		Payload:      sanitizeCollectEventPayload(in.PayloadMap),
	}
	return db.WithContext(ctx).Create(&ev).Error
}

func clampCollectEventPage(page, ps int) (int, int) {
	if page < 1 {
		page = 1
	}
	if ps < 1 {
		ps = 50
	}
	if ps > 100 {
		ps = 100
	}
	return page, ps
}

// TaskEventsListQuery binds GET .../tasks/:id/events.
type TaskEventsListQuery struct {
	Page     int
	PageSize int
}

// TaskEventsListResult paginates task events ascending by created_at.
type TaskEventsListResult struct {
	Items      []TaskEventDTO `json:"list"`
	Total      int64          `json:"total"`
	Page       int            `json:"page"`
	PageSize   int            `json:"pageSize"`
	TotalPages int            `json:"totalPages"`
}

// TaskEventDTO is API-facing event shape (no secrets).
type TaskEventDTO struct {
	ID           uuid.UUID       `json:"id"`
	TaskID       uuid.UUID       `json:"taskId"`
	BatchID      *uuid.UUID      `json:"batchId,omitempty"`
	EventType    string          `json:"eventType"`
	FromStatus   *string         `json:"fromStatus,omitempty"`
	ToStatus     *string         `json:"toStatus,omitempty"`
	Message      string          `json:"message,omitempty"`
	ErrorMessage string          `json:"errorMessage,omitempty"`
	RetryCount   int             `json:"retryCount"`
	MaxRetries   int             `json:"maxRetries"`
	NextRetryAt  *time.Time      `json:"nextRetryAt,omitempty"`
	Payload      json.RawMessage `json:"payload,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
}

func eventToDTO(e *CollectTaskEvent) TaskEventDTO {
	if e == nil {
		return TaskEventDTO{}
	}
	var raw json.RawMessage
	if len(e.Payload) > 0 {
		raw = json.RawMessage(e.Payload)
	}
	return TaskEventDTO{
		ID:           e.ID,
		TaskID:       e.TaskID,
		BatchID:      e.BatchID,
		EventType:    e.EventType,
		FromStatus:   cloneStrPtr(e.FromStatus),
		ToStatus:     cloneStrPtr(e.ToStatus),
		Message:      e.Message,
		ErrorMessage: e.ErrorMessage,
		RetryCount:   e.RetryCount,
		MaxRetries:   e.MaxRetries,
		NextRetryAt:  cloneTimePtr(e.NextRetryAt),
		Payload:      raw,
		CreatedAt:    e.CreatedAt,
	}
}

func cloneStrPtr(p *string) *string {
	if p == nil {
		return nil
	}
	cp := *p
	return &cp
}

func cloneTimePtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	cp := *t
	return &cp
}

// ListTaskEvents paginates timeline for one task by created_at ASC.
func (s *Service) ListTaskEvents(ctx context.Context, taskID uuid.UUID, q TaskEventsListQuery) (*TaskEventsListResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("collect: no db")
	}
	var exists int64
	if err := s.DB.WithContext(ctx).Model(&CollectTask{}).Where("id = ?", taskID).Limit(1).Count(&exists).Error; err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	page, ps := clampCollectEventPage(q.Page, q.PageSize)
	tx := s.DB.WithContext(ctx).Model(&CollectTaskEvent{}).Where("task_id = ?", taskID)
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}
	offset := (page - 1) * ps
	var rows []CollectTaskEvent
	if err := s.DB.WithContext(ctx).
		Where("task_id = ?", taskID).
		Order("created_at ASC").
		Offset(offset).
		Limit(ps).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]TaskEventDTO, 0, len(rows))
	for i := range rows {
		items = append(items, eventToDTO(&rows[i]))
	}

	pages := int(total) / ps
	if int(total)%ps != 0 {
		pages++
	}
	if pages == 0 && total > 0 {
		pages = 1
	}
	return &TaskEventsListResult{
		Items:      items,
		Total:      total,
		Page:       page,
		PageSize:   ps,
		TotalPages: pages,
	}, nil
}
