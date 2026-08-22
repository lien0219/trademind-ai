package inventory

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/warehouse"
	"gorm.io/gorm"
)

type warehouseLedgerFixture struct {
	db      *gorm.DB
	service *Service
	product *product.Product
	sku     *product.ProductSKU
	main    *warehouse.Warehouse
	second  *warehouse.Warehouse
}

func newWarehouseLedgerFixture(t *testing.T, withDefault bool) *warehouseLedgerFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:warehouse_ledger_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&admin.AdminUser{}, &product.Product{}, &product.ProductSKU{}, &warehouse.Warehouse{}, &WarehouseStockBalance{}, &InventoryMovement{}, &InventoryChangeLog{}, &InventoryStocktake{}, &InventoryStocktakeItem{}, &InventoryStocktakeAction{}); err != nil {
		t.Fatal(err)
	}
	warehouseService := &warehouse.Service{DB: db}
	main, err := warehouseService.Create(context.Background(), 1, nil, warehouse.CreateInput{Code: "MAIN", Name: "Main", IsDefault: withDefault})
	if err != nil {
		t.Fatal(err)
	}
	second, err := warehouseService.Create(context.Background(), 1, nil, warehouse.CreateInput{Code: "SECOND", Name: "Second"})
	if err != nil {
		t.Fatal(err)
	}
	productRow := &product.Product{TenantID: 1, Source: "manual", Status: product.StatusDraft, Title: "Ledger product"}
	if err := db.Create(productRow).Error; err != nil {
		t.Fatal(err)
	}
	stock := 10
	sku := &product.ProductSKU{ProductID: productRow.ID, SKUCode: "LEDGER-1", SKUName: "Ledger SKU", Stock: &stock, WarningStock: 5}
	if err := db.Create(sku).Error; err != nil {
		t.Fatal(err)
	}
	return &warehouseLedgerFixture{
		db: db, service: &Service{DB: db, Warehouses: warehouseService},
		product: productRow, sku: sku, main: main, second: second,
	}
}

func TestManualAdjustmentMigratesLegacyStockAndReplaysIdempotently(t *testing.T) {
	fx := newWarehouseLedgerFixture(t, true)
	ctx := context.Background()
	body := AdjustStockBody{
		WarehouseID: fx.main.ID, Stock: 7, Reason: "count", Remark: "shelf count",
		IdempotencyKey: "manual-adjust-001",
	}
	result, err := fx.service.AdjustWarehouseStock(ctx, 1, fx.product.ID, fx.sku.ID, body, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.AggregateStock != 7 || result.WarehouseOnHand != 7 || result.IdempotentReplay {
		t.Fatalf("unexpected adjustment result: %#v", result)
	}
	replay, err := fx.service.AdjustWarehouseStock(ctx, 1, fx.product.ID, fx.sku.ID, body, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !replay.IdempotentReplay || replay.MovementID != result.MovementID {
		t.Fatalf("expected stable idempotent replay: %#v", replay)
	}
	conflict := body
	conflict.Stock = 8
	if _, err := fx.service.AdjustWarehouseStock(ctx, 1, fx.product.ID, fx.sku.ID, conflict, nil); !errors.Is(err, ErrInventoryIdempotency) {
		t.Fatalf("expected idempotency conflict, got %v", err)
	}

	second, err := fx.service.AdjustWarehouseStock(ctx, 1, fx.product.ID, fx.sku.ID, AdjustStockBody{
		WarehouseID: fx.second.ID, Stock: 3, Reason: "count", IdempotencyKey: "manual-adjust-002",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if second.AggregateStock != 10 {
		t.Fatalf("expected aggregate projection 10, got %#v", second)
	}
	replay, err = fx.service.AdjustWarehouseStock(ctx, 1, fx.product.ID, fx.sku.ID, body, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !replay.IdempotentReplay || replay.AggregateStock != result.AggregateStock {
		t.Fatalf("expected replay to return the original aggregate result: %#v", replay)
	}
	var sku product.ProductSKU
	if err := fx.db.First(&sku, "id = ?", fx.sku.ID).Error; err != nil {
		t.Fatal(err)
	}
	if sku.Stock == nil || *sku.Stock != 10 || sku.StockStatus != product.StockStatusNormal {
		t.Fatalf("unexpected SKU projection: %#v", sku)
	}
	var movementCount int64
	if err := fx.db.Model(&InventoryMovement{}).Count(&movementCount).Error; err != nil {
		t.Fatal(err)
	}
	if movementCount != 3 {
		t.Fatalf("expected one import and two manual movements, got %d", movementCount)
	}
}

func TestWarehouseLedgerAllowsLegacyTenantZero(t *testing.T) {
	fx := newWarehouseLedgerFixture(t, true)
	ctx := context.Background()
	if err := fx.db.Model(&warehouse.Warehouse{}).Where("tenant_id = ?", 1).Update("tenant_id", 0).Error; err != nil {
		t.Fatal(err)
	}
	if err := fx.db.Model(&product.Product{}).Where("tenant_id = ?", 1).Update("tenant_id", 0).Error; err != nil {
		t.Fatal(err)
	}
	result, err := fx.service.AdjustWarehouseStock(ctx, 0, fx.product.ID, fx.sku.ID, AdjustStockBody{
		WarehouseID: fx.main.ID, Stock: 6, IdempotencyKey: "legacy-ledger-001",
	}, nil)
	if err != nil {
		t.Fatalf("adjust legacy tenant stock: %v", err)
	}
	if result.AggregateStock != 6 || result.WarehouseOnHand != 6 {
		t.Fatalf("unexpected legacy tenant adjustment: %#v", result)
	}
	if _, err := fx.service.ReconcileWarehouseLedger(ctx, 0, 1, 20, ""); err != nil {
		t.Fatalf("reconcile legacy tenant ledger: %v", err)
	}
}

func TestManualAdjustmentUsesDefaultForLegacyStockAndPreservesCompatibilityDelta(t *testing.T) {
	fx := newWarehouseLedgerFixture(t, true)
	ctx := context.Background()
	result, err := fx.service.AdjustWarehouseStock(ctx, 1, fx.product.ID, fx.sku.ID, AdjustStockBody{
		WarehouseID: fx.second.ID, Stock: 3, Reason: "count", IdempotencyKey: "manual-adjust-other-001",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.AggregateStock != 13 {
		t.Fatalf("expected legacy 10 plus second warehouse 3, got %#v", result)
	}
	balances, err := fx.service.ListWarehouseBalances(ctx, 1, fx.product.ID, fx.sku.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(balances) != 2 || balances[0].WarehouseID != fx.main.ID || balances[0].OnHand != 10 || balances[1].WarehouseID != fx.second.ID || balances[1].OnHand != 3 {
		t.Fatalf("expected legacy stock in default warehouse and adjustment in selected warehouse: %#v", balances)
	}

	// Simulate an order-path compatibility deduction that has not moved to the
	// warehouse ledger yet. A later manual adjustment must preserve that delta.
	if err := fx.db.Model(&product.ProductSKU{}).Where("id = ?", fx.sku.ID).Update("stock", 11).Error; err != nil {
		t.Fatal(err)
	}
	result, err = fx.service.AdjustWarehouseStock(ctx, 1, fx.product.ID, fx.sku.ID, AdjustStockBody{
		WarehouseID: fx.second.ID, Stock: 4, Reason: "count", IdempotencyKey: "manual-adjust-other-002",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.AggregateStock != 12 {
		t.Fatalf("expected compatibility aggregate 11 plus warehouse delta 1, got %#v", result)
	}
	reconciliation, err := fx.service.ReconcileWarehouseLedger(ctx, 1, 1, 20, reconciliationMismatch)
	if err != nil {
		t.Fatal(err)
	}
	if reconciliation.Total != 1 || len(reconciliation.Items) != 1 || reconciliation.Items[0].Difference != 2 {
		t.Fatalf("expected preserved compatibility difference to remain visible: %#v", reconciliation)
	}
}

func TestLegacyMigrationUsesPendingWarehouseAndReconciles(t *testing.T) {
	fx := newWarehouseLedgerFixture(t, false)
	stock := 4
	secondSKU := &product.ProductSKU{ProductID: fx.product.ID, SKUCode: "LEDGER-2", SKUName: "Second", Stock: &stock}
	if err := fx.db.Create(secondSKU).Error; err != nil {
		t.Fatal(err)
	}
	first, err := fx.service.MigrateLegacyStock(context.Background(), 1, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if first.WarehouseCode != pendingAllocationWarehouseCode || first.MigratedCount != 1 || first.RemainingCount != 1 {
		t.Fatalf("unexpected first migration batch: %#v", first)
	}
	second, err := fx.service.MigrateLegacyStock(context.Background(), 1, nil, 100)
	if err != nil {
		t.Fatal(err)
	}
	if second.WarehouseID != first.WarehouseID || second.MigratedCount != 1 || second.RemainingCount != 0 {
		t.Fatalf("unexpected second migration batch: %#v", second)
	}

	reconciliation, err := fx.service.ReconcileWarehouseLedger(context.Background(), 1, 1, 20, "")
	if err != nil {
		t.Fatal(err)
	}
	if reconciliation.Matched != 2 || reconciliation.Unmigrated != 0 || reconciliation.Mismatch != 0 {
		t.Fatalf("unexpected matched reconciliation: %#v", reconciliation)
	}
	if err := fx.db.Model(&product.ProductSKU{}).Where("id = ?", secondSKU.ID).Update("stock", 99).Error; err != nil {
		t.Fatal(err)
	}
	mismatch, err := fx.service.ReconcileWarehouseLedger(context.Background(), 1, 1, 20, reconciliationMismatch)
	if err != nil {
		t.Fatal(err)
	}
	if mismatch.Total != 1 || len(mismatch.Items) != 1 || mismatch.Items[0].Difference != -95 {
		t.Fatalf("unexpected mismatch reconciliation: %#v", mismatch)
	}
}

func TestLegacyMigrationDoesNotCreatePendingWarehouseForEmptyBatch(t *testing.T) {
	fx := newWarehouseLedgerFixture(t, false)
	if err := fx.db.Delete(&product.ProductSKU{}, "id = ?", fx.sku.ID).Error; err != nil {
		t.Fatal(err)
	}

	result, err := fx.service.MigrateLegacyStock(context.Background(), 1, nil, 100)
	if err != nil {
		t.Fatal(err)
	}
	if result.MigratedCount != 0 || result.RemainingCount != 0 || result.WarehouseID != uuid.Nil || result.WarehouseCode != "" {
		t.Fatalf("unexpected empty migration result: %#v", result)
	}
	var pendingCount int64
	if err := fx.db.Model(&warehouse.Warehouse{}).
		Where("tenant_id = ? AND code = ?", 1, pendingAllocationWarehouseCode).
		Count(&pendingCount).Error; err != nil {
		t.Fatal(err)
	}
	if pendingCount != 0 {
		t.Fatalf("expected no pending allocation warehouse, got %d", pendingCount)
	}
}
