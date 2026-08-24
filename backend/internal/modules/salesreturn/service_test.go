package salesreturn

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	ordermod "github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/modules/warehouse"
	"gorm.io/gorm"
)

type fixture struct {
	db        *gorm.DB
	service   *Service
	warehouse *warehouse.Warehouse
	order     *ordermod.Order
	item      *ordermod.OrderItem
	product   *product.Product
	sku       *product.ProductSKU
	shop      *shop.Shop
}

func newFixture(t *testing.T, deducted int) *fixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:sales_return_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})
	if err := db.AutoMigrate(
		&product.Product{}, &product.ProductSKU{}, &warehouse.Warehouse{},
		&ordermod.Order{}, &ordermod.OrderItem{}, &shop.Shop{}, &inventory.OrderInventoryEffect{},
		&inventory.WarehouseStockBalance{}, &inventory.InventoryMovement{}, &inventory.InventoryChangeLog{},
		&SalesReturn{}, &SalesReturnItem{}, &SalesReturnAction{}, &SalesReturnInventoryEffect{},
		&PlatformAfterSale{}, &PlatformAfterSaleEvent{},
		&RefundExecution{}, &RefundExecutionEvent{},
	); err != nil {
		t.Fatal(err)
	}
	warehouseService := &warehouse.Service{DB: db}
	warehouseRow, err := warehouseService.Create(context.Background(), 7, nil, warehouse.CreateInput{Code: "SALES", Name: "Sales warehouse", IsDefault: true})
	if err != nil {
		t.Fatal(err)
	}
	productRow := &product.Product{TenantID: 7, Source: "manual", Status: product.StatusDraft, Title: "Returned product"}
	if err := db.Create(productRow).Error; err != nil {
		t.Fatal(err)
	}
	shopRow := &shop.Shop{TenantID: 7, Platform: "douyin_shop", ExternalShopID: "platform-shop-1", ShopName: "Test Douyin Shop", Status: shop.StatusActive, AuthStatus: shop.AuthAuthorized}
	if err := db.Create(shopRow).Error; err != nil {
		t.Fatal(err)
	}
	stock := 10 - deducted
	sku := &product.ProductSKU{ProductID: productRow.ID, SKUCode: "RETURN-SKU", SKUName: "Returned SKU", Stock: &stock}
	if err := db.Create(sku).Error; err != nil {
		t.Fatal(err)
	}
	orderRow := &ordermod.Order{
		TenantID: 7, Platform: "manual", ShopID: &shopRow.ID, WarehouseID: &warehouseRow.ID, OrderNo: "ORDER-" + uuid.NewString()[:8],
		CustomerName: "Buyer", Status: ordermod.StatusDelivered, PaymentStatus: ordermod.PaymentPaid,
		FulfillmentStatus: ordermod.FulfillmentFulfilled, Currency: "CNY",
	}
	if err := db.Create(orderRow).Error; err != nil {
		t.Fatal(err)
	}
	item := &ordermod.OrderItem{OrderID: orderRow.ID, ProductID: &productRow.ID, ProductSKUID: &sku.ID, ProductTitle: productRow.Title, SKUName: sku.SKUName, SKUCode: sku.SKUCode, Quantity: deducted, UnitPrice: 10, TotalPrice: float64(deducted) * 10}
	if err := db.Create(item).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&inventory.WarehouseStockBalance{TenantID: 7, WarehouseID: warehouseRow.ID, ProductSKUID: sku.ID, OnHand: stock, Version: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&inventory.OrderInventoryEffect{
		TenantID: 7, OrderID: orderRow.ID, OrderItemID: item.ID, WarehouseID: &warehouseRow.ID,
		ProductID: &productRow.ID, ProductSKUID: sku.ID, EffectType: inventory.EffectTypeDeduct,
		Quantity: deducted, Status: inventory.InventoryEffectSuccess,
	}).Error; err != nil {
		t.Fatal(err)
	}
	return &fixture{
		db: db, service: &Service{DB: db, Stock: inventory.WarehouseStockService{}, Warehouses: warehouseService},
		warehouse: warehouseRow, order: orderRow, item: item, product: productRow, sku: sku, shop: shopRow,
	}
}

func (fx *fixture) create(t *testing.T, key, returnType, disposition string, quantity int) *SalesReturn {
	t.Helper()
	row, err := fx.service.Create(t.Context(), 7, uuidPointer(uuid.New()), CreateInput{
		IdempotencyKey: key, OrderID: fx.order.ID, Type: returnType, Reason: "customer request",
		Items: []CreateItemInput{{OrderItemID: fx.item.ID, Quantity: quantity, Disposition: disposition, RefundAmountMinor: int64(quantity * 1000)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return row
}

func advanceToApproved(t *testing.T, fx *fixture, row *SalesReturn, prefix string, approver uuid.UUID) *SalesReturn {
	t.Helper()
	var err error
	row, err = fx.service.Submit(t.Context(), 7, row.ID, uuidPointer(uuid.New()), ActionInput{ExpectedRevision: row.Revision, IdempotencyKey: prefix + "-submit"})
	if err != nil {
		t.Fatal(err)
	}
	row, err = fx.service.Approve(t.Context(), 7, row.ID, &approver, ActionInput{ExpectedRevision: row.Revision, IdempotencyKey: prefix + "-approve"})
	if err != nil {
		t.Fatal(err)
	}
	return row
}

func TestSalesReturnLifecycleProtectsQuantityAndPostsDispositionExactlyOnce(t *testing.T) {
	fx := newFixture(t, 3)
	row := fx.create(t, "sales-return-create-001", TypeReturnRefund, inventory.ReturnDispositionSellable, 2)
	if row.Status != StatusDraft || row.Revision != 1 || row.RefundAmountMinor != 2000 || row.OrderNo == "" || len(row.Items) != 1 || row.Items[0].DeductedQuantity != 3 {
		t.Fatalf("unexpected sales return draft: %#v", row)
	}
	replay, err := fx.service.Create(t.Context(), 7, nil, CreateInput{
		IdempotencyKey: "sales-return-create-001", OrderID: fx.order.ID, Type: TypeReturnRefund, Reason: "customer request",
		Items: []CreateItemInput{{OrderItemID: fx.item.ID, Quantity: 2, Disposition: inventory.ReturnDispositionSellable, RefundAmountMinor: 2000}},
	})
	if err != nil || replay.ID != row.ID {
		t.Fatalf("create replay failed: row=%#v err=%v", replay, err)
	}
	if _, err := fx.service.Create(t.Context(), 7, nil, CreateInput{
		IdempotencyKey: "sales-return-over-001", OrderID: fx.order.ID, Type: TypeReturnRefund, Reason: "customer request",
		Items: []CreateItemInput{{OrderItemID: fx.item.ID, Quantity: 2, Disposition: inventory.ReturnDispositionSellable, RefundAmountMinor: 2000}},
	}); !errors.Is(err, ErrOverReturn) {
		t.Fatalf("expected cumulative over-return rejection, got %v", err)
	}

	approver := uuid.New()
	row = advanceToApproved(t, fx, row, "sales-return-001", approver)
	if _, err := fx.service.Complete(t.Context(), 7, row.ID, &approver, ActionInput{ExpectedRevision: row.Revision, IdempotencyKey: "sales-return-self-complete"}); !errors.Is(err, ErrDutyConflict) {
		t.Fatalf("approver must not receive own return: %v", err)
	}
	executor := uuid.New()
	row, err = fx.service.Complete(t.Context(), 7, row.ID, &executor, ActionInput{ExpectedRevision: row.Revision, IdempotencyKey: "sales-return-001-complete", Reason: "received"})
	if err != nil || row.Status != StatusCompleted || row.Revision != 4 {
		t.Fatalf("complete sellable return: row=%#v err=%v", row, err)
	}
	replay, err = fx.service.Complete(t.Context(), 7, row.ID, &executor, ActionInput{ExpectedRevision: 3, IdempotencyKey: "sales-return-001-complete", Reason: "received"})
	if err != nil || replay.Revision != 4 {
		t.Fatalf("complete replay failed: row=%#v err=%v", replay, err)
	}

	damaged := fx.create(t, "sales-return-create-002", TypeReturnRefund, inventory.ReturnDispositionDamaged, 1)
	damaged = advanceToApproved(t, fx, damaged, "sales-return-002", uuid.New())
	damaged, err = fx.service.Complete(t.Context(), 7, damaged.ID, uuidPointer(uuid.New()), ActionInput{ExpectedRevision: damaged.Revision, IdempotencyKey: "sales-return-002-complete", Reason: "damaged on arrival"})
	if err != nil || damaged.Status != StatusCompleted {
		t.Fatalf("complete damaged return: row=%#v err=%v", damaged, err)
	}

	var balance inventory.WarehouseStockBalance
	if err := fx.db.Where("tenant_id = ? AND warehouse_id = ? AND product_sku_id = ?", 7, fx.warehouse.ID, fx.sku.ID).First(&balance).Error; err != nil {
		t.Fatal(err)
	}
	if balance.OnHand != 10 || balance.Damaged != 1 || balance.Available() != 9 {
		t.Fatalf("unexpected return balance: %#v", balance)
	}
	var sku product.ProductSKU
	if err := fx.db.First(&sku, "id = ?", fx.sku.ID).Error; err != nil {
		t.Fatal(err)
	}
	if sku.Stock == nil || *sku.Stock != 9 {
		t.Fatalf("damaged return must not increase sellable projection: %#v", sku.Stock)
	}
	var movements, changes, effects int64
	_ = fx.db.Model(&inventory.InventoryMovement{}).Where("movement_type = ?", inventory.MovementSalesReturn).Count(&movements).Error
	_ = fx.db.Model(&inventory.InventoryChangeLog{}).Where("change_type = ?", inventory.ChangeSalesReturn).Count(&changes).Error
	_ = fx.db.Model(&SalesReturnInventoryEffect{}).Count(&effects).Error
	if movements != 2 || changes != 2 || effects != 2 {
		t.Fatalf("return facts must be exactly once, movements=%d changes=%d effects=%d", movements, changes, effects)
	}
	returnable, err := fx.service.ListReturnableItems(t.Context(), 7, fx.order.ID)
	if err != nil || len(returnable.List) != 0 {
		t.Fatalf("all deducted quantity should be allocated: result=%#v err=%v", returnable, err)
	}
	if _, err := fx.service.Get(t.Context(), 8, row.ID); !errors.Is(err, ErrAbsent) {
		t.Fatalf("cross-tenant return must look absent: %v", err)
	}
}

func TestRefundOnlyCompletionDoesNotMutateInventory(t *testing.T) {
	fx := newFixture(t, 2)
	if err := fx.db.Model(&warehouse.Warehouse{}).Where("id = ?", fx.warehouse.ID).Update("status", warehouse.StatusInactive).Error; err != nil {
		t.Fatal(err)
	}
	row := fx.create(t, "refund-only-create-001", TypeRefundOnly, "", 1)
	row = advanceToApproved(t, fx, row, "refund-only-001", uuid.New())
	completed, err := fx.service.Complete(t.Context(), 7, row.ID, uuidPointer(uuid.New()), ActionInput{ExpectedRevision: row.Revision, IdempotencyKey: "refund-only-complete-001"})
	if err != nil || completed.Status != StatusCompleted {
		t.Fatalf("complete refund only: row=%#v err=%v", completed, err)
	}
	var movements, changes, effects int64
	_ = fx.db.Model(&inventory.InventoryMovement{}).Count(&movements).Error
	_ = fx.db.Model(&inventory.InventoryChangeLog{}).Count(&changes).Error
	_ = fx.db.Model(&SalesReturnInventoryEffect{}).Count(&effects).Error
	if movements != 0 || changes != 0 || effects != 0 {
		t.Fatalf("refund-only must not write inventory facts: movements=%d changes=%d effects=%d", movements, changes, effects)
	}
}

func TestSalesReturnCancellationReleasesAllocation(t *testing.T) {
	fx := newFixture(t, 2)
	row := fx.create(t, "sales-return-cancel-create", TypeReturnRefund, inventory.ReturnDispositionSellable, 2)
	cancelled, err := fx.service.Cancel(t.Context(), 7, row.ID, uuidPointer(uuid.New()), ActionInput{ExpectedRevision: row.Revision, IdempotencyKey: "sales-return-cancel-action"})
	if err != nil || cancelled.Status != StatusCancelled {
		t.Fatalf("cancel return: row=%#v err=%v", cancelled, err)
	}
	returnable, err := fx.service.ListReturnableItems(t.Context(), 7, fx.order.ID)
	if err != nil || len(returnable.List) != 1 || returnable.List[0].RemainingQuantity != 2 || returnable.List[0].AllocatedReturnQuantity != 0 {
		t.Fatalf("cancelled return must release allocation: result=%#v err=%v", returnable, err)
	}
}

func TestSalesReturnCompletionFailureRollsBackStateAndInventory(t *testing.T) {
	fx := newFixture(t, 1)
	row := fx.create(t, "sales-return-rollback-create", TypeReturnRefund, inventory.ReturnDispositionSellable, 1)
	row = advanceToApproved(t, fx, row, "sales-return-rollback", uuid.New())
	if err := fx.db.Callback().Create().Before("gorm:create").Register("fail_sales_return_change_log", func(db *gorm.DB) {
		if db.Statement != nil && db.Statement.Schema != nil && db.Statement.Schema.Table == "inventory_change_logs" {
			db.AddError(errors.New("forced change log failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = fx.db.Callback().Create().Remove("fail_sales_return_change_log") })
	if _, err := fx.service.Complete(t.Context(), 7, row.ID, uuidPointer(uuid.New()), ActionInput{ExpectedRevision: row.Revision, IdempotencyKey: "sales-return-rollback-complete"}); err == nil {
		t.Fatal("expected forced completion failure")
	}
	reloaded, err := fx.service.Get(t.Context(), 7, row.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Status != StatusApproved || reloaded.Revision != 3 || reloaded.CompletedAt != nil {
		t.Fatalf("failed completion must preserve approved state: %#v", reloaded)
	}
	var actionCount, movementCount, effectCount int64
	_ = fx.db.Model(&SalesReturnAction{}).Where("sales_return_id = ? AND action = ?", row.ID, "complete").Count(&actionCount).Error
	_ = fx.db.Model(&inventory.InventoryMovement{}).Where("source_id = ?", row.ID).Count(&movementCount).Error
	_ = fx.db.Model(&SalesReturnInventoryEffect{}).Where("sales_return_id = ?", row.ID).Count(&effectCount).Error
	if actionCount != 0 || movementCount != 0 || effectCount != 0 {
		t.Fatalf("failed completion leaked facts: actions=%d movements=%d effects=%d", actionCount, movementCount, effectCount)
	}
	var balance inventory.WarehouseStockBalance
	if err := fx.db.First(&balance, "tenant_id = ? AND warehouse_id = ? AND product_sku_id = ?", 7, fx.warehouse.ID, fx.sku.ID).Error; err != nil {
		t.Fatal(err)
	}
	if balance.OnHand != 9 {
		t.Fatalf("failed completion changed balance: %#v", balance)
	}
}

func uuidPointer(value uuid.UUID) *uuid.UUID { return &value }
