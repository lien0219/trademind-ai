package aiproductimage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
)

func (s *Service) GetBatchByID(ctx context.Context, id uuid.UUID) (*AIProductImageBatch, error) {
	var b AIProductImageBatch
	if err := s.DB.WithContext(ctx).First(&b, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &b, nil
}

func (s *Service) toBatchListItem(b *AIProductImageBatch) BatchListItem {
	ops := []string{}
	var in map[string]any
	_ = json.Unmarshal(b.Input, &in)
	if raw, ok := in["operationTypes"].([]any); ok {
		for _, v := range raw {
			ops = append(ops, fmt.Sprint(v))
		}
	}
	fin := (*string)(nil)
	if b.FinishedAt != nil {
		s := b.FinishedAt.UTC().Format(time.RFC3339)
		fin = &s
	}
	return BatchListItem{
		ID:             b.ID.String(),
		BatchNo:        b.BatchNo,
		Status:         b.Status,
		StatusLabel:    batchStatusLabel(b.Status),
		ProductCount:   b.ProductCount,
		ImageCount:     b.ImageCount,
		ItemCount:      b.ItemCount,
		SuccessCount:   b.SuccessCount,
		FailedCount:    b.FailedCount,
		AppliedCount:   b.AppliedCount,
		OperationTypes: ops,
		CreatedAt:      b.CreatedAt.UTC().Format(time.RFC3339),
		FinishedAt:     fin,
	}
}

func (s *Service) buildItemDTO(_ context.Context, item *AIProductImageItem, p *product.Product) ItemDetailDTO {
	dto := ItemDetailDTO{
		ID:                 item.ID.String(),
		ProductID:          item.ProductID.String(),
		ImageType:          item.ImageType,
		ImageTypeLabel:     imageTypeLabel(item.ImageType),
		OperationType:      item.OperationType,
		OperationLabel:     operationTypeLabel(item.OperationType),
		Status:             item.Status,
		StatusLabel:        itemStatusLabel(item.Status),
		SourceImageURL:     item.SourceImageURL,
		ResultImageURL:     item.ResultImageURL,
		SourceSnapshotHash: item.SourceSnapshotHash,
		ErrorMessage:       item.ErrorMessage,
		ApplyMode:          item.ApplyMode,
		ApplyModeLabel:     applyModeLabel(item.ApplyMode),
	}
	if item.ImageID != nil {
		dto.ImageID = item.ImageID.String()
	}
	if p != nil {
		dto.ProductTitle = strings.TrimSpace(p.Title)
	}
	if item.ImageTaskID != nil {
		dto.ImageTaskID = item.ImageTaskID.String()
	}
	if item.ApplicationID != nil {
		dto.ApplicationID = item.ApplicationID.String()
	}
	if item.AppliedAt != nil {
		s := item.AppliedAt.UTC().Format(time.RFC3339)
		dto.AppliedAt = &s
	}
	var warnings []QualityWarning
	_ = json.Unmarshal(item.QualityWarnings, &warnings)
	dto.QualityWarnings = warnings
	return dto
}

func (s *Service) ListBatches(ctx context.Context, page, pageSize int) ([]BatchListItem, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	var total int64
	tx := s.DB.WithContext(ctx).Model(&AIProductImageBatch{})
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []AIProductImageBatch
	offset := (page - 1) * pageSize
	if err := tx.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]BatchListItem, 0, len(rows))
	for _, b := range rows {
		out = append(out, s.toBatchListItem(&b))
	}
	return out, total, nil
}

func (s *Service) GetBatchDetail(ctx context.Context, id uuid.UUID, statusFilter string) (*BatchDetailDTO, error) {
	b, err := s.GetBatchByID(ctx, id)
	if err != nil {
		return nil, err
	}
	q := s.DB.WithContext(ctx).Where("batch_id = ?", id)
	if sf := strings.TrimSpace(statusFilter); sf != "" && sf != "all" {
		q = q.Where("status = ?", sf)
	}
	var items []AIProductImageItem
	if err := q.Order("created_at ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	pids := make([]uuid.UUID, 0, len(items))
	for _, it := range items {
		pids = append(pids, it.ProductID)
	}
	products := map[uuid.UUID]*product.Product{}
	if len(pids) > 0 {
		var rows []product.Product
		_ = s.DB.WithContext(ctx).Where("id IN ?", pids).Find(&rows).Error
		for i := range rows {
			products[rows[i].ID] = &rows[i]
		}
	}
	dtoItems := make([]ItemDetailDTO, 0, len(items))
	for i := range items {
		dtoItems = append(dtoItems, s.buildItemDTO(ctx, &items[i], products[items[i].ProductID]))
	}
	var in, out map[string]any
	_ = json.Unmarshal(b.Input, &in)
	_ = json.Unmarshal(b.Output, &out)
	return &BatchDetailDTO{
		BatchListItem: s.toBatchListItem(b),
		Items:         dtoItems,
		Input:         in,
		Output:        out,
	}, nil
}

func (s *Service) RetryFailed(c *gin.Context, batchID uuid.UUID, adminID *uuid.UUID) (*AIProductImageBatch, error) {
	ctx := c.Request.Context()
	b, err := s.GetBatchByID(ctx, batchID)
	if err != nil {
		return nil, err
	}
	var retryItems []AIProductImageItem
	if err := s.DB.WithContext(ctx).
		Where("batch_id = ? AND status IN ?", batchID, []string{ItemFailed, ItemPending, ItemRunning}).
		Find(&retryItems).Error; err != nil {
		return nil, err
	}
	if len(retryItems) == 0 {
		return b, nil
	}
	var in map[string]any
	_ = json.Unmarshal(b.Input, &in)
	opts := ImageGenerationOptions{}
	if o, ok := in["options"].(map[string]any); ok {
		opts.Language = fmt.Sprint(o["language"])
		opts.BackgroundStyle = fmt.Sprint(o["backgroundStyle"])
		opts.KeepSubject = boolFromAny(o["keepSubject"])
		opts.KeepBrandLogo = boolFromAny(o["keepBrandLogo"])
		opts.OutputFormat = fmt.Sprint(o["outputFormat"])
	}
	_ = s.DB.WithContext(ctx).Model(&AIProductImageBatch{}).Where("id = ?", batchID).Updates(map[string]any{
		"status": BatchRunning, "finished_at": nil,
	}).Error
	for _, item := range retryItems {
		_ = s.DB.WithContext(ctx).Model(&AIProductImageItem{}).Where("id = ?", item.ID).Updates(map[string]any{
			"status": ItemPending, "error_code": "", "error_message": "",
		}).Error
	}
	go func() {
		cp := detachedGinContext(c)
		for _, item := range retryItems {
			itemCopy := item
			s.runOneItem(cp, &itemCopy, opts, adminID)
		}
		s.finalizeBatch(context.Background(), batchID)
	}()
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "ai.product_image.batch.retry_failed",
			Resource:    "ai_product_image_batch",
			ResourceID:  batchID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("batchNo=%s retried=%d", b.BatchNo, len(retryItems)),
		})
	}
	return b, nil
}

func boolFromAny(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return strings.EqualFold(x, "true") || x == "1"
	default:
		return false
	}
}

func (s *Service) CancelPending(ctx context.Context, batchID uuid.UUID) (int, error) {
	res := s.DB.WithContext(ctx).Model(&AIProductImageItem{}).
		Where("batch_id = ? AND status = ?", batchID, ItemPending).
		Update("status", ItemCancelled)
	if res.Error != nil {
		return 0, res.Error
	}
	s.finalizeBatch(ctx, batchID)
	return int(res.RowsAffected), nil
}

func (s *Service) RejectItem(ctx context.Context, itemID uuid.UUID) error {
	var item AIProductImageItem
	if err := s.DB.WithContext(ctx).First(&item, "id = ?", itemID).Error; err != nil {
		return err
	}
	if item.Status != ItemPendingReview && item.Status != ItemSuccess {
		return fmt.Errorf("当前状态不可放弃")
	}
	return s.DB.WithContext(ctx).Model(&item).Update("status", ItemRejected).Error
}

func (s *Service) RegenerateItem(c *gin.Context, itemID uuid.UUID, adminID *uuid.UUID) (*ItemDetailDTO, error) {
	ctx := c.Request.Context()
	var item AIProductImageItem
	if err := s.DB.WithContext(ctx).First(&item, "id = ?", itemID).Error; err != nil {
		return nil, err
	}
	b, err := s.GetBatchByID(ctx, item.BatchID)
	if err != nil {
		return nil, err
	}
	var in map[string]any
	_ = json.Unmarshal(b.Input, &in)
	opts := ImageGenerationOptions{}
	if o, ok := in["options"].(map[string]any); ok {
		opts.Language = fmt.Sprint(o["language"])
		opts.BackgroundStyle = fmt.Sprint(o["backgroundStyle"])
		opts.KeepSubject = boolFromAny(o["keepSubject"])
		opts.KeepBrandLogo = boolFromAny(o["keepBrandLogo"])
		opts.OutputFormat = fmt.Sprint(o["outputFormat"])
	}
	_ = s.DB.WithContext(ctx).Model(&item).Updates(map[string]any{
		"status": ItemPending, "error_code": "", "error_message": "",
	}).Error
	s.runOneItem(c, &item, opts, adminID)
	var refreshed AIProductImageItem
	_ = s.DB.WithContext(ctx).First(&refreshed, "id = ?", itemID).Error
	var p product.Product
	_ = s.DB.WithContext(ctx).First(&p, "id = ?", refreshed.ProductID).Error
	dto := s.buildItemDTO(ctx, &refreshed, &p)
	return &dto, nil
}

func (s *Service) refreshAppliedCount(ctx context.Context, batchID uuid.UUID) {
	var n int64
	_ = s.DB.WithContext(ctx).Model(&AIProductImageItem{}).
		Where("batch_id = ? AND status = ?", batchID, ItemApplied).Count(&n).Error
	_ = s.DB.WithContext(ctx).Model(&AIProductImageBatch{}).Where("id = ?", batchID).Update("applied_count", n).Error
}
