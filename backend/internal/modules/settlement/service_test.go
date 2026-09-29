package settlement

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	ordermod "github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	basemodel "github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/gorm"
)

func openSettlementTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&shop.Shop{}, &ordermod.Order{}, &Import{}, &Transaction{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func settlementCSV(rows ...string) []byte {
	return []byte(strings.Join(append([]string{strings.Join(CSVHeaders, ",")}, rows...), "\n") + "\n")
}

func TestPreviewConfirmAndReconcileImmutableSettlementFacts(t *testing.T) {
	db := openSettlementTestDB(t)
	now := time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC)
	shopRow := shop.Shop{TenantID: 41, Platform: "douyin_shop", ShopName: "旗舰店", Status: "active", AuthStatus: "active"}
	if err := db.Create(&shopRow).Error; err != nil {
		t.Fatalf("create shop: %v", err)
	}
	order := ordermod.Order{
		Base: basemodel.Base{CreatedAt: now}, TenantID: 41, Platform: shopRow.Platform, ShopID: &shopRow.ID,
		OrderNo: "SO-1001", CustomerName: "customer", Status: "confirmed", PaymentStatus: "paid",
		FulfillmentStatus: "fulfilled", Currency: "CNY", TotalAmount: 100,
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create order: %v", err)
	}
	data := settlementCSV(
		"TX-1001,SO-1001,CNY,10000,1000,9000,2026-09-06T01:00:00Z",
		"TX-1001-ADJ,SO-1001,CNY,0,-100,100,2026-09-07T01:00:00Z",
	)
	svc := &Service{DB: db, Clock: func() time.Time { return now }}
	scope := Scope{RestrictStoreScope: true, AllowedShopIDs: []uuid.UUID{shopRow.ID}}

	preview, err := svc.Preview(context.Background(), 41, scope, shopRow.ID, "bill.csv", data)
	if err != nil {
		t.Fatalf("Preview() error = %v", err)
	}
	if !preview.Valid || preview.SourceRows != 2 || preview.NewRows != 2 || preview.DuplicateRows != 0 {
		t.Fatalf("preview = %#v", preview)
	}
	var before int64
	if err := db.Model(&Transaction{}).Count(&before).Error; err != nil || before != 0 {
		t.Fatalf("preview wrote transactions: count=%d err=%v", before, err)
	}

	actor := uuid.New()
	confirmed, err := svc.Confirm(context.Background(), 41, scope, shopRow.ID, "bill.csv", data, preview.FileHash, "settlement-import-1001", &actor)
	if err != nil {
		t.Fatalf("Confirm() error = %v", err)
	}
	if confirmed.Replayed || confirmed.Import.ImportedRows != 2 || confirmed.Import.DuplicateRows != 0 {
		t.Fatalf("confirm = %#v", confirmed)
	}
	replayed, err := svc.Confirm(context.Background(), 41, scope, shopRow.ID, "bill.csv", data, preview.FileHash, "settlement-import-1001", &actor)
	if err != nil || !replayed.Replayed || replayed.Import.ID != confirmed.Import.ID {
		t.Fatalf("replay = %#v err=%v", replayed, err)
	}
	fileReplay, err := svc.Confirm(context.Background(), 41, scope, shopRow.ID, "bill-copy.csv", data, preview.FileHash, "settlement-import-1001-copy", &actor)
	if err != nil || !fileReplay.Replayed || fileReplay.Import.ID != confirmed.Import.ID {
		t.Fatalf("file replay = %#v err=%v", fileReplay, err)
	}
	otherData := settlementCSV("TX-1002,SO-1001,CNY,0,-50,50,2026-09-07T02:00:00Z")
	otherPreview, err := svc.Preview(context.Background(), 41, scope, shopRow.ID, "other.csv", otherData)
	if err != nil || !otherPreview.Valid {
		t.Fatalf("other preview = %#v err=%v", otherPreview, err)
	}
	if _, err := svc.Confirm(context.Background(), 41, scope, shopRow.ID, "other.csv", otherData, otherPreview.FileHash, "settlement-import-1001", &actor); !errors.Is(err, ErrConflict) {
		t.Fatalf("idempotency key conflict = %v, want ErrConflict", err)
	}
	var importCount, transactionCount int64
	_ = db.Model(&Import{}).Count(&importCount).Error
	_ = db.Model(&Transaction{}).Count(&transactionCount).Error
	if importCount != 1 || transactionCount != 2 {
		t.Fatalf("counts imports=%d transactions=%d, want 1/2", importCount, transactionCount)
	}

	list, err := svc.List(context.Background(), 41, scope, ListQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if list.Total != 1 || len(list.List) != 1 {
		t.Fatalf("list = %#v", list)
	}
	row := list.List[0]
	if row.Status != StatusMatched || row.PlatformFeeMinor == nil || *row.PlatformFeeMinor != 900 || row.SettlementAmountMinor == nil || *row.SettlementAmountMinor != 9100 {
		t.Fatalf("reconciliation = %#v", row)
	}
	detail, err := svc.Get(context.Background(), 41, scope, row.ID)
	if err != nil || len(detail.Transactions) != 2 {
		t.Fatalf("detail = %#v err=%v", detail, err)
	}
	fees, err := svc.PlatformFeesForOrders(context.Background(), 41, []uuid.UUID{order.ID})
	if err != nil {
		t.Fatalf("PlatformFeesForOrders() error = %v", err)
	}
	if fees[order.ID].Status != StatusMatched || fees[order.ID].AmountMinor != 900 || len(fees[order.ID].TransactionIDs) != 2 {
		t.Fatalf("platform fee = %#v", fees[order.ID])
	}
}

func TestPreviewRejectsChangedExternalTransactionAndStoreEscape(t *testing.T) {
	db := openSettlementTestDB(t)
	shopRow := shop.Shop{TenantID: 41, Platform: "manual", ShopName: "A", Status: "active", AuthStatus: "active"}
	otherShop := shop.Shop{TenantID: 41, Platform: "manual", ShopName: "B", Status: "active", AuthStatus: "active"}
	if err := db.Create(&shopRow).Error; err != nil {
		t.Fatalf("create shop: %v", err)
	}
	if err := db.Create(&otherShop).Error; err != nil {
		t.Fatalf("create other shop: %v", err)
	}
	svc := &Service{DB: db}
	data := settlementCSV("TX-1,SO-1,CNY,100,10,90,2026-09-07T01:00:00Z")
	preview, err := svc.Preview(context.Background(), 41, Scope{}, shopRow.ID, "bill.csv", data)
	if err != nil {
		t.Fatalf("Preview() error = %v", err)
	}
	if _, err := svc.Confirm(context.Background(), 41, Scope{}, shopRow.ID, "bill.csv", data, preview.FileHash, "settlement-import-1", nil); err != nil {
		t.Fatalf("Confirm() error = %v", err)
	}
	changed := settlementCSV("TX-1,SO-1,CNY,100,20,80,2026-09-07T01:00:00Z")
	conflict, err := svc.Preview(context.Background(), 41, Scope{}, shopRow.ID, "changed.csv", changed)
	if err != nil {
		t.Fatalf("conflict Preview() error = %v", err)
	}
	if conflict.Valid || len(conflict.Issues) != 1 || conflict.Issues[0].Code != "external_transaction_conflict" {
		t.Fatalf("conflict preview = %#v", conflict)
	}
	_, err = svc.Preview(context.Background(), 41, Scope{RestrictStoreScope: true, AllowedShopIDs: []uuid.UUID{shopRow.ID}}, otherShop.ID, "bill.csv", data)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("store escape error = %v, want record not found", err)
	}
}

func TestCSVValidationFailsClosed(t *testing.T) {
	rows, issues := parseCSV(settlementCSV(
		"TX-1,SO-1,CNY,100,10,80,not-a-time",
		"TX-1,SO-2,US,100,10,90,2026-09-07T01:00:00Z",
	))
	if len(rows) != 0 || len(issues) < 3 {
		t.Fatalf("rows=%#v issues=%#v", rows, issues)
	}
	codes := make(map[string]bool)
	for _, issue := range issues {
		codes[issue.Code] = true
	}
	for _, code := range []string{"settlement_amount_mismatch", "invalid_settled_at", "invalid_currency", "duplicate_external_transaction"} {
		if !codes[code] {
			t.Fatalf("missing validation code %s: %#v", code, issues)
		}
	}
}

func TestCalculateGroupKeepsBlockedAsHighestPriority(t *testing.T) {
	groupID, groupShopID := uuid.New(), uuid.New()
	now := time.Now().UTC()
	row := calculateGroup(
		groupFact{ID: groupID, ShopID: groupShopID, ShopName: "A", Platform: "manual", OrderNo: "SO-1"},
		[]Transaction{{
			ReconciliationID: groupID, ShopID: uuid.New(), Platform: "other", OrderNo: "SO-2",
			Currency: "CNY", OrderGrossMinor: 100, PlatformFeeMinor: 10, SettlementAmountMinor: 90,
			SettledAt: now,
		}},
		[]orderFact{{ID: uuid.New(), Platform: "manual", Currency: "USD", TotalAmountText: "1.00"}},
	)
	if row.Status != StatusBlocked {
		t.Fatalf("status = %s, want blocked: %#v", row.Status, row.Issues)
	}
}

func TestUniqueConstraintErrorsAreBusinessConflicts(t *testing.T) {
	for _, message := range []string{
		`ERROR: duplicate key value violates unique constraint "ux_settlement_external_transaction" (SQLSTATE 23505)`,
		"UNIQUE constraint failed: settlement_transactions.external_transaction_id",
		"Error 1062: Duplicate entry 'TX-1' for key 'ux_settlement_external_transaction'",
	} {
		if !isUniqueConstraintError(errors.New(message)) {
			t.Fatalf("unique error not recognized: %s", message)
		}
	}
	if isUniqueConstraintError(errors.New("connection refused")) {
		t.Fatal("non-constraint database error must not be treated as a business conflict")
	}
}
