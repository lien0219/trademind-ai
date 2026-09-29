package advertisingfee

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	ordermodule "github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"gorm.io/gorm"
)

func newAdvertisingFeeService(t *testing.T) (*Service, shop.Shop, []ordermodule.Order) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:advertising-fee-%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&shop.Shop{}, &ordermodule.Order{}, &Import{}, &Spend{}, &Allocation{}, &Adjustment{}); err != nil {
		t.Fatal(err)
	}
	shopRow := shop.Shop{TenantID: 7, Platform: "manual", ShopName: "Advertising test shop", Status: "active", AuthStatus: "mock", Currency: "CNY", Timezone: "UTC"}
	if err := db.Create(&shopRow).Error; err != nil {
		t.Fatal(err)
	}
	day := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	paidA, paidB, paidCancelled := day.Add(time.Hour), day.Add(2*time.Hour), day.Add(3*time.Hour)
	orderedUnpaid := day.Add(4 * time.Hour)
	orders := []ordermodule.Order{
		{TenantID: 7, Platform: "manual", ShopID: &shopRow.ID, OrderNo: "AD-ORDER-A", CustomerName: "A", Status: ordermodule.StatusPaid, PaymentStatus: ordermodule.PaymentPaid, FulfillmentStatus: ordermodule.FulfillmentUnfulfilled, Currency: "CNY", TotalAmount: 100, PaidAt: &paidA, OrderedAt: &paidA},
		{TenantID: 7, Platform: "manual", ShopID: &shopRow.ID, OrderNo: "AD-ORDER-B", CustomerName: "B", Status: ordermodule.StatusProcessing, PaymentStatus: ordermodule.PaymentPartiallyRefunded, FulfillmentStatus: ordermodule.FulfillmentUnfulfilled, Currency: "CNY", TotalAmount: 200, PaidAt: &paidB, OrderedAt: &paidB},
		{TenantID: 7, Platform: "manual", ShopID: &shopRow.ID, OrderNo: "AD-ORDER-C", CustomerName: "C", Status: ordermodule.StatusCancelled, PaymentStatus: ordermodule.PaymentPaid, FulfillmentStatus: ordermodule.FulfillmentUnfulfilled, Currency: "CNY", TotalAmount: 300, PaidAt: &paidCancelled, OrderedAt: &paidCancelled},
		{TenantID: 7, Platform: "manual", ShopID: &shopRow.ID, OrderNo: "AD-ORDER-D", CustomerName: "D", Status: ordermodule.StatusPending, PaymentStatus: ordermodule.PaymentUnpaid, FulfillmentStatus: ordermodule.FulfillmentUnfulfilled, Currency: "CNY", TotalAmount: 400, OrderedAt: &orderedUnpaid},
	}
	if err := db.Create(&orders).Error; err != nil {
		t.Fatal(err)
	}
	now := day.Add(12 * time.Hour)
	return &Service{DB: db, Clock: func() time.Time { return now }}, shopRow, orders
}

func advertisingCSV(coverage string, amount int64) []byte {
	return []byte(fmt.Sprintf("spend_date,currency,spend_minor,settlement_coverage\n2026-09-08,CNY,%d,%s\n", amount, coverage))
}

func TestPreviewConfirmAndProfitabilityConserveSpend(t *testing.T) {
	svc, shopRow, orders := newAdvertisingFeeService(t)
	data := advertisingCSV(CoverageExcluded, 101)
	preview, err := svc.Preview(context.Background(), 7, Scope{}, shopRow.ID, "advertising.csv", data)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Valid || preview.NewSpends != 1 || preview.AllocationCount != 2 || len(preview.Rows) != 1 || preview.Rows[0].EligibleOrderCount != 2 || preview.Rows[0].ExcludedOrderCount != 2 {
		t.Fatalf("unexpected preview: %#v", preview)
	}
	amounts := make(map[uuid.UUID]int64)
	for _, row := range preview.Orders {
		if row.Included && row.AmountMinor != nil {
			amounts[row.OrderID] = *row.AmountMinor
		}
	}
	if amounts[orders[0].ID] != 51 || amounts[orders[1].ID] != 50 {
		t.Fatalf("deterministic remainder allocation = %#v", amounts)
	}
	for _, target := range []any{&Import{}, &Spend{}, &Allocation{}} {
		var count int64
		if err := svc.DB.Model(target).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("preview wrote %T: count=%d err=%v", target, count, err)
		}
	}
	input := func() (*ImportResult, error) {
		return svc.Confirm(context.Background(), 7, Scope{}, shopRow.ID, "advertising.csv", data, preview.FileHash, preview.CalculationHash, "advertising-confirm-1", nil)
	}
	confirmed, err := input()
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.Replayed || confirmed.Import.ImportedSpends != 1 || confirmed.Import.AllocationCount != 2 {
		t.Fatalf("unexpected confirmation: %#v", confirmed)
	}
	replayed, err := input()
	if err != nil || !replayed.Replayed || replayed.Import.ID != confirmed.Import.ID {
		t.Fatalf("unexpected replay: %#v err=%v", replayed, err)
	}
	facts, err := svc.ProfitabilityFeesForOrders(context.Background(), 7, []uuid.UUID{orders[0].ID, orders[1].ID})
	if err != nil {
		t.Fatal(err)
	}
	if facts[orders[0].ID].Status != "confirmed" || facts[orders[0].ID].AmountMinor != 51 || facts[orders[1].ID].AmountMinor != 50 {
		t.Fatalf("unexpected profitability facts: %#v", facts)
	}
}

func TestConfirmationRejectsChangedOrderFacts(t *testing.T) {
	svc, shopRow, orders := newAdvertisingFeeService(t)
	data := advertisingCSV(CoverageExcluded, 100)
	preview, err := svc.Preview(context.Background(), 7, Scope{}, shopRow.ID, "stale.csv", data)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.DB.Model(&ordermodule.Order{}).Where("id = ?", orders[0].ID).Update("payment_status", ordermodule.PaymentUnpaid).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Confirm(context.Background(), 7, Scope{}, shopRow.ID, "stale.csv", data, preview.FileHash, preview.CalculationHash, "advertising-stale-1", nil); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed order facts should conflict, got %v", err)
	}
	var count int64
	if err := svc.DB.Model(&Import{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("stale confirmation wrote imports: count=%d err=%v", count, err)
	}
}

func TestCoverageAndAppendOnlyCorrectionsFailClosed(t *testing.T) {
	svc, shopRow, orders := newAdvertisingFeeService(t)
	data := advertisingCSV(CoverageIncluded, 100)
	preview, err := svc.Preview(context.Background(), 7, Scope{}, shopRow.ID, "included.csv", data)
	if err != nil || !preview.Valid || len(preview.Warnings) != 1 {
		t.Fatalf("included preview = %#v err=%v", preview, err)
	}
	if _, err := svc.Confirm(context.Background(), 7, Scope{}, shopRow.ID, "included.csv", data, preview.FileHash, preview.CalculationHash, "advertising-included-1", nil); err != nil {
		t.Fatal(err)
	}
	list, err := svc.ListAllocations(context.Background(), 7, Scope{}, ListQuery{Page: 1, PageSize: 20, OrderNo: orders[0].OrderNo})
	if err != nil || len(list.List) != 1 || list.List[0].ProfitStatus != "mismatch" {
		t.Fatalf("included allocation = %#v err=%v", list, err)
	}
	allocation := list.List[0]
	adjusted, err := svc.CreateAdjustment(context.Background(), 7, Scope{}, nil, allocation.ID, CreateAdjustmentInput{AmountMinor: 25, Reason: "补录广告费用", IdempotencyKey: "advertising-adjust-1"})
	if err != nil {
		t.Fatal(err)
	}
	reversed, err := svc.ReverseAdjustment(context.Background(), 7, Scope{}, nil, allocation.ID, adjusted.Adjustment.ID, ReverseAdjustmentInput{Reason: "撤销误录费用", IdempotencyKey: "advertising-reverse-1"})
	if err != nil || reversed.Adjustment.AmountMinor != -25 {
		t.Fatalf("reversal = %#v err=%v", reversed, err)
	}
	if _, err := svc.ReverseAdjustment(context.Background(), 7, Scope{}, nil, allocation.ID, adjusted.Adjustment.ID, ReverseAdjustmentInput{Reason: "再次冲正费用", IdempotencyKey: "advertising-reverse-2"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("double reversal should conflict, got %v", err)
	}
	detail, err := svc.GetAllocation(context.Background(), 7, Scope{}, allocation.ID)
	if err != nil || detail.NetAmountMinor != allocation.AmountMinor || detail.AdjustmentCount != 2 {
		t.Fatalf("append-only detail = %#v err=%v", detail, err)
	}
	facts, err := svc.ProfitabilityFeesForOrders(context.Background(), 7, []uuid.UUID{orders[0].ID})
	if err != nil || facts[orders[0].ID].Status != "mismatch" || facts[orders[0].ID].ReasonCode != "advertising_in_platform_fee" {
		t.Fatalf("coverage did not fail closed: %#v err=%v", facts, err)
	}
}

func TestScopeAndInvalidCSVFailClosed(t *testing.T) {
	svc, shopRow, _ := newAdvertisingFeeService(t)
	invalid := []byte("spend_date,currency,spend_minor,settlement_coverage\n2026/09/08,cny,0,maybe\n")
	preview, err := svc.Preview(context.Background(), 7, Scope{}, shopRow.ID, "invalid.csv", invalid)
	if err != nil || preview.Valid || len(preview.Issues) < 3 {
		t.Fatalf("invalid preview = %#v err=%v", preview, err)
	}
	denied := Scope{RestrictStoreScope: true, AllowedShopIDs: []uuid.UUID{}}
	if _, err := svc.Preview(context.Background(), 7, denied, shopRow.ID, "denied.csv", advertisingCSV(CoverageExcluded, 100)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("restricted preview should not reveal shop, got %v", err)
	}
	list, err := svc.ListAllocations(context.Background(), 7, denied, ListQuery{Page: 1, PageSize: 20})
	if err != nil || list.Total != 0 {
		t.Fatalf("restricted list leaked rows: %#v err=%v", list, err)
	}
}
