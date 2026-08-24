package order

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/warehouse"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

type warehouseAllocationFixture struct {
	db        *gorm.DB
	orders    *Service
	inv       *inventory.Service
	ctx       *gin.Context
	order     *Order
	item      *OrderItem
	sku       *product.ProductSKU
	main      *warehouse.Warehouse
	secondary *warehouse.Warehouse
}

func newWarehouseAllocationFixture(t *testing.T) *warehouseAllocationFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:order_warehouse_allocation_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&Order{}, &OrderItem{}, &product.Product{}, &product.ProductSKU{}, &warehouse.Warehouse{},
		&inventory.WarehouseStockBalance{}, &inventory.InventoryMovement{}, &inventory.InventoryChangeLog{},
		&inventory.OrderInventoryEffect{}, &idempotency.Record{},
	); err != nil {
		t.Fatal(err)
	}
	warehouseSvc := &warehouse.Service{DB: db}
	main, err := warehouseSvc.Create(context.Background(), 41, nil, warehouse.CreateInput{Code: "ALLOC-MAIN", Name: "Allocation main", IsDefault: true})
	if err != nil {
		t.Fatal(err)
	}
	secondary, err := warehouseSvc.Create(context.Background(), 41, nil, warehouse.CreateInput{Code: "ALLOC-SECOND", Name: "Allocation secondary"})
	if err != nil {
		t.Fatal(err)
	}
	productRow := &product.Product{TenantID: 41, Source: "manual", Status: product.StatusDraft, Title: "Allocation product"}
	if err := db.Create(productRow).Error; err != nil {
		t.Fatal(err)
	}
	stock := 12
	sku := &product.ProductSKU{ProductID: productRow.ID, SKUCode: "ALLOC-SKU", SKUName: "Allocation SKU", Stock: &stock}
	if err := db.Create(sku).Error; err != nil {
		t.Fatal(err)
	}
	for _, balance := range []inventory.WarehouseStockBalance{
		{TenantID: 41, WarehouseID: main.ID, ProductSKUID: sku.ID, OnHand: 10, Version: 1},
		{TenantID: 41, WarehouseID: secondary.ID, ProductSKUID: sku.ID, OnHand: 2, Version: 1},
	} {
		row := balance
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	orderRow := &Order{
		TenantID: 41, Platform: "manual", OrderNo: "ALLOC-ORDER-1", CustomerName: "Buyer",
		Status: StatusPaid, PaymentStatus: PaymentPaid, FulfillmentStatus: FulfillmentUnfulfilled, Currency: "CNY",
	}
	if err := db.Create(orderRow).Error; err != nil {
		t.Fatal(err)
	}
	item := &OrderItem{OrderID: orderRow.ID, ProductID: &productRow.ID, ProductSKUID: &sku.ID, ProductTitle: productRow.Title, Quantity: 3}
	if err := db.Create(item).Error; err != nil {
		t.Fatal(err)
	}
	idem := &idempotency.Service{DB: db}
	inv := &inventory.Service{DB: db, Warehouses: warehouseSvc, Idempotency: idem}
	orders := &Service{DB: db, Idempotency: idem, Warehouses: warehouseSvc}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/api/v1/orders/"+orderRow.ID.String()+"/warehouse-allocation", nil)
	ctx.Set(ctxkey.TenantID, int64(41))
	ctx.Set(ctxkey.AdminID, uuid.NewString())
	return &warehouseAllocationFixture{db: db, orders: orders, inv: inv, ctx: ctx, order: orderRow, item: item, sku: sku, main: main, secondary: secondary}
}

func TestWarehouseAllocationCandidatesAreSingleWarehouseAndFailClosed(t *testing.T) {
	fx := newWarehouseAllocationFixture(t)
	evaluations, err := fx.inv.EvaluateOrderWarehouseAllocations(t.Context(), 41, []uuid.UUID{fx.order.ID})
	if err != nil {
		t.Fatal(err)
	}
	evaluation := evaluations[fx.order.ID]
	if evaluation.Status != inventory.WarehouseAllocationAllocatable || evaluation.EligibleCandidateCount != 1 || evaluation.CandidateCount != 2 {
		t.Fatalf("unexpected allocation evaluation: %#v", evaluation)
	}
	if evaluation.RecommendedWarehouseID == nil || *evaluation.RecommendedWarehouseID != fx.main.ID {
		t.Fatalf("default sufficient warehouse should be recommended: %#v", evaluation.RecommendedWarehouseID)
	}
	if evaluation.Candidates[0].WarehouseID != fx.main.ID || !evaluation.Candidates[0].Eligible || evaluation.Candidates[1].Eligible {
		t.Fatalf("candidate ordering and eligibility must be deterministic: %#v", evaluation.Candidates)
	}

	if err := fx.db.Model(&product.ProductSKU{}).Where("id = ?", fx.sku.ID).Update("stock", 99).Error; err != nil {
		t.Fatal(err)
	}
	evaluations, err = fx.inv.EvaluateOrderWarehouseAllocations(t.Context(), 41, []uuid.UUID{fx.order.ID})
	if err != nil {
		t.Fatal(err)
	}
	blocked := evaluations[fx.order.ID]
	if blocked.Status != inventory.WarehouseAllocationBlocked || len(blocked.Candidates) != 0 || len(blocked.Blocks) == 0 || blocked.Blocks[0].Code != "INVENTORY_PROJECTION_MISMATCH" {
		t.Fatalf("projection mismatch must fail closed: %#v", blocked)
	}

	otherTenant, err := fx.inv.EvaluateOrderWarehouseAllocations(t.Context(), 42, []uuid.UUID{fx.order.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(otherTenant) != 0 {
		t.Fatalf("tenant scope must not expose another tenant order: %#v", otherTenant)
	}
}

func TestConfirmWarehouseAllocationReservesAtomicallyAndReplays(t *testing.T) {
	fx := newWarehouseAllocationFixture(t)
	evaluation, err := fx.orders.GetWarehouseAllocation(fx.ctx, fx.inv, fx.order.ID)
	if err != nil {
		t.Fatal(err)
	}
	candidate := evaluation.Candidates[0]
	input := ConfirmWarehouseAllocationInput{
		WarehouseID: candidate.WarehouseID, ExpectedRevision: candidate.Revision, IdempotencyKey: "allocation-confirm-1",
	}
	result, err := fx.orders.ConfirmWarehouseAllocation(fx.ctx, fx.inv, fx.order.ID, input, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Allocation.Status != inventory.WarehouseAllocationAllocated || result.Allocation.WarehouseID == nil || *result.Allocation.WarehouseID != fx.main.ID {
		t.Fatalf("unexpected confirmed allocation: %#v", result)
	}
	var stored Order
	if err := fx.db.First(&stored, "id = ?", fx.order.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.WarehouseID == nil || *stored.WarehouseID != fx.main.ID {
		t.Fatalf("order warehouse was not atomically bound: %#v", stored.WarehouseID)
	}
	var balance inventory.WarehouseStockBalance
	if err := fx.db.Where("tenant_id = ? AND warehouse_id = ? AND product_sku_id = ?", 41, fx.main.ID, fx.sku.ID).First(&balance).Error; err != nil {
		t.Fatal(err)
	}
	if balance.OnHand != 10 || balance.Reserved != 3 || balance.Version != 2 {
		t.Fatalf("unexpected reserved balance: %#v", balance)
	}
	var effects int64
	if err := fx.db.Model(&inventory.OrderInventoryEffect{}).Where("order_id = ? AND status = ?", fx.order.ID, inventory.InventoryEffectSuccess).Count(&effects).Error; err != nil {
		t.Fatal(err)
	}
	if effects != 1 {
		t.Fatalf("expected one immutable reserve effect, got %d", effects)
	}

	replay, err := fx.orders.ConfirmWarehouseAllocation(fx.ctx, fx.inv, fx.order.ID, input, nil)
	if err != nil || replay.Allocation.Status != inventory.WarehouseAllocationAllocated {
		t.Fatalf("same idempotency key must replay: result=%#v err=%v", replay, err)
	}
	if err := fx.db.First(&balance, "id = ?", balance.ID).Error; err != nil {
		t.Fatal(err)
	}
	if balance.Reserved != 3 || balance.Version != 2 {
		t.Fatalf("replay must not reserve twice: %#v", balance)
	}
}

func TestConfirmWarehouseAllocationRejectsStaleCandidateWithoutWrites(t *testing.T) {
	fx := newWarehouseAllocationFixture(t)
	evaluation, err := fx.orders.GetWarehouseAllocation(fx.ctx, fx.inv, fx.order.ID)
	if err != nil {
		t.Fatal(err)
	}
	candidate := evaluation.Candidates[0]
	if err := fx.db.Model(&inventory.WarehouseStockBalance{}).
		Where("tenant_id = ? AND warehouse_id = ? AND product_sku_id = ?", 41, fx.main.ID, fx.sku.ID).
		Updates(map[string]any{"reserved": 1, "version": 2}).Error; err != nil {
		t.Fatal(err)
	}
	_, err = fx.orders.ConfirmWarehouseAllocation(fx.ctx, fx.inv, fx.order.ID, ConfirmWarehouseAllocationInput{
		WarehouseID: candidate.WarehouseID, ExpectedRevision: candidate.Revision, IdempotencyKey: "allocation-stale-1",
	}, nil)
	if !errors.Is(err, inventory.ErrOrderAllocationRevisionConflict) {
		t.Fatalf("expected stale revision conflict, got %v", err)
	}
	var stored Order
	if err := fx.db.First(&stored, "id = ?", fx.order.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.WarehouseID != nil {
		t.Fatalf("stale confirmation must not bind a warehouse: %#v", stored.WarehouseID)
	}
	var effectCount int64
	if err := fx.db.Model(&inventory.OrderInventoryEffect{}).Where("order_id = ?", fx.order.ID).Count(&effectCount).Error; err != nil {
		t.Fatal(err)
	}
	if effectCount != 0 {
		t.Fatalf("stale confirmation must not write inventory effects: %d", effectCount)
	}
}
