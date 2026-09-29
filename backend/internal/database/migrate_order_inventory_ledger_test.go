package database

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/orderexception"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/gorm"
)

func TestOrderInventoryLedgerMigrationAddsWarehouseFieldsAndBackfillsTenant(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`
		CREATE TABLE orders (
			id TEXT PRIMARY KEY,
			tenant_id INTEGER NOT NULL DEFAULT 0
		)
	`).Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE order_inventory_effects (
			id TEXT PRIMARY KEY,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			order_id TEXT NOT NULL,
			order_item_id TEXT NOT NULL,
			product_sku_id TEXT NOT NULL,
			effect_type TEXT NOT NULL,
			quantity INTEGER NOT NULL,
			status TEXT NOT NULL
		)
	`).Error)

	orderID := uuid.New()
	effectID := uuid.New()
	require.NoError(t, db.Exec(`INSERT INTO orders (id, tenant_id) VALUES (?, ?)`, orderID, 73).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO order_inventory_effects (
			id, created_at, updated_at, order_id, order_item_id, product_sku_id, effect_type, quantity, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, effectID, time.Now().UTC(), time.Now().UTC(), orderID, uuid.New(), uuid.New(), inventory.EffectTypeDeduct, 2, inventory.InventoryEffectSuccess).Error)

	require.NoError(t, db.Exec(`ALTER TABLE order_inventory_effects ADD COLUMN tenant_id INTEGER NOT NULL DEFAULT 0`).Error)
	require.NoError(t, db.Exec(`ALTER TABLE order_inventory_effects ADD COLUMN warehouse_id TEXT`).Error)
	require.NoError(t, db.AutoMigrate(&inventory.InventoryMovement{}))
	require.NoError(t, backfillOrderInventoryEffectTenants(db))
	require.NoError(t, backfillOrderInventoryEffectTenants(db))

	require.True(t, db.Migrator().HasColumn(&inventory.OrderInventoryEffect{}, "tenant_id"))
	require.True(t, db.Migrator().HasColumn(&inventory.OrderInventoryEffect{}, "warehouse_id"))
	require.True(t, db.Migrator().HasColumn(&inventory.InventoryMovement{}, "before_reserved"))
	require.True(t, db.Migrator().HasColumn(&inventory.InventoryMovement{}, "after_reserved"))

	var migrated inventory.OrderInventoryEffect
	require.NoError(t, db.First(&migrated, "id = ?", effectID).Error)
	require.EqualValues(t, 73, migrated.TenantID)
	require.Nil(t, migrated.WarehouseID)
}

func TestLegacyOrderSKUMatchColumnIsRenamed(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`
		CREATE TABLE order_item_sku_matches (
			id TEXT PRIMARY KEY,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			order_id TEXT NOT NULL,
			order_item_id TEXT NOT NULL,
			product_sku_i_d TEXT,
			external_sk_uid TEXT
		)
	`).Error)

	require.NoError(t, migrateLegacyInventorySKUColumns(db))
	require.True(t, db.Migrator().HasColumn(&order.OrderItemSKUMatch{}, "product_sku_id"))
	require.False(t, db.Migrator().HasColumn(&order.OrderItemSKUMatch{}, "product_sku_i_d"))
	require.True(t, db.Migrator().HasColumn(&order.OrderItemSKUMatch{}, "external_sku_id"))
	require.False(t, db.Migrator().HasColumn(&order.OrderItemSKUMatch{}, "external_sk_uid"))
}

func TestOrderExceptionMarkMigrationBackfillsTenant(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&order.Order{}, &order.OrderItem{}, &orderexception.OrderExceptionMark{}))
	orderID := uuid.New()
	require.NoError(t, db.Create(&order.Order{Base: model.Base{ID: orderID}, TenantID: 73, Platform: "manual", OrderNo: "MIGRATE-EXC", CustomerName: "Buyer", Status: order.StatusPaid, PaymentStatus: order.PaymentPaid, FulfillmentStatus: order.FulfillmentUnfulfilled, Currency: "USD"}).Error)
	itemID := uuid.New()
	require.NoError(t, db.Create(&order.OrderItem{HardDeleteBase: model.HardDeleteBase{ID: itemID}, OrderID: orderID, ProductTitle: "Legacy exception", Quantity: 1}).Error)
	mark := orderexception.OrderExceptionMark{ExceptionType: orderexception.TypeSKUUnmatched, SourceType: orderexception.SourceOrderItem, SourceID: itemID.String(), MarkType: orderexception.MarkHandled}
	require.NoError(t, db.Create(&mark).Error)
	require.NoError(t, backfillOrderExceptionMarkTenants(db))
	var migrated orderexception.OrderExceptionMark
	require.NoError(t, db.First(&migrated, "id = ?", mark.ID).Error)
	require.EqualValues(t, 73, migrated.TenantID)
}
