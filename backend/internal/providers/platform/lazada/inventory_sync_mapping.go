package lazada

import (
	"context"
	"fmt"
	"strings"
)

func loadLazadaPublishPlain(ctx context.Context) map[string]string {
	if bridges == nil {
		return map[string]string{}
	}
	m, err := bridges.LazadaPublishSettings(ctx)
	if err != nil || m == nil {
		return map[string]string{}
	}
	return m
}

func stringFromOptsLazInv(m map[string]any, keys ...string) string {
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

// resolveLazadaInventoryWarehouse prefers task options, then settings.platform_publish_lazada.warehouse_id.
func resolveLazadaInventoryWarehouse(opts map[string]any, publish map[string]string) string {
	if got := strings.TrimSpace(stringFromOptsLazInv(opts,
		"warehouse_id", "warehouseId", "WarehouseID",
		"WarehouseCode", "warehouse_code",
		"location_id", "locationId", "LocationID",
	)); got != "" {
		return got
	}
	if publish != nil {
		if w := strings.TrimSpace(publish["warehouse_id"]); w != "" {
			return w
		}
	}
	return ""
}

func lazadaSKULooksNumericID(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

// resolveSellerSKUForStockUpdate maps publication identifiers to Lazada SellerSku required by UpdatePriceQuantity.
func resolveSellerSKUForStockUpdate(ctx context.Context, cfg RuntimeConfig, access, itemID, externalSKUID, skuCode string) (string, error) {
	ext := strings.TrimSpace(externalSKUID)
	if ext == "" {
		return "", fmt.Errorf("product publication sku mapping incomplete")
	}
	cc := strings.TrimSpace(skuCode)
	if cc != "" {
		return cc, nil
	}
	if !lazadaSKULooksNumericID(ext) {
		return ext, nil
	}

	root, err := getSigned(ctx, cfg, cfg.APIRESTBase, PathProductItemGet, access, map[string]string{"item_id": strings.TrimSpace(itemID)})
	if err != nil {
		return "", mapLazadaInventorySyncTransportErr(err)
	}

	var data map[string]any
	if d, ok := root["data"].(map[string]any); ok && d != nil {
		data = d
	}
	rows := coalesceLazadaSKUResponse(data)
	for _, row := range rows {
		if row == nil {
			continue
		}
		sid := lazadaIDString(row, "sku_id", "SkuId", "skuId")
		seller := strings.TrimSpace(pickStr(row, "seller_sku", "SellerSku", "sellerSku"))
		if seller == ext || sid == ext {
			if seller != "" {
				return seller, nil
			}
		}
	}
	return "", fmt.Errorf("product publication sku mapping incomplete")
}
