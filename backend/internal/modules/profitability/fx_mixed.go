package profitability

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// FXComponentRate preserves immutable evidence of a separately denominated fee.
// A rate ID is omitted only for an identity conversion to the report currency.
type FXComponentRate struct {
	Component            string     `json:"component"`
	SourceCurrency       string     `json:"sourceCurrency"`
	OriginalAmountMinor  int64      `json:"originalAmountMinor"`
	ConvertedAmountMinor int64      `json:"convertedAmountMinor"`
	Rate                 string     `json:"rate"`
	RateID               *uuid.UUID `json:"rateId,omitempty"`
	RateEffectiveAt      *time.Time `json:"rateEffectiveAt,omitempty"`
}

type fxMixedFees struct {
	platform    map[uuid.UUID]PlatformFeeFact
	advertising map[uuid.UUID]AdvertisingFeeFact
	warehouse   map[uuid.UUID]WarehouseFeeFact
}

func (s *Service) loadFXMixedFees(ctx context.Context, tenantID int64, rows []OrderProfit) (*fxMixedFees, error) {
	facts := &fxMixedFees{}
	var platforms, advertisements, warehouses []uuid.UUID
	for _, row := range rows {
		if row.Components.PlatformFee.ReasonCode == "platform_fee_currency_mismatch" {
			platforms = append(platforms, row.OrderID)
		}
		if row.Components.Advertising.ReasonCode == "advertising_fee_currency_mismatch" {
			advertisements = append(advertisements, row.OrderID)
		}
		if row.Components.WarehouseFee.ReasonCode == "warehouse_fee_currency_mismatch" {
			warehouses = append(warehouses, row.OrderID)
		}
	}
	var err error
	if len(platforms) != 0 && s.PlatformFees != nil {
		facts.platform, err = s.PlatformFees.ListPlatformFees(ctx, tenantID, platforms)
		if err != nil {
			return nil, err
		}
	}
	if len(advertisements) != 0 && s.AdvertisingFees != nil {
		facts.advertising, err = s.AdvertisingFees.ListAdvertisingFees(ctx, tenantID, advertisements)
		if err != nil {
			return nil, err
		}
	}
	if len(warehouses) != 0 && s.WarehouseFees != nil {
		facts.warehouse, err = s.WarehouseFees.ListWarehouseFees(ctx, tenantID, warehouses)
		if err != nil {
			return nil, err
		}
	}
	return facts, nil
}

func fxMixedRate(amount int64, from, to, component string, orderDate *time.Time, rates map[string][]FXRate) (*int64, *FXComponentRate, bool, error) {
	from = strings.ToUpper(strings.TrimSpace(from))
	if !validFXCurrency(from) || !validFXCurrency(to) || !jsonSafeInteger(amount) || orderDate == nil {
		return nil, nil, false, nil
	}
	selected := FXComponentRate{Component: component, SourceCurrency: from, OriginalAmountMinor: amount, Rate: "1"}
	if from != to {
		candidates := rates[from]
		index := sort.Search(len(candidates), func(i int) bool {
			return candidates[i].EffectiveAt.After(*orderDate)
		})
		if index == 0 {
			return nil, nil, true, nil
		}
		rate := candidates[index-1]
		selected.Rate = rate.Rate
		id, at := rate.ID, rate.EffectiveAt
		selected.RateID, selected.RateEffectiveAt = &id, &at
	}
	value, err := convertFXMinor(amount, from, to, selected.Rate)
	if err != nil {
		return nil, nil, false, err
	}
	selected.ConvertedAmountMinor = value
	return &value, &selected, false, nil
}

func fxVerifiedComponent(c MoneyComponent, component string, converted *int64, rates []FXComponentRate) bool {
	if converted == nil {
		return false
	}
	if c.Status == ComponentAvailable {
		return true
	}
	allowed := map[string]string{
		"platformFee":    "platform_fee_currency_mismatch",
		"advertisingFee": "advertising_fee_currency_mismatch",
		"warehouseFee":   "warehouse_fee_currency_mismatch",
	}
	if c.Status != ComponentMismatch || c.ReasonCode != allowed[component] {
		return false
	}
	for _, evidence := range rates {
		if evidence.Component == component {
			return true
		}
	}
	return false
}

// resolveFXMixedFees reads only confirmed, tenant-scoped raw fee facts. It
// leaves unresolved V6 sources excluded and never rewrites the original V6
// status or its costs. Rounding is once per verified component in base units.
func resolveFXMixedFees(row *FXReportRow, raw OrderProfit, facts *fxMixedFees, rates map[string][]FXRate) (bool, error) {
	missingRate := false
	if facts == nil {
		return false, nil
	}
	apply := func(component string, amount int64, currency string, target **int64) error {
		value, evidence, missing, err := fxMixedRate(amount, currency, row.BaseCurrency, component, raw.OrderedAt, rates)
		if err != nil {
			return err
		}
		if missing {
			missingRate = true
		}
		if value != nil && evidence != nil {
			*target = value
			row.ComponentRates = append(row.ComponentRates, *evidence)
		}
		return nil
	}
	platform := raw.Components.PlatformFee
	if platform.Status == ComponentMismatch && platform.ReasonCode == "platform_fee_currency_mismatch" {
		if fact, ok := facts.platform[raw.OrderID]; ok && fact.OrderID == raw.OrderID &&
			fact.Status == "matched" && jsonSafeInteger(fact.AmountMinor) {
			if err := apply("platformFee", fact.AmountMinor, fact.Currency, &row.Converted.PlatformFeeMinor); err != nil {
				return false, err
			}
		}
	}
	advertising := raw.Components.Advertising
	if advertising.Status == ComponentMismatch && advertising.ReasonCode == "advertising_fee_currency_mismatch" {
		if fact, ok := facts.advertising[raw.OrderID]; ok && fact.OrderID == raw.OrderID &&
			fact.Status == "confirmed" && fact.AmountMinor >= 0 && jsonSafeInteger(fact.AmountMinor) {
			if err := apply("advertisingFee", fact.AmountMinor, fact.Currency, &row.Converted.AdvertisingFeeMinor); err != nil {
				return false, err
			}
		}
	}
	warehouse := raw.Components.WarehouseFee
	if warehouse.Status == ComponentMismatch && warehouse.ReasonCode == "warehouse_fee_currency_mismatch" {
		if fact, ok := facts.warehouse[raw.OrderID]; ok && fact.OrderID == raw.OrderID &&
			fact.Status == "confirmed" && fact.AmountMinor >= 0 && jsonSafeInteger(fact.AmountMinor) {
			if err := apply("warehouseFee", fact.AmountMinor, fact.Currency, &row.Converted.WarehouseFeeMinor); err != nil {
				return false, err
			}
		}
	}
	if raw.Status != StatusMismatch || len(row.ComponentRates) == 0 {
		return missingRate, nil
	}
	c := row.Converted
	verified := fxVerifiedComponent(raw.Components.Revenue, "revenue", c.RevenueMinor, row.ComponentRates) &&
		fxVerifiedComponent(raw.Components.ProductCost, "productCost", c.ProductCostMinor, row.ComponentRates) &&
		fxVerifiedComponent(raw.Components.Freight, "freight", c.FreightMinor, row.ComponentRates) &&
		fxVerifiedComponent(raw.Components.Refund, "refund", c.RefundMinor, row.ComponentRates) &&
		fxVerifiedComponent(raw.Components.PlatformFee, "platformFee", c.PlatformFeeMinor, row.ComponentRates) &&
		fxVerifiedComponent(raw.Components.Advertising, "advertisingFee", c.AdvertisingFeeMinor, row.ComponentRates) &&
		fxVerifiedComponent(raw.Components.WarehouseFee, "warehouseFee", c.WarehouseFeeMinor, row.ComponentRates)
	if !verified {
		return missingRate, nil
	}
	profit, ok := subtractKnown(*c.RevenueMinor, *c.ProductCostMinor, *c.FreightMinor,
		*c.RefundMinor, *c.PlatformFeeMinor, *c.AdvertisingFeeMinor, *c.WarehouseFeeMinor)
	if !ok || !jsonSafeInteger(profit) {
		return false, ErrFXOverflow
	}
	row.Converted.EstimatedProfitMinor = int64Pointer(profit)
	row.Converted.KnownContributionMinor = int64Pointer(profit)
	row.CalculationMethod = "independently_converted_fees"
	return missingRate, nil
}
