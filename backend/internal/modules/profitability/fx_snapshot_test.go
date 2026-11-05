package profitability

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestFXSnapshotImmutableTenantScopedAndIntegrityChecked(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:fx_report_snapshot_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&FXReportSnapshot{}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 3, 1, 0, 0, 0, time.UTC)
	report := &FXReport{
		BaseCurrency: "CNY", CalculatedAt: now, FormulaVersion: FormulaVersion,
		TotalOrders: 1, ExcludedOrders: 1,
		Rows: []FXReportRow{{OrderProfit: OrderProfit{OrderID: uuid.New(), Currency: "USD"}, ConversionStatus: "rate_missing"}},
	}
	snapshot, err := freezeFXReport(41, uuid.New(), report, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(snapshot).Error; err != nil {
		t.Fatal(err)
	}
	service := &Service{DB: db}
	meta, saved, err := service.GetFXSnapshot(context.Background(), 41, snapshot.ID)
	if err != nil {
		t.Fatal(err)
	}
	if meta.ContentSHA256 != snapshot.ContentSHA256 || saved.Rows[0].ConversionStatus != "rate_missing" {
		t.Fatalf("snapshot was changed: %#v %#v", meta, saved)
	}
	if _, _, err := service.GetFXSnapshot(context.Background(), 99, snapshot.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("tenant isolation failed: %v", err)
	}
	if err := db.Model(&FXReportSnapshot{}).Where("id = ?", snapshot.ID).UpdateColumn("report_json", []byte("tampered")).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.GetFXSnapshot(context.Background(), 41, snapshot.ID); !errors.Is(err, ErrFXSnapshotCorrupt) {
		t.Fatalf("tampered snapshot accepted: %v", err)
	}
}
