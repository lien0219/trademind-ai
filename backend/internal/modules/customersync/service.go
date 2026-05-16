package customersync

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/customerchat"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/modules/worker"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	"github.com/trademind-ai/trademind/backend/internal/rdb"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// QueueMessage is Redis LIST payload for workers.
type QueueMessage struct {
	TaskID string `json:"taskId"`
}

// SyncCustomerMessagesBody POST /shops/:id/sync-customer-messages
type SyncCustomerMessagesBody struct {
	Mode   string `json:"mode"`
	Start  string `json:"start"`
	End    string `json:"end"`
	Cursor string `json:"cursor"`
	Limit  int    `json:"limit"`
}

type syncInputSnapshot struct {
	Mode   string `json:"mode"`
	Start  string `json:"start"`
	End    string `json:"end"`
	Cursor string `json:"cursor"`
	Limit  int    `json:"limit"`
}

// ListQuery filters GET /customer/message-sync/tasks
type ListQuery struct {
	Page     int
	PageSize int
	ShopID   *uuid.UUID
	Platform string
	Status   string
	Start    *time.Time
	End      *time.Time
}

// TaskDTO API projection.
type TaskDTO struct {
	ID           uuid.UUID  `json:"id"`
	ShopID       uuid.UUID  `json:"shopId"`
	ShopName     string     `json:"shopName,omitempty"`
	Platform     string     `json:"platform"`
	TaskType     string     `json:"taskType"`
	Status       string     `json:"status"`
	Mode         string     `json:"mode"`
	Cursor       string     `json:"cursor,omitempty"`
	StartedAt    *time.Time `json:"startedAt,omitempty"`
	FinishedAt   *time.Time `json:"finishedAt,omitempty"`
	TotalCount   int        `json:"totalCount"`
	SuccessCount int        `json:"successCount"`
	FailedCount  int        `json:"failedCount"`
	ErrorMessage string     `json:"errorMessage,omitempty"`
	Input        any        `json:"input,omitempty"`
	Output       any        `json:"output,omitempty"`
	CreatedBy    *uuid.UUID `json:"createdBy,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

// ListResult paginates tasks.
type ListResult struct {
	Items      []TaskDTO
	Total      int64
	Page       int
	PageSize   int
	TotalPages int
}

func pagesOf(total int64, ps int) int {
	if ps < 1 {
		ps = 20
	}
	pages := int(total) / ps
	if int(total)%ps != 0 {
		pages++
	}
	if pages == 0 && total > 0 {
		pages = 1
	}
	return pages
}

// Service orchestrates customer_message_sync_tasks + provider PullMessages + customerchat upsert.
type Service struct {
	DB           *gorm.DB
	Redis        *rdb.Client
	Shops        *shop.Service
	Settings     *settings.Service
	CustomerChat *customerchat.Service
	OpLog        *operationlog.Service

	QueueEnabled bool
	QueueName    string
	TaskTimeout  time.Duration
}

func (s *Service) normalizedQueueName() string {
	q := strings.TrimSpace(s.QueueName)
	if q == "" {
		return "customer:message:sync:tasks"
	}
	return q
}

func parseRFC3339Ptr(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, fmt.Errorf("invalid time (RFC3339 expected)")
	}
	return &t, nil
}

func normalizeMode(mode string) string {
	m := strings.TrimSpace(strings.ToLower(mode))
	switch m {
	case "", ModeIncremental:
		return ModeIncremental
	case ModeManual, ModeFull:
		return m
	default:
		return ModeIncremental
	}
}

func (s *Service) bindInput(body SyncCustomerMessagesBody) (syncInputSnapshot, datatypes.JSON, error) {
	mode := normalizeMode(body.Mode)
	lim := body.Limit
	if lim <= 0 {
		lim = 50
	}
	if lim > 200 {
		lim = 200
	}
	if strings.TrimSpace(body.Start) != "" {
		if _, err := time.Parse(time.RFC3339, strings.TrimSpace(body.Start)); err != nil {
			return syncInputSnapshot{}, nil, fmt.Errorf("invalid start time (RFC3339 expected)")
		}
	}
	if strings.TrimSpace(body.End) != "" {
		if _, err := time.Parse(time.RFC3339, strings.TrimSpace(body.End)); err != nil {
			return syncInputSnapshot{}, nil, fmt.Errorf("invalid end time (RFC3339 expected)")
		}
	}
	snap := syncInputSnapshot{
		Mode:   mode,
		Start:  strings.TrimSpace(body.Start),
		End:    strings.TrimSpace(body.End),
		Cursor: strings.TrimSpace(body.Cursor),
		Limit:  lim,
	}
	b, err := json.Marshal(snap)
	if err != nil {
		return syncInputSnapshot{}, nil, err
	}
	return snap, b, nil
}

func snapFromJSON(raw datatypes.JSON) syncInputSnapshot {
	var snap syncInputSnapshot
	if len(raw) == 0 {
		snap.Mode = ModeIncremental
		snap.Limit = 50
		return snap
	}
	_ = json.Unmarshal(raw, &snap)
	if snap.Limit <= 0 {
		snap.Limit = 50
	}
	if strings.TrimSpace(snap.Mode) == "" {
		snap.Mode = ModeIncremental
	}
	return snap
}

// CreateShopSync starts a sync task from HTTP.
func (s *Service) CreateShopSync(c *gin.Context, shopID uuid.UUID, body SyncCustomerMessagesBody, adminID *uuid.UUID) (*TaskDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("customersync: no db")
	}
	var row shop.Shop
	if err := s.DB.WithContext(c.Request.Context()).First(&row, "id = ?", shopID).Error; err != nil {
		return nil, err
	}
	prov := platformp.Get(strings.TrimSpace(row.Platform))
	if err := ValidateShopCustomerMessageSync(&row, prov); err != nil {
		return nil, err
	}
	_, auth, err := s.Shops.PlainAuthForProvider(c, shopID)
	if err != nil {
		return nil, err
	}
	if err := ensureShopAuthorizedForSync(&row, auth); err != nil {
		return nil, err
	}
	if err := ensurePlatformPartnerConfigStatic(s.Settings, c.Request.Context(), prov); err != nil {
		return nil, err
	}

	snap, inputJSON, err := s.bindInput(body)
	if err != nil {
		return nil, err
	}

	task := CustomerMessageSyncTask{
		ShopID:    shopID,
		Platform:  strings.TrimSpace(row.Platform),
		TaskType:  TaskTypeCustomerMessageSync,
		Status:    StatusPending,
		Mode:      snap.Mode,
		Cursor:    snap.Cursor,
		Input:     inputJSON,
		CreatedBy: adminID,
	}
	if err := s.DB.WithContext(c.Request.Context()).Create(&task).Error; err != nil {
		return nil, err
	}

	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "customer.message_sync.create",
			Resource:    "customer_message_sync_task",
			ResourceID:  task.ID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("taskId=%s shopId=%s platform=%s mode=%s", task.ID.String(), shopID.String(), task.Platform, task.Mode),
		})
	}

	runInline := func() error {
		return s.ProcessQueuedTask(context.Background(), task.ID, worker.GenerateInlineWorkerID(worker.TypeCustomerMessageSync))
	}

	if s.QueueEnabled && s.Redis != nil && s.Redis.Client != nil {
		if err := s.enqueue(c.Request.Context(), task.ID); err != nil {
			slog.Warn("customer_message_sync_enqueue_failed_run_inline", "taskId", task.ID.String(), "error", err)
			if err := runInline(); err != nil {
				return nil, err
			}
		}
	} else {
		if err := runInline(); err != nil {
			return nil, err
		}
	}

	out, err := s.GetDTO(c.Request.Context(), task.ID)
	return &out, err
}

func (s *Service) enqueue(ctx context.Context, taskID uuid.UUID) error {
	if s.Redis == nil || s.Redis.Client == nil {
		return ErrRedisQueueUnavailable
	}
	msg := QueueMessage{TaskID: taskID.String()}
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return s.Redis.LPush(ctx, s.normalizedQueueName(), string(b)).Err()
}

// ProcessQueuedTask executes one task (worker or inline).
func (s *Service) ProcessQueuedTask(ctx context.Context, taskID uuid.UUID, workerID string) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("customersync: no db")
	}
	defer func() {
		if r := recover(); r != nil {
			s.handlePanic(ctx, taskID, workerID, r)
		}
	}()

	lease := s.taskLeaseTTL()
	task, ok, err := s.tryClaimTask(ctx, taskID, workerID, lease)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	stopRen := s.startLeaseRenewal(ctx, taskID, workerID, lease)
	defer stopRen()

	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: task.CreatedBy,
			Action:      "customer.message_sync.running",
			Resource:    "customer_message_sync_task",
			ResourceID:  taskID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("taskId=%s shopId=%s platform=%s", taskID.String(), task.ShopID.String(), task.Platform),
		})
	}

	fail := func(msg string) error {
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
				AdminUserID: task.CreatedBy,
				Action:      "customer.message_sync.failed",
				Resource:    "customer_message_sync_task",
				ResourceID:  taskID.String(),
				Status:      "failed",
				Message:     fmt.Sprintf("taskId=%s shopId=%s platform=%s error=%s", taskID.String(), task.ShopID.String(), task.Platform, msg),
			})
		}
		return fmt.Errorf("%s", msg)
	}

	shopRow, auth, err := s.Shops.PlainAuthForProviderCtx(ctx, task.ShopID)
	if err != nil {
		return fail(err.Error())
	}
	if err := ensureShopAuthorizedForSync(shopRow, auth); err != nil {
		return fail(err.Error())
	}
	prov := platformp.Get(shopRow.Platform)
	if err := ensurePlatformPartnerConfigStatic(s.Settings, ctx, prov); err != nil {
		return fail(err.Error())
	}
	cm, okp := platformp.AsCustomerMessage(prov)
	if !okp || cm == nil {
		return fail("platform does not implement customer messaging")
	}

	snap := snapFromJSON(task.Input)
	st, _ := parseRFC3339Ptr(snap.Start)
	et, _ := parseRFC3339Ptr(snap.End)

	timeout := s.TaskTimeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	res, err := cm.PullMessages(runCtx, platformp.PullMessagesRequest{
		ShopID:    task.ShopID,
		Platform:  shopRow.Platform,
		Auth:      auth,
		StartTime: st,
		EndTime:   et,
		Cursor:    snap.Cursor,
		Limit:     snap.Limit,
	})
	if err != nil {
		return fail(mapCustomerMessageErr(err).Error())
	}
	if s.CustomerChat == nil {
		return fail("customer chat service unavailable")
	}
	convN, msgN, err := s.CustomerChat.SyncPlatformCustomerMessages(ctx, shopRow, res)
	if err != nil {
		return fail(err.Error())
	}

	outMap := map[string]any{
		"conversationsTouched": convN,
		"messagesInserted":     msgN,
		"hasMore":              res.HasMore,
		"nextCursor":           res.NextCursor,
	}
	if len(res.RawSummary) > 0 {
		outMap["providerSummary"] = platformp.TrimRawMap(res.RawSummary, 16, 400)
	}
	outJSON, _ := json.Marshal(outMap)

	fin := time.Now().UTC()
	nextCur := strings.TrimSpace(res.NextCursor)
	_ = s.DB.WithContext(ctx).Model(&CustomerMessageSyncTask{}).Where("id = ?", taskID).
		Updates(map[string]any{
			"status":        StatusSuccess,
			"finished_at":   &fin,
			"total_count":   convN,
			"success_count": msgN,
			"failed_count":  0,
			"cursor":        nextCur,
			"output":        datatypes.JSON(outJSON),
			"error_message": "",
			"locked_by":     nil,
			"locked_until":  nil,
			"updated_at":    fin,
		}).Error

	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: task.CreatedBy,
			Action:      "customer.message_sync.success",
			Resource:    "customer_message_sync_task",
			ResourceID:  taskID.String(),
			Status:      "success",
			Message: fmt.Sprintf("taskId=%s shopId=%s platform=%s conversations=%d messages=%d",
				taskID.String(), task.ShopID.String(), task.Platform, convN, msgN),
		})
	}
	return nil
}

func mapCustomerMessageErr(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "403") || strings.Contains(msg, "forbidden") || strings.Contains(msg, "permission") {
		return platformp.ErrPlatformCustomerMessagePermissionDenied
	}
	return err
}

func (s *Service) taskToDTO(ctx context.Context, t *CustomerMessageSyncTask, shopName string) TaskDTO {
	var input any
	if len(t.Input) > 0 {
		_ = json.Unmarshal(t.Input, &input)
	}
	var output any
	if len(t.Output) > 0 {
		_ = json.Unmarshal(t.Output, &output)
	}
	return TaskDTO{
		ID:           t.ID,
		ShopID:       t.ShopID,
		ShopName:     shopName,
		Platform:     t.Platform,
		TaskType:     t.TaskType,
		Status:       t.Status,
		Mode:         t.Mode,
		Cursor:       t.Cursor,
		StartedAt:    t.StartedAt,
		FinishedAt:   t.FinishedAt,
		TotalCount:   t.TotalCount,
		SuccessCount: t.SuccessCount,
		FailedCount:  t.FailedCount,
		ErrorMessage: t.ErrorMessage,
		Input:        input,
		Output:       output,
		CreatedBy:    t.CreatedBy,
		CreatedAt:    t.CreatedAt,
		UpdatedAt:    t.UpdatedAt,
	}
}

func (s *Service) shopNameLookup(ctx context.Context, shopID uuid.UUID) string {
	var sh shop.Shop
	if err := s.DB.WithContext(ctx).First(&sh, "id = ?", shopID).Error; err != nil {
		return ""
	}
	return sh.ShopName
}

// GetDTO loads one task.
func (s *Service) GetDTO(ctx context.Context, id uuid.UUID) (TaskDTO, error) {
	var zero TaskDTO
	var t CustomerMessageSyncTask
	if err := s.DB.WithContext(ctx).First(&t, "id = ?", id).Error; err != nil {
		return zero, err
	}
	name := s.shopNameLookup(ctx, t.ShopID)
	return s.taskToDTO(ctx, &t, name), nil
}

// List paginates tasks.
func (s *Service) List(ctx context.Context, q ListQuery) (*ListResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("customersync: no db")
	}
	page := q.Page
	if page < 1 {
		page = 1
	}
	ps := q.PageSize
	if ps < 1 {
		ps = 20
	}
	if ps > 100 {
		ps = 100
	}

	tx := s.DB.WithContext(ctx).Model(&CustomerMessageSyncTask{})
	if q.ShopID != nil && *q.ShopID != uuid.Nil {
		tx = tx.Where("shop_id = ?", *q.ShopID)
	}
	if v := strings.TrimSpace(q.Platform); v != "" {
		tx = tx.Where("platform = ?", v)
	}
	if v := strings.TrimSpace(q.Status); v != "" {
		tx = tx.Where("status = ?", v)
	}
	if q.Start != nil {
		tx = tx.Where("created_at >= ?", *q.Start)
	}
	if q.End != nil {
		tx = tx.Where("created_at <= ?", *q.End)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}
	offset := (page - 1) * ps
	var rows []CustomerMessageSyncTask
	if err := tx.Order("created_at DESC").Offset(offset).Limit(ps).Find(&rows).Error; err != nil {
		return nil, err
	}

	out := make([]TaskDTO, len(rows))
	for i := range rows {
		out[i] = s.taskToDTO(ctx, &rows[i], s.shopNameLookup(ctx, rows[i].ShopID))
	}

	return &ListResult{
		Items:      out,
		Total:      total,
		Page:       page,
		PageSize:   ps,
		TotalPages: pagesOf(total, ps),
	}, nil
}

// RetryFailed resets a failed task and re-runs or re-enqueues it.
func (s *Service) RetryFailed(c *gin.Context, taskID uuid.UUID, adminID *uuid.UUID) (*TaskDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("customersync: no db")
	}
	var task CustomerMessageSyncTask
	if err := s.DB.WithContext(c.Request.Context()).First(&task, "id = ?", taskID).Error; err != nil {
		return nil, err
	}
	if strings.TrimSpace(task.Status) != StatusFailed {
		return nil, fmt.Errorf("only failed tasks can be retried")
	}

	reset := time.Now().UTC()
	if err := s.DB.WithContext(c.Request.Context()).Model(&CustomerMessageSyncTask{}).Where("id = ?", taskID).
		Updates(map[string]any{
			"status":        StatusPending,
			"error_message": "",
			"started_at":    nil,
			"finished_at":   nil,
			"total_count":   0,
			"success_count": 0,
			"failed_count":  0,
			"output":        datatypes.JSON(nil),
			"locked_by":     nil,
			"locked_until":  nil,
			"updated_at":    reset,
		}).Error; err != nil {
		return nil, err
	}

	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "customer.message_sync.retry",
			Resource:    "customer_message_sync_task",
			ResourceID:  taskID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("taskId=%s shopId=%s platform=%s", taskID.String(), task.ShopID.String(), task.Platform),
		})
	}

	runInline := func() error {
		return s.ProcessQueuedTask(context.Background(), taskID, worker.GenerateInlineWorkerID(worker.TypeCustomerMessageSync))
	}

	if s.QueueEnabled && s.Redis != nil && s.Redis.Client != nil {
		if err := s.enqueue(c.Request.Context(), taskID); err != nil {
			slog.Warn("customer_message_sync_retry_enqueue_failed_run_inline", "taskId", taskID.String(), "error", err)
			if err := runInline(); err != nil {
				return nil, err
			}
		}
	} else {
		if err := runInline(); err != nil {
			return nil, err
		}
	}

	out, err := s.GetDTO(c.Request.Context(), taskID)
	return &out, err
}
