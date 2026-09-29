package freightfee

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

func TestParseCSVAcceptsBOMAndRejectsDuplicateTrackingAndUnsafeAmounts(t *testing.T) {
	data := []byte("\ufeff" + strings.Join(CSVHeaders, ",") + "\n" +
		"line-1,Carrier A,TRACK-1,125,CNY,2026-09-01T12:00:00Z\n" +
		"line-2,Carrier A,TRACK-1,130,CNY,2026-09-01T12:00:00Z\n" +
		"line-3,Carrier B,TRACK-3,9007199254740992,CNY,2026-09-01T12:00:00Z\n")

	rows, issues := parseCSV(data)
	if len(rows) != 1 || rows[0].ExternalLineID != "line-1" || rows[0].Currency != "CNY" {
		t.Fatalf("parsed rows = %#v", rows)
	}
	if len(issues) != 2 || issues[0].Code != "duplicate_tracking_line" || issues[1].Code != "invalid_amount_minor" {
		t.Fatalf("validation issues = %#v", issues)
	}
}

func TestSummarizeChargeIncludesAppendOnlyCorrectionsAndRejectsNegativeNet(t *testing.T) {
	chargeID, orderID, shopID := uuid.New(), uuid.New(), uuid.New()
	charge := Charge{
		TenantID: 7, ShopID: shopID, OrderID: orderID, ShipmentID: uuid.New(), ImportID: uuid.New(),
		OrderNo: "SO-1", OrderCurrency: "CNY", Carrier: "Carrier A", TrackingNo: "TRACK-1",
		AmountMinor: 100, Currency: "CNY", RowHash: strings.Repeat("a", 64), BilledAt: time.Now().UTC(),
	}
	charge.ID = chargeID
	adjustmentID := uuid.New()
	rows := []Adjustment{
		{ChargeID: chargeID, OrderID: orderID, ShopID: shopID, FactType: FactAdjustment, AmountMinor: 25, Currency: "CNY"},
		{ChargeID: chargeID, OrderID: orderID, ShopID: shopID, FactType: FactReversal, AmountMinor: -25, Currency: "CNY", ReversesAdjustmentID: &adjustmentID},
	}
	rows[0].ID = adjustmentID
	rows[1].ID = uuid.New()

	summary, err := summarizeCharge(charge, rows)
	if err != nil {
		t.Fatal(err)
	}
	if summary.NetAmountMinor != 100 || summary.AdjustmentMinor != 0 || summary.AdjustmentCount != 2 {
		t.Fatalf("summary = %#v", summary)
	}

	negativeAdjustment := Adjustment{
		ChargeID: chargeID, OrderID: orderID, ShopID: shopID,
		FactType: FactAdjustment, AmountMinor: -101, Currency: "CNY",
	}
	negativeAdjustment.ID = uuid.New()
	if _, err := summarizeCharge(charge, []Adjustment{negativeAdjustment}); err == nil {
		t.Fatal("expected negative net amount to be rejected")
	}
}

func TestChargeMatchesShipmentRequiresCurrentShipmentAndImportScope(t *testing.T) {
	shopID, orderID, shipmentID, importID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	charge := Charge{
		TenantID: 7, ShopID: shopID, OrderID: orderID, ShipmentID: shipmentID, ImportID: importID, OrderNo: "SO-1", OrderCurrency: "CNY",
		Carrier: "CARRIER A", TrackingNo: "TRACK-1",
	}
	charge.ID = uuid.New()
	shopRef := shopID
	shipment := shipmentReference{
		ShipmentID: shipmentID, OrderID: orderID, ShopID: &shopRef, OrderNo: "SO-1", Carrier: "Carrier A", TrackingNo: "TRACK-1", Currency: "cny",
	}
	batch := Import{HardDeleteBase: model.HardDeleteBase{ID: importID}, TenantID: 7, ShopID: shopID, PolicyVersion: PolicyVersion, FileHash: strings.Repeat("a", 64), CalculationHash: strings.Repeat("b", 64)}

	if !chargeMatchesShipment(7, charge, shipment, batch) {
		t.Fatal("expected matching shipment and import scope")
	}
	shipment.TrackingNo = "TRACK-CHANGED"
	if chargeMatchesShipment(7, charge, shipment, batch) {
		t.Fatal("expected changed tracking number to block the freight fact")
	}
	shipment.TrackingNo = "TRACK-1"
	batch.PolicyVersion = "unknown"
	if chargeMatchesShipment(7, charge, shipment, batch) {
		t.Fatal("expected unknown import policy to block the freight fact")
	}
}

func TestNormalizeListQueryBoundsPagination(t *testing.T) {
	query, err := normalizeListQuery(ListQuery{})
	if err != nil || query.Page != 1 || query.PageSize != 20 {
		t.Fatalf("default query = %#v, err = %v", query, err)
	}
	query = ListQuery{Page: int(^uint(0) >> 1), PageSize: MaxPageSize}
	if _, err := normalizeListQuery(query); err == nil {
		t.Fatal("expected pagination offset overflow to be rejected")
	}
}

func TestValidSHA256HashChecksHexEncoding(t *testing.T) {
	if !validSHA256Hash(strings.Repeat("a", 64)) || validSHA256Hash(strings.Repeat("z", 64)) || validSHA256Hash(strings.Repeat("a", 63)) {
		t.Fatal("SHA-256 hash validation did not enforce a 32-byte hex digest")
	}
}

func TestPreviewRejectsUnavailableService(t *testing.T) {
	if _, err := (*Service)(nil).Preview(context.Background(), 7, Scope{}, uuid.New(), "freight.csv", []byte("data")); err != ErrInvalidInput {
		t.Fatalf("Preview error = %v, want %v", err, ErrInvalidInput)
	}
}
