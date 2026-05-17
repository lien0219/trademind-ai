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
		{"auto_restore_cancelled_orders", "true"},
		{"auto_sync_platform_inventory_after_deduct", "false"},
		{"allow_negative_stock", "false"},
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
