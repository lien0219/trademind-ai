package douyinshop

import (
	"context"
	"strings"

	"github.com/google/uuid"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

const MethodSkuSyncStock = "sku.syncStock"

// SyncInventory pushes one absolute stock snapshot to Douyin via sku.syncStock (full update).
// Official Douyin OpenAPI (Phase 9): sku.syncStock — product_id, sku_id, stock_num (>=0), incremental=false.
func (provider) SyncInventory(ctx context.Context, req platformp.SyncInventoryRequest) (*platformp.SyncInventoryResult, error) {
	if req.ShopID == uuid.Nil {
		return nil, NewError(CodeDouyinInventorySyncFailed, "shop id required", "", "", "")
	}
	if err := assertShopAuthorized(req.Auth); err != nil {
		return nil, err
	}

	extPID := strings.TrimSpace(req.ExternalProductID)
	extSKU := strings.TrimSpace(req.ExternalSKUID)
	if extPID == "" {
		return nil, NewError(CodeDouyinProductNotBound, "douyin product id not bound", "", "", "")
	}
	if extSKU == "" {
		return nil, NewError(CodeDouyinSKUNotBound, "douyin sku id not bound", "", "", "")
	}
	if req.Stock < 0 {
		return nil, NewError(CodeDouyinStockInvalid, "invalid stock quantity", "", "", "")
	}

	client, cfg, err := ensureFreshClient(ctx, req.ShopID, req.Auth)
	if err != nil {
		_ = setAuthStatusMaybe(ctx, req.ShopID, "error")
		return nil, mapInventorySyncError(err)
	}
	if err := validateInventorySyncConfig(cfg); err != nil {
		return nil, err
	}

	params := map[string]any{
		"product_id":  extPID,
		"sku_id":      extSKU,
		"stock_num":   req.Stock,
		"incremental": false,
	}

	var data map[string]any
	if err := client.Do(ctx, MethodSkuSyncStock, params, &data); err != nil {
		_ = setAuthStatusMaybe(ctx, req.ShopID, "error")
		return nil, mapInventorySyncError(err)
	}

	sum := platformp.TrimRawMap(map[string]any{
		"provider":       "douyin_shop",
		"productId":      extPID,
		"skuId":          extSKU,
		"appliedStock":   req.Stock,
		"incremental":    false,
		"method":         MethodSkuSyncStock,
		"skuCode":        strings.TrimSpace(req.SKUCode),
		"publicationId":  req.PublicationID.String(),
		"publicationSku": req.PublicationSKUID.String(),
	}, 14, 200)
	if data != nil {
		if v, ok := data["product_id"]; ok && v != nil {
			sum["platformProductId"] = v
		}
		if v, ok := data["sku_id"]; ok && v != nil {
			sum["platformSkuId"] = v
		}
	}

	return &platformp.SyncInventoryResult{
		ExternalProductID: extPID,
		ExternalSKUID:     extSKU,
		Stock:             req.Stock,
		Status:            "success",
		RawSummary:        sum,
	}, nil
}

func validateInventorySyncConfig(cfg RuntimeConfig) error {
	if !inventorySyncEnabled(cfg) {
		return NewError(CodeDouyinInventorySyncNotReady, "douyin inventory sync is disabled in platform settings (inventory_sync_enabled=false)", "", "", "")
	}
	return nil
}

func inventorySyncEnabled(cfg RuntimeConfig) bool {
	return cfg.InventoryEnabled
}

func mapInventorySyncError(err error) error {
	if err == nil {
		return nil
	}
	var de *Error
	if AsError(err, &de) {
		switch de.Code {
		case CodeDouyinAuthExpired, CodeDouyinStoreNotAuthorized:
			return de
		case CodeDouyinPermissionDenied:
			return NewError(CodeDouyinInventoryPermissionDenied, "douyin inventory permission denied", de.PlatformCode, de.PlatformMessage, de.RequestID)
		case CodeDouyinRateLimited:
			return NewError(CodeDouyinInventoryRateLimited, "douyin inventory sync rate limited", de.PlatformCode, de.PlatformMessage, de.RequestID)
		case CodeDouyinRequestTimeout:
			return NewError(CodeDouyinInventorySyncFailed, "douyin inventory sync timeout", de.PlatformCode, de.PlatformMessage, de.RequestID)
		case CodeDouyinProductNotBound, CodeDouyinSKUNotBound, CodeDouyinStockInvalid, CodeDouyinInventorySyncNotReady:
			return de
		default:
			return NewError(CodeDouyinInventorySyncFailed, de.Message, de.PlatformCode, de.PlatformMessage, de.RequestID)
		}
	}
	low := strings.ToLower(err.Error())
	if strings.Contains(low, "permission") || strings.Contains(low, "forbid") {
		return NewError(CodeDouyinInventoryPermissionDenied, "douyin inventory permission denied", "", SanitizeErrorText(err.Error()), "")
	}
	if strings.Contains(low, "rate") || strings.Contains(low, "limit") || strings.Contains(low, "frequency") {
		return NewError(CodeDouyinInventoryRateLimited, "douyin inventory sync rate limited", "", SanitizeErrorText(err.Error()), "")
	}
	return NewError(CodeUnknownDouyinInventoryError, "douyin inventory sync failed", "", SanitizeErrorText(err.Error()), "")
}
