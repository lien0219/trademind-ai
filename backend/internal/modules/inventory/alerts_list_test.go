package inventory

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/modules/warehouse"
	"gorm.io/gorm"
)

func newInventoryAlertsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(
		&product.Product{},
		&product.ProductSKU{},
		&productpublish.ProductPublication{},
		&productpublish.ProductPublicationSKU{},
		&shop.Shop{},
		&warehouse.Warehouse{},
		&WarehouseStockBalance{},
		&InventorySyncTask{},
	))
	return db
}

func TestListInventoryCenterUsesWarehouseFactsAndScopedAvailability(t *testing.T) {
	db := newInventoryAlertsTestDB(t)
	item := product.Product{TenantID: 11, Source: "manual", Title: "Warehouse stock item", Status: product.StatusDraft}
	require.NoError(t, db.Create(&item).Error)
	projection := 13
	sku := product.ProductSKU{
		ProductID: item.ID, SKUCode: "CENTER-SKU-1", SKUName: "Center SKU", Stock: &projection,
		WarningStock: 8, SafetyStock: 2,
	}
	require.NoError(t, db.Create(&sku).Error)
	main := warehouse.Warehouse{TenantID: 11, Code: "MAIN", Name: "Main warehouse", Status: warehouse.StatusActive, IsDefault: true}
	second := warehouse.Warehouse{TenantID: 11, Code: "SECOND", Name: "Second warehouse", Status: warehouse.StatusActive}
	require.NoError(t, db.Create(&main).Error)
	require.NoError(t, db.Create(&second).Error)
	require.NoError(t, db.Create(&[]WarehouseStockBalance{
		{TenantID: 11, WarehouseID: main.ID, ProductSKUID: sku.ID, OnHand: 10, Reserved: 2, InTransit: 3, Damaged: 1, Version: 1},
		{TenantID: 11, WarehouseID: second.ID, ProductSKUID: sku.ID, OnHand: 4, Reserved: 1, Damaged: 0, Version: 1},
	}).Error)

	svc := &Service{DB: db}
	global, err := svc.ListInventoryCenter(context.Background(), CenterListQuery{TenantID: 11, Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Len(t, global.Items, 1)
	row := global.Items[0]
	require.Equal(t, 13, row.ProjectionStock)
	require.Equal(t, 14, row.OnHandStock)
	require.Equal(t, 3, row.ReservedStock)
	require.Equal(t, 3, row.InTransitStock)
	require.Equal(t, 1, row.DamagedStock)
	require.Equal(t, 13, row.SellableStock)
	require.Equal(t, 10, row.AvailableStock)
	require.Equal(t, 2, row.WarehouseBalanceCount)
	require.Equal(t, reconciliationMatched, row.ReconciliationStatus)
	require.Equal(t, product.StockStatusNormal, row.StockStatus)
	require.Equal(t, centerInventoryScopeGlobal, row.InventoryScope)

	scoped, err := svc.ListInventoryCenter(context.Background(), CenterListQuery{
		TenantID: 11, WarehouseID: &main.ID, StockStatus: product.StockStatusLowStock, Page: 1, PageSize: 20,
	})
	require.NoError(t, err)
	require.Len(t, scoped.Items, 1)
	row = scoped.Items[0]
	require.Equal(t, main.ID, *row.WarehouseID)
	require.Equal(t, "MAIN", row.WarehouseCode)
	require.Equal(t, "Main warehouse", row.WarehouseName)
	require.Equal(t, 10, row.OnHandStock)
	require.Equal(t, 2, row.ReservedStock)
	require.Equal(t, 1, row.DamagedStock)
	require.Equal(t, 9, row.SellableStock)
	require.Equal(t, 7, row.AvailableStock)
	require.Equal(t, product.StockStatusLowStock, row.StockStatus)
	require.Equal(t, reconciliationMatched, row.ReconciliationStatus)
	require.Equal(t, centerInventoryScopeWarehouse, row.InventoryScope)
}

func TestListInventoryAlertsUsesCanonicalPublicationSKUColumn(t *testing.T) {
	db := newInventoryAlertsTestDB(t)
	item := product.Product{Source: "manual", Title: "Low stock item", Status: product.StatusDraft}
	require.NoError(t, db.Create(&item).Error)

	stock := 0
	sku := product.ProductSKU{
		ProductID:    item.ID,
		SKUCode:      "ALERT-SKU-1",
		SKUName:      "Alert SKU",
		Stock:        &stock,
		WarningStock: 5,
	}
	require.NoError(t, db.Create(&sku).Error)

	result, err := (&Service{DB: db}).ListInventoryAlerts(context.Background(), AlertsListQuery{
		Page:     1,
		PageSize: 20,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), result.Total)
	require.Len(t, result.Items, 1)
	require.Equal(t, sku.ID, result.Items[0].ProductSKUID)
	require.Contains(t, result.Items[0].AlertTypes, AlertTypeOutOfStock)
}

func TestListInventoryAlertsScopesTenant(t *testing.T) {
	db := newInventoryAlertsTestDB(t)
	for tenantID, code := range map[int64]string{11: "TENANT-11", 22: "TENANT-22"} {
		item := product.Product{TenantID: tenantID, Source: "manual", Title: code}
		require.NoError(t, db.Create(&item).Error)
		stock := 0
		sku := product.ProductSKU{ProductID: item.ID, SKUCode: code, SKUName: code, Stock: &stock, WarningStock: 5}
		require.NoError(t, db.Create(&sku).Error)
	}

	result, err := (&Service{DB: db}).ListInventoryAlerts(context.Background(), AlertsListQuery{
		TenantID: 11,
		Page:     1,
		PageSize: 20,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), result.Total)
	require.Len(t, result.Items, 1)
	require.Equal(t, "TENANT-11", result.Items[0].SKUCode)
}

func TestHasDuplicateInventorySyncScopesTenant(t *testing.T) {
	db := newInventoryAlertsTestDB(t)
	publicationSKU := uuid.New()
	for _, tenantID := range []int64{11, 22} {
		task := InventorySyncTask{
			TenantID: tenantID, ProductID: uuid.New(), PublicationSKUID: &publicationSKU,
			ShopID: uuid.New(), Platform: "mock", TaskType: TaskTypeInventorySync,
			Status: StatusPending, TargetStock: 7,
		}
		require.NoError(t, db.Create(&task).Error)
	}

	svc := &Service{DB: db}
	dup, err := svc.hasDuplicateInventorySync(context.Background(), 11, publicationSKU, 7)
	require.NoError(t, err)
	require.True(t, dup)

	dup, err = svc.hasDuplicateInventorySync(context.Background(), 33, publicationSKU, 7)
	require.NoError(t, err)
	require.False(t, dup)
}
