package integration

import (
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	ordermod "github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/salesreturn"
	"github.com/trademind-ai/trademind/backend/internal/modules/warehouse"
	"github.com/trademind-ai/trademind/backend/internal/testing/postgrestest"
	"github.com/trademind-ai/trademind/backend/internal/testing/safeenv"
)

func TestSalesReturnConcurrentAllocationRejectsOverReturn(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	_, ok, err := safeenv.TestDatabaseURLFromEnv()
	require.NoError(t, err)
	if !ok {
		t.Skip("TEST_DATABASE_URL is not set; skipping sales return PostgreSQL concurrency test")
	}
	db := postgrestest.Require(t).DB
	require.NoError(t, db.AutoMigrate(
		&product.Product{}, &product.ProductSKU{}, &warehouse.Warehouse{},
		&ordermod.Order{}, &ordermod.OrderItem{}, &inventory.OrderInventoryEffect{},
		&inventory.WarehouseStockBalance{}, &inventory.InventoryMovement{}, &inventory.InventoryChangeLog{},
		&salesreturn.SalesReturn{}, &salesreturn.SalesReturnItem{}, &salesreturn.SalesReturnAction{}, &salesreturn.SalesReturnInventoryEffect{},
	))

	warehouseService := &warehouse.Service{DB: db}
	warehouseRow, err := warehouseService.Create(t.Context(), 71, nil, warehouse.CreateInput{Code: "SALES-PG", Name: "Sales PG warehouse", IsDefault: true})
	require.NoError(t, err)
	productRow := &product.Product{TenantID: 71, Source: "manual", Status: product.StatusDraft, Title: "Concurrent return product"}
	require.NoError(t, db.Create(productRow).Error)
	stock := 7
	sku := &product.ProductSKU{ProductID: productRow.ID, SKUCode: "SALES-PG-SKU", SKUName: "Concurrent SKU", Stock: &stock}
	require.NoError(t, db.Create(sku).Error)
	orderRow := &ordermod.Order{
		TenantID: 71, Platform: "manual", WarehouseID: &warehouseRow.ID, OrderNo: "SALES-PG-" + uuid.NewString(),
		CustomerName: "Buyer", Status: ordermod.StatusDelivered, PaymentStatus: ordermod.PaymentPaid,
		FulfillmentStatus: ordermod.FulfillmentFulfilled, Currency: "CNY",
	}
	require.NoError(t, db.Create(orderRow).Error)
	item := &ordermod.OrderItem{OrderID: orderRow.ID, ProductID: &productRow.ID, ProductSKUID: &sku.ID, ProductTitle: productRow.Title, SKUCode: sku.SKUCode, Quantity: 3, UnitPrice: 10, TotalPrice: 30}
	require.NoError(t, db.Create(item).Error)
	require.NoError(t, db.Create(&inventory.WarehouseStockBalance{TenantID: 71, WarehouseID: warehouseRow.ID, ProductSKUID: sku.ID, OnHand: 7, Version: 1}).Error)
	require.NoError(t, db.Create(&inventory.OrderInventoryEffect{
		TenantID: 71, OrderID: orderRow.ID, OrderItemID: item.ID, WarehouseID: &warehouseRow.ID,
		ProductID: &productRow.ID, ProductSKUID: sku.ID, EffectType: inventory.EffectTypeDeduct,
		Quantity: 3, Status: inventory.InventoryEffectSuccess,
	}).Error)

	service := &salesreturn.Service{DB: db, Stock: inventory.WarehouseStockService{}, Warehouses: warehouseService}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for index := 0; index < 2; index++ {
		index := index
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, createErr := service.Create(t.Context(), 71, nil, salesreturn.CreateInput{
				IdempotencyKey: "sales-return-pg-concurrent-" + string(rune('a'+index)),
				OrderID:        orderRow.ID, Type: salesreturn.TypeReturnRefund, Reason: "concurrent return",
				Items: []salesreturn.CreateItemInput{{OrderItemID: item.ID, Quantity: 2, Disposition: inventory.ReturnDispositionSellable, RefundAmountMinor: 2000}},
			})
			results <- createErr
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	var successes, overReturns int
	for result := range results {
		if result == nil {
			successes++
		} else if errors.Is(result, salesreturn.ErrOverReturn) {
			overReturns++
		} else {
			t.Fatalf("unexpected concurrent create result: %v", result)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, overReturns)
	returnable, err := service.ListReturnableItems(t.Context(), 71, orderRow.ID)
	require.NoError(t, err)
	require.Len(t, returnable.List, 1)
	require.Equal(t, 1, returnable.List[0].RemainingQuantity)
}
