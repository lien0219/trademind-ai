package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	douyinmetrics "github.com/trademind-ai/trademind/backend/internal/metrics/douyin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	"gorm.io/datatypes"
)

func (s *Service) appendChange(ctx context.Context, tenantID int64, productID uuid.UUID, skuID uuid.UUID, typ string,
	before int, after int, delta int, reason string, remark string, admin *uuid.UUID,
) {
	if s == nil || s.DB == nil {
		return
	}
	row := InventoryChangeLog{
		TenantID:     tenantID,
		ProductID:    productID,
		ProductSKUID: skuID,
		ChangeType:   typ,
		BeforeStock:  before,
		AfterStock:   after,
		Delta:        delta,
		Reason:       clampStr(reason, 128),
		Remark:       clampStr(remark, 520),
		CreatedBy:    admin,
	}
	_ = s.DB.WithContext(ctx).Create(&row).Error
}

// ProcessQueuedTask executes one outbound sync with DB leases + changelog side effects.
func (s *Service) ProcessQueuedTask(ctx context.Context, taskID uuid.UUID, workerID string) error {
	start := time.Now()
	if s == nil || s.DB == nil {
		return fmt.Errorf("inventory: no db")
	}
	defer func() {
		if r := recover(); r != nil {
			s.handleInventoryPanic(ctx, taskID, workerID, r)
		}
	}()
	lease := s.inventoryLeaseTTL()
	taskRow, claim, ok, err := s.tryClaimInventorySyncTask(ctx, taskID, workerID, lease)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	if err := s.guardDouyinInventoryWorker(ctx, taskID, taskRow); err != nil {
		return err
	}
	s.InventoryRateObserveStarted(ctx, taskRow.Platform)
	stop := s.startInventoryLeaseRenewal(ctx, taskID, workerID, claim, lease)
	defer stop()

	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: taskRow.CreatedBy,
			Action:      "inventory.sync.running",
			Resource:    "inventory_sync_task",
			ResourceID:  taskID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("taskId=%s shopId=%s platform=%s", taskID.String(), taskRow.ShopID.String(), taskRow.Platform),
		})
		if strings.TrimSpace(strings.ToLower(taskRow.Platform)) == "douyin_shop" {
			_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
				AdminUserID: taskRow.CreatedBy,
				Action:      "douyin.inventory.sync.start",
				Resource:    "inventory_sync_task",
				ResourceID:  taskID.String(),
				Status:      "success",
				Message:     fmt.Sprintf("taskId=%s shopId=%s target=%d", taskID.String(), taskRow.ShopID.String(), taskRow.TargetStock),
			})
		}
	}

	fail := func(msg string) error {
		fin := time.Now().UTC()
		_ = s.finishInventorySyncTask(ctx, taskID, workerID, claim, map[string]any{
			"status":        StatusFailed,
			"error_message": clampStr(msg, 4000),
			"finished_at":   &fin,
		})
		if taskRow.ProductSKUID != nil && *taskRow.ProductSKUID != uuid.Nil {
			pskuSnap := snapshotPublicationSKUStock(ctx, s, taskRow)
			beforePL := derefStock(pskuSnap.stockPtr)
			s.appendChange(ctx, taskRow.TenantID, taskRow.ProductID, *taskRow.ProductSKUID, ChangeSyncFailed, beforePL, beforePL, 0, "inventory_sync_failed", clampStr(msg, 520), taskRow.CreatedBy)
		}
		if s.OpLog != nil {
			_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
				AdminUserID: taskRow.CreatedBy,
				Action:      "inventory.sync.failed",
				Resource:    "inventory_sync_task",
				ResourceID:  taskID.String(),
				Status:      "failed",
				Message:     fmt.Sprintf("taskId=%s shopId=%s platform=%s err=%s", taskID.String(), taskRow.ShopID.String(), taskRow.Platform, clampStr(msg, 400)),
			})
			if strings.TrimSpace(strings.ToLower(taskRow.Platform)) == "douyin_shop" {
				_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
					AdminUserID: taskRow.CreatedBy,
					Action:      "douyin.inventory.sync.failed",
					Resource:    "inventory_sync_task",
					ResourceID:  taskID.String(),
					Status:      "failed",
					Message:     fmt.Sprintf("taskId=%s shopId=%s err=%s", taskID.String(), taskRow.ShopID.String(), clampStr(msg, 400)),
				})
				if taskRow.ProductSKUID != nil {
					_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
						AdminUserID: taskRow.CreatedBy,
						Action:      "douyin.inventory.sku.failed",
						Resource:    "product_sku",
						ResourceID:  taskRow.ProductSKUID.String(),
						Status:      "failed",
						Message:     fmt.Sprintf("taskId=%s err=%s", taskID.String(), clampStr(msg, 400)),
					})
				}
			}
		}
		if strings.TrimSpace(strings.ToLower(taskRow.Platform)) == "douyin_shop" {
			douyinmetrics.RecordInventorySync("failed")
		}
		s.ObserveInventory(taskRow.Platform, "push", "push_failure", "failure", classifyInventoryError(msg), 1, time.Since(start))
		s.maybeReconcileInventoryBatch(ctx, taskRow.BatchID)
		return fmt.Errorf("%s", msg)
	}

	if taskRow.ProductSKUID == nil || *taskRow.ProductSKUID == uuid.Nil {
		return fail("task missing product SKU binding")
	}
	skuUUID := *taskRow.ProductSKUID
	var sku product.ProductSKU
	if err := s.DB.WithContext(ctx).Joins("JOIN products ON products.id = product_skus.product_id AND products.deleted_at IS NULL").
		First(&sku, "product_skus.id = ? AND product_skus.product_id = ? AND products.tenant_id = ?", skuUUID, taskRow.ProductID, taskRow.TenantID).Error; err != nil {
		return fail("product sku not found")
	}

	if taskRow.PublicationSKUID == nil || *taskRow.PublicationSKUID == uuid.Nil {
		return fail("missing publication sku id")
	}
	var psku productpublish.ProductPublicationSKU
	if err := s.DB.WithContext(ctx).Joins("JOIN product_publications pp ON pp.id = product_publication_skus.publication_id AND pp.deleted_at IS NULL").
		Joins("JOIN products p ON p.id = pp.product_id AND p.deleted_at IS NULL").
		First(&psku, "product_publication_skus.id = ? AND pp.product_id = ? AND p.tenant_id = ?", *taskRow.PublicationSKUID, taskRow.ProductID, taskRow.TenantID).Error; err != nil {
		return fail("listing sku row not found")
	}
	var pub productpublish.ProductPublication
	if err := s.DB.WithContext(ctx).Where("id = ? AND product_id = ? AND deleted_at IS NULL", psku.PublicationID, taskRow.ProductID).First(&pub).Error; err != nil {
		return fail("publication snapshot not found")
	}
	pl := strings.TrimSpace(strings.ToLower(taskRow.Platform))
	extPID := strings.TrimSpace(pub.ExternalProductID)
	extSK := strings.TrimSpace(psku.ExternalSKUID)
	if extSK == "" {
		return fail("external sku id missing")
	}
	if extPID == "" && pl != "amazon" {
		return fail("external product id missing")
	}

	var scopedShop shop.Shop
	if err := s.DB.WithContext(ctx).Where("id = ? AND tenant_id = ? AND deleted_at IS NULL", taskRow.ShopID, taskRow.TenantID).First(&scopedShop).Error; err != nil {
		return fail("shop not found")
	}
	shopRow, auth, err := s.Shops.PlainAuthForProviderCtx(ctx, scopedShop.ID)
	if err != nil {
		return fail(err.Error())
	}
	prov := platformp.Get(strings.TrimSpace(taskRow.Platform))
	if err := ValidateShopInventoryPush(shopRow, auth, prov); err != nil {
		return fail(err.Error())
	}
	syncer, okProv := platformp.AsInventorySync(prov)
	if !okProv || syncer == nil {
		return fail("inventory sync adapter missing")
	}

	options := map[string]any{}
	if len(taskRow.Input) > 0 {
		var envelope map[string]any
		_ = json.Unmarshal(taskRow.Input, &envelope)
		if raw, exists := envelope["options"]; exists {
			switch t := raw.(type) {
			case map[string]any:
				options = platformp.TrimRawMap(t, 16, 200)
			}
		}
	}

	timeout := s.TaskTimeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	res, err := syncer.SyncInventory(runCtx, platformp.SyncInventoryRequest{
		ShopID:            taskRow.ShopID,
		Platform:          pl,
		Auth:              auth,
		PublicationID:     pub.ID,
		PublicationSKUID:  psku.ID,
		ExternalProductID: extPID,
		ExternalSKUID:     extSK,
		SKUCode:           strings.TrimSpace(sku.SKUCode),
		Stock:             taskRow.TargetStock,
		Options:           options,
	})
	if err != nil {
		if isInventoryTimeout(err.Error()) {
			s.ObserveInventory(taskRow.Platform, "push", "unknown_result", "unknown_result", "provider_timeout", 1, time.Since(start))
		}
		return fail(err.Error())
	}
	beforeMirror := derefStock(psku.Stock)
	st := strings.TrimSpace(res.Status)
	stOK := strings.EqualFold(st, "success") || st == ""
	if !stOK {
		return fail(fmt.Sprintf("provider reported non-success status: %s", clampStr(st, 64)))
	}
	if got := strings.TrimSpace(res.ExternalSKUID); got != "" && !strings.EqualFold(got, extSK) {
		return fail("provider sku id mismatch in response summary")
	}

	stockOut := res.Stock
	stkPtr := stockOut
	tx := s.DB.WithContext(ctx).Begin()
	if err := tx.Model(&productpublish.ProductPublicationSKU{}).Where("id = ?", psku.ID).
		Updates(map[string]any{"stock": &stkPtr, "updated_at": time.Now().UTC()}).Error; err != nil {
		_ = tx.Rollback().Error
		return fail("persist listing sku stock failed")
	}
	if err := tx.Commit().Error; err != nil {
		return fail("commit listing sku stock failed")
	}

	sum := platformp.TrimRawMap(res.RawSummary, 12, 200)
	payload := map[string]any{
		"status":          "success",
		"appliedStock":    stockOut,
		"skuCode":         strings.TrimSpace(sku.SKUCode),
		"providerSummary": sum,
	}
	outJSON, _ := json.Marshal(payload)
	fin := time.Now().UTC()
	_ = s.finishInventorySyncTask(ctx, taskID, workerID, claim, map[string]any{
		"status":        StatusSuccess,
		"finished_at":   &fin,
		"output":        datatypes.JSON(outJSON),
		"error_message": "",
	})

	delta := stockOut - beforeMirror
	s.appendChange(ctx, taskRow.TenantID, taskRow.ProductID, skuUUID, ChangeSyncSuccess, beforeMirror, stockOut, delta, "inventory_sync_success", fmt.Sprintf("task=%s platform=%s", taskID.String(), pl), taskRow.CreatedBy)

	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: taskRow.CreatedBy,
			Action:      "inventory.sync.success",
			Resource:    "inventory_sync_task",
			ResourceID:  taskID.String(),
			Status:      "success",
			Message: fmt.Sprintf("taskId=%s shop=%s sku=%s target=%d mirrored=%d",
				taskID.String(), taskRow.ShopID.String(), extSK, taskRow.TargetStock, stockOut),
		})
		if strings.TrimSpace(strings.ToLower(taskRow.Platform)) == "douyin_shop" {
			_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
				AdminUserID: taskRow.CreatedBy,
				Action:      "douyin.inventory.sync.success",
				Resource:    "inventory_sync_task",
				ResourceID:  taskID.String(),
				Status:      "success",
				Message: fmt.Sprintf("taskId=%s shop=%s sku=%s target=%d",
					taskID.String(), taskRow.ShopID.String(), extSK, taskRow.TargetStock),
			})
			douyinmetrics.RecordInventorySync("success")
		}
	}
	s.ObserveInventory(taskRow.Platform, "push", "push", "success", "", 1, time.Since(start))
	s.maybeReconcileInventoryBatch(ctx, taskRow.BatchID)
	return nil
}

type skuStockSnap struct {
	stockPtr *int
}

func snapshotPublicationSKUStock(ctx context.Context, s *Service, task *InventorySyncTask) skuStockSnap {
	out := skuStockSnap{}
	if s == nil || task == nil || task.PublicationSKUID == nil {
		return out
	}
	var psku productpublish.ProductPublicationSKU
	if err := s.DB.WithContext(ctx).Joins("JOIN product_publications pp ON pp.id = product_publication_skus.publication_id AND pp.deleted_at IS NULL").
		Joins("JOIN products p ON p.id = pp.product_id AND p.deleted_at IS NULL").
		First(&psku, "product_publication_skus.id = ? AND p.tenant_id = ?", *task.PublicationSKUID, task.TenantID).Error; err != nil {
		return out
	}
	out.stockPtr = psku.Stock
	return out
}
