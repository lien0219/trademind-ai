package procurement

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/supplier"
	"github.com/trademind-ai/trademind/backend/internal/modules/warehouse"
	"gorm.io/gorm"
)

type procurementFixture struct {
	DB          *gorm.DB
	Service     *Service
	Warehouse   *warehouse.Warehouse
	Supplier    *supplier.Supplier
	SupplierSKU *supplier.SupplierSKU
	ProductSKU  *product.ProductSKU
}

func newProcurementFixture(t *testing.T) *procurementFixture {
	t.Helper()
	dsn := fmt.Sprintf("file:procurement_%s?mode=memory&cache=shared", uuid.NewString())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})
	if err := db.AutoMigrate(
		&product.Product{}, &product.ProductSKU{},
		&warehouse.Warehouse{}, &supplier.Supplier{}, &supplier.SupplierSKU{},
		&inventory.WarehouseStockBalance{}, &inventory.InventoryMovement{}, &inventory.InventoryChangeLog{},
		&PurchaseOrder{}, &PurchaseOrderItem{}, &GoodsReceipt{}, &GoodsReceiptItem{},
	); err != nil {
		t.Fatalf("migrate fixture: %v", err)
	}

	ctx := context.Background()
	warehouseService := &warehouse.Service{DB: db}
	supplierService := &supplier.Service{DB: db}
	warehouseRow, err := warehouseService.Create(ctx, 1, nil, warehouse.CreateInput{Code: "MAIN", Name: "Main warehouse", IsDefault: true})
	if err != nil {
		t.Fatalf("create warehouse: %v", err)
	}
	supplierRow, err := supplierService.Create(ctx, 1, nil, supplier.CreateInput{Code: "SUP-001", Name: "Primary supplier"})
	if err != nil {
		t.Fatalf("create supplier: %v", err)
	}
	productRow := &product.Product{TenantID: 1, Source: "manual", Status: product.StatusDraft, Title: "Test product"}
	if err := db.Create(productRow).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}
	legacyStock := 10
	productSKU := &product.ProductSKU{ProductID: productRow.ID, SKUCode: "SKU-001", SKUName: "Test SKU", Stock: &legacyStock}
	if err := db.Create(productSKU).Error; err != nil {
		t.Fatalf("create product SKU: %v", err)
	}
	supplierSKU, err := supplierService.BindSKU(ctx, 1, supplierRow.ID, supplier.BindSKUInput{
		ProductSKUID: productSKU.ID, SupplierSKUCode: "VENDOR-001", UnitCostMinor: 2500, Currency: "CNY", MinOrderQty: 1,
	})
	if err != nil {
		t.Fatalf("bind supplier SKU: %v", err)
	}
	service := &Service{
		DB: db, Warehouses: warehouseService, Suppliers: supplierService, Stock: inventory.WarehouseStockService{},
	}
	return &procurementFixture{DB: db, Service: service, Warehouse: warehouseRow, Supplier: supplierRow, SupplierSKU: supplierSKU, ProductSKU: productSKU}
}

func TestPurchaseOrderPartialReceiptIsTransactionalAndIdempotent(t *testing.T) {
	fx := newProcurementFixture(t)
	ctx := context.Background()
	createInput := CreatePurchaseOrderInput{
		IdempotencyKey: "purchase-order-001",
		SupplierID:     fx.Supplier.ID, WarehouseID: fx.Warehouse.ID, Currency: "cny",
		Items: []CreatePurchaseOrderItemInput{{ProductSKUID: fx.ProductSKU.ID, SupplierSKUID: &fx.SupplierSKU.ID, Quantity: 5, UnitCostMinor: 2500}},
	}
	po, err := fx.Service.Create(ctx, 1, nil, createInput)
	if err != nil {
		t.Fatalf("create purchase order: %v", err)
	}
	if po.Status != StatusDraft || po.Revision != 1 || po.TotalAmountMinor != 12500 || len(po.Items) != 1 || po.Items[0].ProductTitle != "Test product" || po.Items[0].SKUName != "Test SKU" {
		t.Fatalf("unexpected draft: %#v", po)
	}
	replayedPO, err := fx.Service.Create(ctx, 1, nil, createInput)
	if err != nil || replayedPO.ID != po.ID {
		t.Fatalf("purchase order create should replay idempotently: row=%#v err=%v", replayedPO, err)
	}
	conflictingCreate := createInput
	conflictingCreate.Items = []CreatePurchaseOrderItemInput{{ProductSKUID: fx.ProductSKU.ID, Quantity: 4, UnitCostMinor: 2500}}
	if _, err := fx.Service.Create(ctx, 1, nil, conflictingCreate); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("expected purchase create idempotency conflict, got %v", err)
	}
	po, err = fx.Service.Submit(ctx, 1, po.ID, 1)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	po, err = fx.Service.Approve(ctx, 1, po.ID, 2, nil)
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if po.Status != StatusApproved || po.Revision != 3 {
		t.Fatalf("unexpected approved order: %#v", po)
	}

	firstInput := ReceivePurchaseOrderInput{
		ExpectedRevision: 3, IdempotencyKey: "receipt-attempt-001",
		Items: []ReceivePurchaseOrderItemInput{{PurchaseOrderItemID: po.Items[0].ID, Quantity: 2}},
	}
	first, err := fx.Service.Receive(ctx, 1, po.ID, nil, firstInput)
	if err != nil {
		t.Fatalf("first receipt: %v", err)
	}
	if first.PurchaseOrder.Status != StatusPartiallyReceived || first.PurchaseOrder.Revision != 4 || first.PurchaseOrder.Items[0].ReceivedQuantity != 2 {
		t.Fatalf("unexpected partial receipt result: %#v", first.PurchaseOrder)
	}
	var balance inventory.WarehouseStockBalance
	if err := fx.DB.Where("tenant_id = ? AND warehouse_id = ? AND product_sku_id = ?", 1, fx.Warehouse.ID, fx.ProductSKU.ID).First(&balance).Error; err != nil {
		t.Fatalf("load balance: %v", err)
	}
	if balance.OnHand != 12 || balance.Available() != 12 {
		t.Fatalf("legacy stock should be bootstrapped once, balance=%#v", balance)
	}
	var sku product.ProductSKU
	if err := fx.DB.First(&sku, "id = ?", fx.ProductSKU.ID).Error; err != nil {
		t.Fatalf("reload SKU: %v", err)
	}
	if sku.Stock == nil || *sku.Stock != 12 {
		t.Fatalf("legacy stock projection should be 12, got %#v", sku.Stock)
	}

	replay, err := fx.Service.Receive(ctx, 1, po.ID, nil, firstInput)
	if err != nil {
		t.Fatalf("idempotent replay with stale revision: %v", err)
	}
	if replay.Receipt.ID != first.Receipt.ID || replay.PurchaseOrder.Revision != 4 {
		t.Fatalf("replay should return original receipt and current order")
	}
	conflictInput := firstInput
	conflictInput.Items = []ReceivePurchaseOrderItemInput{{PurchaseOrderItemID: po.Items[0].ID, Quantity: 1}}
	if _, err := fx.Service.Receive(ctx, 1, po.ID, nil, conflictInput); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("expected idempotency conflict, got %v", err)
	}
	overInput := ReceivePurchaseOrderInput{
		ExpectedRevision: 4, IdempotencyKey: "receipt-attempt-over",
		Items: []ReceivePurchaseOrderItemInput{{PurchaseOrderItemID: po.Items[0].ID, Quantity: 4}},
	}
	if _, err := fx.Service.Receive(ctx, 1, po.ID, nil, overInput); !errors.Is(err, ErrOverReceipt) {
		t.Fatalf("expected over-receipt rejection, got %v", err)
	}
	var receiptCount int64
	if err := fx.DB.Model(&GoodsReceipt{}).Where("tenant_id = ? AND purchase_order_id = ?", 1, po.ID).Count(&receiptCount).Error; err != nil {
		t.Fatal(err)
	}
	if receiptCount != 1 {
		t.Fatalf("failed receipt transaction must roll back, count=%d", receiptCount)
	}

	second, err := fx.Service.Receive(ctx, 1, po.ID, nil, ReceivePurchaseOrderInput{
		ExpectedRevision: 4, IdempotencyKey: "receipt-attempt-002",
		Items: []ReceivePurchaseOrderItemInput{{PurchaseOrderItemID: po.Items[0].ID, Quantity: 3}},
	})
	if err != nil {
		t.Fatalf("final receipt: %v", err)
	}
	if second.PurchaseOrder.Status != StatusReceived || second.PurchaseOrder.Revision != 5 || second.PurchaseOrder.Items[0].ReceivedQuantity != 5 {
		t.Fatalf("unexpected completed order: %#v", second.PurchaseOrder)
	}
	if err := fx.DB.First(&sku, "id = ?", fx.ProductSKU.ID).Error; err != nil {
		t.Fatal(err)
	}
	if sku.Stock == nil || *sku.Stock != 15 {
		t.Fatalf("projected stock should be 15, got %#v", sku.Stock)
	}
	var movements int64
	if err := fx.DB.Model(&inventory.InventoryMovement{}).Count(&movements).Error; err != nil {
		t.Fatal(err)
	}
	if movements != 3 {
		t.Fatalf("expected one legacy opening fact and two receipt movements, got %d", movements)
	}
}

func TestPurchaseOrderAllowsLegacyTenantZero(t *testing.T) {
	t.Helper()
	dsn := fmt.Sprintf("file:procurement_legacy_%s?mode=memory&cache=shared", uuid.NewString())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})
	if err := db.AutoMigrate(
		&product.Product{}, &product.ProductSKU{},
		&warehouse.Warehouse{}, &supplier.Supplier{}, &supplier.SupplierSKU{},
		&inventory.WarehouseStockBalance{}, &inventory.InventoryMovement{}, &inventory.InventoryChangeLog{},
		&PurchaseOrder{}, &PurchaseOrderItem{}, &GoodsReceipt{}, &GoodsReceiptItem{},
	); err != nil {
		t.Fatalf("migrate fixture: %v", err)
	}
	ctx := context.Background()
	warehouseService := &warehouse.Service{DB: db}
	supplierService := &supplier.Service{DB: db}
	warehouseRow, err := warehouseService.Create(ctx, 0, nil, warehouse.CreateInput{Code: "MAIN", Name: "Legacy main", IsDefault: true})
	if err != nil {
		t.Fatalf("create warehouse: %v", err)
	}
	supplierRow, err := supplierService.Create(ctx, 0, nil, supplier.CreateInput{Code: "SUP-0", Name: "Legacy supplier"})
	if err != nil {
		t.Fatalf("create supplier: %v", err)
	}
	productRow := &product.Product{TenantID: 0, Source: "manual", Status: product.StatusDraft, Title: "Legacy product"}
	if err := db.Create(productRow).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}
	sku := &product.ProductSKU{ProductID: productRow.ID, SKUCode: "LEGACY-SKU", SKUName: "Legacy SKU"}
	if err := db.Create(sku).Error; err != nil {
		t.Fatalf("create sku: %v", err)
	}
	service := &Service{DB: db, Warehouses: warehouseService, Suppliers: supplierService, Stock: inventory.WarehouseStockService{}}
	row, err := service.Create(ctx, 0, nil, CreatePurchaseOrderInput{
		IdempotencyKey: "legacy-po-0001", SupplierID: supplierRow.ID, WarehouseID: warehouseRow.ID, Currency: "CNY",
		Items: []CreatePurchaseOrderItemInput{{ProductSKUID: sku.ID, Quantity: 1, UnitCostMinor: 100}},
	})
	if err != nil {
		t.Fatalf("create legacy tenant purchase order: %v", err)
	}
	if row.TenantID != 0 || row.Status != StatusDraft {
		t.Fatalf("unexpected legacy tenant purchase order: %#v", row)
	}
}

func TestPurchaseOrderTenantAndRevisionIsolation(t *testing.T) {
	fx := newProcurementFixture(t)
	ctx := context.Background()
	po, err := fx.Service.Create(ctx, 1, nil, CreatePurchaseOrderInput{
		IdempotencyKey: "purchase-order-tenant-test",
		SupplierID:     fx.Supplier.ID, WarehouseID: fx.Warehouse.ID, Currency: "CNY",
		Items: []CreatePurchaseOrderItemInput{{ProductSKUID: fx.ProductSKU.ID, Quantity: 1, UnitCostMinor: 100}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fx.Service.Get(ctx, 2, po.ID); !errors.Is(err, ErrPurchaseOrderAbsent) {
		t.Fatalf("cross-tenant read must look absent, got %v", err)
	}
	if _, err := fx.Service.Submit(ctx, 1, po.ID, 2); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale revision must be rejected, got %v", err)
	}
	reloaded, err := fx.Service.Get(ctx, 1, po.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Status != StatusDraft || reloaded.Revision != 1 {
		t.Fatalf("failed transition must not mutate order: %#v", reloaded)
	}
}

func TestERPFoundationCreatesBusinessUniquenessIndexes(t *testing.T) {
	fx := newProcurementFixture(t)
	cases := []struct {
		model any
		index string
	}{
		{&PurchaseOrder{}, "ux_purchase_order_idempotency"},
		{&PurchaseOrderItem{}, "ux_purchase_order_item_sku"},
		{&GoodsReceipt{}, "ux_goods_receipt_idempotency"},
		{&GoodsReceipt{}, "ux_goods_receipt_tenant_no"},
		{&GoodsReceiptItem{}, "ux_goods_receipt_item"},
		{&inventory.WarehouseStockBalance{}, "ux_warehouse_stock_balance"},
		{&supplier.SupplierSKU{}, "ux_supplier_sku_binding"},
	}
	for _, tc := range cases {
		if !fx.DB.Migrator().HasIndex(tc.model, tc.index) {
			t.Errorf("missing index %s", tc.index)
		}
	}
}
