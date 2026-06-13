package settings

import (
	"context"

	"gorm.io/gorm"
)

// EnsureDouyinAlertDefaults seeds Douyin platform alert threshold keys (idempotent).
func EnsureDouyinAlertDefaults(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return nil
	}
	type def struct {
		key string
		val string
	}
	defs := []def{
		{"alert_scan_enabled", "true"},
		{"alert_scan_interval_seconds", "120"},
		{"alert_token_refresh_fail_threshold", "3"},
		{"alert_stale_tasks_threshold", "5"},
		{"alert_failure_backlog_threshold", "20"},
		{"alert_rate_limit_threshold", "10"},
		{"alert_product_draft_fail_threshold", "3"},
		{"alert_inventory_sync_fail_threshold", "5"},
		{"alert_image_upload_fail_rate_pct", "30"},
		{"gray_release_enabled", "false"},
		{"gray_shop_ids", "[]"},
		{"write_operations_enabled", "false"},
		{"scheduled_order_sync_enabled", "false"},
		{"scheduled_inventory_sync_enabled", "false"},
	}
	for _, d := range defs {
		var n int64
		if err := db.WithContext(ctx).Model(&Setting{}).
			Where("tenant_id = ? AND group_key = ? AND item_key = ?", 0, "platform_douyin_shop", d.key).
			Count(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		row := Setting{
			TenantID:    0,
			GroupKey:    "platform_douyin_shop",
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
