package inventory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

// AdjustSKUStock updates local SKU snapshot and optionally enqueues platform pushes for eligible mappings.
func (s *Service) AdjustSKUStock(c *gin.Context, productID uuid.UUID, skuID uuid.UUID, body AdjustStockBody, admin *uuid.UUID) (*product.ProductSKU, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("inventory: no db")
	}
	if body.Stock < 0 {
		return nil, fmt.Errorf("stock must be >= 0")
	}
	body.Reason = clampStr(body.Reason, 128)
	body.Remark = clampStr(body.Remark, 520)
	ctx := c.Request.Context()

	var sku product.ProductSKU
	if err := s.DB.WithContext(ctx).First(&sku, "id = ? AND product_id = ?", skuID, productID).Error; err != nil {
		return nil, err
	}
	before := derefStock(sku.Stock)
	delta := body.Stock - before

	tx := s.DB.WithContext(ctx).Begin()
	defer func() {
		if tx != nil {
			_ = tx.Rollback().Error
		}
	}()

	if err := tx.Model(&product.ProductSKU{}).Where("id = ? AND product_id = ?", skuID, productID).
		Updates(map[string]any{"stock": body.Stock, "updated_at": time.Now().UTC()}).Error; err != nil {
		return nil, err
	}
	logRow := InventoryChangeLog{
		ProductID:    productID,
		ProductSKUID: skuID,
		ChangeType:   ChangeManualAdjust,
		BeforeStock:  before,
		AfterStock:   body.Stock,
		Delta:        delta,
		Reason:       body.Reason,
		Remark:       body.Remark,
		CreatedBy:    admin,
	}
	if strings.TrimSpace(logRow.Reason) == "" {
		logRow.Reason = ChangeManualAdjust
	}
	if err := tx.Create(&logRow).Error; err != nil {
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	tx = nil

	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: admin,
			Action:      "inventory.stock.adjust",
			Resource:    "product_sku",
			ResourceID:  skuID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("productId=%s before=%d after=%d delta=%d", productID.String(), before, body.Stock, delta),
		})
	}

	if body.Sync {
		n, syncErr := s.CreateInventorySyncTasksForSKUStock(ctx, productID, skuID, body.Stock, admin)
		if syncErr != nil {
			return nil, syncErr
		}
		if n == 0 {
			return nil, fmt.Errorf("sync requested but no linked publication SKU rows eligible for inventory sync")
		}
	}

	var updated product.ProductSKU
	if err := s.DB.WithContext(ctx).First(&updated, "id = ?", skuID).Error; err != nil {
		return nil, err
	}
	return &updated, nil
}

// CreateInventorySyncTasksForSKUStock enqueues outbound tasks for every mapped publication SKU whose platform supports runnable inventory_sync.
func (s *Service) CreateInventorySyncTasksForSKUStock(ctx context.Context, productID uuid.UUID, skuID uuid.UUID, target int, admin *uuid.UUID) (int, error) {
	return s.enqueueSKUPublicationSyncTasks(ctx, productID, skuID, target, admin, map[string]any{"fromOrderStockWorkflow": true})
}

func (s *Service) enqueueMappingsForSKU(ctx context.Context, productID uuid.UUID, skuID uuid.UUID, target int, admin *uuid.UUID) (int, error) {
	return s.enqueueSKUPublicationSyncTasks(ctx, productID, skuID, target, admin, map[string]any{"fromAdjustStockSync": true})
}

func (s *Service) enqueueSKUPublicationSyncTasks(ctx context.Context, productID uuid.UUID, skuID uuid.UUID, target int, admin *uuid.UUID, opt map[string]any) (int, error) {
	var psRows []productpublish.ProductPublicationSKU
	if err := s.DB.WithContext(ctx).Where("product_sku_id = ?", skuID).Find(&psRows).Error; err != nil {
		return 0, err
	}
	optCopy := platformp.TrimRawMap(opt, 12, 200)
	n := 0
	for _, psku := range psRows {
		if strings.TrimSpace(psku.ExternalSKUID) == "" {
			continue
		}
		var pub productpublish.ProductPublication
		if err := s.DB.WithContext(ctx).Where("id = ? AND product_id = ? AND deleted_at IS NULL", psku.PublicationID, productID).
			First(&pub).Error; err != nil {
			continue
		}
		pl := strings.TrimSpace(strings.ToLower(pub.Platform))
		extPID := strings.TrimSpace(pub.ExternalProductID)
		if extPID == "" && pl != "amazon" {
			continue
		}

		prov := platformp.Get(pl)
		shopRow, auth, err := s.Shops.PlainAuthForProviderCtx(ctx, pub.ShopID)
		if err != nil {
			return n, fmt.Errorf("shop auth: %w", err)
		}
		if err := ValidateShopInventoryPush(shopRow, auth, prov); err != nil {
			// Planned/disabled/mock manual — skip silently
			continue
		}

		pskuIDCopy := psku.ID
		pubIDCopy := pub.ID
		t := &InventorySyncTask{
			ProductID:        productID,
			ProductSKUID:     ptrUUID(skuID),
			PublicationID:    &pubIDCopy,
			PublicationSkuID: &pskuIDCopy,
			ShopID:           pub.ShopID,
			Platform:         pl,
			TaskType:         TaskTypeInventorySync,
			Status:           StatusPending,
			Mode:             ModeManual,
			TargetStock:      target,
			Input:            taskInputSnap(ModeManual, target, psku.ID, ptrUUID(skuID), pub.ID, pub.ShopID, optCopy),
			CreatedBy:        admin,
		}
		if err := s.persistTaskAndMaybeRun(ctx, t, admin); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// CreatePublicationSKUInventoryTask submits one outbound task for linked listing SKU.
func (s *Service) CreatePublicationSKUInventoryTask(c *gin.Context, publicationSkuID uuid.UUID, body PublicationSkuSyncBody, admin *uuid.UUID) (*TaskDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("inventory: no db")
	}
	if body.Stock < 0 {
		return nil, fmt.Errorf("stock must be >= 0")
	}
	opCopy := body.Options
	if opCopy != nil {
		opCopy = platformp.TrimRawMap(opCopy, 12, 200)
	}
	ctx := c.Request.Context()
	var psku productpublish.ProductPublicationSKU
	if err := s.DB.WithContext(ctx).First(&psku, "id = ?", publicationSkuID).Error; err != nil {
		return nil, err
	}
	var pub productpublish.ProductPublication
	if err := s.DB.WithContext(ctx).First(&pub, "id = ?", psku.PublicationID).Error; err != nil {
		return nil, err
	}
	if strings.TrimSpace(psku.ExternalSKUID) == "" {
		return nil, fmt.Errorf("external sku id missing for mapped listing SKU; cannot inventory sync without platform sku id")
	}
	if strings.TrimSpace(pub.ExternalProductID) == "" {
		return nil, fmt.Errorf("external product id missing for publication row")
	}
	shopRow, auth, err := s.Shops.PlainAuthForProviderCtx(ctx, pub.ShopID)
	if err != nil {
		return nil, err
	}
	prov := platformp.Get(pub.Platform)
	if err := ValidateShopInventoryPush(shopRow, auth, prov); err != nil {
		return nil, err
	}
	var prodSku uuid.UUID
	if psku.ProductSKUID != nil {
		prodSku = *psku.ProductSKUID
	} else {
		return nil, fmt.Errorf("listing sku is not linked to a local sku id")
	}
	task := InventorySyncTask{
		ProductID:        pub.ProductID,
		ProductSKUID:     psku.ProductSKUID,
		PublicationID:    &pub.ID,
		PublicationSkuID: &psku.ID,
		ShopID:           pub.ShopID,
		Platform:         strings.TrimSpace(strings.ToLower(pub.Platform)),
		TaskType:         TaskTypeInventorySync,
		Status:           StatusPending,
		Mode:             ModePublication,
		TargetStock:      body.Stock,
		Input:            taskInputSnap(ModePublication, body.Stock, psku.ID, psku.ProductSKUID, pub.ID, pub.ShopID, opCopy),
		CreatedBy:        admin,
	}
	if err := s.persistTaskAndMaybeRun(ctx, &task, admin); err != nil {
		return nil, err
	}
	out, err := s.GetDTO(ctx, task.ID, prodSku, psku.SKUCode)
	return &out, err
}

// CreateProductShopInventoryTasks enqueues outbound tasks using local SKU stock per mapping.
func (s *Service) CreateProductShopInventoryTasks(c *gin.Context, productID uuid.UUID, body ProductBatchInventoryBody, admin *uuid.UUID) ([]TaskDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("inventory: no db")
	}
	shopID, err := uuid.Parse(strings.TrimSpace(body.ShopID))
	if err != nil {
		return nil, fmt.Errorf("invalid shopId")
	}
	if len(body.SKUIDs) == 0 {
		return nil, fmt.Errorf("skuIds required")
	}
	optCopy := platformp.TrimRawMap(body.Options, 12, 200)
	ctx := c.Request.Context()
	var pub productpublish.ProductPublication
	if err := s.DB.WithContext(ctx).Where("product_id = ? AND shop_id = ?", productID, shopID).
		Order("updated_at DESC").First(&pub).Error; err != nil {
		return nil, fmt.Errorf("no publication snapshot for product in this shop: %w", err)
	}
	shopRow, auth, err := s.Shops.PlainAuthForProviderCtx(ctx, shopID)
	if err != nil {
		return nil, err
	}
	prov := platformp.Get(pub.Platform)
	if err := ValidateShopInventoryPush(shopRow, auth, prov); err != nil {
		return nil, err
	}
	outDTOs := make([]TaskDTO, 0, len(body.SKUIDs))
	createdAny := false
	for _, raw := range body.SKUIDs {
		sid, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			continue
		}
		var sku product.ProductSKU
		if err := s.DB.WithContext(ctx).First(&sku, "id = ? AND product_id = ?", sid, productID).Error; err != nil {
			continue
		}
		target := derefStock(sku.Stock)
		var psku productpublish.ProductPublicationSKU
		if err := s.DB.WithContext(ctx).Where("publication_id = ? AND product_sku_id = ?", pub.ID, sid).First(&psku).Error; err != nil {
			continue
		}
		if strings.TrimSpace(psku.ExternalSKUID) == "" || strings.TrimSpace(pub.ExternalProductID) == "" {
			continue
		}
		t := InventorySyncTask{
			ProductID:        productID,
			ProductSKUID:     ptrUUID(sku.ID),
			PublicationID:    &pub.ID,
			PublicationSkuID: &psku.ID,
			ShopID:           shopID,
			Platform:         strings.TrimSpace(strings.ToLower(pub.Platform)),
			TaskType:         TaskTypeInventorySync,
			Status:           StatusPending,
			Mode:             ModeSKU,
			TargetStock:      target,
			Input:            taskInputSnap(ModeSKU, target, psku.ID, ptrUUID(sid), pub.ID, shopID, optCopy),
			CreatedBy:        admin,
		}
		if err := s.persistTaskAndMaybeRun(ctx, &t, admin); err != nil {
			return outDTOs, err
		}
		dto, err := s.GetDTO(ctx, t.ID, sid, psku.SKUCode)
		if err != nil {
			continue
		}
		outDTOs = append(outDTOs, dto)
		createdAny = true
	}
	if !createdAny {
		return nil, fmt.Errorf("no matching publication sku rows or missing external sku ids")
	}
	return outDTOs, nil
}
