//go:build inventorypostgres

package integration

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/database"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/procurement"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/supplier"
	"github.com/trademind-ai/trademind/backend/internal/modules/warehouse"
	"github.com/trademind-ai/trademind/backend/internal/testing/postgrestest"
)

func TestPurchaseReturnPostgresSerializesReturnAllocation(t *testing.T) {
	harness := postgrestest.Require(t)
	harness.EmitMetadata(t)
	db := harness.DB
	require.NoError(t, database.AutoMigrate(db))
	ctx := context.Background()
	tenantID := int64(910303)

	warehouseService := &warehouse.Service{DB: db}
	supplierService := &supplier.Service{DB: db}
	warehouseRow, err := warehouseService.Create(ctx, tenantID, nil, warehouse.CreateInput{Code: "RETURN-PG", Name: "Return PostgreSQL warehouse", IsDefault: true})
	require.NoError(t, err)
	supplierRow, err := supplierService.Create(ctx, tenantID, nil, supplier.CreateInput{Code: "RETURN-PG-SUP", Name: "Return PostgreSQL supplier"})
	require.NoError(t, err)
	productRow := product.Product{TenantID: tenantID, Source: "test", Status: product.StatusDraft, Title: "Return PostgreSQL product"}
	require.NoError(t, db.Create(&productRow).Error)
	stock := 0
	sku := product.ProductSKU{ProductID: productRow.ID, SKUCode: "RETURN-PG-SKU", SKUName: "Return PostgreSQL SKU", Stock: &stock}
	require.NoError(t, db.Create(&sku).Error)

	service := &procurement.Service{DB: db, Warehouses: warehouseService, Suppliers: supplierService, Stock: inventory.WarehouseStockService{}}
	orderRow, err := service.Create(ctx, tenantID, nil, procurement.CreatePurchaseOrderInput{
		IdempotencyKey: "return-pg-order-create", SupplierID: supplierRow.ID, WarehouseID: warehouseRow.ID, Currency: "CNY",
		Items: []procurement.CreatePurchaseOrderItemInput{{ProductSKUID: sku.ID, Quantity: 5, UnitCostMinor: 100}},
	})
	require.NoError(t, err)
	orderRow, err = service.Submit(ctx, tenantID, orderRow.ID, orderRow.Revision)
	require.NoError(t, err)
	approver := uuid.New()
	orderRow, err = service.Approve(ctx, tenantID, orderRow.ID, orderRow.Revision, &approver)
	require.NoError(t, err)
	receipt, err := service.Receive(ctx, tenantID, orderRow.ID, nil, procurement.ReceivePurchaseOrderInput{
		ExpectedRevision: orderRow.Revision, IdempotencyKey: "return-pg-receipt-create",
		Items: []procurement.ReceivePurchaseOrderItemInput{{PurchaseOrderItemID: orderRow.Items[0].ID, Quantity: 5}},
	})
	require.NoError(t, err)
	require.Len(t, receipt.Receipt.Items, 1)

	start := make(chan struct{})
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			_, errs[index] = service.CreatePurchaseReturn(ctx, tenantID, nil, procurement.CreatePurchaseReturnInput{
				IdempotencyKey:  []string{"return-pg-allocation-one", "return-pg-allocation-two"}[index],
				PurchaseOrderID: orderRow.ID, Reason: "concurrent quality return",
				Items: []procurement.CreatePurchaseReturnItemInput{{GoodsReceiptItemID: receipt.Receipt.Items[0].ID, Quantity: 3}},
			})
		}(i)
	}
	close(start)
	wg.Wait()

	successes, overReturns := 0, 0
	for _, createErr := range errs {
		switch {
		case createErr == nil:
			successes++
		case errors.Is(createErr, procurement.ErrOverReturn):
			overReturns++
		default:
			t.Fatalf("unexpected concurrent return error: %v", createErr)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, overReturns)
	var count int64
	var allocated int
	require.NoError(t, db.Model(&procurement.PurchaseReturn{}).Where("tenant_id = ? AND status <> ?", tenantID, procurement.ReturnStatusCancelled).Count(&count).Error)
	require.NoError(t, db.Model(&procurement.PurchaseReturnItem{}).Where("tenant_id = ? AND goods_receipt_item_id = ?", tenantID, receipt.Receipt.Items[0].ID).Select("COALESCE(SUM(quantity), 0)").Scan(&allocated).Error)
	require.Equal(t, int64(1), count)
	require.Equal(t, 3, allocated)
}
