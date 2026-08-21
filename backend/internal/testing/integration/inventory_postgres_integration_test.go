//go:build inventorypostgres

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/database"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/warehouse"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"github.com/trademind-ai/trademind/backend/internal/testing/postgrestest"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type inventorySKUSearchFixture struct {
	productA product.Product
	productB product.Product
}

func seedInventoryTenantSKUs(t *testing.T, db interface {
	Create(value any) *gorm.DB
	Model(value any) *gorm.DB
}, tenantID int64, suffix string) (product.Product, []product.ProductSKU) {
	t.Helper()
	item := product.Product{TenantID: tenantID, Source: "manual", Title: "Shared PostgreSQL Product " + suffix, Status: product.StatusDraft}
	require.NoError(t, db.Create(&item).Error)
	stock := 5
	skus := []product.ProductSKU{
		{ProductID: item.ID, SKUCode: "INVPG-SHARED-CODE", SKUName: "INVPG Shared Blue " + suffix, Stock: &stock, StockStatus: "normal", RawData: datatypes.JSON([]byte(`{"barcode":"INVPG-SHARED-BARCODE"}`))},
		{ProductID: item.ID, SKUCode: "INVPG-SECOND-" + suffix, SKUName: "INVPG Shared Green " + suffix, Stock: &stock, StockStatus: "low_stock", RawData: datatypes.JSON([]byte(fmt.Sprintf(`{"barcode":"INVPG-%s-BARCODE"}`, suffix)))},
	}
	require.NoError(t, db.Create(&skus).Error)
	return item, skus
}

func inventorySKUSearchRequest(t *testing.T, handler *product.Handler, tenantID int64, actorID uuid.UUID, query string) (response.Envelope, int) {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/product-skus/search"+query, nil)
	ctx.Set(ctxkey.TraceID, "trace-inventorypg-sku-search")
	ctx.Set(ctxkey.TenantID, tenantID)
	ctx.Set(ctxkey.AdminID, actorID.String())
	security.SetGin(ctx, &security.TenantContext{TenantID: tenantID, UserID: actorID, RequestID: "trace-inventorypg-sku-search", AuthSource: security.AuthSourceAccessToken})
	handler.SearchSKUs(ctx)
	var envelope response.Envelope
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	return envelope, recorder.Code
}

func inventorySKUSearchRows(t *testing.T, envelope response.Envelope) []map[string]any {
	t.Helper()
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	raw, ok := data["list"].([]any)
	require.True(t, ok)
	rows := make([]map[string]any, 0, len(raw))
	for _, value := range raw {
		row, ok := value.(map[string]any)
		require.True(t, ok)
		rows = append(rows, row)
	}
	return rows
}

func TestInventoryPostgresTenantScopedProductSKUSearch(t *testing.T) {
	harness := postgrestest.Require(t)
	harness.EmitMetadata(t)
	db := harness.DB
	require.NoError(t, database.AutoMigrate(db))
	gin.SetMode(gin.TestMode)

	productA, _ := seedInventoryTenantSKUs(t, db, 910101, "TENANT-A")
	productB, _ := seedInventoryTenantSKUs(t, db, 910202, "TENANT-B")
	require.NoError(t, db.Model(&product.Product{}).Where("id = ?", productA.ID).Update("updated_at", time.Now().UTC().Add(-time.Hour)).Error)
	require.NoError(t, db.Model(&product.Product{}).Where("id = ?", productB.ID).Update("updated_at", time.Now().UTC()).Error)
	actorA := uuid.New()
	actorB := uuid.New()
	require.NoError(t, db.Create(&admin.AdminUser{Base: model.Base{ID: actorA}, TenantID: 910101, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: admin.RoleAdmin, Status: admin.StatusActive}).Error)
	require.NoError(t, db.Create(&admin.AdminUser{Base: model.Base{ID: actorB}, TenantID: 910202, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: admin.RoleAdmin, Status: admin.StatusActive}).Error)
	handler := &product.Handler{Svc: &product.Service{DB: db}}

	cases := []struct {
		name      string
		tenantID  int64
		actorID   uuid.UUID
		query     string
		productID uuid.UUID
		want      int
	}{
		{name: "tenant A default", tenantID: 910101, actorID: actorA, productID: productA.ID, want: 2},
		{name: "tenant B default", tenantID: 910202, actorID: actorB, productID: productB.ID, want: 2},
		{name: "shared sku code", tenantID: 910101, actorID: actorA, query: "?keyword=INVPG-SHARED-CODE", productID: productA.ID, want: 1},
		{name: "shared sku name", tenantID: 910101, actorID: actorA, query: "?keyword=INVPG%20Shared%20Blue", productID: productA.ID, want: 1},
		{name: "shared product title", tenantID: 910101, actorID: actorA, query: "?keyword=Shared%20PostgreSQL%20Product", productID: productA.ID, want: 2},
		{name: "own product id", tenantID: 910101, actorID: actorA, query: "?productId=" + productA.ID.String(), productID: productA.ID, want: 2},
		{name: "own product plus keyword", tenantID: 910101, actorID: actorA, query: "?productId=" + productA.ID.String() + "&keyword=INVPG%20Shared%20Blue", productID: productA.ID, want: 1},
		{name: "tenant spoof ignored", tenantID: 910101, actorID: actorA, query: "?keyword=Shared&tenantId=910202&tenant_id=910202&status=normal&barcode=INVPG-SHARED-BARCODE", productID: productA.ID, want: 2},
		{name: "foreign rows cannot displace limit", tenantID: 910101, actorID: actorA, query: "?keyword=Shared&limit=1", productID: productA.ID, want: 1},
		{name: "limit two stays local", tenantID: 910101, actorID: actorA, query: "?keyword=Shared&limit=2", productID: productA.ID, want: 2},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			envelope, status := inventorySKUSearchRequest(t, handler, test.tenantID, test.actorID, test.query)
			require.Equal(t, http.StatusOK, status)
			require.Equal(t, response.CodeOK, envelope.Code)
			require.Equal(t, "trace-inventorypg-sku-search", envelope.TraceID)
			rows := inventorySKUSearchRows(t, envelope)
			require.Len(t, rows, test.want)
			for _, row := range rows {
				require.Equal(t, test.productID.String(), row["productId"])
				require.NotContains(t, row, "tenantId")
			}
		})
	}

	for _, query := range []string{
		"?productId=" + productB.ID.String(),
		"?productId=" + productB.ID.String() + "&keyword=Shared",
		"?productId=" + productB.ID.String() + "&keyword=TENANT-B",
	} {
		envelope, status := inventorySKUSearchRequest(t, handler, 910101, actorA, query)
		require.Equal(t, http.StatusOK, status)
		require.Empty(t, inventorySKUSearchRows(t, envelope))
	}

	for range 3 {
		envelope, status := inventorySKUSearchRequest(t, handler, 910101, actorA, "?keyword=Shared&limit=1")
		require.Equal(t, http.StatusOK, status)
		rows := inventorySKUSearchRows(t, envelope)
		require.Len(t, rows, 1)
		require.Equal(t, productA.ID.String(), rows[0]["productId"])
	}

	barcodeOnly, status := inventorySKUSearchRequest(t, handler, 910101, actorA, "?keyword=INVPG-SHARED-BARCODE")
	require.Equal(t, http.StatusOK, status)
	require.Empty(t, inventorySKUSearchRows(t, barcodeOnly), "barcode remains outside the existing public search contract")
}

func TestInventoryPostgresAutoMigrateAgainstIsolatedDatabase(t *testing.T) {
	harness := postgrestest.Require(t)
	harness.EmitMetadata(t)
	db := harness.DB

	require.NoError(t, database.AutoMigrate(db))
	require.NoError(t, database.AutoMigrate(db))

	for _, table := range []string{
		"admin_users",
		"products",
		"product_skus",
		"inventory_sync_runs",
		"inventory_snapshot_items",
		"sku_bindings",
		"sku_binding_calibrations",
		"manual_binding_requests",
		"manual_binding_decisions",
	} {
		require.Truef(t, db.Migrator().HasTable(table), "expected migrated table %s", table)
	}
}

func TestInventoryWarehouseAdjustmentSerializesConcurrentWrites(t *testing.T) {
	harness := postgrestest.Require(t)
	harness.EmitMetadata(t)
	db := harness.DB
	require.NoError(t, database.AutoMigrate(db))

	tenantID := time.Now().UnixNano()
	warehouseService := &warehouse.Service{DB: db}
	warehouseRow, err := warehouseService.Create(context.Background(), tenantID, nil, warehouse.CreateInput{Code: "INV-CONCURRENT", Name: "Concurrent warehouse", IsDefault: true})
	require.NoError(t, err)
	productRow := product.Product{TenantID: tenantID, Source: "manual", Status: product.StatusDraft, Title: "Concurrent ledger product"}
	require.NoError(t, db.Create(&productRow).Error)
	legacyStock := 10
	sku := product.ProductSKU{ProductID: productRow.ID, SKUCode: "INV-CONCURRENT-SKU", SKUName: "Concurrent SKU", Stock: &legacyStock, WarningStock: 5}
	require.NoError(t, db.Create(&sku).Error)
	service := &inventory.Service{DB: db, Warehouses: warehouseService}

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i, target := range []int{7, 9} {
		wg.Add(1)
		go func(index, stock int) {
			defer wg.Done()
			<-start
			_, adjustErr := service.AdjustWarehouseStock(context.Background(), tenantID, productRow.ID, sku.ID, inventory.AdjustStockBody{
				WarehouseID: warehouseRow.ID, Stock: stock, Reason: "concurrency test",
				IdempotencyKey: fmt.Sprintf("inventory-concurrent-%d", index),
			}, nil)
			errs <- adjustErr
		}(i, target)
	}
	close(start)
	wg.Wait()
	close(errs)
	for adjustErr := range errs {
		require.NoError(t, adjustErr)
	}

	var balance inventory.WarehouseStockBalance
	require.NoError(t, db.Where("tenant_id = ? AND warehouse_id = ? AND product_sku_id = ?", tenantID, warehouseRow.ID, sku.ID).First(&balance).Error)
	var reloaded product.ProductSKU
	require.NoError(t, db.First(&reloaded, "id = ?", sku.ID).Error)
	require.NotNil(t, reloaded.Stock)
	require.Equal(t, balance.OnHand, *reloaded.Stock, "aggregate projection must match the serialized warehouse balance")
	require.Contains(t, []int{7, 9}, balance.OnHand)

	var movements []inventory.InventoryMovement
	require.NoError(t, db.Where("tenant_id = ? AND product_sku_id = ?", tenantID, sku.ID).Order("created_at ASC, id ASC").Find(&movements).Error)
	require.Len(t, movements, 3, "legacy import plus both adjustments must remain immutable")
	manualTargets := map[int]bool{}
	for _, movement := range movements {
		if movement.MovementType == inventory.MovementManualAdjust {
			manualTargets[movement.AfterOnHand] = true
		}
	}
	require.Equal(t, map[int]bool{7: true, 9: true}, manualTargets)
}

func TestOrderInventoryReservationSerializesConcurrentOrders(t *testing.T) {
	harness := postgrestest.Require(t)
	harness.EmitMetadata(t)
	db := harness.DB
	require.NoError(t, database.AutoMigrate(db))

	tenantID := time.Now().UnixNano()
	warehouseService := &warehouse.Service{DB: db}
	warehouseRow, err := warehouseService.Create(context.Background(), tenantID, nil, warehouse.CreateInput{Code: "ORDER-CONCURRENT", Name: "Order concurrent", IsDefault: true})
	require.NoError(t, err)
	productRow := product.Product{TenantID: tenantID, Source: "manual", Status: product.StatusDraft, Title: "Concurrent order ledger product"}
	require.NoError(t, db.Create(&productRow).Error)
	stock := 5
	sku := product.ProductSKU{ProductID: productRow.ID, SKUCode: "ORDER-CONCURRENT-SKU", SKUName: "Concurrent order SKU", Stock: &stock, WarningStock: 1}
	require.NoError(t, db.Create(&sku).Error)

	orders := []order.Order{
		{TenantID: tenantID, Platform: "manual", WarehouseID: &warehouseRow.ID, OrderNo: "ORDER-CONCURRENT-A-" + uuid.NewString(), CustomerName: "A", Status: order.StatusPaid, PaymentStatus: order.PaymentPaid, FulfillmentStatus: order.FulfillmentUnfulfilled, Currency: "USD"},
		{TenantID: tenantID, Platform: "manual", WarehouseID: &warehouseRow.ID, OrderNo: "ORDER-CONCURRENT-B-" + uuid.NewString(), CustomerName: "B", Status: order.StatusPaid, PaymentStatus: order.PaymentPaid, FulfillmentStatus: order.FulfillmentUnfulfilled, Currency: "USD"},
	}
	require.NoError(t, db.Create(&orders).Error)
	for i := range orders {
		require.NoError(t, db.Create(&order.OrderItem{OrderID: orders[i].ID, ProductID: &productRow.ID, ProductSKUID: &sku.ID, ProductTitle: "Concurrent order item", Quantity: 4}).Error)
	}
	service := &inventory.Service{DB: db, Warehouses: warehouseService}

	start := make(chan struct{})
	errs := make(chan error, len(orders))
	var wg sync.WaitGroup
	for i := range orders {
		wg.Add(1)
		go func(orderID uuid.UUID) {
			defer wg.Done()
			<-start
			_, reserveErr := service.DeductInventoryForOrder(context.Background(), orderID, inventory.OrderInventoryOptions{Reason: "concurrent reserve"})
			errs <- reserveErr
		}(orders[i].ID)
	}
	close(start)
	wg.Wait()
	close(errs)
	var successCount, insufficientCount int
	for reserveErr := range errs {
		if reserveErr == nil {
			successCount++
		} else if errors.Is(reserveErr, inventory.ErrInsufficientSKUStock) {
			insufficientCount++
		} else {
			require.NoError(t, reserveErr)
		}
	}
	require.Equal(t, 1, successCount)
	require.Equal(t, 1, insufficientCount)

	var balance inventory.WarehouseStockBalance
	require.NoError(t, db.Where("tenant_id = ? AND warehouse_id = ? AND product_sku_id = ?", tenantID, warehouseRow.ID, sku.ID).First(&balance).Error)
	require.Equal(t, 5, balance.OnHand)
	require.Equal(t, 4, balance.Reserved)
	var reloaded product.ProductSKU
	require.NoError(t, db.First(&reloaded, "id = ?", sku.ID).Error)
	require.NotNil(t, reloaded.Stock)
	require.Equal(t, 5, *reloaded.Stock)

	var reserveMovements int64
	require.NoError(t, db.Model(&inventory.InventoryMovement{}).Where("product_sku_id = ? AND movement_type = ?", sku.ID, inventory.MovementOrderReserve).Count(&reserveMovements).Error)
	require.EqualValues(t, 1, reserveMovements)
}
