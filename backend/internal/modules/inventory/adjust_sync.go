package inventory

import (
	"context"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
)

// CreateInventorySyncTasksForSKUStock enqueues outbound tasks for every mapped publication SKU whose platform supports runnable inventory_sync.
func (s *Service) CreateInventorySyncTasksForSKUStock(ctx context.Context, productID uuid.UUID, skuID uuid.UUID, target int, admin *uuid.UUID) (int, error) {
	return s.enqueueSKUPublicationSyncTasks(ctx, productID, skuID, target, admin, map[string]any{"fromOrderStockWorkflow": true})
}

func (s *Service) enqueueMappingsForSKU(ctx context.Context, productID uuid.UUID, skuID uuid.UUID, target int, admin *uuid.UUID) (int, error) {
	return s.enqueueSKUPublicationSyncTasks(ctx, productID, skuID, target, admin, map[string]any{"fromAdjustStockSync": true})
}

func (s *Service) enqueueSKUPublicationSyncTasks(ctx context.Context, productID uuid.UUID, skuID uuid.UUID, target int, admin *uuid.UUID, opt map[string]any) (int, error) {
	var owner product.Product
	if err := s.DB.WithContext(ctx).First(&owner, "id = ? AND deleted_at IS NULL", productID).Error; err != nil {
		return 0, err
	}
	var psRows []productpublish.ProductPublicationSKU
	if err := s.DB.WithContext(ctx).Where("product_sku_id = ?", skuID).Find(&psRows).Error; err != nil {
		return 0, err
	}
	optCopy := platformp.TrimRawMap(opt, 12, 200)
	n := 0
	for _, psku := range psRows {
		var pub productpublish.ProductPublication
		if err := s.DB.WithContext(ctx).Joins("JOIN shops sh ON sh.id = product_publications.shop_id AND sh.deleted_at IS NULL AND sh.tenant_id = ?", owner.TenantID).
			Where("product_publications.id = ? AND product_publications.product_id = ? AND product_publications.deleted_at IS NULL", psku.PublicationID, productID).
			First(&pub).Error; err != nil {
			continue
		}
		pl := strings.TrimSpace(strings.ToLower(pub.Platform))
		if err := productpublish.ValidateDouyinSKUBindingForInventorySync(pl, strings.TrimSpace(psku.ExternalSKUID), strings.TrimSpace(psku.BindStatus)); err != nil {
			continue
		}
		if strings.TrimSpace(psku.ExternalSKUID) == "" {
			continue
		}
		dup, err := s.hasDuplicateInventorySync(ctx, owner.TenantID, psku.ID, target)
		if err != nil {
			return n, err
		}
		if dup {
			continue
		}
		pushJob, _, pushErr := s.acquireInventoryPush(ctx, pl, pub.ShopID, skuID, psku.ID, target, admin)
		if pushErr != nil {
			return n, pushErr
		}
		if pushJob == nil && s.Idempotency != nil {
			continue
		}
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
			TenantID:         owner.TenantID,
			ProductID:        productID,
			ProductSKUID:     ptrUUID(skuID),
			PublicationID:    &pubIDCopy,
			PublicationSKUID: &pskuIDCopy,
			ShopID:           pub.ShopID,
			Platform:         pl,
			TaskType:         TaskTypeInventorySync,
			Status:           StatusPending,
			Mode:             ModeManual,
			TargetStock:      target,
			Input:            taskInputSnap(ModeManual, target, psku.ID, ptrUUID(skuID), pub.ID, pub.ShopID, optCopy, nil, ""),
			CreatedBy:        admin,
		}
		if err := s.persistTaskAndMaybeRun(ctx, t, admin); err != nil {
			s.failInventoryPush(ctx, pushJob, err.Error(), true)
			return n, err
		}
		if err := s.completeInventoryPush(ctx, pushJob, t.ID); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// CreatePublicationSKUInventoryTask submits one outbound task for linked listing SKU.
func (s *Service) CreatePublicationSKUInventoryTask(c *gin.Context, publicationSKUID uuid.UUID, body PublicationSKUSyncBody, admin *uuid.UUID) (*TaskDTO, error) {
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
	if err := s.DB.WithContext(ctx).First(&psku, "id = ?", publicationSKUID).Error; err != nil {
		return nil, err
	}
	var pub productpublish.ProductPublication
	if err := s.DB.WithContext(ctx).First(&pub, "id = ?", psku.PublicationID).Error; err != nil {
		return nil, err
	}
	var owner product.Product
	if err := s.DB.WithContext(ctx).First(&owner, "id = ? AND deleted_at IS NULL", pub.ProductID).Error; err != nil {
		return nil, err
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	if owner.TenantID != tenantID {
		return nil, fmt.Errorf("publication sku not found")
	}
	if strings.TrimSpace(psku.ExternalSKUID) == "" {
		return nil, fmt.Errorf("%s: external sku id missing for mapped listing SKU; please bind douyin sku first", platformdouyin.CodeDouyinSKUBindingRequired)
	}
	if err := productpublish.ValidateDouyinSKUBindingForInventorySync(pub.Platform, psku.ExternalSKUID, psku.BindStatus); err != nil {
		return nil, err
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
	var prodSKU uuid.UUID
	if psku.ProductSKUID != nil {
		prodSKU = *psku.ProductSKUID
	} else {
		return nil, fmt.Errorf("listing sku is not linked to a local sku id")
	}
	dup, err := s.hasDuplicateInventorySync(ctx, owner.TenantID, psku.ID, body.Stock)
	if err != nil {
		return nil, err
	}
	if dup {
		return nil, fmt.Errorf("duplicate inventory sync task already pending for this listing sku and stock level")
	}
	task := InventorySyncTask{
		TenantID:         owner.TenantID,
		ProductID:        pub.ProductID,
		ProductSKUID:     psku.ProductSKUID,
		PublicationID:    &pub.ID,
		PublicationSKUID: &psku.ID,
		ShopID:           pub.ShopID,
		Platform:         strings.TrimSpace(strings.ToLower(pub.Platform)),
		TaskType:         TaskTypeInventorySync,
		Status:           StatusPending,
		Mode:             ModePublication,
		TargetStock:      body.Stock,
		Input:            taskInputSnap(ModePublication, body.Stock, psku.ID, psku.ProductSKUID, pub.ID, pub.ShopID, opCopy, nil, ""),
		CreatedBy:        admin,
	}
	if err := s.persistTaskAndMaybeRun(ctx, &task, admin); err != nil {
		return nil, err
	}
	if body.FromInventoryAlert && s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: admin,
			Action:      "inventory.alert.sync_inventory",
			Resource:    "product_publication_sku",
			ResourceID:  publicationSKUID.String(),
			Status:      "success",
			Message: fmt.Sprintf("taskId=%s productSkuId=%s targetStock=%d",
				task.ID.String(), prodSKU.String(), body.Stock),
		})
	}
	out, err := s.GetDTO(ctx, task.TenantID, task.ID, prodSKU, psku.SKUCode)
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
	var owner product.Product
	if err := s.DB.WithContext(ctx).First(&owner, "id = ? AND deleted_at IS NULL", productID).Error; err != nil {
		return nil, err
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	if owner.TenantID != tenantID {
		return nil, fmt.Errorf("product not found")
	}
	var pub productpublish.ProductPublication
	if err := s.DB.WithContext(ctx).Joins("JOIN shops sh ON sh.id = product_publications.shop_id AND sh.deleted_at IS NULL AND sh.tenant_id = ?", tenantID).
		Where("product_publications.product_id = ? AND product_publications.shop_id = ?", productID, shopID).
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
			TenantID:         owner.TenantID,
			ProductID:        productID,
			ProductSKUID:     ptrUUID(sku.ID),
			PublicationID:    &pub.ID,
			PublicationSKUID: &psku.ID,
			ShopID:           shopID,
			Platform:         strings.TrimSpace(strings.ToLower(pub.Platform)),
			TaskType:         TaskTypeInventorySync,
			Status:           StatusPending,
			Mode:             ModeSKU,
			TargetStock:      target,
			Input:            taskInputSnap(ModeSKU, target, psku.ID, ptrUUID(sid), pub.ID, shopID, optCopy, nil, ""),
			CreatedBy:        admin,
		}
		if err := s.persistTaskAndMaybeRun(ctx, &t, admin); err != nil {
			return outDTOs, err
		}
		dto, err := s.GetDTO(ctx, t.TenantID, t.ID, sid, psku.SKUCode)
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
