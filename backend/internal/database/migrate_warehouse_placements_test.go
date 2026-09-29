package database

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/warehouse"
	"gorm.io/gorm"
)

func TestMigrateWarehousePlacementIndexesAddsActiveBarcodeUniqueness(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:warehouse_placement_migration_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&warehouse.Warehouse{}, &inventory.WarehouseSKUPlacement{}))
	require.NoError(t, migrateWarehousePlacementIndexes(db))
	require.True(t, db.Migrator().HasIndex(&inventory.WarehouseSKUPlacement{}, "ux_warehouse_sku_active_barcode"))
}
