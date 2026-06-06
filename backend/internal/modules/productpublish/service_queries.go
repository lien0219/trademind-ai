package productpublish

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/worker"
)

func (s *Service) shopNameLookup(ctx context.Context, id uuid.UUID) string {
	var row struct {
		ShopName string `gorm:"column:shop_name"`
	}
	_ = s.DB.WithContext(ctx).Table(`shops`).
		Select(`shop_name`).Where(`id = ?`, id).
		Take(&row).Error
	return strings.TrimSpace(row.ShopName)
}

func (s *Service) taskToDTO(ctx context.Context, t *ProductPublishTask) TaskDTO {
	if t == nil {
		return TaskDTO{}
	}
	var p product.Product
	_ = s.DB.WithContext(ctx).First(&p, "id = ?", t.ProductID).Error
	pt := strings.TrimSpace(p.Title)
	if pt == "" {
		pt = strings.TrimSpace(p.AITitle)
	}
	return TaskDTO{
		ID:              t.ID,
		ProductID:       t.ProductID,
		ShopID:          t.ShopID,
		TargetStoreID:   t.TargetStoreID,
		ShopName:        s.shopNameLookup(ctx, t.ShopID),
		ProductTitle:    pt,
		Platform:        t.Platform,
		TargetPlatform:  t.Platform,
		TaskType:        t.TaskType,
		Status:          t.Status,
		PublishStatus:   effectivePublishStatus(t),
		Mode:            t.Mode,
		PublishMode:     firstNonEmpty(t.PublishMode, t.Mode),
		Title:           t.Title,
		Description:     t.Description,
		Images:          dtoTrimJSON(t.Images),
		SKUs:            dtoTrimJSON(t.SKUs),
		Price:           t.Price,
		Currency:        t.Currency,
		CheckResult:     dtoTrimJSON(t.CheckResult),
		PlatformPayload: dtoTrimJSON(t.PlatformPayload),
		PlatformResult:  dtoTrimJSON(t.PlatformResult),
		StartedAt:       t.StartedAt,
		FinishedAt:      t.FinishedAt,
		ErrorCode:       t.ErrorCode,
		ErrorMessage:    t.ErrorMessage,
		Input:           dtoTrimJSON(t.Input),
		Output:          dtoTrimJSON(t.Output),
		CreatedBy:       t.CreatedBy,
		CreatedAt:       t.CreatedAt,
		UpdatedAt:       t.UpdatedAt,
	}
}

func effectivePublishStatus(t *ProductPublishTask) string {
	if t == nil {
		return ""
	}
	if strings.TrimSpace(t.PublishStatus) != "" {
		return strings.TrimSpace(t.PublishStatus)
	}
	switch strings.TrimSpace(t.Status) {
	case TaskPending:
		return StatusReady
	case TaskRunning:
		return StatusPublishing
	case TaskSuccess:
		return StatusSuccess
	case TaskFailed:
		return StatusPubFailed
	case TaskCancelled:
		return TaskCancelled
	default:
		return StatusDraft
	}
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return strings.TrimSpace(a)
	}
	return strings.TrimSpace(b)
}

func (s *Service) GetDTO(ctx context.Context, taskID uuid.UUID) (TaskDTO, error) {
	if s == nil || s.DB == nil {
		return TaskDTO{}, fmt.Errorf("productpublish: no db")
	}
	var t ProductPublishTask
	if err := s.DB.WithContext(ctx).First(&t, "id = ?", taskID).Error; err != nil {
		return TaskDTO{}, err
	}
	return s.taskToDTO(ctx, &t), nil
}

func (s *Service) ListTasks(ctx context.Context, q ListTasksQuery) (*ListTasksResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("productpublish: no db")
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
	tx := s.DB.WithContext(ctx).Model(&ProductPublishTask{})
	if q.ProductID != nil {
		tx = tx.Where("product_id = ?", *q.ProductID)
	}
	if q.ShopID != nil {
		tx = tx.Where("shop_id = ?", *q.ShopID)
	}
	if strings.TrimSpace(q.Platform) != "" {
		tx = tx.Where("platform = ?", strings.TrimSpace(q.Platform))
	}
	if strings.TrimSpace(q.Status) != "" {
		tx = tx.Where("status = ?", strings.TrimSpace(q.Status))
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
	var rows []ProductPublishTask
	if err := tx.Order("created_at DESC").Offset(offset).Limit(ps).Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]TaskDTO, 0, len(rows))
	for i := range rows {
		items = append(items, s.taskToDTO(ctx, &rows[i]))
	}
	return &ListTasksResult{
		Items:      items,
		Total:      total,
		Page:       page,
		PageSize:   ps,
		TotalPages: pagesOf(total, ps),
	}, nil
}

func (s *Service) RetryFailed(c *gin.Context, taskID uuid.UUID, adminID *uuid.UUID) (*TaskDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("productpublish: no db")
	}
	var task ProductPublishTask
	if err := s.DB.WithContext(c.Request.Context()).First(&task, "id = ?", taskID).Error; err != nil {
		return nil, err
	}
	if strings.TrimSpace(task.Status) != TaskFailed {
		return nil, fmt.Errorf("only failed tasks can be retried")
	}
	reset := time.Now().UTC()
	if err := s.DB.WithContext(c.Request.Context()).Model(&ProductPublishTask{}).Where("id = ?", taskID).
		Updates(map[string]any{
			"status":          TaskPending,
			"publish_status":  StatusReady,
			"error_code":      "",
			"error_message":   "",
			"started_at":      nil,
			"finished_at":     nil,
			"output":          nil,
			"platform_result": nil,
			"locked_by":       nil,
			"locked_until":    nil,
			"updated_at":      reset,
		}).Error; err != nil {
		return nil, err
	}
	if rid, ok := snapshotPublicationFromTask(&task); ok {
		_ = s.DB.WithContext(c.Request.Context()).Model(&ProductPublication{}).Where("id = ?", rid).
			Updates(map[string]any{
				"status":          StatusPublishing,
				"publish_status":  StatusPublishing,
				"publish_task_id": taskID,
				"updated_at":      reset,
			}).Error
	}

	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "product.publish.retry",
			Resource:    "product_publish_task",
			ResourceID:  taskID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("taskId=%s shopId=%s platform=%s", taskID.String(), task.ShopID.String(), task.Platform),
		})
	}

	bg := context.Background()
	runInline := func() error {
		return s.ProcessQueuedTask(bg, taskID, worker.GenerateInlineWorkerID(worker.TypeProductPublish))
	}

	if s.QueueEnabled && s.Redis != nil && s.Redis.Client != nil {
		if err := s.enqueue(c.Request.Context(), taskID); err != nil {
			slog.Warn("product_publish_retry_enqueue_failed_run_inline", "taskId", taskID.String(), "error", err)
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

func (s *Service) skuMappingSummaryLines(ctx context.Context, publicationID uuid.UUID) ([]string, error) {
	var rows []ProductPublicationSKU
	if err := s.DB.WithContext(ctx).Where("publication_id = ?", publicationID).
		Order("created_at ASC").Limit(40).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		code := strings.TrimSpace(r.SKUCode)
		ext := strings.TrimSpace(r.ExternalSKUID)
		if code == "" && ext == "" {
			continue
		}
		if code != "" {
			out = append(out, code+"="+ext)
		} else {
			out = append(out, ext)
		}
	}
	return out, nil
}

// ListPublicationsByProduct returns persisted publication rows for a draft product.
func (s *Service) ListPublicationsByProduct(ctx context.Context, productID uuid.UUID) ([]PublicationDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("productpublish: no db")
	}
	var rows []ProductPublication
	if err := s.DB.WithContext(ctx).
		Where("product_id = ?", productID).
		Order("updated_at DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]PublicationDTO, 0, len(rows))
	for i := range rows {
		sum, _ := s.skuMappingSummaryLines(ctx, rows[i].ID)
		pid := rows[i].PublishTaskID
		out = append(out, PublicationDTO{
			ID:                rows[i].ID,
			ProductID:         rows[i].ProductID,
			ShopID:            rows[i].ShopID,
			ShopName:          s.shopNameLookup(ctx, rows[i].ShopID),
			Platform:          rows[i].Platform,
			PublishTaskID:     pid,
			ExternalProductID: rows[i].ExternalProductID,
			ExternalURL:       rows[i].ExternalURL,
			Status:            rows[i].Status,
			PublishStatus:     rows[i].PublishStatus,
			PublishedAt:       rows[i].PublishedAt,
			LastSyncedAt:      rows[i].LastSyncedAt,
			SKUMappingSummary: sum,
		})
	}
	return out, nil
}
