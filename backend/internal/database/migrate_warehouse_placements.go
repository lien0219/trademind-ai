package database

import (
	"fmt"

	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"gorm.io/gorm"
)

// migrateWarehousePlacementIndexes closes the race between the service-level
// duplicate check and concurrent writes to the same active barcode.
func migrateWarehousePlacementIndexes(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&inventory.WarehouseSKUPlacement{}) {
		return nil
	}
	if err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS ux_warehouse_sku_active_barcode
		ON warehouse_sku_placements (tenant_id, warehouse_id, barcode)
		WHERE deleted_at IS NULL AND status = 'active' AND barcode <> ''
	`).Error; err != nil {
		return fmt.Errorf("create active warehouse sku barcode index: %w", err)
	}
	return nil
}
