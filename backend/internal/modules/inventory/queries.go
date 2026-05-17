package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/modules/worker"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	"gorm.io/datatypes"
)

func (s *Service) shopNameLookup(ctx context.Context, shopID uuid.UUID) string {
	var sh shop.Shop
	if err := s.DB.WithContext(ctx).First(&sh, "id = ?", shopID).Error; err != nil {
		return ""
	}
	return sh.ShopName
}

func (s *Service) taskToDTO(ctx context.Context, t *InventorySyncTask, skuHint string, title string) TaskDTO {
	var input any
	if len(t.Input) > 0 {
		_ = json.Unmarshal(t.Input, &input)
	}
	var output any
	if len(t.Output) > 0 {
		_ = json.Unmarshal(t.Output, &output)
	}
	code := skuHint
	if strings.TrimSpace(code) == "" && t.ProductSKUID != nil && *t.ProductSKUID != uuid.Nil {
		var sku product.ProductSKU
		if err := s.DB.WithContext(ctx).First(&sku, "id = ?", *t.ProductSKUID).Error; err == nil {
			code = sku.SKUCode
		}
	}
	ptitle := title
	if strings.TrimSpace(ptitle) == "" {
		var p product.Product
		if err := s.DB.WithContext(ctx).First(&p, "id = ?", t.ProductID).Error; err == nil {
			ptitle = p.Title
		}
	}
	return TaskDTO{
		ID:               t.ID,
		ProductID:        t.ProductID,
		ProductTitle:     ptitle,
		ProductSKUID:     t.ProductSKUID,
		SKUCode:          code,
		PublicationID:    t.PublicationID,
		PublicationSkuID: t.PublicationSkuID,
		ShopID:           t.ShopID,
		ShopName:         s.shopNameLookup(ctx, t.ShopID),
		Platform:         t.Platform,
		TaskType:         t.TaskType,
		Status:           t.Status,
		Mode:             t.Mode,
		TargetStock:      t.TargetStock,
		StartedAt:        t.StartedAt,
		FinishedAt:       t.FinishedAt,
		ErrorMessage:     t.ErrorMessage,
		Input:            input,
		Output:           output,
		CreatedBy:        t.CreatedBy,
		CreatedAt:        t.CreatedAt,
		UpdatedAt:        t.UpdatedAt,
	}
}

// GetDTO returns one task envelope.
func (s *Service) GetDTO(ctx context.Context, id uuid.UUID, skuUUID uuid.UUID, skuCode string) (TaskDTO, error) {
	var zero TaskDTO
	var t InventorySyncTask
	if err := s.DB.WithContext(ctx).First(&t, "id = ?", id).Error; err != nil {
		return zero, err
	}
	title := ""
	var p product.Product
	if err := s.DB.WithContext(ctx).First(&p, "id = ?", t.ProductID).Error; err == nil {
		title = p.Title
	}
	hint := skuCode
	if strings.TrimSpace(hint) == "" && skuUUID != uuid.Nil {
		var sku product.ProductSKU
		if err := s.DB.WithContext(ctx).First(&sku, "id = ?", skuUUID).Error; err == nil {
			hint = sku.SKUCode
		}
	}
	return s.taskToDTO(ctx, &t, hint, title), nil
}

// ListPublicationSkus lists listing SKU rows mapped to one product draft.
func (s *Service) ListPublicationSkus(ctx context.Context, productID uuid.UUID, productSkuFilter *uuid.UUID) ([]PublicationSkuListingRow, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("inventory: no db")
	}
	type rowLite struct {
		ID                uuid.UUID  `gorm:"column:id"`
		PublicationID     uuid.UUID  `gorm:"column:publication_id"`
		ProductSkuIDCol   *uuid.UUID `gorm:"column:product_sku_id"`
		ExternalSKU       string     `gorm:"column:external_sku_id"`
		SKUCode           string     `gorm:"column:sku_code"`
		Stock             *int       `gorm:"column:stock"`
		ShopUID           uuid.UUID  `gorm:"column:shop_uid"`
		ShopName          string     `gorm:"column:shop_name"`
		PlatformRaw       string     `gorm:"column:plat"`
		ExternalProductID string     `gorm:"column:ext_pid"`
	}
	tx := s.DB.WithContext(ctx).Table("product_publication_skus AS ps").
		Select(`ps.id, ps.publication_id, ps.product_sku_id, ps.external_sku_id, ps.sku_code, ps.stock,
			pp.shop_id AS shop_uid, sh.shop_name, pp.platform AS plat, pp.external_product_id AS ext_pid`).
		Joins(`JOIN product_publications pp ON pp.id = ps.publication_id AND pp.product_id = ? AND pp.deleted_at IS NULL`, productID).
		Joins(`JOIN shops sh ON sh.id = pp.shop_id`)
	if productSkuFilter != nil && *productSkuFilter != uuid.Nil {
		tx = tx.Where("ps.product_sku_id = ?", *productSkuFilter)
	}
	var rows []rowLite
	if err := tx.Order("pp.updated_at DESC, ps.created_at ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]PublicationSkuListingRow, 0, len(rows))
	for _, r := range rows {
		pl := strings.TrimSpace(strings.ToLower(r.PlatformRaw))
		prov := platformp.Get(pl)
		cap := ""
		if prov != nil {
			cap = platformp.ImplementationStatusForCapability(prov, platformp.CapInventorySync)
		}
		out = append(out, PublicationSkuListingRow{
			PublicationSKUID:  r.ID,
			PublicationID:     r.PublicationID,
			ProductSKUID:      r.ProductSkuIDCol,
			ShopID:            r.ShopUID,
			ShopName:          r.ShopName,
			Platform:          pl,
			ExternalProductID: r.ExternalProductID,
			ExternalSKUID:     r.ExternalSKU,
			SKUCode:           r.SKUCode,
			PlatformStock:     r.Stock,
			InventoryCap:      cap,
		})
	}
	return out, nil
}

// ListSKUChangeLogs pages ledger rows for one SKU snapshot line.
func (s *Service) ListSKUChangeLogs(ctx context.Context, productID uuid.UUID, skuID uuid.UUID, page, ps int) (*PaginatedLogs, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("inventory: no db")
	}
	if page < 1 {
		page = 1
	}
	if ps < 1 || ps > 100 {
		ps = 20
	}
	tx := s.DB.WithContext(ctx).Model(&InventoryChangeLog{}).
		Where("product_id = ? AND product_sku_id = ?", productID, skuID)
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}
	offset := (page - 1) * ps
	var rows []InventoryChangeLog
	if err := tx.Order("created_at DESC").Offset(offset).Limit(ps).Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]ChangeLogDTO, 0, len(rows))
	for _, r := range rows {
		items = append(items, ChangeLogDTO{
			ID:          r.ID,
			CreatedAt:   r.CreatedAt,
			ChangeType:  r.ChangeType,
			BeforeStock: r.BeforeStock,
			AfterStock:  r.AfterStock,
			Delta:       r.Delta,
			Reason:      r.Reason,
			Remark:      r.Remark,
			CreatedBy:   r.CreatedBy,
		})
	}
	return &PaginatedLogs{Items: items, Total: total, Page: page, PageSize: ps, TotalPages: pagesOf(total, ps)}, nil
}

// ListGlobalLogs audits inventory_change_logs across SKUs.
func (s *Service) ListGlobalLogs(ctx context.Context, q GlobalLogsQuery) (*PaginatedLogs, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("inventory: no db")
	}
	page := q.Page
	ps := q.PageSize
	if page < 1 {
		page = 1
	}
	if ps < 1 || ps > 100 {
		ps = 20
	}
	tx := s.DB.WithContext(ctx).Model(&InventoryChangeLog{})
	if q.ProductID != nil && *q.ProductID != uuid.Nil {
		tx = tx.Where("product_id = ?", *q.ProductID)
	}
	if q.ProductSKUID != nil && *q.ProductSKUID != uuid.Nil {
		tx = tx.Where("product_sku_id = ?", *q.ProductSKUID)
	}
	if strings.TrimSpace(q.ChangeType) != "" {
		tx = tx.Where("change_type = ?", strings.TrimSpace(q.ChangeType))
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
	var rows []InventoryChangeLog
	if err := tx.Order("created_at DESC").Offset(offset).Limit(ps).Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]ChangeLogDTO, 0, len(rows))
	for _, r := range rows {
		items = append(items, ChangeLogDTO{
			ID:          r.ID,
			CreatedAt:   r.CreatedAt,
			ChangeType:  r.ChangeType,
			BeforeStock: r.BeforeStock,
			AfterStock:  r.AfterStock,
			Delta:       r.Delta,
			Reason:      r.Reason,
			Remark:      r.Remark,
			CreatedBy:   r.CreatedBy,
		})
	}
	return &PaginatedLogs{Items: items, Total: total, Page: page, PageSize: ps, TotalPages: pagesOf(total, ps)}, nil
}

// ListTasks paginates outbound rows.
func (s *Service) ListTasks(ctx context.Context, q ListQuery) (*ListTasksResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("inventory: no db")
	}
	page := q.Page
	ps := q.PageSize
	if page < 1 {
		page = 1
	}
	if ps < 1 || ps > 100 {
		ps = 20
	}
	tx := s.DB.WithContext(ctx).Model(&InventorySyncTask{})
	if q.ProductID != nil && *q.ProductID != uuid.Nil {
		tx = tx.Where("product_id = ?", *q.ProductID)
	}
	if q.ProductSKUID != nil && *q.ProductSKUID != uuid.Nil {
		tx = tx.Where("product_sku_id = ?", *q.ProductSKUID)
	}
	if q.ShopID != nil && *q.ShopID != uuid.Nil {
		tx = tx.Where("shop_id = ?", *q.ShopID)
	}
	if strings.TrimSpace(q.Platform) != "" {
		tx = tx.Where("platform = ?", strings.TrimSpace(strings.ToLower(q.Platform)))
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
	var rows []InventorySyncTask
	if err := tx.Order("created_at DESC").Offset(offset).Limit(ps).Find(&rows).Error; err != nil {
		return nil, err
	}
	titleMap := map[uuid.UUID]string{}
	items := make([]TaskDTO, 0, len(rows))
	for _, t := range rows {
		title := titleMap[t.ProductID]
		if strings.TrimSpace(title) == "" {
			var p product.Product
			if err := s.DB.WithContext(ctx).First(&p, "id = ?", t.ProductID).Error; err == nil {
				title = p.Title
				titleMap[t.ProductID] = title
			}
		}
		skuHint := ""
		if t.ProductSKUID != nil {
			var sku product.ProductSKU
			if err := s.DB.WithContext(ctx).First(&sku, "id = ?", *t.ProductSKUID).Error; err == nil {
				skuHint = sku.SKUCode
			}
		}
		items = append(items, s.taskToDTO(ctx, &t, skuHint, title))
	}
	return &ListTasksResult{
		Items:      items,
		Total:      total,
		Page:       page,
		PageSize:   ps,
		TotalPages: pagesOf(total, ps),
	}, nil
}

// RetryFailed requeues failed rows preserving Input snapshot.
func (s *Service) RetryFailed(c *gin.Context, taskID uuid.UUID, admin *uuid.UUID) (*TaskDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("inventory: no db")
	}
	var task InventorySyncTask
	if err := s.DB.WithContext(c.Request.Context()).First(&task, "id = ?", taskID).Error; err != nil {
		return nil, err
	}
	if strings.TrimSpace(task.Status) != StatusFailed {
		return nil, fmt.Errorf("only failed tasks can be retried")
	}
	reset := time.Now().UTC()
	if err := s.DB.WithContext(c.Request.Context()).Model(&InventorySyncTask{}).Where("id = ?", taskID).
		Updates(map[string]any{
			"status":        StatusPending,
			"error_message": "",
			"started_at":    nil,
			"finished_at":   nil,
			"output":        datatypes.JSON(nil),
			"locked_by":     nil,
			"locked_until":  nil,
			"updated_at":    reset,
		}).Error; err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: admin,
			Action:      "inventory.sync.retry",
			Resource:    "inventory_sync_task",
			ResourceID:  taskID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("taskId=%s shopId=%s platform=%s", taskID.String(), task.ShopID.String(), task.Platform),
		})
	}
	runInline := func() error {
		return s.ProcessQueuedTask(context.Background(), taskID, worker.GenerateInlineWorkerID(worker.TypeInventorySync))
	}
	if s.QueueEnabled && s.Redis != nil && s.Redis.Client != nil {
		if err := s.enqueue(c.Request.Context(), taskID); err != nil {
			slog.Warn("inventory_sync_retry_enqueue_failed_run_inline", "taskId", taskID.String(), "error", err)
			if err := runInline(); err != nil {
				return nil, err
			}
		}
	} else if err := runInline(); err != nil {
		return nil, err
	}
	var skuUuid uuid.UUID
	if task.ProductSKUID != nil {
		skuUuid = *task.ProductSKUID
	}
	out, err := s.GetDTO(c.Request.Context(), taskID, skuUuid, "")
	return &out, err
}
