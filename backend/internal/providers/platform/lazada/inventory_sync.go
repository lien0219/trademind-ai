package lazada

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

func summarizeLazadaInventoryPlatformResponse(root map[string]any) map[string]any {
	out := map[string]any{}
	if root == nil {
		return out
	}
	if v, ok := root["request_id"]; ok && v != nil {
		s := strings.TrimSpace(fmt.Sprint(v))
		if s != "" {
			out["requestId"] = s
		}
	}
	return out
}

func (lazadaProvider) SyncInventory(ctx context.Context, req platformp.SyncInventoryRequest) (*platformp.SyncInventoryResult, error) {
	if req.ShopID == uuid.Nil {
		return nil, fmt.Errorf("shop id required")
	}

	if _, err := ResolveRuntime(req.Auth); err != nil {
		return nil, fmt.Errorf("platform config incomplete: please configure settings.platform_lazada first")
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
		return nil, mapLazadaInventorySyncTransportErr(err)
	}
	if strings.TrimSpace(access) == "" {
		return nil, fmt.Errorf("shop is not authorized")
	}

	cfg, err := ResolveRuntime(auth2)
	if err != nil {
		return nil, fmt.Errorf("platform config incomplete: please configure settings.platform_lazada first")
	}

	sellerSKU, err := resolveSellerSKUForStockUpdate(ctx, cfg, access, extPID, extSKU, strings.TrimSpace(req.SKUCode))
	if err != nil {
		return nil, err
	}

	warehouse := resolveLazadaInventoryWarehouse(req.Options, loadLazadaPublishPlain(ctx))
	payload, err := buildLazadaPriceQuantityPayload(extPID, sellerSKU, req.Stock, warehouse)
	if err != nil {
		return nil, err
	}

	root, httpSt, err := postLazadaPriceQuantityUpdate(ctx, cfg, access, payload)
	if err != nil {
		return nil, mapLazadaInventoryPostErr(httpSt, root, err)
	}

	sum := platformp.TrimRawMap(map[string]any{
		"provider":      "lazada",
		"path":          PathProductPriceQuantityUpdate,
		"warehouseUsed": warehouse != "",
		"receivedAt":    time.Now().UTC().Format(time.RFC3339),
	}, 14, 220)
	for k, v := range summarizeLazadaInventoryPlatformResponse(root) {
		sum[k] = v
	}
	return &platformp.SyncInventoryResult{
		ExternalProductID: extPID,
		ExternalSKUID:     extSKU,
		Stock:             req.Stock,
		Status:            "success",
		RawSummary:        sum,
	}, nil
}
