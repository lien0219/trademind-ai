package profitability

import (
	"bytes"
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type fakeRepository struct {
	orders    []OrderFact
	items     []OrderItemFact
	costs     []SupplierCostFact
	freight   []FreightFact
	refunds   []RefundFact
	listErr   error
	lastScope Scope
}

type fakePlatformFeeReader struct {
	facts map[uuid.UUID]PlatformFeeFact
}

func (r fakePlatformFeeReader) ListPlatformFees(context.Context, int64, []uuid.UUID) (map[uuid.UUID]PlatformFeeFact, error) {
	return r.facts, nil
}

func (r *fakeRepository) ListOrders(_ context.Context, _ int64, scope Scope, _ ListQuery, offset, limit int) ([]OrderFact, int64, error) {
	r.lastScope = scope
	if r.listErr != nil {
		return nil, 0, r.listErr
	}
	start := offset
	if start > len(r.orders) {
		start = len(r.orders)
	}
	end := start + limit
	if end > len(r.orders) {
		end = len(r.orders)
	}
	return append([]OrderFact(nil), r.orders[start:end]...), int64(len(r.orders)), nil
}

func (r *fakeRepository) GetOrder(_ context.Context, _ int64, scope Scope, orderID uuid.UUID) (*OrderFact, error) {
	r.lastScope = scope
	for _, row := range r.orders {
		if row.ID == orderID {
			copy := row
			return &copy, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *fakeRepository) ListOrderItems(context.Context, int64, []uuid.UUID) ([]OrderItemFact, error) {
	return append([]OrderItemFact(nil), r.items...), nil
}

func (r *fakeRepository) ListSupplierCosts(context.Context, int64, []uuid.UUID) ([]SupplierCostFact, error) {
	return append([]SupplierCostFact(nil), r.costs...), nil
}

func (r *fakeRepository) ListFreightFacts(context.Context, int64, []uuid.UUID) ([]FreightFact, error) {
	return append([]FreightFact(nil), r.freight...), nil
}

func (r *fakeRepository) ListRefundFacts(context.Context, int64, []uuid.UUID) ([]RefundFact, error) {
	return append([]RefundFact(nil), r.refunds...), nil
}

func TestCalculateOrderProfitKeepsMissingFeesOutOfEstimatedProfit(t *testing.T) {
	now := time.Date(2026, 9, 6, 2, 0, 0, 0, time.UTC)
	orderID, itemID, skuID, supplierID, waveID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	repo := &fakeRepository{
		orders:  []OrderFact{{ID: orderID, OrderNo: "TM-1001", Platform: "douyin_shop", Currency: "CNY", TotalAmountText: "12.34", Status: "confirmed", PaymentStatus: "paid", FulfillmentStatus: "fulfilled", CreatedAt: now, UpdatedAt: now}},
		items:   []OrderItemFact{{ID: itemID, OrderID: orderID, ProductSKUID: &skuID, ProductTitle: "商品", Quantity: 2}},
		costs:   []SupplierCostFact{{SupplierID: supplierID, ProductSKUID: skuID, SupplierName: "供应商", UnitCostMinor: 300, Currency: "CNY", UpdatedAt: now.Add(-time.Hour)}},
		freight: []FreightFact{{ID: uuid.New(), OrderID: orderID, WaveID: waveID, Version: 1, AmountMinor: 100, Currency: "CNY", CreatedAt: now.Add(-30 * time.Minute)}},
		refunds: []RefundFact{{SalesReturnID: uuid.New(), OrderID: orderID, SalesReturnAmount: 200, SalesReturnCurrency: "CNY", ExecutionID: uuidPointer(uuid.New()), ExecutionStatus: "succeeded", ExecutionAmount: 200, ExecutionCurrency: "CNY", ExecutionUpdatedAt: timePointer(now.Add(-time.Minute))}},
	}
	svc := &Service{Repo: repo, Clock: func() time.Time { return now }}

	result, err := svc.List(context.Background(), 8, Scope{}, ListQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(result.List) != 1 {
		t.Fatalf("List() rows = %d, want 1", len(result.List))
	}
	row := result.List[0]
	if row.Components.Revenue.AmountMinor == nil || *row.Components.Revenue.AmountMinor != 1234 {
		t.Fatalf("revenue = %#v, want 1234", row.Components.Revenue.AmountMinor)
	}
	if row.Components.ProductCost.AmountMinor == nil || *row.Components.ProductCost.AmountMinor != 600 {
		t.Fatalf("product cost = %#v, want 600", row.Components.ProductCost.AmountMinor)
	}
	if row.KnownContributionMinor == nil || *row.KnownContributionMinor != 334 {
		t.Fatalf("known contribution = %#v, want 334", row.KnownContributionMinor)
	}
	if row.Status != StatusPending || row.EstimatedProfitMinor != nil {
		t.Fatalf("status/profit = %s/%v, want pending/nil", row.Status, row.EstimatedProfitMinor)
	}
	for _, component := range []MoneyComponent{row.Components.PlatformFee, row.Components.Advertising, row.Components.WarehouseFee} {
		if component.Status != ComponentMissing || component.AmountMinor != nil {
			t.Fatalf("missing fee was treated as available: %#v", component)
		}
	}
}

func TestCalculateOrderProfitConsumesOnlyMatchedSettlementFee(t *testing.T) {
	now := time.Date(2026, 9, 7, 2, 0, 0, 0, time.UTC)
	orderID, reconciliationID, transactionID := uuid.New(), uuid.New(), uuid.New()
	repo := &fakeRepository{orders: []OrderFact{{ID: orderID, OrderNo: "TM-SETTLED", Currency: "CNY", TotalAmountText: "10.00", CreatedAt: now, UpdatedAt: now}}}
	svc := &Service{
		Repo: repo,
		PlatformFees: fakePlatformFeeReader{facts: map[uuid.UUID]PlatformFeeFact{
			orderID: {OrderID: orderID, ReconciliationID: reconciliationID, SettlementTransactionIDs: []uuid.UUID{transactionID}, AmountMinor: 100, Currency: "CNY", Status: "matched", SourceAt: now},
		}},
		Clock: func() time.Time { return now },
	}

	result, err := svc.List(context.Background(), 41, Scope{}, ListQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	row := result.List[0]
	if row.Components.PlatformFee.Status != ComponentAvailable || row.Components.PlatformFee.AmountMinor == nil || *row.Components.PlatformFee.AmountMinor != 100 {
		t.Fatalf("platform fee = %#v", row.Components.PlatformFee)
	}
	if row.KnownContributionMinor == nil || *row.KnownContributionMinor != 900 {
		t.Fatalf("known contribution = %#v, want 900", row.KnownContributionMinor)
	}
	if row.Related.SettlementReconciliationID == nil || *row.Related.SettlementReconciliationID != reconciliationID || len(row.Related.SettlementTransactionIDs) != 1 {
		t.Fatalf("related settlement facts = %#v", row.Related)
	}
	if row.FormulaVersion != "order_profit_estimate_v2" {
		t.Fatalf("formula version = %s", row.FormulaVersion)
	}
}

func TestCalculateOrderProfitRejectsMismatchedSettlementFee(t *testing.T) {
	now := time.Date(2026, 9, 7, 3, 0, 0, 0, time.UTC)
	orderID := uuid.New()
	repo := &fakeRepository{orders: []OrderFact{{ID: orderID, OrderNo: "TM-MISMATCH", Currency: "CNY", TotalAmountText: "10.00", CreatedAt: now, UpdatedAt: now}}}
	svc := &Service{
		Repo: repo,
		PlatformFees: fakePlatformFeeReader{facts: map[uuid.UUID]PlatformFeeFact{
			orderID: {OrderID: orderID, ReconciliationID: uuid.New(), Status: "mismatch", Currency: "CNY", ReasonCode: "order_amount_mismatch", Reason: "账单交易总额与本地订单金额不一致", SourceAt: now},
		}},
	}

	result, err := svc.List(context.Background(), 41, Scope{}, ListQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	row := result.List[0]
	if row.Status != StatusMismatch || row.Components.PlatformFee.Status != ComponentMismatch || row.Components.PlatformFee.AmountMinor != nil {
		t.Fatalf("mismatched platform fee was consumed: %#v", row)
	}
	if row.KnownContributionMinor == nil || *row.KnownContributionMinor != 1000 {
		t.Fatalf("known contribution = %#v, want revenue only", row.KnownContributionMinor)
	}
}

func TestCalculateOrderProfitFailsClosedForAmbiguousSupplierCostAndCurrency(t *testing.T) {
	now := time.Date(2026, 9, 6, 2, 0, 0, 0, time.UTC)
	orderID, itemID, skuID := uuid.New(), uuid.New(), uuid.New()
	repo := &fakeRepository{
		orders: []OrderFact{{ID: orderID, OrderNo: "TM-1002", Currency: "USD", TotalAmountText: "10.00", CreatedAt: now, UpdatedAt: now}},
		items:  []OrderItemFact{{ID: itemID, OrderID: orderID, ProductSKUID: &skuID, ProductTitle: "item", Quantity: 1}},
		costs: []SupplierCostFact{
			{SupplierID: uuid.New(), ProductSKUID: skuID, UnitCostMinor: 100, Currency: "USD", UpdatedAt: now},
			{SupplierID: uuid.New(), ProductSKUID: skuID, UnitCostMinor: 90, Currency: "USD", UpdatedAt: now},
		},
		freight: []FreightFact{{ID: uuid.New(), OrderID: orderID, WaveID: uuid.New(), Version: 1, AmountMinor: 20, Currency: "CNY", CreatedAt: now}},
	}
	svc := &Service{Repo: repo, Clock: func() time.Time { return now }}

	detail, err := svc.Get(context.Background(), 9, Scope{}, orderID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if detail.Status != StatusMismatch || detail.Components.ProductCost.AmountMinor != nil || detail.Components.Freight.AmountMinor != nil {
		t.Fatalf("mismatch did not fail closed: %#v", detail.OrderProfit)
	}
	if detail.KnownContributionMinor == nil || *detail.KnownContributionMinor != 1000 {
		t.Fatalf("known contribution = %#v, want revenue-only 1000", detail.KnownContributionMinor)
	}
}

func TestParseMajorToMinorRejectsLossyPrecisionAndOverflow(t *testing.T) {
	if value, err := parseMajorToMinor("123.45", "USD"); err != nil || value != 12345 {
		t.Fatalf("parse USD = %d, %v", value, err)
	}
	if value, err := parseMajorToMinor("123", "JPY"); err != nil || value != 123 {
		t.Fatalf("parse JPY = %d, %v", value, err)
	}
	if _, err := parseMajorToMinor("1.001", "USD"); err == nil {
		t.Fatal("expected precision error")
	}
	if _, err := parseMajorToMinor("999999999999999999999999", "USD"); err == nil {
		t.Fatal("expected overflow error")
	}
	if _, ok := subtractKnown(math.MinInt64, 1); ok {
		t.Fatal("expected subtraction overflow")
	}
}

func TestCalculateOrderRejectsRevenueOutsideJSONSafeIntegerRange(t *testing.T) {
	now := time.Now().UTC()
	profit, _ := calculateOrder(
		OrderFact{ID: uuid.New(), Currency: "USD", TotalAmountText: "90071992547409.92", UpdatedAt: now},
		nil,
		nil,
		nil,
		nil,
		nil,
		now,
	)
	if profit.Status != StatusBlocked || profit.Components.Revenue.Status != ComponentBlocked || profit.Components.Revenue.AmountMinor != nil {
		t.Fatalf("unsafe revenue did not fail closed: %#v", profit)
	}
}

func TestCalculateRefundsDoesNotDowngradeBlockedStatus(t *testing.T) {
	now := time.Now().UTC()
	executionID := uuid.New()
	component, _, issues := calculateRefunds("CNY", []RefundFact{
		{SalesReturnAmount: maxJSONSafeInteger + 1, SalesReturnCurrency: "CNY", ExecutionID: &executionID, ExecutionStatus: "succeeded", ExecutionAmount: maxJSONSafeInteger + 1, ExecutionCurrency: "CNY", ExecutionUpdatedAt: &now},
		{SalesReturnAmount: 100, SalesReturnCurrency: "USD"},
	})
	if component.Status != ComponentBlocked || component.ReasonCode != "refund_amount_invalid" || component.AmountMinor != nil {
		t.Fatalf("refund component = %#v, want blocked unsafe amount", component)
	}
	if len(issues) != 1 || issues[0].Code != "refund_amount_invalid" {
		t.Fatalf("refund issues = %#v", issues)
	}
}

func TestCalculateMarginBpsRejectsUnsafeJSONInteger(t *testing.T) {
	if margin, ok := calculateMarginBps(2500, 10000); !ok || margin != 2500 {
		t.Fatalf("margin = %d, %v; want 2500, true", margin, ok)
	}
	if _, ok := calculateMarginBps(1_000_000_000_000, 1); ok {
		t.Fatal("expected unsafe JavaScript margin to be rejected")
	}
}

func TestListFiltersDerivedStatusAndRejectsUnboundedScan(t *testing.T) {
	now := time.Now().UTC()
	pending := OrderFact{ID: uuid.New(), OrderNo: "pending", Currency: "CNY", TotalAmountText: "1.00", CreatedAt: now, UpdatedAt: now}
	blocked := OrderFact{ID: uuid.New(), OrderNo: "blocked", Currency: "CNY", TotalAmountText: "1.001", CreatedAt: now, UpdatedAt: now}
	repo := &fakeRepository{orders: []OrderFact{pending, blocked}}
	svc := &Service{Repo: repo, Clock: func() time.Time { return now }}

	result, err := svc.List(context.Background(), 1, Scope{}, ListQuery{Page: 1, PageSize: 20, Status: StatusBlocked})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if result.Total != 1 || len(result.List) != 1 || result.List[0].OrderNo != "blocked" {
		t.Fatalf("filtered result = %#v", result)
	}

	many := make([]OrderFact, maxExportRows+1)
	for i := range many {
		many[i] = OrderFact{ID: uuid.New(), Currency: "CNY", TotalAmountText: "1.00", CreatedAt: now, UpdatedAt: now}
	}
	repo.orders = many
	_, err = svc.List(context.Background(), 1, Scope{}, ListQuery{Page: 1, PageSize: 20, Export: true})
	if !errors.Is(err, ErrTooManyRows) {
		t.Fatalf("export error = %v, want ErrTooManyRows", err)
	}
}

func TestWriteCSVNeutralizesSpreadsheetFormulas(t *testing.T) {
	now := time.Now().UTC()
	value := int64(100)
	var output bytes.Buffer
	err := WriteCSV(&output, []OrderProfit{{OrderNo: "=1+1", Platform: "+cmd", ShopName: "@shop", Currency: "CNY", Status: StatusPending, KnownContributionMinor: &value, CalculatedAt: now, FormulaVersion: FormulaVersion}})
	if err != nil {
		t.Fatalf("WriteCSV() error = %v", err)
	}
	text := output.String()
	for _, want := range []string{"'=1+1", "'+cmd", "'@shop"} {
		if !strings.Contains(text, want) {
			t.Fatalf("CSV missing neutralized value %q: %s", want, text)
		}
	}
}

func uuidPointer(value uuid.UUID) *uuid.UUID { return &value }

func timePointer(value time.Time) *time.Time { return &value }
