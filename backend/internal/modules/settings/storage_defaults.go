package settings

import (
	"context"

	"github.com/trademind-ai/trademind/backend/internal/providers/storage/localroot"
	"gorm.io/gorm"
)

const defaultLocalPublicBase = "/static"

// EnsureStorageDefaults inserts local storage keys when missing (idempotent).
func EnsureStorageDefaults(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return nil
	}
	type def struct {
		key string
		val string
	}
	defs := []def{
		{"kind", "local"},
		{"local_root", localroot.DefaultRelative},
		{"public_base", defaultLocalPublicBase},
	}
	for _, d := range defs {
		var n int64
		if err := db.WithContext(ctx).Model(&Setting{}).
			Where("tenant_id = ? AND group_key = ? AND item_key = ?", 0, "storage", d.key).
			Count(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		row := Setting{
			TenantID:    0,
			GroupKey:    "storage",
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
