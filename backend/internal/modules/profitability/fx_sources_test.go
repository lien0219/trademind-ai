package profitability

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func testFXOrder(at time.Time) OrderProfit {
	return OrderProfit{
		OrderID: uuid.New(), Currency: "USD", OrderedAt: &at,
		Status: StatusMismatch,
	}
}

func testFXRates(at time.Time) map[string][]FXRate {
	return map[string][]FXRate{
		"EUR": {{
			ID: uuid.New(), FromCurrency: "EUR", ToCurrency: "CNY",
			Rate: "8", EffectiveAt: at.Add(-time.Hour),
		}},
	}
}

func TestFXSourceCostRequiresMatchingHistoricalSnapshot(t *testing.T) {
	at := time.Date(2026, time.November, 3, 12, 0, 0, 0, time.UTC)
	order := testFXOrder(at)
	order.FulfillmentStatus = "fulfilled"
	skuID, itemID, snapshotID := uuid.New(), uuid.New(), uuid.New()
	item := OrderItemFact{ID: itemID, OrderID: order.OrderID, ProductSKUID: &skuID, Quantity: 2}
	snapshot := OrderItemCostSnapshotFact{
		ID: snapshotID, OrderID: order.OrderID, OrderItemID: itemID,
		ProductSKUID: skuID, Quantity: 2, OrderCurrency: "USD",
		UnitCostMinor: int64Pointer(100), LineCostMinor: int64Pointer(200),
		CostCurrency: "EUR", ResolutionStatus: "currency_mismatch",
		ReasonCode: "supplier_cost_currency_mismatch", CapturedAt: at,
	}
	facts := &fxSourceFacts{
		items: map[uuid.UUID][]OrderItemFact{order.OrderID: {item}},
		snapshots: map[uuid.UUID][]OrderItemCostSnapshotFact{itemID: {snapshot}},
	}
	got, proofs, missing, err := fxSourceCost(order, "CNY", facts, testFXRates(at))
	if err != nil || missing || got == nil || *got != 1600 {
		t.Fatalf("verified snapshot cost = %v, missing %v, error %v", got, missing, err)
	}
	if len(proofs) != 1 || proofs[0].SourceType != "fulfillment_cost_snapshot" ||
		proofs[0].SourceFactID != snapshotID.String() || proofs[0].RateID == nil {
		t.Fatalf("missing immutable snapshot/rate evidence: %#v", proofs)
	}
	snapshot.Quantity = 3
	facts.snapshots[itemID] = []OrderItemCostSnapshotFact{snapshot}
	got, _, _, err = fxSourceCost(order, "CNY", facts, testFXRates(at))
	if err != nil || got != nil {
		t.Fatalf("quantity mismatched snapshot was accepted: %v %v", got, err)
	}
	snapshot.Quantity = 2
	facts.snapshots[itemID] = []OrderItemCostSnapshotFact{snapshot}
	got, _, missing, err = fxSourceCost(order, "CNY", facts, nil)
	if err != nil || !missing || got != nil {
		t.Fatalf("missing snapshot FX was not rejected: %v %v %v", got, missing, err)
	}
}

func TestFXSourceCostRejectsLateOrAmbiguousSupplierCatalog(t *testing.T) {
	at := time.Date(2026, time.November, 3, 12, 0, 0, 0, time.UTC)
	order := testFXOrder(at)
	skuID, itemID := uuid.New(), uuid.New()
	item := OrderItemFact{ID: itemID, OrderID: order.OrderID, ProductSKUID: &skuID, Quantity: 2}
	price := SupplierCostFact{
		SupplierSKUID: uuid.New(), ProductSKUID: skuID, UnitCostMinor: 100,
		Currency: "EUR", UpdatedAt: at.Add(-time.Hour),
	}
	facts := &fxSourceFacts{
		items: map[uuid.UUID][]OrderItemFact{order.OrderID: {item}},
		suppliers: map[uuid.UUID][]SupplierCostFact{skuID: {price}},
	}
	value, proofs, missing, err := fxSourceCost(order, "CNY", facts, testFXRates(at))
	if err != nil || missing || value == nil || *value != 1600 ||
		len(proofs) != 1 || proofs[0].SourceType != "supplier_catalog_estimate" {
		t.Fatalf("valid unique historical supplier quote rejected: %v %v %v", value, missing, err)
	}
	price.UpdatedAt = at.Add(time.Minute)
	facts.suppliers[skuID] = []SupplierCostFact{price}
	value, _, _, err = fxSourceCost(order, "CNY", facts, testFXRates(at))
	if err != nil || value != nil {
		t.Fatalf("late catalog quote was accepted: %v %v", value, err)
	}
	price.UpdatedAt = at.Add(-time.Hour)
	facts.suppliers[skuID] = []SupplierCostFact{price, price}
	value, _, _, err = fxSourceCost(order, "CNY", facts, testFXRates(at))
	if err != nil || value != nil {
		t.Fatalf("ambiguous supplier quote was accepted: %v %v", value, err)
	}
}

func TestFXSourceFreightHonorsCarrierCoverageAndLocalQuoteUniqueness(t *testing.T) {
	at := time.Date(2026, time.November, 3, 12, 0, 0, 0, time.UTC)
	order := testFXOrder(at)
	charge := uuid.New()
	carrier := FreightFeeFact{
		OrderID: order.OrderID, ChargeIDs: []uuid.UUID{charge},
		ShipmentCount: 1, BilledShipmentCount: 1, AmountMinor: 300,
		Currency: "EUR", Status: "confirmed",
	}
	facts := &fxSourceFacts{carrier: map[uuid.UUID]FreightFeeFact{order.OrderID: carrier}}
	amount, proof, missing, err := fxSourceFreight(order, "CNY", facts, testFXRates(at))
	if err != nil || missing || amount == nil || *amount != 2400 ||
		proof == nil || proof.SourceType != "carrier_freight_ledger" {
		t.Fatalf("verified carrier freight rejected: %v %v %v", amount, missing, err)
	}
	carrier.BilledShipmentCount = 0
	facts.carrier[order.OrderID] = carrier
	amount, _, _, err = fxSourceFreight(order, "CNY", facts, testFXRates(at))
	if err != nil || amount != nil {
		t.Fatalf("incomplete carrier billing was accepted: %v %v", amount, err)
	}
	delete(facts.carrier, order.OrderID)
	facts.freight = map[uuid.UUID][]FreightFact{order.OrderID: {{
		ID: uuid.New(), OrderID: order.OrderID, WaveID: uuid.New(),
		Version: 1, Currency: "EUR", AmountMinor: 200, CreatedAt: at,
	}}}
	amount, proof, missing, err = fxSourceFreight(order, "CNY", facts, testFXRates(at))
	if err != nil || missing || amount == nil || *amount != 1600 ||
		proof == nil || proof.SourceType != "confirmed_local_freight_quote" {
		t.Fatalf("valid local quote rejected: %v %v %v", amount, missing, err)
	}
	facts.freight[order.OrderID] = append(facts.freight[order.OrderID], FreightFact{
		ID: uuid.New(), OrderID: order.OrderID, WaveID: uuid.New(),
		Version: 1, Currency: "EUR", AmountMinor: 100, CreatedAt: at,
	})
	amount, _, _, err = fxSourceFreight(order, "CNY", facts, testFXRates(at))
	if err != nil || amount != nil {
		t.Fatalf("multiple waves produced a complete freight amount: %v %v", amount, err)
	}
}

func TestFXSourceRefundRejectsDuplicateOrUnsettledExecutions(t *testing.T) {
	at := time.Date(2026, time.November, 3, 12, 0, 0, 0, time.UTC)
	order := testFXOrder(at)
	execID := uuid.New()
	refund := RefundFact{
		OrderID: order.OrderID, SalesReturnID: uuid.New(),
		SalesReturnAmount: 100, SalesReturnCurrency: "EUR",
		ExecutionID: &execID, ExecutionStatus: "succeeded",
		ExecutionAmount: 100, ExecutionCurrency: "EUR",
		ExecutionUpdatedAt: &at,
	}
	facts := &fxSourceFacts{refunds: map[uuid.UUID][]RefundFact{order.OrderID: {refund}}}
	value, proofs, missing, err := fxSourceRefunds(order, "CNY", facts, testFXRates(at))
	if err != nil || missing || value == nil || *value != 800 ||
		len(proofs) != 1 || proofs[0].SourceFactID != execID.String() {
		t.Fatalf("valid refund execution rejected: %v %v %v", value, missing, err)
	}
	refund.ExecutionStatus = "pending"
	facts.refunds[order.OrderID] = []RefundFact{refund}
	value, _, _, err = fxSourceRefunds(order, "CNY", facts, testFXRates(at))
	if err != nil || value != nil {
		t.Fatalf("pending refund counted: %v %v", value, err)
	}
	refund.ExecutionStatus = "succeeded"
	refund.ExecutionCurrency = "USD"
	facts.refunds[order.OrderID] = []RefundFact{refund}
	value, _, _, err = fxSourceRefunds(order, "CNY", facts, testFXRates(at))
	if err != nil || value != nil {
		t.Fatalf("mismatched refund currencies counted: %v %v", value, err)
	}
	refund.ExecutionCurrency = "EUR"
	facts.refunds[order.OrderID] = []RefundFact{refund, refund}
	value, _, _, err = fxSourceRefunds(order, "CNY", facts, testFXRates(at))
	if err != nil || value != nil {
		t.Fatalf("duplicate refunds counted: %v %v", value, err)
	}
	facts.refunds[order.OrderID] = []RefundFact{refund}
	value, _, missing, err = fxSourceRefunds(order, "CNY", facts, nil)
	if err != nil || !missing || value != nil {
		t.Fatalf("missing refund rate accepted: %v %v %v", value, missing, err)
	}
}
