package profitability

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
)

// fxSourceFacts contains only tenant-filtered facts for orders already visible
// under the caller's original shop scope.
type fxSourceFacts struct {
	items     map[uuid.UUID][]OrderItemFact
	snapshots map[uuid.UUID][]OrderItemCostSnapshotFact
	suppliers map[uuid.UUID][]SupplierCostFact
	freight   map[uuid.UUID][]FreightFact
	carrier   map[uuid.UUID]FreightFeeFact
	refunds   map[uuid.UUID][]RefundFact
}

func (s *Service) loadFXSourceFacts(ctx context.Context, tenantID int64, orders []OrderProfit) (*fxSourceFacts, error) {
	facts := &fxSourceFacts{
		items:     make(map[uuid.UUID][]OrderItemFact),
		snapshots: make(map[uuid.UUID][]OrderItemCostSnapshotFact),
		suppliers: make(map[uuid.UUID][]SupplierCostFact),
		freight:   make(map[uuid.UUID][]FreightFact),
		refunds:   make(map[uuid.UUID][]RefundFact),
	}
	var costIDs, freightIDs, refundIDs []uuid.UUID
	for _, row := range orders {
		if row.Status != StatusMismatch {
			continue
		}
		if row.Components.ProductCost.Status == ComponentMismatch &&
			row.Components.ProductCost.ReasonCode == "supplier_cost_currency_mismatch" {
			costIDs = append(costIDs, row.OrderID)
		}
		if row.Components.Freight.Status == ComponentMismatch &&
			row.Components.Freight.ReasonCode == "freight_currency_mismatch" {
			freightIDs = append(freightIDs, row.OrderID)
		}
		if row.Components.Refund.Status == ComponentMismatch &&
			row.Components.Refund.ReasonCode == "sales_return_currency_mismatch" {
			refundIDs = append(refundIDs, row.OrderID)
		}
	}
	repository := s.repository()
	if len(costIDs) > 0 {
		items, err := repository.ListOrderItems(ctx, tenantID, costIDs)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			facts.items[item.OrderID] = append(facts.items[item.OrderID], item)
		}
		snapshots, err := repository.ListOrderItemCostSnapshots(ctx, tenantID, costIDs)
		if err != nil {
			return nil, err
		}
		for _, snapshot := range snapshots {
			facts.snapshots[snapshot.OrderItemID] = append(facts.snapshots[snapshot.OrderItemID], snapshot)
		}
		skuSet := make(map[uuid.UUID]struct{})
		for _, item := range items {
			if item.ProductSKUID != nil && *item.ProductSKUID != uuid.Nil {
				skuSet[*item.ProductSKUID] = struct{}{}
			}
		}
		skus := make([]uuid.UUID, 0, len(skuSet))
		for sku := range skuSet {
			skus = append(skus, sku)
		}
		suppliers, err := repository.ListSupplierCosts(ctx, tenantID, skus)
		if err != nil {
			return nil, err
		}
		for _, supplier := range suppliers {
			facts.suppliers[supplier.ProductSKUID] = append(facts.suppliers[supplier.ProductSKUID], supplier)
		}
	}
	if len(freightIDs) > 0 {
		quotes, err := repository.ListFreightFacts(ctx, tenantID, freightIDs)
		if err != nil {
			return nil, err
		}
		for _, quote := range quotes {
			facts.freight[quote.OrderID] = append(facts.freight[quote.OrderID], quote)
		}
		if s.FreightFees != nil {
			facts.carrier, err = s.FreightFees.ListFreightFees(ctx, tenantID, freightIDs)
			if err != nil {
				return nil, err
			}
		}
	}
	if len(refundIDs) > 0 {
		refunds, err := repository.ListRefundFacts(ctx, tenantID, refundIDs)
		if err != nil {
			return nil, err
		}
		for _, refund := range refunds {
			facts.refunds[refund.OrderID] = append(facts.refunds[refund.OrderID], refund)
		}
	}
	return facts, nil
}

func fxRawConverted(amount int64, currency, source, sourceID, name string, orderDate *time.Time, base string, rates map[string][]FXRate) (*int64, *FXComponentRate, bool, error) {
	value, evidence, missing, err := fxMixedRate(amount, currency, base, name, orderDate, rates)
	if evidence != nil {
		evidence.SourceType = source
		evidence.SourceFactID = sourceID
	}
	return value, evidence, missing, err
}

func fxSourceCost(row OrderProfit, base string, facts *fxSourceFacts, rates map[string][]FXRate) (*int64, []FXComponentRate, bool, error) {
	if facts == nil || row.OrderedAt == nil {
		return nil, nil, false, nil
	}
	items := facts.items[row.OrderID]
	if len(items) == 0 {
		return nil, nil, false, nil
	}
	snapshotted := usesFulfillmentCostSnapshot(OrderFact{Status: row.OrderStatus, FulfillmentStatus: row.FulfillmentStatus})
	var total int64
	evidence := make([]FXComponentRate, 0, len(items))
	seen := make(map[uuid.UUID]struct{}, len(items))
	for _, item := range items {
		if item.ID == uuid.Nil || item.OrderID != row.OrderID || item.Quantity < 1 ||
			item.ProductSKUID == nil || *item.ProductSKUID == uuid.Nil {
			return nil, nil, false, nil
		}
		if _, exists := seen[item.ID]; exists {
			return nil, nil, false, nil
		}
		seen[item.ID] = struct{}{}
		var minor int64
		var currency, source, sourceID string
		if snapshotted {
			matches := facts.snapshots[item.ID]
			if len(matches) != 1 {
				return nil, nil, false, nil
			}
			fact := matches[0]
			if fact.OrderID != row.OrderID || fact.OrderItemID != item.ID ||
				fact.ProductSKUID != *item.ProductSKUID || fact.Quantity != item.Quantity ||
				strings.ToUpper(strings.TrimSpace(fact.OrderCurrency)) != row.Currency ||
				fact.CapturedAt.IsZero() || fact.UnitCostMinor == nil || fact.LineCostMinor == nil ||
				*fact.UnitCostMinor < 0 || !jsonSafeInteger(*fact.UnitCostMinor) ||
				*fact.LineCostMinor < 0 || !jsonSafeInteger(*fact.LineCostMinor) ||
				*fact.UnitCostMinor > math.MaxInt64/int64(item.Quantity) ||
				*fact.UnitCostMinor*int64(item.Quantity) != *fact.LineCostMinor {
				return nil, nil, false, nil
			}
			currency = strings.ToUpper(strings.TrimSpace(fact.CostCurrency))
			if !((fact.ResolutionStatus == "resolved" && currency == row.Currency) ||
				(fact.ResolutionStatus == "currency_mismatch" && currency != row.Currency &&
					fact.ReasonCode == "supplier_cost_currency_mismatch")) {
				return nil, nil, false, nil
			}
			minor, source, sourceID = *fact.LineCostMinor, "fulfillment_cost_snapshot", fact.ID.String()
		} else {
			bindings := facts.suppliers[*item.ProductSKUID]
			if len(bindings) != 1 {
				return nil, nil, false, nil
			}
			binding := bindings[0]
			if binding.ProductSKUID != *item.ProductSKUID || binding.SupplierSKUID == uuid.Nil ||
				binding.UnitCostMinor < 0 || !jsonSafeInteger(binding.UnitCostMinor) ||
				binding.UnitCostMinor > math.MaxInt64/int64(item.Quantity) ||
				binding.UpdatedAt.IsZero() || binding.UpdatedAt.After(*row.OrderedAt) {
				// A catalog price updated after the order is not historical evidence.
				return nil, nil, false, nil
			}
			minor = binding.UnitCostMinor * int64(item.Quantity)
			if !jsonSafeInteger(minor) {
				return nil, nil, false, nil
			}
			currency, source, sourceID = strings.ToUpper(strings.TrimSpace(binding.Currency)),
				"supplier_catalog_estimate", binding.SupplierSKUID.String()
		}
		converted, proof, missing, err := fxRawConverted(minor, currency, source, sourceID, "productCost", row.OrderedAt, base, rates)
		if err != nil || missing || converted == nil || proof == nil {
			return nil, nil, missing, err
		}
		next, ok := safeAdd(total, *converted)
		if !ok || !jsonSafeInteger(next) {
			return nil, nil, false, ErrFXOverflow
		}
		total = next
		evidence = append(evidence, *proof)
	}
	return &total, evidence, false, nil
}

func fxSourceFreight(row OrderProfit, base string, facts *fxSourceFacts, rates map[string][]FXRate) (*int64, *FXComponentRate, bool, error) {
	if facts == nil || row.OrderedAt == nil {
		return nil, nil, false, nil
	}
	if carrier, ok := facts.carrier[row.OrderID]; ok && carrier.Status != "" && carrier.Status != "missing" {
		if carrier.OrderID != row.OrderID || carrier.Status != "confirmed" ||
			carrier.ShipmentCount < 1 || carrier.BilledShipmentCount != carrier.ShipmentCount ||
			len(carrier.ChargeIDs) != carrier.BilledShipmentCount ||
			carrier.AmountMinor < 0 || !jsonSafeInteger(carrier.AmountMinor) {
			return nil, nil, false, nil
		}
		seen := make(map[uuid.UUID]struct{})
		ids := make([]string, 0, len(carrier.ChargeIDs))
		for _, id := range carrier.ChargeIDs {
			if id == uuid.Nil {
				return nil, nil, false, nil
			}
			if _, duplicate := seen[id]; duplicate {
				return nil, nil, false, nil
			}
			seen[id] = struct{}{}
			ids = append(ids, id.String())
		}
		return fxRawConverted(carrier.AmountMinor, carrier.Currency, "carrier_freight_ledger",
			strings.Join(ids, ","), "freight", row.OrderedAt, base, rates)
	}
	var latest FreightFact
	haveWave := false
	for _, quote := range facts.freight[row.OrderID] {
		if quote.OrderID != row.OrderID || quote.WaveID == uuid.Nil || quote.ID == uuid.Nil {
			return nil, nil, false, nil
		}
		if haveWave && quote.WaveID != latest.WaveID {
			return nil, nil, false, nil
		}
		if !haveWave || quote.Version > latest.Version ||
			(quote.Version == latest.Version && quote.CreatedAt.After(latest.CreatedAt)) {
			latest = quote
		}
		haveWave = true
	}
	if !haveWave || latest.AmountMinor < 0 || !jsonSafeInteger(latest.AmountMinor) ||
		latest.CreatedAt.IsZero() {
		return nil, nil, false, nil
	}
	return fxRawConverted(latest.AmountMinor, latest.Currency, "confirmed_local_freight_quote",
		latest.ID.String(), "freight", row.OrderedAt, base, rates)
}

func fxSourceRefunds(row OrderProfit, base string, facts *fxSourceFacts, rates map[string][]FXRate) (*int64, []FXComponentRate, bool, error) {
	if facts == nil || row.OrderedAt == nil {
		return nil, nil, false, nil
	}
	refunds := facts.refunds[row.OrderID]
	if len(refunds) == 0 {
		return nil, nil, false, nil
	}
	var total int64
	seenReturns := make(map[uuid.UUID]struct{})
	seenExecutions := make(map[uuid.UUID]struct{})
	evidence := make([]FXComponentRate, 0, len(refunds))
	positive := false
	for _, refund := range refunds {
		if refund.OrderID != row.OrderID || refund.SalesReturnID == uuid.Nil {
			return nil, nil, false, nil
		}
		if _, exists := seenReturns[refund.SalesReturnID]; exists {
			return nil, nil, false, nil
		}
		seenReturns[refund.SalesReturnID] = struct{}{}
		if refund.SalesReturnAmount <= 0 {
			continue
		}
		positive = true
		if !jsonSafeInteger(refund.SalesReturnAmount) || refund.ExecutionID == nil ||
			*refund.ExecutionID == uuid.Nil || refund.ExecutionStatus != "succeeded" ||
			refund.ExecutionAmount != refund.SalesReturnAmount ||
			!jsonSafeInteger(refund.ExecutionAmount) || refund.ExecutionUpdatedAt == nil ||
			strings.ToUpper(strings.TrimSpace(refund.ExecutionCurrency)) !=
				strings.ToUpper(strings.TrimSpace(refund.SalesReturnCurrency)) {
			return nil, nil, false, nil
		}
		if _, exists := seenExecutions[*refund.ExecutionID]; exists {
			return nil, nil, false, nil
		}
		seenExecutions[*refund.ExecutionID] = struct{}{}
		converted, proof, missing, err := fxRawConverted(refund.ExecutionAmount, refund.ExecutionCurrency,
			"refund_execution_ledger", refund.ExecutionID.String(), "refund", row.OrderedAt, base, rates)
		if err != nil || missing || converted == nil || proof == nil {
			return nil, nil, missing, err
		}
		next, ok := safeAdd(total, *converted)
		if !ok || !jsonSafeInteger(next) {
			return nil, nil, false, ErrFXOverflow
		}
		total = next
		evidence = append(evidence, *proof)
	}
	if !positive {
		return nil, nil, false, nil
	}
	return &total, evidence, false, nil
}

// The only state mutated is the read-only converted report row. The V6 result
// and its missing/mismatch statuses remain authoritative and unchanged.
func resolveFXSourceFacts(row *FXReportRow, raw OrderProfit, facts *fxSourceFacts, rates map[string][]FXRate) (bool, error) {
	if row == nil || facts == nil || raw.Status != StatusMismatch {
		return false, nil
	}
	missingRate := false
	if raw.Components.ProductCost.Status == ComponentMismatch &&
		raw.Components.ProductCost.ReasonCode == "supplier_cost_currency_mismatch" {
		amount, evidence, missing, err := fxSourceCost(raw, row.BaseCurrency, facts, rates)
		if err != nil {
			return false, err
		}
		missingRate = missingRate || missing
		if amount != nil {
			row.Converted.ProductCostMinor = amount
			row.ComponentRates = append(row.ComponentRates, evidence...)
		}
	}
	if raw.Components.Freight.Status == ComponentMismatch &&
		raw.Components.Freight.ReasonCode == "freight_currency_mismatch" {
		amount, evidence, missing, err := fxSourceFreight(raw, row.BaseCurrency, facts, rates)
		if err != nil {
			return false, err
		}
		missingRate = missingRate || missing
		if amount != nil && evidence != nil {
			row.Converted.FreightMinor = amount
			row.ComponentRates = append(row.ComponentRates, *evidence)
		}
	}
	if raw.Components.Refund.Status == ComponentMismatch &&
		raw.Components.Refund.ReasonCode == "sales_return_currency_mismatch" {
		amount, evidence, missing, err := fxSourceRefunds(raw, row.BaseCurrency, facts, rates)
		if err != nil {
			return false, err
		}
		missingRate = missingRate || missing
		if amount != nil {
			row.Converted.RefundMinor = amount
			row.ComponentRates = append(row.ComponentRates, evidence...)
		}
	}
	return missingRate, nil
}
