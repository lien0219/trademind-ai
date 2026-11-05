package profitability

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func completeFXMoney(currency string, amount int64) MoneyComponent {
	return MoneyComponent{Currency: currency, AmountMinor: int64Pointer(amount), Status: ComponentAvailable}
}

func TestResolveFXMixedFeesWithIndependentVerifiedRates(t *testing.T) {
	at := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	orderID := uuid.New()
	raw := OrderProfit{
		OrderID: orderID, Currency: "USD", OrderedAt: &at, Status: StatusMismatch,
		Components: ProfitComponents{
			Revenue:      completeFXMoney("USD", 10000),
			ProductCost:  completeFXMoney("USD", 3000),
			Freight:      completeFXMoney("USD", 1000),
			Refund:       completeFXMoney("USD", 0),
			PlatformFee:  MoneyComponent{Status: ComponentMismatch, ReasonCode: "platform_fee_currency_mismatch"},
			Advertising:  completeFXMoney("USD", 100),
			WarehouseFee: completeFXMoney("USD", 100),
		},
	}
	row := FXReportRow{
		OrderProfit: raw, BaseCurrency: "CNY",
		Converted: FXConverted{
			RevenueMinor: int64Pointer(72500), ProductCostMinor: int64Pointer(21750),
			FreightMinor: int64Pointer(7250), RefundMinor: int64Pointer(0),
			AdvertisingFeeMinor: int64Pointer(725), WarehouseFeeMinor: int64Pointer(725),
		},
	}
	rateID := uuid.New()
	rates := map[string][]FXRate{
		"EUR": {{ID: rateID, FromCurrency: "EUR", ToCurrency: "CNY", Rate: "8",
			EffectiveAt: at.Add(-time.Hour)}},
	}
	facts := &fxMixedFees{platform: map[uuid.UUID]PlatformFeeFact{
		orderID: {OrderID: orderID, Currency: "EUR", Status: "matched", AmountMinor: 200},
	}}
	missing, err := resolveFXMixedFees(&row, raw, facts, rates)
	if err != nil || missing || row.CalculationMethod != "independently_converted_fees" {
		t.Fatalf("verified fee not calculated: missing=%t err=%v row=%#v", missing, err, row)
	}
	if row.Converted.PlatformFeeMinor == nil || *row.Converted.PlatformFeeMinor != 1600 {
		t.Fatalf("platform fee not converted: %#v", row.Converted.PlatformFeeMinor)
	}
	if row.Converted.EstimatedProfitMinor == nil || *row.Converted.EstimatedProfitMinor != 40450 {
		t.Fatalf("profit not recomputed from verified components: %#v", row.Converted.EstimatedProfitMinor)
	}
	if len(row.ComponentRates) != 1 || row.ComponentRates[0].RateID == nil ||
		*row.ComponentRates[0].RateID != rateID || row.ComponentRates[0].OriginalAmountMinor != 200 {
		t.Fatalf("missing original amount or rate evidence: %#v", row.ComponentRates)
	}
}

func TestResolveFXMixedFeesRejectsUnverifiedSources(t *testing.T) {
	at := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	id := uuid.New()
	base := OrderProfit{
		OrderID: id, Currency: "USD", OrderedAt: &at, Status: StatusMismatch,
		Components: ProfitComponents{
			Revenue: completeFXMoney("USD", 1000), ProductCost: completeFXMoney("USD", 100),
			Freight: completeFXMoney("USD", 50), Refund: completeFXMoney("USD", 0),
			PlatformFee: MoneyComponent{Status: ComponentMismatch, ReasonCode: "platform_fee_currency_mismatch"},
			Advertising: completeFXMoney("USD", 20), WarehouseFee: completeFXMoney("USD", 30),
		},
	}
	rates := map[string][]FXRate{"EUR": {{ID: uuid.New(), Rate: "8", EffectiveAt: at.Add(-time.Hour)}}}
	cases := []struct {
		name       string
		fact       PlatformFeeFact
		rates      map[string][]FXRate
		incomplete bool
	}{
		{name: "wrong order", fact: PlatformFeeFact{OrderID: uuid.New(), Currency: "EUR", Status: "matched", AmountMinor: 100}, rates: rates},
		{name: "unconfirmed", fact: PlatformFeeFact{OrderID: id, Currency: "EUR", Status: "pending", AmountMinor: 100}, rates: rates},
		{name: "missing FX", fact: PlatformFeeFact{OrderID: id, Currency: "EUR", Status: "matched", AmountMinor: 100}, rates: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := FXReportRow{OrderProfit: base, BaseCurrency: "CNY", Converted: FXConverted{
				RevenueMinor: int64Pointer(7250), ProductCostMinor: int64Pointer(725),
				FreightMinor: int64Pointer(363), RefundMinor: int64Pointer(0),
				AdvertisingFeeMinor: int64Pointer(145), WarehouseFeeMinor: int64Pointer(218),
			}}
			facts := &fxMixedFees{platform: map[uuid.UUID]PlatformFeeFact{id: tc.fact}}
			missing, err := resolveFXMixedFees(&row, base, facts, tc.rates)
			if err != nil || row.Converted.EstimatedProfitMinor != nil || row.CalculationMethod != "" {
				t.Fatalf("unverified record produced profit: %#v %v", row, err)
			}
			if tc.name == "missing FX" && !missing {
				t.Fatalf("missing FX rate was not reported")
			}
		})
	}
}

func TestResolveFXMixedFeesDoesNotMaskOtherV6Issues(t *testing.T) {
	at := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	id := uuid.New()
	raw := OrderProfit{
		OrderID: id, Currency: "USD", OrderedAt: &at, Status: StatusMismatch,
		Components: ProfitComponents{
			Revenue: completeFXMoney("USD", 1000), ProductCost: MoneyComponent{Status: ComponentMissing},
			Freight: completeFXMoney("USD", 10), Refund: completeFXMoney("USD", 0),
			PlatformFee: MoneyComponent{Status: ComponentMismatch, ReasonCode: "platform_fee_currency_mismatch"},
			Advertising: completeFXMoney("USD", 5), WarehouseFee: completeFXMoney("USD", 5),
		},
	}
	row := FXReportRow{OrderProfit: raw, BaseCurrency: "CNY", Converted: FXConverted{
		RevenueMinor: int64Pointer(7250), FreightMinor: int64Pointer(73),
		RefundMinor: int64Pointer(0), AdvertisingFeeMinor: int64Pointer(36), WarehouseFeeMinor: int64Pointer(36),
	}}
	facts := &fxMixedFees{platform: map[uuid.UUID]PlatformFeeFact{id: {
		OrderID: id, Currency: "EUR", Status: "matched", AmountMinor: 100,
	}}}
	_, err := resolveFXMixedFees(&row, raw, facts, map[string][]FXRate{
		"EUR": {{Rate: "8", EffectiveAt: at.Add(-time.Hour)}},
	})
	if err != nil || row.Converted.EstimatedProfitMinor != nil || row.CalculationMethod != "" {
		t.Fatalf("missing product cost was incorrectly treated as complete: %#v %v", row, err)
	}
}
