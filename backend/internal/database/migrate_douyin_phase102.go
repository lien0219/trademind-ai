package database

import (
	"fmt"

	"gorm.io/gorm"
)

// migrateDouyinPhase102Indexes adds idempotency indexes for Douyin production hardening (Phase 10.2).
// Safe to re-run (IF NOT EXISTS). Rollback: DROP INDEX IF EXISTS ux_orders_shop_platform_ext_order;
func migrateDouyinPhase102Indexes(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migrate phase102: db is nil")
	}
	if db.Dialector.Name() != "postgres" {
		return nil
	}
	stmts := []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_orders_shop_platform_ext_order
		 ON orders (shop_id, platform, external_order_id)
		 WHERE external_order_id IS NOT NULL AND external_order_id <> '' AND deleted_at IS NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_order_items_order_ext_item
		 ON order_items (order_id, external_item_id)
		 WHERE external_item_id IS NOT NULL AND external_item_id <> ''`,
	}
	for _, sql := range stmts {
		if err := db.Exec(sql).Error; err != nil {
			return fmt.Errorf("phase102 index: %w", err)
		}
	}
	return nil
}
