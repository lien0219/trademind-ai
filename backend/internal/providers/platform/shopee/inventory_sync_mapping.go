package shopee

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

func loadShopeePublishPlain(ctx context.Context) map[string]string {
	if bridges == nil {
		return map[string]string{}
	}
	m, err := bridges.ShopeePublishSettings(ctx)
	if err != nil || m == nil {
		return map[string]string{}
	}
	return m
}

func stringFromOpts(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if m == nil {
			break
		}
		raw, ok := m[k]
		if !ok {
			continue
		}
		s := strings.TrimSpace(fmt.Sprint(raw))
		if s != "" {
			return s
		}
	}
	return ""
}

// resolveShopeeInventoryLocation prefers task options, then settings.platform_publish_shopee.warehouse_id.
func resolveShopeeInventoryLocation(opts map[string]any, publish map[string]string) string {
	if got := strings.TrimSpace(stringFromOpts(opts, "warehouse_id", "warehouseId", "WarehouseID", "location_id", "locationId", "LocationID")); got != "" {
		return got
	}
	if publish != nil {
		if w := strings.TrimSpace(publish["warehouse_id"]); w != "" {
			return w
		}
	}
	return ""
}

// shopeeListingModelAndItem derives API identifiers from publication mapping.
// Single-SKU listings reuse item_id as external_sku_id (see PublishProduct mappings).
func shopeeListingModelAndItem(extPID, extSK string) (itemID int64, variationModelID int64, hasVariation bool, err error) {
	pid := strings.TrimSpace(extPID)
	sk := strings.TrimSpace(extSK)
	if pid == "" || sk == "" {
		return 0, 0, false, fmt.Errorf("product publication sku mapping incomplete")
	}
	itemID, err = strconv.ParseInt(pid, 10, 64)
	if err != nil || itemID <= 0 {
		return 0, 0, false, fmt.Errorf("product publication sku mapping incomplete")
	}
	if strings.EqualFold(sk, pid) {
		return itemID, 0, false, nil
	}
	mid, err := strconv.ParseInt(sk, 10, 64)
	if err != nil || mid <= 0 {
		return 0, 0, false, fmt.Errorf("product publication sku mapping incomplete")
	}
	return itemID, mid, true, nil
}

func buildShopeeUpdateStockPayload(itemID int64, hasVariation bool, modelID int64, stock int, locationID string) map[string]any {
	entry := map[string]any{}
	if hasVariation {
		entry["model_id"] = modelID
	}
	loc := strings.TrimSpace(locationID)
	if loc != "" {
		entry["seller_stock"] = []any{map[string]any{"location_id": loc, "stock": stock}}
	} else {
		entry["normal_stock"] = stock
	}
	return map[string]any{
		"item_id":    itemID,
		"stock_list": []any{entry},
	}
}
