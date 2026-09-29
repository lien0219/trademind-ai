package procurement

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
)

type receivedPurchaseFixture struct {
	*procurementFixture
	Order       *PurchaseOrder
	ReceiptItem GoodsReceiptItem
}

func newReceivedPurchaseFixture(t *testing.T, quantity int) *receivedPurchaseFixture {
	t.Helper()
	fx := newProcurementFixture(t)
	ctx := context.Background()
	po, err := fx.Service.Create(ctx, 1, uuidPtr(uuid.New()), CreatePurchaseOrderInput{
		IdempotencyKey: "purchase-return-source-order", SupplierID: fx.Supplier.ID, WarehouseID: fx.Warehouse.ID, Currency: "CNY",
		Items: []CreatePurchaseOrderItemInput{{ProductSKUID: fx.ProductSKU.ID, SupplierSKUID: &fx.SupplierSKU.ID, Quantity: quantity, UnitCostMinor: 2500}},
	})
	if err != nil {
		t.Fatalf("create source order: %v", err)
	}
	po, err = fx.Service.Submit(ctx, 1, po.ID, po.Revision)
	if err != nil {
		t.Fatalf("submit source order: %v", err)
	}
	po, err = fx.Service.Approve(ctx, 1, po.ID, po.Revision, uuidPtr(uuid.New()))
	if err != nil {
		t.Fatalf("approve source order: %v", err)
	}
	receipt, err := fx.Service.Receive(ctx, 1, po.ID, uuidPtr(uuid.New()), ReceivePurchaseOrderInput{
		ExpectedRevision: po.Revision, IdempotencyKey: "purchase-return-source-receipt",
		Items: []ReceivePurchaseOrderItemInput{{PurchaseOrderItemID: po.Items[0].ID, Quantity: quantity}},
	})
	if err != nil {
		t.Fatalf("receive source order: %v", err)
	}
	if len(receipt.Receipt.Items) != 1 {
		t.Fatalf("expected one receipt item, got %#v", receipt.Receipt.Items)
	}
	return &receivedPurchaseFixture{procurementFixture: fx, Order: receipt.PurchaseOrder, ReceiptItem: receipt.Receipt.Items[0]}
}

func TestPurchaseReturnLifecycleIsIdempotentAndTransactional(t *testing.T) {
	fx := newReceivedPurchaseFixture(t, 5)
	ctx := context.Background()
	creator, approver, executor := uuid.New(), uuid.New(), uuid.New()
	createInput := CreatePurchaseReturnInput{
		IdempotencyKey: "purchase-return-create-001", PurchaseOrderID: fx.Order.ID, Reason: "quality issue", Remark: "outer packaging damaged",
		Items: []CreatePurchaseReturnItemInput{{GoodsReceiptItemID: fx.ReceiptItem.ID, Quantity: 2}},
	}
	row, err := fx.Service.CreatePurchaseReturn(ctx, 1, &creator, createInput)
	if err != nil {
		t.Fatalf("create purchase return: %v", err)
	}
	if row.Status != ReturnStatusDraft || row.Revision != 1 || len(row.Items) != 1 || row.Items[0].ReceiptNo == "" || row.Items[0].ProductTitle != "Test product" {
		t.Fatalf("unexpected purchase return draft: %#v", row)
	}
	replayed, err := fx.Service.CreatePurchaseReturn(ctx, 1, &creator, createInput)
	if err != nil || replayed.ID != row.ID {
		t.Fatalf("create replay failed: row=%#v err=%v", replayed, err)
	}
	conflicting := createInput
	conflicting.Items = []CreatePurchaseReturnItemInput{{GoodsReceiptItemID: fx.ReceiptItem.ID, Quantity: 1}}
	if _, err := fx.Service.CreatePurchaseReturn(ctx, 1, &creator, conflicting); !errors.Is(err, ErrReturnIdempotencyConflict) {
		t.Fatalf("expected create idempotency conflict, got %v", err)
	}
	if _, err := fx.Service.CreatePurchaseReturn(ctx, 1, &creator, CreatePurchaseReturnInput{
		IdempotencyKey: "purchase-return-over-001", PurchaseOrderID: fx.Order.ID, Reason: "quality issue",
		Items: []CreatePurchaseReturnItemInput{{GoodsReceiptItemID: fx.ReceiptItem.ID, Quantity: 4}},
	}); !errors.Is(err, ErrOverReturn) {
		t.Fatalf("expected cumulative over-return rejection, got %v", err)
	}

	row, err = fx.Service.SubmitPurchaseReturn(ctx, 1, row.ID, &creator, PurchaseReturnActionInput{ExpectedRevision: 1, IdempotencyKey: "purchase-return-submit-001", Reason: "request review"})
	if err != nil || row.Status != ReturnStatusPendingApproval || row.Revision != 2 {
		t.Fatalf("submit purchase return: row=%#v err=%v", row, err)
	}
	replayed, err = fx.Service.SubmitPurchaseReturn(ctx, 1, row.ID, &creator, PurchaseReturnActionInput{ExpectedRevision: 1, IdempotencyKey: "purchase-return-submit-001", Reason: "request review"})
	if err != nil || replayed.Status != ReturnStatusPendingApproval || replayed.Revision != 2 {
		t.Fatalf("submit replay failed: row=%#v err=%v", replayed, err)
	}
	if _, err := fx.Service.SubmitPurchaseReturn(ctx, 1, row.ID, &creator, PurchaseReturnActionInput{ExpectedRevision: 1, IdempotencyKey: "purchase-return-submit-other", Reason: "request review"}); !errors.Is(err, ErrReturnIdempotencyConflict) {
		t.Fatalf("expected submit idempotency conflict, got %v", err)
	}

	row, err = fx.Service.ApprovePurchaseReturn(ctx, 1, row.ID, &approver, PurchaseReturnActionInput{ExpectedRevision: 2, IdempotencyKey: "purchase-return-approve-001", Reason: "approved"})
	if err != nil || row.Status != ReturnStatusApproved || row.Revision != 3 {
		t.Fatalf("approve purchase return: row=%#v err=%v", row, err)
	}
	if _, err := fx.Service.CompletePurchaseReturn(ctx, 1, row.ID, &approver, PurchaseReturnActionInput{ExpectedRevision: 3, IdempotencyKey: "purchase-return-complete-self", Reason: "ship to supplier"}); !errors.Is(err, ErrReturnDutyConflict) {
		t.Fatalf("approver must not execute return, got %v", err)
	}

	row, err = fx.Service.CompletePurchaseReturn(ctx, 1, row.ID, &executor, PurchaseReturnActionInput{ExpectedRevision: 3, IdempotencyKey: "purchase-return-complete-001", Reason: "ship to supplier"})
	if err != nil || row.Status != ReturnStatusCompleted || row.Revision != 4 || row.CompletedBy == nil || *row.CompletedBy != executor {
		t.Fatalf("complete purchase return: row=%#v err=%v", row, err)
	}
	replayed, err = fx.Service.CompletePurchaseReturn(ctx, 1, row.ID, &executor, PurchaseReturnActionInput{ExpectedRevision: 3, IdempotencyKey: "purchase-return-complete-001", Reason: "ship to supplier"})
	if err != nil || replayed.Status != ReturnStatusCompleted || replayed.Revision != 4 {
		t.Fatalf("complete replay failed: row=%#v err=%v", replayed, err)
	}
	if _, err := fx.Service.CancelPurchaseReturn(ctx, 1, row.ID, &creator, PurchaseReturnActionInput{ExpectedRevision: 4, IdempotencyKey: "purchase-return-cancel-completed"}); !errors.Is(err, ErrReturnInvalidTransition) {
		t.Fatalf("completed return must be immutable, got %v", err)
	}

	var balance inventory.WarehouseStockBalance
	if err := fx.DB.Where("tenant_id = ? AND warehouse_id = ? AND product_sku_id = ?", 1, fx.Warehouse.ID, fx.ProductSKU.ID).First(&balance).Error; err != nil {
		t.Fatal(err)
	}
	if balance.OnHand != 13 {
		t.Fatalf("expected warehouse on-hand 13 after return, got %d", balance.OnHand)
	}
	var sku product.ProductSKU
	if err := fx.DB.First(&sku, "id = ?", fx.ProductSKU.ID).Error; err != nil {
		t.Fatal(err)
	}
	if sku.Stock == nil || *sku.Stock != 13 {
		t.Fatalf("expected compatibility stock 13, got %#v", sku.Stock)
	}
	var movementCount, changeCount int64
	if err := fx.DB.Model(&inventory.InventoryMovement{}).Where("movement_type = ? AND source_id = ?", inventory.MovementPurchaseReturn, row.ID).Count(&movementCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := fx.DB.Model(&inventory.InventoryChangeLog{}).Where("change_type = ?", inventory.ChangePurchaseReturn).Count(&changeCount).Error; err != nil {
		t.Fatal(err)
	}
	if movementCount != 1 || changeCount != 1 {
		t.Fatalf("return facts must be written once, movements=%d changes=%d", movementCount, changeCount)
	}
	returnable, err := fx.Service.ListReturnableReceiptItems(ctx, 1, fx.Order.ID)
	if err != nil || len(returnable.List) != 1 || returnable.List[0].RemainingQuantity != 3 || returnable.List[0].AllocatedReturnQuantity != 2 {
		t.Fatalf("unexpected remaining returnable quantity: result=%#v err=%v", returnable, err)
	}
	if _, err := fx.Service.GetPurchaseReturn(ctx, 2, row.ID); !errors.Is(err, ErrPurchaseReturnAbsent) {
		t.Fatalf("cross-tenant return read must look absent, got %v", err)
	}
}

func TestPurchaseReturnInsufficientStockRollsBackCompletion(t *testing.T) {
	fx := newReceivedPurchaseFixture(t, 3)
	ctx := context.Background()
	creator, approver, executor := uuid.New(), uuid.New(), uuid.New()
	row, err := fx.Service.CreatePurchaseReturn(ctx, 1, &creator, CreatePurchaseReturnInput{
		IdempotencyKey: "purchase-return-stock-create", PurchaseOrderID: fx.Order.ID, Reason: "quality issue",
		Items: []CreatePurchaseReturnItemInput{{GoodsReceiptItemID: fx.ReceiptItem.ID, Quantity: 3}},
	})
	if err != nil {
		t.Fatal(err)
	}
	row, err = fx.Service.SubmitPurchaseReturn(ctx, 1, row.ID, &creator, PurchaseReturnActionInput{ExpectedRevision: 1, IdempotencyKey: "purchase-return-stock-submit"})
	if err != nil {
		t.Fatal(err)
	}
	row, err = fx.Service.ApprovePurchaseReturn(ctx, 1, row.ID, &approver, PurchaseReturnActionInput{ExpectedRevision: 2, IdempotencyKey: "purchase-return-stock-approve"})
	if err != nil {
		t.Fatal(err)
	}
	if err := fx.DB.Model(&inventory.WarehouseStockBalance{}).Where("tenant_id = ? AND warehouse_id = ? AND product_sku_id = ?", 1, fx.Warehouse.ID, fx.ProductSKU.ID).Update("reserved", 11).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := fx.Service.CompletePurchaseReturn(ctx, 1, row.ID, &executor, PurchaseReturnActionInput{ExpectedRevision: 3, IdempotencyKey: "purchase-return-stock-complete"}); !errors.Is(err, ErrReturnInsufficientStock) {
		t.Fatalf("expected insufficient stock, got %v", err)
	}
	reloaded, err := fx.Service.GetPurchaseReturn(ctx, 1, row.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Status != ReturnStatusApproved || reloaded.Revision != 3 || reloaded.CompletedAt != nil {
		t.Fatalf("failed completion must roll back return state: %#v", reloaded)
	}
	var actionCount, movementCount int64
	if err := fx.DB.Model(&PurchaseReturnAction{}).Where("purchase_return_id = ? AND action = ?", row.ID, "complete").Count(&actionCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := fx.DB.Model(&inventory.InventoryMovement{}).Where("source_type = ? AND source_id = ?", "purchase_return", row.ID).Count(&movementCount).Error; err != nil {
		t.Fatal(err)
	}
	if actionCount != 0 || movementCount != 0 {
		t.Fatalf("failed completion must not leave facts, actions=%d movements=%d", actionCount, movementCount)
	}
}

func TestPurchaseReturnCancellationReleasesReceiptAllocation(t *testing.T) {
	fx := newReceivedPurchaseFixture(t, 3)
	ctx := context.Background()
	creator := uuid.New()
	row, err := fx.Service.CreatePurchaseReturn(ctx, 1, &creator, CreatePurchaseReturnInput{
		IdempotencyKey: "purchase-return-cancel-create", PurchaseOrderID: fx.Order.ID, Reason: "quality issue",
		Items: []CreatePurchaseReturnItemInput{{GoodsReceiptItemID: fx.ReceiptItem.ID, Quantity: 3}},
	})
	if err != nil {
		t.Fatal(err)
	}
	row, err = fx.Service.CancelPurchaseReturn(ctx, 1, row.ID, &creator, PurchaseReturnActionInput{
		ExpectedRevision: row.Revision, IdempotencyKey: "purchase-return-cancel-action", Reason: "supplier waived return",
	})
	if err != nil || row.Status != ReturnStatusCancelled {
		t.Fatalf("cancel purchase return: row=%#v err=%v", row, err)
	}
	returnable, err := fx.Service.ListReturnableReceiptItems(ctx, 1, fx.Order.ID)
	if err != nil || len(returnable.List) != 1 || returnable.List[0].AllocatedReturnQuantity != 0 || returnable.List[0].RemainingQuantity != 3 {
		t.Fatalf("cancelled return must release receipt allocation: result=%#v err=%v", returnable, err)
	}
}

func uuidPtr(value uuid.UUID) *uuid.UUID { return &value }
