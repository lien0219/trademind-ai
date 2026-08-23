package order

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

func reconciliationFixtureOrder(status, paymentStatus, fulfillmentStatus string) (Order, reconciliationItemAgg, reconciliationBuildData) {
	orderID, itemID, skuID, warehouseID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	o := Order{Base: model.Base{ID: orderID}, TenantID: 7, OrderNo: "RECON-" + orderID.String(), Status: status, PaymentStatus: paymentStatus, FulfillmentStatus: fulfillmentStatus, WarehouseID: &warehouseID}
	item := reconciliationItemAgg{OrderID: orderID, ID: itemID, ProductSKUID: &skuID, Quantity: 3}
	d := reconciliationBuildData{
		Items:       map[uuid.UUID][]reconciliationItemAgg{orderID: {item}},
		Effects:     map[uuid.UUID][]reconciliationEffectAgg{},
		Movements:   map[uuid.UUID][]reconciliationMovementAgg{},
		Shipments:   map[uuid.UUID][]OrderShipment{},
		LegacyFacts: map[uuid.UUID]bool{},
		Balances:    map[string]bool{fmtBalanceKey(warehouseID, skuID): true},
		Warehouses:  map[uuid.UUID]bool{warehouseID: true},
		SKUs:        map[uuid.UUID]bool{skuID: true},
	}
	return o, item, d
}

func fmtBalanceKey(warehouseID, skuID uuid.UUID) string {
	return warehouseID.String() + ":" + skuID.String()
}

func reconciliationEffect(item reconciliationItemAgg, effectType, status string, quantity int) reconciliationEffectAgg {
	return reconciliationEffectAgg{ID: uuid.New(), OrderID: item.OrderID, OrderItemID: item.ID, ProductSKUID: *item.ProductSKUID, EffectType: effectType, Status: status, Quantity: quantity, CreatedAt: time.Unix(100, 0).UTC()}
}

func reconciliationMovement(item reconciliationItemAgg, movementType string, quantity int) reconciliationMovementAgg {
	return reconciliationMovementAgg{ID: uuid.New(), SourceID: item.ID, MovementType: movementType, Quantity: quantity, CreatedAt: time.Unix(101, 0).UTC()}
}

func TestBuildOneReconciliationClassifiesExpectedStates(t *testing.T) {
	t.Run("matched", func(t *testing.T) {
		o, item, d := reconciliationFixtureOrder(StatusPaid, PaymentPaid, FulfillmentUnfulfilled)
		d.Effects[o.ID] = []reconciliationEffectAgg{reconciliationEffect(item, inventory.EffectTypeReserve, inventory.InventoryEffectSuccess, 3)}
		d.Movements[item.ID] = []reconciliationMovementAgg{reconciliationMovement(item, inventory.MovementOrderReserve, 3)}
		row := buildOneReconciliation(o, d, false)
		if row.ReconciliationStatus != ReconciliationMatched || row.Reserve.Expected != 3 || row.Reserve.Actual != 3 {
			t.Fatalf("unexpected matched reconciliation: %#v", row)
		}
	})

	t.Run("pending", func(t *testing.T) {
		o, _, d := reconciliationFixtureOrder(StatusPending, PaymentUnpaid, FulfillmentUnfulfilled)
		row := buildOneReconciliation(o, d, false)
		if row.ReconciliationStatus != ReconciliationPending || len(row.Issues) != 0 {
			t.Fatalf("unexpected pending reconciliation: %#v", row)
		}
	})

	t.Run("shipped lifecycle keeps the completed reservation fact", func(t *testing.T) {
		o, item, d := reconciliationFixtureOrder(StatusShipped, PaymentPaid, FulfillmentFulfilled)
		d.Shipments[o.ID] = []OrderShipment{{HardDeleteBase: model.HardDeleteBase{ID: uuid.New()}, OrderID: o.ID, Status: ShipmentShipped}}
		d.Effects[o.ID] = []reconciliationEffectAgg{
			reconciliationEffect(item, inventory.EffectTypeReserve, inventory.InventoryEffectSuccess, 3),
			reconciliationEffect(item, inventory.EffectTypeDeduct, inventory.InventoryEffectSuccess, 3),
		}
		d.Movements[item.ID] = []reconciliationMovementAgg{
			reconciliationMovement(item, inventory.MovementOrderReserve, 3),
			reconciliationMovement(item, inventory.MovementOrderDeduct, -3),
		}
		row := buildOneReconciliation(o, d, false)
		if row.ReconciliationStatus != ReconciliationMatched || row.Reserve.Expected != 3 || row.Deduct.Expected != 3 {
			t.Fatalf("shipped lifecycle should match reservation and deduction: %#v", row)
		}
	})

	t.Run("mismatch partial and duplicate", func(t *testing.T) {
		o, item, d := reconciliationFixtureOrder(StatusShipped, PaymentPaid, FulfillmentFulfilled)
		d.Shipments[o.ID] = []OrderShipment{{HardDeleteBase: model.HardDeleteBase{ID: uuid.New()}, OrderID: o.ID, Status: ShipmentShipped}}
		d.Effects[o.ID] = []reconciliationEffectAgg{
			reconciliationEffect(item, inventory.EffectTypeDeduct, inventory.InventoryEffectSuccess, 2),
			reconciliationEffect(item, inventory.EffectTypeDeduct, inventory.InventoryEffectSuccess, 2),
		}
		row := buildOneReconciliation(o, d, false)
		if row.ReconciliationStatus != ReconciliationMismatch || !containsIssue(row.Issues, "duplicate_effect") || !containsIssue(row.Issues, "deduct_quantity_mismatch") {
			t.Fatalf("unexpected mismatch reconciliation: %#v", row)
		}
	})

	t.Run("blocked", func(t *testing.T) {
		o, _, d := reconciliationFixtureOrder(StatusPaid, PaymentPaid, FulfillmentUnfulfilled)
		d.Items[o.ID][0].ProductSKUID = nil
		row := buildOneReconciliation(o, d, false)
		if row.ReconciliationStatus != ReconciliationBlocked || !containsIssue(row.Issues, "sku_not_bound") {
			t.Fatalf("unexpected blocked reconciliation: %#v", row)
		}
	})
}

func TestBuildOneReconciliationRejectsUnexpectedDeductionWhilePaid(t *testing.T) {
	o, item, d := reconciliationFixtureOrder(StatusPaid, PaymentPaid, FulfillmentUnfulfilled)
	d.Effects[o.ID] = []reconciliationEffectAgg{reconciliationEffect(item, inventory.EffectTypeDeduct, inventory.InventoryEffectSuccess, 3)}
	d.Movements[item.ID] = []reconciliationMovementAgg{reconciliationMovement(item, inventory.MovementOrderDeduct, -3)}
	row := buildOneReconciliation(o, d, false)
	if row.ReconciliationStatus != ReconciliationMismatch || row.Deduct.Expected != 0 || row.Deduct.Actual != 3 || !containsIssue(row.Issues, "deduct_quantity_mismatch") {
		t.Fatalf("paid order with deduction must be mismatch: %#v", row)
	}
}

func TestBuildOneReconciliationDetectsMovementQuantityMismatch(t *testing.T) {
	o, item, d := reconciliationFixtureOrder(StatusPaid, PaymentPaid, FulfillmentUnfulfilled)
	d.Effects[o.ID] = []reconciliationEffectAgg{reconciliationEffect(item, inventory.EffectTypeReserve, inventory.InventoryEffectSuccess, 3)}
	d.Movements[item.ID] = []reconciliationMovementAgg{reconciliationMovement(item, inventory.MovementOrderReserve, 2)}
	row := buildOneReconciliation(o, d, false)
	if row.ReconciliationStatus != ReconciliationMismatch || !containsIssue(row.Issues, "inventory_movement_quantity_mismatch") {
		t.Fatalf("effect and movement quantities must mismatch: %#v", row)
	}
}

func TestBuildOneReconciliationBlocksLegacyTenantZeroFacts(t *testing.T) {
	o, item, d := reconciliationFixtureOrder(StatusPaid, PaymentPaid, FulfillmentUnfulfilled)
	d.Effects[o.ID] = []reconciliationEffectAgg{reconciliationEffect(item, inventory.EffectTypeReserve, inventory.InventoryEffectSuccess, 3)}
	d.Movements[item.ID] = []reconciliationMovementAgg{reconciliationMovement(item, inventory.MovementOrderReserve, 3)}
	d.LegacyFacts[o.ID] = true
	row := buildOneReconciliation(o, d, false)
	if row.ReconciliationStatus != ReconciliationBlocked || !containsIssue(row.Issues, "inventory_history_not_migrated") {
		t.Fatalf("tenant zero history must be blocked: %#v", row)
	}
}

func TestBuildOneReconciliationRequiresReleaseAndRestoreFacts(t *testing.T) {
	t.Run("cancelled reservation must release", func(t *testing.T) {
		o, item, d := reconciliationFixtureOrder(StatusCancelled, PaymentPaid, FulfillmentUnfulfilled)
		d.Effects[o.ID] = []reconciliationEffectAgg{reconciliationEffect(item, inventory.EffectTypeReserve, inventory.InventoryEffectSuccess, 3)}
		row := buildOneReconciliation(o, d, false)
		if row.ReconciliationStatus != ReconciliationMismatch || row.Release.Expected != 3 || row.Release.Actual != 0 {
			t.Fatalf("cancelled reservation should require release: %#v", row)
		}
	})

	t.Run("refunded deduction must restore", func(t *testing.T) {
		o, item, d := reconciliationFixtureOrder(StatusRefunded, PaymentRefunded, FulfillmentFulfilled)
		d.Effects[o.ID] = []reconciliationEffectAgg{reconciliationEffect(item, inventory.EffectTypeDeduct, inventory.InventoryEffectSuccess, 3)}
		d.Shipments[o.ID] = []OrderShipment{{HardDeleteBase: model.HardDeleteBase{ID: uuid.New()}, OrderID: o.ID, Status: ShipmentDelivered}}
		row := buildOneReconciliation(o, d, false)
		if row.ReconciliationStatus != ReconciliationMismatch || row.Restore.Expected != 3 || row.Restore.Actual != 0 {
			t.Fatalf("refunded deduction should require restore: %#v", row)
		}
	})

	t.Run("refunded shipped order matches after restore", func(t *testing.T) {
		o, item, d := reconciliationFixtureOrder(StatusRefunded, PaymentRefunded, FulfillmentFulfilled)
		d.Shipments[o.ID] = []OrderShipment{{HardDeleteBase: model.HardDeleteBase{ID: uuid.New()}, OrderID: o.ID, Status: ShipmentDelivered}}
		d.Effects[o.ID] = []reconciliationEffectAgg{
			reconciliationEffect(item, inventory.EffectTypeReserve, inventory.InventoryEffectSuccess, 3),
			reconciliationEffect(item, inventory.EffectTypeDeduct, inventory.InventoryEffectSuccess, 3),
			reconciliationEffect(item, inventory.EffectTypeRestore, inventory.InventoryEffectSuccess, 3),
		}
		d.Movements[item.ID] = []reconciliationMovementAgg{
			reconciliationMovement(item, inventory.MovementOrderReserve, 3),
			reconciliationMovement(item, inventory.MovementOrderDeduct, -3),
			reconciliationMovement(item, inventory.MovementOrderRestore, 3),
		}
		row := buildOneReconciliation(o, d, false)
		if row.ReconciliationStatus != ReconciliationMatched || row.Deduct.Expected != 3 || row.Restore.Expected != 3 {
			t.Fatalf("refunded shipped order should match after restore: %#v", row)
		}
	})
}

func containsIssue(issues []string, want string) bool {
	for _, issue := range issues {
		if issue == want {
			return true
		}
	}
	return false
}
