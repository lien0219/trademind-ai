package settings

import (
	"context"

	"gorm.io/gorm"
)

// EnsureInventoryDefaults inserts conservative inventory/order stock policy keys when missing.
func EnsureInventoryDefaults(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return nil
	}
	type def struct {
		key string
		val string
	}
	defs := []def{
		{"auto_deduct_manual_orders", "false"},
		{"auto_deduct_platform_orders", "false"},
		{"auto_deduct_after_sku_match", "false"},
		{"auto_match_order_skus", "true"},
		{"auto_restore_cancelled_orders", "true"},
		{"auto_sync_platform_inventory_after_deduct", "false"},
		{"auto_sync_inventory_after_order_deduct", "false"},
		{"allow_manual_sku_bind_after_deduct", "true"},
		{"allow_negative_stock", "false"},
		{"default_warning_stock", "5"},
		{"default_safety_stock", "0"},
		{"enable_inventory_alerts", "true"},
		{"alert_out_of_stock", "true"},
		{"alert_platform_stock_mismatch", "true"},
		{"platform_stock_mismatch_threshold", "0"},
		{"inventory_sync_batch_max_size", "500"},
		{"inventory_stock_settings_batch_max_size", "500"},
		{"inventory_sync_platform_rate_limit_enabled", "true"},
		{"inventory_sync_platform_rate_limit_per_minute_tiktok", "60"},
		{"inventory_sync_platform_rate_limit_per_minute_shopee", "60"},
		{"inventory_sync_platform_rate_limit_per_minute_lazada", "60"},
		{"inventory_sync_platform_rate_limit_per_minute_amazon", "30"},
	}
	for _, d := range defs {
		var n int64
		if err := db.WithContext(ctx).Model(&Setting{}).
			Where("tenant_id = ? AND group_key = ? AND item_key = ?", 0, "inventory", d.key).
			Count(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		row := Setting{
			TenantID:    0,
			GroupKey:    "inventory",
			ItemKey:     d.key,
			ItemValue:   d.val,
			ValueType:   "string",
			IsEncrypted: false,
		}
		if err := db.WithContext(ctx).Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}
