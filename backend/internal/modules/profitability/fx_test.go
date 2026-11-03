package profitability

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestConvertFXMinorExactCurrencyPrecision(t *testing.T) {
	cases := []struct {
		name     string
		amount   int64
		from     string
		to       string
		rate     string
		expected int64
	}{
		{name: "USD to CNY fractional midpoint", amount: 1234, from: "USD", to: "CNY", rate: "7.25", expected: 8947},
		{name: "negative halfway rounds away from zero", amount: -1234, from: "USD", to: "CNY", rate: "7.25", expected: -8947},
		{name: "JPY zero decimals", amount: 123, from: "USD", to: "JPY", rate: "150", expected: 185},
		{name: "JPY to USD cents", amount: 100, from: "JPY", to: "USD", rate: "0.0067", expected: 67},
		{name: "identity", amount: 1200, from: "USD", to: "USD", rate: "1", expected: 1200},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := convertFXMinor(tc.amount, tc.from, tc.to, tc.rate)
			if err != nil || got != tc.expected {
				t.Fatalf("convertFXMinor(%d, %s, %s, %s) = %d, %v; want %d",
					tc.amount, tc.from, tc.to, tc.rate, got, err, tc.expected)
			}
		})
	}
	for _, tc := range []struct {
		amount int64
		rate   string
	}{
		{amount: math.MaxInt64, rate: "1"},
		{amount: 9007199254740991, rate: "1000000"},
		{amount: 100, rate: "0"},
		{amount: 100, rate: "invalid"},
	} {
		_, err := convertFXMinor(tc.amount, "USD", "CNY", tc.rate)
		if err == nil {
			t.Fatalf("expected rejection of amount=%d rate=%q", tc.amount, tc.rate)
		}
	}
}

func TestValidateFXRateRequiresEvidenceAndTenant(t *testing.T) {
	now := time.Date(2026, time.November, 3, 0, 0, 0, 0, time.UTC)
	rate := FXRate{
		TenantID: 7, CreatedBy: uuid.New(), FromCurrency: "USD",
		ToCurrency: "CNY", Rate: "7.250001", EffectiveAt: now.Add(-time.Hour),
		Evidence: "Manual ledger reference FX-001",
	}
	if err := ValidateFXRate(&rate, now); err != nil {
		t.Fatalf("valid rate rejected: %v", err)
	}
	rate.Evidence = "short"
	if !errors.Is(ValidateFXRate(&rate, now), ErrFXInvalid) {
		t.Fatal("short evidence accepted")
	}
	rate.Evidence = "Manual ledger reference FX-001"
	rate.Rate = "7e2"
	if !errors.Is(ValidateFXRate(&rate, now), ErrFXInvalid) {
		t.Fatal("scientific rate accepted")
	}
	rate.Rate = "7.2"
	rate.EffectiveAt = now.Add(time.Hour)
	if !errors.Is(ValidateFXRate(&rate, now), ErrFXInvalid) {
		t.Fatal("future rate accepted")
	}
	rate.EffectiveAt = now.Add(-time.Hour)
	rate.TenantID = 0
	if !errors.Is(ValidateFXRate(&rate, now), ErrFXInvalid) {
		t.Fatal("tenant-less rate accepted")
	}
}

func TestFXReportSelectsEffectiveRateAndExcludesMissing(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:fx_report_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&FXRate{}); err != nil {
		t.Fatalf("migrate rates: %v", err)
	}
	now := time.Date(2026, time.November, 3, 12, 0, 0, 0, time.UTC)
	orderDate := now.Add(-24 * time.Hour)
	oldRate := FXRate{
		ID: uuid.New(), TenantID: 41, FromCurrency: "USD", ToCurrency: "CNY",
		Rate: "7.25", EffectiveAt: orderDate.Add(-time.Hour),
		Evidence: "Approved enterprise FX-2026", CreatedBy: uuid.New(),
		CreatedAt: now.Add(-2 * time.Hour),
	}
	newRate := FXRate{
		ID: uuid.New(), TenantID: 41, FromCurrency: "USD", ToCurrency: "CNY",
		Rate: "8.00", EffectiveAt: orderDate.Add(time.Hour),
		Evidence: "Approved enterprise FX-2026", CreatedBy: uuid.New(),
		CreatedAt: now.Add(-time.Hour),
	}
	otherTenant := oldRate
	otherTenant.ID = uuid.New()
	otherTenant.TenantID = 99
	otherTenant.Rate = "99.00"
	for _, rate := range []FXRate{oldRate, newRate, otherTenant} {
		if err := db.Create(&rate).Error; err != nil {
			t.Fatalf("insert rate: %v", err)
		}
	}
	repo := &fakeRepository{orders: []OrderFact{{
		ID: uuid.New(), OrderNo: "EUR-001", Currency: "USD",
		TotalAmountText: "12.34", OrderedAt: &orderDate,
		CreatedAt: orderDate, UpdatedAt: orderDate,
	}}}
	service := &Service{DB: db, Repo: repo, Clock: func() time.Time { return now }}
	report, err := service.BuildFXReport(context.Background(), 41, Scope{}, ListQuery{}, "CNY")
	if err != nil {
		t.Fatalf("build report: %v", err)
	}
	if report.TotalOrders != 1 || len(report.Rows) != 1 {
		t.Fatalf("report rows: %#v", report)
	}
	row := report.Rows[0]
	if row.RateID == nil || *row.RateID != oldRate.ID || row.Rate != "7.25" {
		t.Fatalf("wrong effective rate: %#v", row)
	}
	if row.Converted.RevenueMinor == nil || *row.Converted.RevenueMinor != 8947 {
		t.Fatalf("converted revenue = %#v", row.Converted.RevenueMinor)
	}
	withoutRates, err := service.BuildFXReport(context.Background(), 42, Scope{}, ListQuery{}, "CNY")
	if err != nil {
		t.Fatalf("other tenant report: %v", err)
	}
	if withoutRates.ExcludedOrders != 1 || withoutRates.Rows[0].ConversionStatus != "rate_missing" {
		t.Fatalf("missing FX was not excluded: %#v", withoutRates)
	}
}

func TestWriteFXCSVPreservesMissingAndNeutralizesFormula(t *testing.T) {
	now := time.Now().UTC()
	report := &FXReport{BaseCurrency: "CNY", CalculatedAt: now, FormulaVersion: FormulaVersion}
	report.Rows = []FXReportRow{{
		OrderProfit: OrderProfit{
			OrderNo: "=1+1", Platform: "+cmd", ShopName: "@shop", Currency: "USD",
			CalculatedAt: now,
		},
		BaseCurrency: "CNY", ConversionStatus: "rate_missing",
	}}
	var output bytes.Buffer
	if err := WriteFXCSV(&output, report); err != nil {
		t.Fatalf("write report: %v", err)
	}
	records, err := csv.NewReader(&output).ReadAll()
	if err != nil || len(records) != 2 {
		t.Fatalf("parse CSV: %v, %#v", err, records)
	}
	if !strings.HasPrefix(records[1][0], "'") || records[1][8] != "" || records[1][17] != "" {
		t.Fatalf("formula or missing amount semantics changed: %v", records[1])
	}
}
