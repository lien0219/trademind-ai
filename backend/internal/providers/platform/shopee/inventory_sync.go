package shopee

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

func (shopeeProvider) SyncInventory(ctx context.Context, req platformp.SyncInventoryRequest) (*platformp.SyncInventoryResult, error) {
	if req.ShopID == uuid.Nil {
		return nil, fmt.Errorf("shop id required")
	}
	if _, err := ResolveRuntime(req.Auth); err != nil {
		return nil, fmt.Errorf("platform config incomplete: please configure settings.platform_shopee first")
	}
	if strings.TrimSpace(req.Auth.AccessToken) == "" && strings.TrimSpace(req.Auth.RefreshToken) == "" {
		return nil, fmt.Errorf("shop is not authorized")
	}

	extPID := strings.TrimSpace(req.ExternalProductID)
	extSKU := strings.TrimSpace(req.ExternalSKUID)
	if extPID == "" || extSKU == "" {
		return nil, fmt.Errorf("product publication sku mapping incomplete")
	}
	if req.Stock < 0 {
		return nil, fmt.Errorf("invalid stock quantity")
	}

	access, auth2, err := ensureFreshAccess(ctx, req.ShopID, req.Auth)
	if err != nil {
		return nil, mapShopeeInventorySyncErr(0, maybeRetryableInventoryTransportErr(err))
	}
	if strings.TrimSpace(access) == "" {
		return nil, fmt.Errorf("shop is not authorized")
	}

	cfg, err := ResolveRuntime(auth2)
	if err != nil {
		return nil, fmt.Errorf("platform config incomplete: please configure settings.platform_shopee first")
	}
	sid, err := parseShopID(auth2)
	if err != nil {
		return nil, err
	}

	itemID, modelID, hasVar, err := shopeeListingModelAndItem(extPID, extSKU)
	if err != nil {
		return nil, err
	}

	loc := resolveShopeeInventoryLocation(req.Options, loadShopeePublishPlain(ctx))
	body := buildShopeeUpdateStockPayload(itemID, hasVar, modelID, req.Stock, loc)

	resp, httpSt, err := postShopWithStatus(ctx, cfg, PathProductUpdateStock, sid, access, body)
	if err != nil {
		return nil, mapShopeeInventorySyncErr(httpSt, maybeRetryableInventoryTransportErr(err))
	}

	if fail := shopeeInventoryFailureFromResponse(resp); fail != nil {
		return nil, mapShopeeInventorySyncErr(httpSt, fail)
	}

	sum := platformp.TrimRawMap(map[string]any{
		"provider":          "shopee",
		"itemId":            itemID,
		"hasVariation":      hasVar,
		"usedLocationStock": loc != "",
	}, 12, 200)

	if resp != nil {
		if v, ok := resp["stock_limit_info"]; ok && v != nil {
			sum["stockLimitInfoPresent"] = true
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

func shopeeInventoryFailureFromResponse(resp map[string]any) error {
	if resp == nil {
		return nil
	}
	raw, ok := resp["failure_list"]
	if !ok || raw == nil {
		return nil
	}
	arr, ok := raw.([]any)
	if !ok || len(arr) == 0 {
		return nil
	}
	first, _ := arr[0].(map[string]any)
	msg := strings.TrimSpace(fmt.Sprint(first["failed_reason"]))
	if msg == "" {
		msg = strings.TrimSpace(fmt.Sprint(first["message"]))
	}
	if msg == "" {
		msg = "shopee inventory sync: failure_list returned by platform"
	}
	if isPermissionLikeInventoryMsg(msg) {
		return platformp.ErrPlatformInventorySyncPermissionDenied
	}
	if strings.Contains(strings.ToLower(msg), "location") || strings.Contains(strings.ToLower(msg), "warehouse") {
		return fmt.Errorf("platform inventory config incomplete: missing warehouse_id (%s)", clampShopeeInvErr(msg, 180))
	}
	return fmt.Errorf("shopee inventory sync: %s", clampShopeeInvErr(msg, 400))
}

func clampShopeeInvErr(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
