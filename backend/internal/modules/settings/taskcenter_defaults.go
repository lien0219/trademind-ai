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
		{"enable_task_alerts", "true"},
		{"alert_min_severity", "high"},
		{"alert_on_platform_permission", "true"},
		{"alert_on_platform_config", "true"},
		{"alert_on_inventory_mapping_missing", "true"},
		{"alert_on_worker_lease_expired", "true"},
		{"alert_on_repeated_failures", "true"},
		{"repeated_failure_threshold", "3"},
		{"repeated_failure_window_minutes", "60"},
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
