package tiktok

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

func loadTikTokPublishPlain(ctx context.Context) map[string]string {
	if bridges == nil {
		return map[string]string{}
	}
	m, err := bridges.TikTokPublishSettings(ctx)
	if err != nil || m == nil {
		return map[string]string{}
	}
	return m
}

func (tikTokProvider) SyncInventory(ctx context.Context, req platformp.SyncInventoryRequest) (*platformp.SyncInventoryResult, error) {
	if req.ShopID == uuid.Nil {
		return nil, fmt.Errorf("shop id required")
	}
	cfg, err := ResolveRuntime(req.Auth)
	if err != nil {
		return nil, errIncompleteTiktokPlatformSettings
	}
	if strings.TrimSpace(cfg.ShopCipher) == "" {
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

	access, _, err := ensureFreshAccess(ctx, req.ShopID, req.Auth)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(access) == "" {
		return nil, fmt.Errorf("shop is not authorized")
	}

	warehouse := resolveInventoryWarehouse(req.Options, loadTikTokPublishPlain(ctx))
	if warehouse == "" {
		return nil, fmt.Errorf("platform inventory config incomplete: missing warehouse_id")
	}

	body := map[string]interface{}{
		"skus": []interface{}{
			map[string]interface{}{
				"id": extSKU,
				"inventory": []interface{}{
					map[string]interface{}{
						"warehouse_id": warehouse,
						"quantity":     req.Stock,
					},
				},
			},
		},
	}
	client := http.Client{Timeout: cfg.HTTPTimeout}
	path := inventoryUpdateAPIPath(cfg.APIVersion, extPID)
	raw, httpStatus, err := signedPOSTJSONStatus(ctx, client, cfg, path, access, body)
	if err != nil {
		return nil, maybeRetryableInventoryTransportErr(err)
	}

	root, err := decodeInventoryAPIResponse(raw, httpStatus)
	if err != nil {
		return nil, err
	}

	sum := platformp.TrimRawMap(map[string]any{
		"provider": "tiktok",
		"bizCode":  bizCodeSummary(root),
	}, 12, 200)
	return &platformp.SyncInventoryResult{
		ExternalProductID: extPID,
		ExternalSKUID:     extSKU,
		Stock:             req.Stock,
		Status:            "success",
		RawSummary:        sum,
	}, nil
}
