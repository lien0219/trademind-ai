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
		&InventorySyncTask{},
	))
	return db
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
