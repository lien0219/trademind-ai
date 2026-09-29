package inventory

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
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/repository"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	"gorm.io/datatypes"
)

func (s *Service) shopNameLookup(ctx context.Context, tenantID int64, shopID uuid.UUID) string {
	var sh shop.Shop
	if err := s.DB.WithContext(ctx).First(&sh, "id = ? AND tenant_id = ?", shopID, tenantID).Error; err != nil {
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
		if err := s.DB.WithContext(ctx).Joins("JOIN products ON products.id = product_skus.product_id AND products.deleted_at IS NULL").
			First(&sku, "product_skus.id = ? AND products.tenant_id = ?", *t.ProductSKUID, t.TenantID).Error; err == nil {
			code = sku.SKUCode
		}
	}
	ptitle := title
	if strings.TrimSpace(ptitle) == "" {
		var p product.Product
		if err := s.DB.WithContext(ctx).First(&p, "id = ? AND tenant_id = ?", t.ProductID, t.TenantID).Error; err == nil {
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
		PublicationSKUID: t.PublicationSKUID,
		ShopID:           t.ShopID,
		ShopName:         s.shopNameLookup(ctx, t.TenantID, t.ShopID),
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
		BatchID:          t.BatchID,
		BatchNo:          t.BatchNo,
		CreatedAt:        t.CreatedAt,
		UpdatedAt:        t.UpdatedAt,
	}
}

// GetDTO returns one task envelope with tenant scope.
func (s *Service) GetDTO(ctx context.Context, tenantID int64, id uuid.UUID, skuUUID uuid.UUID, skuCode string) (TaskDTO, error) {
	var zero TaskDTO
	var t InventorySyncTask
	if err := repository.FindByID(ctx, s.DB, &t, tenantID, id); err != nil {
		return zero, err
	}
	title := ""
	var p product.Product
	if err := s.DB.WithContext(ctx).First(&p, "id = ? AND tenant_id = ?", t.ProductID, tenantID).Error; err == nil {
		title = p.Title
	}
	hint := skuCode
	if strings.TrimSpace(hint) == "" && skuUUID != uuid.Nil {
		var sku product.ProductSKU
		if err := s.DB.WithContext(ctx).Joins("JOIN products ON products.id = product_skus.product_id AND products.deleted_at IS NULL").
			First(&sku, "product_skus.id = ? AND products.tenant_id = ?", skuUUID, tenantID).Error; err == nil {
			hint = sku.SKUCode
		}
	}
	return s.taskToDTO(ctx, &t, hint, title), nil
}

// ListPublicationSKUs lists listing SKU rows mapped to one product draft.
func (s *Service) ListPublicationSKUs(ctx context.Context, tenantID int64, productID uuid.UUID, productSKUFilter *uuid.UUID) ([]PublicationSKUListingRow, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("inventory: no db")
	}
	type rowLite struct {
		ID                uuid.UUID  `gorm:"column:id"`
		PublicationID     uuid.UUID  `gorm:"column:publication_id"`
		ProductSKUIDCol   *uuid.UUID `gorm:"column:product_sku_id"`
		ExternalSKU       string     `gorm:"column:external_sku_id"`
		SKUCode           string     `gorm:"column:sku_code"`
		Stock             *int       `gorm:"column:stock"`
		BindStatus        string     `gorm:"column:bind_status"`
		BindConfidence    int        `gorm:"column:bind_confidence"`
		BindMessage       string     `gorm:"column:bind_message"`
		LastSyncedAt      *time.Time `gorm:"column:last_synced_at"`
		ShopUID           uuid.UUID  `gorm:"column:shop_uid"`
		ShopName          string     `gorm:"column:shop_name"`
		PlatformRaw       string     `gorm:"column:plat"`
		ExternalProductID string     `gorm:"column:ext_pid"`
	}
	tx := s.DB.WithContext(ctx).Table("product_publication_skus AS ps").
		Select(`ps.id, ps.publication_id, ps.product_sku_id, ps.external_sku_id, ps.sku_code, ps.stock,
			ps.bind_status, ps.bind_confidence, ps.bind_message, ps.last_synced_at,
			pp.shop_id AS shop_uid, sh.shop_name, pp.platform AS plat, pp.external_product_id AS ext_pid`).
		Joins(`JOIN product_publications pp ON pp.id = ps.publication_id AND pp.product_id = ? AND pp.deleted_at IS NULL`, productID).
		Joins(`JOIN products p ON p.id = pp.product_id AND p.tenant_id = ? AND p.deleted_at IS NULL`, tenantID).
		Joins(`JOIN shops sh ON sh.id = pp.shop_id AND sh.deleted_at IS NULL AND sh.tenant_id = ?`, tenantID)
	if productSKUFilter != nil && *productSKUFilter != uuid.Nil {
		tx = tx.Where("ps.product_sku_id = ?", *productSKUFilter)
	}
	var rows []rowLite
	if err := tx.Order("pp.updated_at DESC, ps.created_at ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]PublicationSKUListingRow, 0, len(rows))
	for _, r := range rows {
		pl := strings.TrimSpace(strings.ToLower(r.PlatformRaw))
		prov := platformp.Get(pl)
		cap := ""
		if prov != nil {
			cap = platformp.ImplementationStatusForCapability(prov, platformp.CapInventorySync)
		}
		out = append(out, PublicationSKUListingRow{
			PublicationSKUID:  r.ID,
			PublicationID:     r.PublicationID,
			ProductSKUID:      r.ProductSKUIDCol,
			ShopID:            r.ShopUID,
			ShopName:          r.ShopName,
			Platform:          pl,
			ExternalProductID: r.ExternalProductID,
			ExternalSKUID:     r.ExternalSKU,
			SKUCode:           r.SKUCode,
			PlatformStock:     r.Stock,
			InventoryCap:      cap,
			BindStatus:        strings.TrimSpace(r.BindStatus),
			BindConfidence:    r.BindConfidence,
			BindMessage:       strings.TrimSpace(r.BindMessage),
			LastSyncedAt:      r.LastSyncedAt,
		})
	}
	return out, nil
}

// ListSKUChangeLogs pages ledger rows for one SKU snapshot line.
func (s *Service) ListSKUChangeLogs(ctx context.Context, tenantID int64, productID uuid.UUID, skuID uuid.UUID, page, ps int) (*PaginatedLogs, error) {
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
		Where("tenant_id = ? AND product_id = ? AND product_sku_id = ?", tenantID, productID, skuID)
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
			ID:             r.ID,
			CreatedAt:      r.CreatedAt,
			ChangeType:     r.ChangeType,
			BeforeStock:    r.BeforeStock,
			AfterStock:     r.AfterStock,
			Delta:          r.Delta,
			Reason:         r.Reason,
			Remark:         r.Remark,
			CreatedBy:      r.CreatedBy,
			RefOrderID:     r.RefOrderID,
			RefOrderItemID: r.RefOrderItemID,
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
	tx = tx.Where("tenant_id = ?", q.TenantID)
	if q.ProductID != nil && *q.ProductID != uuid.Nil {
		tx = tx.Where("product_id = ?", *q.ProductID)
	}
	if q.ProductSKUID != nil && *q.ProductSKUID != uuid.Nil {
		tx = tx.Where("product_sku_id = ?", *q.ProductSKUID)
	}
	if q.RefOrderID != nil && *q.RefOrderID != uuid.Nil {
		tx = tx.Where("ref_order_id = ?", *q.RefOrderID)
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
			ID:             r.ID,
			CreatedAt:      r.CreatedAt,
			ChangeType:     r.ChangeType,
			BeforeStock:    r.BeforeStock,
			AfterStock:     r.AfterStock,
			Delta:          r.Delta,
			Reason:         r.Reason,
			Remark:         r.Remark,
			CreatedBy:      r.CreatedBy,
			RefOrderID:     r.RefOrderID,
			RefOrderItemID: r.RefOrderItemID,
		})
	}
	return &PaginatedLogs{Items: items, Total: total, Page: page, PageSize: ps, TotalPages: pagesOf(total, ps)}, nil
}

// ListTasks paginates outbound rows with tenant scope.
func (s *Service) ListTasks(c *gin.Context, q ListQuery) (*ListTasksResult, error) {
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
	tx := s.DB.WithContext(c.Request.Context()).Model(&InventorySyncTask{})
	if scoped, _, err := adminperm.ApplyTenantScope(c, tx); err != nil {
		return nil, err
	} else {
		tx = scoped
	}
	if q.ProductID != nil && *q.ProductID != uuid.Nil {
		tx = tx.Where("product_id = ?", *q.ProductID)
	}
	if q.ProductSKUID != nil && *q.ProductSKUID != uuid.Nil {
		tx = tx.Where("product_sku_id = ?", *q.ProductSKUID)
	}
	if q.ShopID != nil && *q.ShopID != uuid.Nil {
		tx = tx.Where("shop_id = ?", *q.ShopID)
		if scoped, err := adminperm.ApplyStoreScope(c, s.DB, tx, "shop_id"); err != nil {
			return nil, err
		} else {
			tx = scoped
		}
	}
	if q.BatchID != nil && *q.BatchID != uuid.Nil {
		tx = tx.Where("batch_id = ?", *q.BatchID)
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
	ctx := c.Request.Context()
	titleMap := map[uuid.UUID]string{}
	items := make([]TaskDTO, 0, len(rows))
	for _, t := range rows {
		title := titleMap[t.ProductID]
		if strings.TrimSpace(title) == "" {
			var p product.Product
			if err := s.DB.WithContext(ctx).First(&p, "id = ? AND tenant_id = ?", t.ProductID, t.TenantID).Error; err == nil {
				title = p.Title
				titleMap[t.ProductID] = title
			}
		}
		skuHint := ""
		if t.ProductSKUID != nil {
			var sku product.ProductSKU
			if err := s.DB.WithContext(ctx).Joins("JOIN products ON products.id = product_skus.product_id AND products.deleted_at IS NULL").
				First(&sku, "product_skus.id = ? AND products.tenant_id = ?", *t.ProductSKUID, t.TenantID).Error; err == nil {
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

// retryInventorySyncTaskScoped retries with explicit tenant (internal/batch use).
func (s *Service) retryInventorySyncTaskScoped(ctx context.Context, tenantID int64, taskID uuid.UUID, admin *uuid.UUID) (*TaskDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("inventory: no db")
	}
	var task InventorySyncTask
	if err := repository.FindByID(ctx, s.DB, &task, tenantID, taskID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(task.Status) != StatusFailed {
		return nil, fmt.Errorf("only failed tasks can be retried")
	}
	reset := time.Now().UTC()
	if err := repository.UpdateByID(ctx, s.DB, &InventorySyncTask{}, tenantID, taskID, map[string]any{
		"status":        StatusPending,
		"error_message": "",
		"started_at":    nil,
		"finished_at":   nil,
		"output":        datatypes.JSON(nil),
		"locked_by":     nil,
		"locked_until":  nil,
		"updated_at":    reset,
	}); err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: admin,
			Action:      "inventory.sync.retry",
			Resource:    "inventory_sync_task",
			ResourceID:  taskID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("taskId=%s shopId=%s platform=%s", taskID.String(), task.ShopID.String(), task.Platform),
		})
	}
	if err := s.enqueueOrRunInventoryTask(ctx, taskID); err != nil {
		return nil, err
	}
	var skuUUID uuid.UUID
	if task.ProductSKUID != nil {
		skuUUID = *task.ProductSKUID
	}
	out, err := s.GetDTO(ctx, tenantID, taskID, skuUUID, "")
	return &out, err
}

// RetryInventorySyncTask resets a failed task to pending and enqueues / runs inline.
func (s *Service) RetryInventorySyncTask(c *gin.Context, taskID uuid.UUID, admin *uuid.UUID) (*TaskDTO, error) {
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	return s.retryInventorySyncTaskScoped(c.Request.Context(), tid, taskID, admin)
}

// RetryFailed requeues failed rows preserving Input snapshot.
func (s *Service) RetryFailed(c *gin.Context, taskID uuid.UUID, admin *uuid.UUID) (*TaskDTO, error) {
	return s.RetryInventorySyncTask(c, taskID, admin)
}
