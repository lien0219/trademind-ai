package settings

import (
	"context"

	"gorm.io/gorm"
)

// EnsureTaskcenterDefaults seeds task center / in-site alert tuning keys (idempotent).
func EnsureTaskcenterDefaults(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return nil
	}
	type def struct {
		key string
		val string
	}
	defs := []def{
		{"enable_task_alerts", "false"},
		{"alert_min_severity", ""},
		{"alert_on_platform_permission", "false"},
		{"alert_on_platform_config", "false"},
		{"alert_on_inventory_mapping_missing", "false"},
		{"alert_on_worker_lease_expired", "false"},
		{"alert_on_repeated_failures", "false"},
		{"repeated_failure_threshold", ""},
		{"repeated_failure_window_minutes", ""},
		{"enable_alert_scan_worker", "false"},
		{"alert_scan_interval_seconds", ""},
		{"alert_detail_public_base", ""},
		{"enable_external_notifications", "false"},
		{"notification_min_severity", ""},
		{"notify_on_alert_generated", "false"},
		{"notify_on_repeated_alert", "false"},
		{"notification_channels", "[]"},
	}
	for _, d := range defs {
		var n int64
		if err := db.WithContext(ctx).Model(&Setting{}).
			Where("tenant_id = ? AND group_key = ? AND item_key = ?", 0, "taskcenter", d.key).
			Count(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		row := Setting{
			TenantID:    0,
			GroupKey:    "taskcenter",
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
