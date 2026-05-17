package tiktok

import (
	"fmt"
	"strings"
)

func stringFromAnyMap(m map[string]any, keys ...string) string {
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

// resolveInventoryWarehouse prefers task overrides, then defaults from settings.platform_publish_tiktok (lowercase keys).
func resolveInventoryWarehouse(opts map[string]any, publish map[string]string) string {
	if got := strings.TrimSpace(stringFromAnyMap(opts, "warehouse_id", "warehouseId", "WarehouseID")); got != "" {
		return got
	}
	if publish != nil {
		if w := strings.TrimSpace(publish["warehouse_id"]); w != "" {
			return w
		}
	}
	return ""
}
