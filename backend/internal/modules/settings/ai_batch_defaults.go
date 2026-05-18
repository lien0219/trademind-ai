package settings

import (
	"context"

	"gorm.io/gorm"
)

// EnsureAIBatchDefaults inserts bulk AI orchestration keys under group "ai" when missing (idempotent).
func EnsureAIBatchDefaults(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return nil
	}
	type def struct {
		key string
		val string
	}
	defs := []def{
		{"ai_batch_enabled", "true"},
		{"ai_batch_max_size", "100"},
		{"ai_batch_concurrency", "2"},
		{"ai_batch_auto_save_ai_field", "true"},
	}
	for _, d := range defs {
		var n int64
		if err := db.WithContext(ctx).Model(&Setting{}).
			Where("tenant_id = ? AND group_key = ? AND item_key = ?", 0, "ai", d.key).
			Count(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		row := Setting{
			TenantID:    0,
			GroupKey:    "ai",
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
