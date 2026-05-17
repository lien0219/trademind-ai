package inventory

import (
	"context"
	"strconv"
	"strings"
)

type inventoryAlertPolicy struct {
	EnableInventoryAlerts       bool
	AlertOutOfStock             bool
	AlertPlatformStockMismatch  bool
	PlatformStockMismatchThresh int
}

func inventoryBool(m map[string]string, key string, def bool) bool {
	if m == nil {
		return def
	}
	v, ok := m[key]
	if !ok || strings.TrimSpace(v) == "" {
		return def
	}
	s := strings.ToLower(strings.TrimSpace(v))
	return s == "1" || s == "true" || s == "yes" || s == "on"
}

func (s *Service) loadInventoryAlertPolicy(ctx context.Context) (inventoryAlertPolicy, error) {
	pol := inventoryAlertPolicy{
		EnableInventoryAlerts:       true,
		AlertOutOfStock:             true,
		AlertPlatformStockMismatch:  true,
		PlatformStockMismatchThresh: 0,
	}
	if s == nil || s.Settings == nil {
		return pol, nil
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "inventory")
	if err != nil {
		return pol, err
	}
	pol.EnableInventoryAlerts = inventoryBool(m, "enable_inventory_alerts", true)
	pol.AlertOutOfStock = inventoryBool(m, "alert_out_of_stock", true)
	pol.AlertPlatformStockMismatch = inventoryBool(m, "alert_platform_stock_mismatch", true)
	if v := strings.TrimSpace(m["platform_stock_mismatch_threshold"]); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			pol.PlatformStockMismatchThresh = n
		}
	}
	return pol, nil
}
