package order

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

func createBatchTestOrder(t *testing.T, orderSvc *Service, productID, skuID uuid.UUID, warehouseID *uuid.UUID, orderNo string, quantity int) *Order {
	t.Helper()
	row := &Order{
		Base: model.Base{ID: uuid.New()}, TenantID: 41, Platform: "manual", WarehouseID: warehouseID,
		OrderNo: orderNo, CustomerName: "Batch buyer", Status: StatusPaid, PaymentStatus: PaymentPaid,
		FulfillmentStatus: FulfillmentUnfulfilled, Currency: "CNY", TotalAmount: 99,
	}
	if err := orderSvc.DB.Create(row).Error; err != nil {
		t.Fatal(err)
	}
	if err := orderSvc.DB.Create(&OrderItem{
		OrderID: row.ID, ProductID: &productID, ProductSKUID: &skuID,
		ProductTitle: "Batch product", SKUCode: "FULFILL-SKU", SKUName: "Batch SKU", Quantity: quantity,
	}).Error; err != nil {
		t.Fatal(err)
	}
	return row
}

func TestFulfillOrdersCommitsEachAllocatedOrderAndBuildsPickList(t *testing.T) {
	orderSvc, inv, ctx, first, sku, warehouseRow, _ := newFulfillmentFixture(t)
	second := createBatchTestOrder(t, orderSvc, sku.ProductID, sku.ID, &warehouseRow.ID, "FULFILL-ORDER-2", 3)
	input := BatchFulfillOrdersInput{
		BatchIdempotencyKey: "batch-fulfill-1",
		Items: []BatchFulfillOrderItem{
			{OrderID: first.ID, Carrier: "Carrier", TrackingNo: "TRACK-BATCH-1"},
			{OrderID: second.ID, Carrier: "Carrier", TrackingNo: "TRACK-BATCH-2"},
		},
	}
	result, err := orderSvc.FulfillOrders(ctx, inv, input, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary.Succeeded != 2 || result.Summary.Blocked != 0 || len(result.PickList) != 1 {
		t.Fatalf("unexpected batch result: %#v", result)
	}
	line := result.PickList[0]
	if line.ProductSKUID != sku.ID || line.Quantity != 6 || line.OrderCount != 2 || line.WarehouseID != warehouseRow.ID {
		t.Fatalf("unexpected pick list line: %#v", line)
	}
	var shipmentCount int64
	if err := orderSvc.DB.Model(&OrderShipment{}).Count(&shipmentCount).Error; err != nil {
		t.Fatal(err)
	}
	if shipmentCount != 2 {
		t.Fatalf("expected two shipments, got %d", shipmentCount)
	}

	replay, err := orderSvc.FulfillOrders(ctx, inv, input, nil)
	if err != nil || replay.Summary.Succeeded != 2 || len(replay.PickList) != 1 {
		t.Fatalf("same batch key must replay each item: result=%#v err=%v", replay, err)
	}
	if err := orderSvc.DB.Model(&OrderShipment{}).Count(&shipmentCount).Error; err != nil {
		t.Fatal(err)
	}
	if shipmentCount != 2 {
		t.Fatalf("batch replay must not create duplicate shipments, got %d", shipmentCount)
	}
}

func TestFulfillOrdersReportsUnallocatedItemWithoutWritingIt(t *testing.T) {
	orderSvc, inv, ctx, first, sku, warehouseRow, _ := newFulfillmentFixture(t)
	unallocated := createBatchTestOrder(t, orderSvc, sku.ProductID, sku.ID, nil, "FULFILL-ORDER-UNALLOCATED", 1)
	result, err := orderSvc.FulfillOrders(ctx, inv, BatchFulfillOrdersInput{
		BatchIdempotencyKey: "batch-fulfill-partial",
		Items: []BatchFulfillOrderItem{
			{OrderID: first.ID, WarehouseID: &warehouseRow.ID, Carrier: "Carrier", TrackingNo: "TRACK-BATCH-OK"},
			{OrderID: unallocated.ID, Carrier: "Carrier", TrackingNo: "TRACK-BATCH-BLOCKED"},
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary.Succeeded != 1 || result.Summary.Blocked != 1 {
		t.Fatalf("expected one success and one blocked item: %#v", result.Summary)
	}
	var stored Order
	if err := orderSvc.DB.First(&stored, "id = ?", unallocated.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.FulfillmentStatus != FulfillmentUnfulfilled || stored.Status != StatusPaid {
		t.Fatalf("blocked order must remain unchanged: %#v", stored)
	}
	if result.Items[1].Error != ErrBatchFulfillmentNotAllocated.Error() {
		t.Fatalf("unexpected blocked error: %#v", result.Items[1])
	}
}

func TestFulfillOrdersRejectsInvalidBatchPayload(t *testing.T) {
	orderSvc, inv, ctx, orderRow, _, _, _ := newFulfillmentFixture(t)
	_, err := orderSvc.FulfillOrders(ctx, inv, BatchFulfillOrdersInput{
		BatchIdempotencyKey: "batch-invalid",
		Items:               []BatchFulfillOrderItem{{OrderID: orderRow.ID}, {OrderID: orderRow.ID}},
	}, nil)
	if !errors.Is(err, ErrBatchFulfillmentInputInvalid) {
		t.Fatalf("expected duplicate order ids to be rejected, got %v", err)
	}
	_, err = orderSvc.FulfillOrders(ctx, inv, BatchFulfillOrdersInput{BatchIdempotencyKey: "batch-invalid"}, nil)
	if !errors.Is(err, ErrBatchFulfillmentInputInvalid) {
		t.Fatalf("expected empty items to be rejected, got %v", err)
	}
}
