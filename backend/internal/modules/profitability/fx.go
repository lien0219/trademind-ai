package profitability

import (
	"context"
	"errors"
	"math/big"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrFXInvalid  = errors.New("invalid foreign exchange input")
	ErrFXOverflow = errors.New("converted amount exceeds JavaScript safe integer")
	ErrFXTooMany  = errors.New("too many foreign exchange rates")
	fxRatePattern = regexp.MustCompile(`^[0-9]{1,12}(\.[0-9]{1,10})?$`)
)

const maxFXRateRows = 50000

// FXRate is append-only, tenant scoped evidence. The quoted rate converts
// one major unit of FromCurrency into ToCurrency major units.
type FXRate struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID     int64     `gorm:"not null;uniqueIndex:ux_tenant_fx_effective,priority:1;index:idx_tenant_fx_lookup,priority:1" json:"-"`
	FromCurrency string    `gorm:"size:3;not null;uniqueIndex:ux_tenant_fx_effective,priority:2;index:idx_tenant_fx_lookup,priority:2" json:"fromCurrency"`
	ToCurrency   string    `gorm:"size:3;not null;uniqueIndex:ux_tenant_fx_effective,priority:3;index:idx_tenant_fx_lookup,priority:3" json:"toCurrency"`
	EffectiveAt  time.Time `gorm:"not null;uniqueIndex:ux_tenant_fx_effective,priority:4;index:idx_tenant_fx_lookup,priority:4" json:"effectiveAt"`
	Rate         string    `gorm:"size:40;not null" json:"rate"`
	Evidence     string    `gorm:"size:500;not null" json:"evidence"`
	CreatedBy    uuid.UUID `gorm:"type:uuid;not null" json:"createdBy"`
	CreatedAt    time.Time `gorm:"not null" json:"createdAt"`
}

func (FXRate) TableName() string {
	return "tenant_fx_rates"
}

func validFXCurrency(currency string) bool {
	switch currency {
	case "CNY", "USD", "JPY", "EUR", "GBP", "KRW", "SGD", "AUD", "CAD":
		return true
	default:
		return false
	}
}

func ValidateFXRate(rate *FXRate, now time.Time) error {
	if rate == nil || rate.TenantID <= 0 || rate.CreatedBy == uuid.Nil ||
		!validFXCurrency(rate.FromCurrency) || !validFXCurrency(rate.ToCurrency) ||
		rate.FromCurrency == rate.ToCurrency || rate.EffectiveAt.IsZero() ||
		rate.EffectiveAt.After(now.Add(time.Minute)) {
		return ErrFXInvalid
	}
	if rate.Rate != strings.TrimSpace(rate.Rate) || !fxRatePattern.MatchString(rate.Rate) {
		return ErrFXInvalid
	}
	value, ok := new(big.Rat).SetString(rate.Rate)
	if !ok || value.Sign() <= 0 || value.Cmp(big.NewRat(1000000, 1)) > 0 {
		return ErrFXInvalid
	}
	rate.Evidence = strings.TrimSpace(rate.Evidence)
	if len([]rune(rate.Evidence)) < 8 || len([]rune(rate.Evidence)) > 250 {
		return ErrFXInvalid
	}
	return nil
}

// convertFXMinor converts minor-unit integers through a rational number,
// rounding halfway away from zero and checking JavaScript-safe boundaries.
func convertFXMinor(amount int64, from, to, quotedRate string) (int64, error) {
	if !validFXCurrency(from) || !validFXCurrency(to) || !jsonSafeInteger(amount) {
		return 0, ErrFXInvalid
	}
	rate, ok := new(big.Rat).SetString(quotedRate)
	if !ok || rate.Sign() <= 0 {
		return 0, ErrFXInvalid
	}
	fromScale := int64(1)
	for i := 0; i < currencyExponent(from); i++ {
		fromScale *= 10
	}
	toScale := int64(1)
	for i := 0; i < currencyExponent(to); i++ {
		toScale *= 10
	}
	value := new(big.Rat).SetInt64(amount)
	value.Mul(value, rate)
	value.Mul(value, big.NewRat(toScale, fromScale))
	numerator := new(big.Int).Abs(value.Num())
	quotient, remainder := new(big.Int).QuoRem(numerator, value.Denom(), new(big.Int))
	if new(big.Int).Mul(remainder, big.NewInt(2)).Cmp(value.Denom()) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	if value.Sign() < 0 {
		quotient.Neg(quotient)
	}
	if !quotient.IsInt64() || !jsonSafeInteger(quotient.Int64()) {
		return 0, ErrFXOverflow
	}
	return quotient.Int64(), nil
}

type FXConverted struct {
	RevenueMinor           *int64 `json:"revenueMinor"`
	ProductCostMinor       *int64 `json:"productCostMinor"`
	FreightMinor           *int64 `json:"freightMinor"`
	RefundMinor            *int64 `json:"refundMinor"`
	PlatformFeeMinor       *int64 `json:"platformFeeMinor"`
	AdvertisingFeeMinor    *int64 `json:"advertisingFeeMinor"`
	WarehouseFeeMinor      *int64 `json:"warehouseFeeMinor"`
	KnownContributionMinor *int64 `json:"knownContributionMinor"`
	EstimatedProfitMinor   *int64 `json:"estimatedProfitMinor"`
}

type FXReportRow struct {
	OrderProfit
	BaseCurrency      string            `json:"baseCurrency"`
	ConversionStatus  string            `json:"conversionStatus"`
	CalculationMethod string            `json:"calculationMethod,omitempty"`
	ComponentRates    []FXComponentRate `json:"componentRates,omitempty"`
	RateID            *uuid.UUID        `json:"rateId,omitempty"`
	Rate              string            `json:"rate,omitempty"`
	RateEffectiveAt   *time.Time        `json:"rateEffectiveAt,omitempty"`
	Converted         FXConverted       `json:"converted"`
}

type FXReport struct {
	BaseCurrency              string        `json:"baseCurrency"`
	CalculatedAt              time.Time     `json:"calculatedAt"`
	FormulaVersion            string        `json:"formulaVersion"`
	TotalOrders               int           `json:"totalOrders"`
	IncludedOrders            int           `json:"includedOrders"`
	ExcludedOrders            int           `json:"excludedOrders"`
	RevenueTotalMinor         int64         `json:"revenueTotalMinor"`
	EstimatedProfitTotalMinor int64         `json:"estimatedProfitTotalMinor"`
	Rows                      []FXReportRow `json:"rows"`
}

func (s *Service) ListFXRates(ctx context.Context, tenantID int64, base string) ([]FXRate, error) {
	if s == nil || s.DB == nil || tenantID <= 0 || !validFXCurrency(base) {
		return nil, ErrFXInvalid
	}
	rows := make([]FXRate, 0)
	err := s.DB.WithContext(ctx).Where("tenant_id = ? AND to_currency = ?", tenantID, base).
		Order("effective_at DESC, created_at DESC, id DESC").Limit(100).Find(&rows).Error
	return rows, err
}

// BuildFXReport keeps the existing V6 calculation unchanged and uses the
// existing tenant/store visibility rules and 5000-row export limit.
func (s *Service) BuildFXReport(ctx context.Context, tenantID int64, scope Scope, query ListQuery, baseCurrency string) (*FXReport, error) {
	if s == nil || s.DB == nil || tenantID <= 0 || !validFXCurrency(baseCurrency) {
		return nil, ErrFXInvalid
	}
	query.Export = true
	query.Page = 1
	query.PageSize = maxPageSize
	original, err := s.List(ctx, tenantID, scope, query)
	if err != nil {
		return nil, err
	}
	report := &FXReport{
		BaseCurrency: baseCurrency, CalculatedAt: original.CalculatedAt,
		FormulaVersion: FormulaVersion, TotalOrders: len(original.List),
		Rows: make([]FXReportRow, 0, len(original.List)),
	}
	var rates []FXRate
	if err := s.DB.WithContext(ctx).
		Where("tenant_id = ? AND to_currency = ? AND effective_at <= ? AND created_at <= ?",
			tenantID, baseCurrency, report.CalculatedAt, report.CalculatedAt).
		Order("effective_at ASC, created_at ASC, id ASC").
		Limit(maxFXRateRows + 1).Find(&rates).Error; err != nil {
		return nil, err
	}
	if len(rates) > maxFXRateRows {
		return nil, ErrFXTooMany
	}
	bySource := make(map[string][]FXRate)
	for _, rate := range rates {
		bySource[rate.FromCurrency] = append(bySource[rate.FromCurrency], rate)
	}
	mixedFees, err := s.loadFXMixedFees(ctx, tenantID, original.List)
	if err != nil {
		return nil, err
	}
	sourceFacts, err := s.loadFXSourceFacts(ctx, tenantID, original.List)
	if err != nil {
		return nil, err
	}
	for _, originalRow := range original.List {
		row := FXReportRow{OrderProfit: originalRow, BaseCurrency: baseCurrency}
		quote := "1"
		if originalRow.Currency != baseCurrency {
			if originalRow.OrderedAt == nil {
				row.ConversionStatus = "order_date_missing"
				report.ExcludedOrders++
				report.Rows = append(report.Rows, row)
				continue
			}
			candidates := bySource[originalRow.Currency]
			index := sort.Search(len(candidates), func(i int) bool {
				return candidates[i].EffectiveAt.After(*originalRow.OrderedAt)
			})
			if index == 0 {
				row.ConversionStatus = "rate_missing"
				report.ExcludedOrders++
				report.Rows = append(report.Rows, row)
				continue
			}
			selected := candidates[index-1]
			quote = selected.Rate
			id, effectiveAt := selected.ID, selected.EffectiveAt
			row.RateID, row.RateEffectiveAt = &id, &effectiveAt
		}
		row.Rate = quote
		var conversionError error
		convert := func(value *int64) *int64 {
			if value == nil || conversionError != nil {
				return nil
			}
			minor, err := convertFXMinor(*value, originalRow.Currency, baseCurrency, quote)
			if err != nil {
				conversionError = err
				return nil
			}
			return &minor
		}
		c := &row.Converted
		c.RevenueMinor = convert(originalRow.Components.Revenue.AmountMinor)
		c.ProductCostMinor = convert(originalRow.Components.ProductCost.AmountMinor)
		c.FreightMinor = convert(originalRow.Components.Freight.AmountMinor)
		c.RefundMinor = convert(originalRow.Components.Refund.AmountMinor)
		c.PlatformFeeMinor = convert(originalRow.Components.PlatformFee.AmountMinor)
		c.AdvertisingFeeMinor = convert(originalRow.Components.Advertising.AmountMinor)
		c.WarehouseFeeMinor = convert(originalRow.Components.WarehouseFee.AmountMinor)
		c.KnownContributionMinor = convert(originalRow.KnownContributionMinor)
		c.EstimatedProfitMinor = convert(originalRow.EstimatedProfitMinor)
		if conversionError != nil {
			return nil, conversionError
		}
		missingSourceRate, err := resolveFXSourceFacts(&row, originalRow, sourceFacts, bySource)
		if err != nil {
			return nil, err
		}
		missingFeeRate, err := resolveFXMixedFees(&row, originalRow, mixedFees, bySource)
		if err != nil {
			return nil, err
		}
		missingComponentRate := missingSourceRate || missingFeeRate
		if (originalRow.Status != StatusComplete && row.CalculationMethod == "") ||
			c.EstimatedProfitMinor == nil || c.RevenueMinor == nil {
			if missingComponentRate {
				row.ConversionStatus = "rate_missing"
			} else {
				row.ConversionStatus = "source_incomplete"
			}
			report.ExcludedOrders++
		} else {
			revenue, revenueOK := safeAdd(report.RevenueTotalMinor, *c.RevenueMinor)
			profit, profitOK := safeAdd(report.EstimatedProfitTotalMinor, *c.EstimatedProfitMinor)
			if !revenueOK || !profitOK || !jsonSafeInteger(revenue) || !jsonSafeInteger(profit) {
				return nil, ErrFXOverflow
			}
			report.RevenueTotalMinor = revenue
			report.EstimatedProfitTotalMinor = profit
			row.ConversionStatus = "complete"
			if row.CalculationMethod == "" {
				row.CalculationMethod = "whole_order_conversion"
			}
			report.IncludedOrders++
		}
		report.Rows = append(report.Rows, row)
	}
	return report, nil
}

// An application-level guard complements the append-only API contract.
func (rate *FXRate) BeforeUpdate(_ *gorm.DB) error {
	return ErrFXInvalid
}

func (rate *FXRate) BeforeDelete(_ *gorm.DB) error {
	return ErrFXInvalid
}
