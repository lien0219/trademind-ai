package profitability

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrInvalidQuery = errors.New("invalid order profit query")
	ErrTooManyRows  = errors.New("order profit result exceeds 5000 rows; narrow the filters")
)

const maxJSONSafeInteger int64 = 9007199254740991

type Service struct {
	DB            *gorm.DB
	Repo          Repository
	PlatformFees  PlatformFeeReader
	WarehouseFees WarehouseFeeReader
	Clock         func() time.Time
}

func (s *Service) repository() Repository {
	if s.Repo != nil {
		return s.Repo
	}
	return &GormRepository{DB: s.DB}
}

func (s *Service) now() time.Time {
	if s.Clock != nil {
		return s.Clock().UTC()
	}
	return time.Now().UTC()
}

func normalizeQuery(q ListQuery) (ListQuery, error) {
	q.OrderNo = strings.TrimSpace(q.OrderNo)
	q.Platform = strings.TrimSpace(strings.ToLower(q.Platform))
	q.Currency = strings.TrimSpace(strings.ToUpper(q.Currency))
	q.Status = strings.TrimSpace(strings.ToLower(q.Status))
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 {
		q.PageSize = 20
	}
	if q.PageSize > maxPageSize {
		return q, ErrInvalidQuery
	}
	if q.Status != "" && q.Status != StatusComplete && q.Status != StatusPending && q.Status != StatusMismatch && q.Status != StatusBlocked {
		return q, ErrInvalidQuery
	}
	if q.Currency != "" && !validCurrency(q.Currency) {
		return q, ErrInvalidQuery
	}
	if q.Start != nil && q.End != nil && q.Start.After(*q.End) {
		return q, ErrInvalidQuery
	}
	return q, nil
}

func (s *Service) List(ctx context.Context, tenantID int64, scope Scope, q ListQuery) (*ListResult, error) {
	if tenantID < 0 || (s.DB == nil && s.Repo == nil) {
		return nil, fmt.Errorf("profitability service unavailable")
	}
	q, err := normalizeQuery(q)
	if err != nil {
		return nil, err
	}
	calculatedAt := s.now()
	loadDerivedSet := q.Export || q.Status != ""
	offset := (q.Page - 1) * q.PageSize
	limit := q.PageSize
	if loadDerivedSet {
		offset = 0
		limit = maxExportRows + 1
	}
	orders, rawTotal, err := s.repository().ListOrders(ctx, tenantID, scope, q, offset, limit)
	if err != nil {
		return nil, err
	}
	if loadDerivedSet && rawTotal > maxExportRows {
		return nil, ErrTooManyRows
	}
	profits, _, err := s.calculate(ctx, tenantID, orders, calculatedAt)
	if err != nil {
		return nil, err
	}
	total := rawTotal
	if q.Status != "" {
		filtered := make([]OrderProfit, 0, len(profits))
		for _, row := range profits {
			if row.Status == q.Status {
				filtered = append(filtered, row)
			}
		}
		total = int64(len(filtered))
		if !q.Export {
			start := (q.Page - 1) * q.PageSize
			if start > len(filtered) {
				start = len(filtered)
			}
			end := start + q.PageSize
			if end > len(filtered) {
				end = len(filtered)
			}
			filtered = filtered[start:end]
		}
		profits = filtered
	}
	if q.Export {
		q.Page = 1
		q.PageSize = maxExportRows
	}
	return &ListResult{
		List: profits, Page: q.Page, PageSize: q.PageSize, Total: total,
		TotalPages: pagesOf(total, q.PageSize), CalculatedAt: calculatedAt, Formula: formulaDescriptor(),
	}, nil
}

func (s *Service) Get(ctx context.Context, tenantID int64, scope Scope, orderID uuid.UUID) (*Detail, error) {
	if tenantID < 0 || orderID == uuid.Nil || (s.DB == nil && s.Repo == nil) {
		return nil, ErrInvalidQuery
	}
	order, err := s.repository().GetOrder(ctx, tenantID, scope, orderID)
	if err != nil {
		return nil, err
	}
	profits, lines, err := s.calculate(ctx, tenantID, []OrderFact{*order}, s.now())
	if err != nil {
		return nil, err
	}
	if len(profits) != 1 {
		return nil, gorm.ErrRecordNotFound
	}
	return &Detail{OrderProfit: profits[0], ProductCostLines: lines[orderID]}, nil
}

func (s *Service) calculate(ctx context.Context, tenantID int64, orders []OrderFact, calculatedAt time.Time) ([]OrderProfit, map[uuid.UUID][]ProductCostLine, error) {
	if len(orders) == 0 {
		return []OrderProfit{}, map[uuid.UUID][]ProductCostLine{}, nil
	}
	orderIDs := make([]uuid.UUID, 0, len(orders))
	for _, order := range orders {
		orderIDs = append(orderIDs, order.ID)
	}
	repo := s.repository()
	items, err := repo.ListOrderItems(ctx, tenantID, orderIDs)
	if err != nil {
		return nil, nil, err
	}
	snapshots, err := repo.ListOrderItemCostSnapshots(ctx, tenantID, orderIDs)
	if err != nil {
		return nil, nil, err
	}
	snapshotOrders := make(map[uuid.UUID]bool, len(orders))
	for _, order := range orders {
		snapshotOrders[order.ID] = usesFulfillmentCostSnapshot(order)
	}
	skuSet := make(map[uuid.UUID]struct{})
	itemsByOrder := make(map[uuid.UUID][]OrderItemFact)
	for _, item := range items {
		itemsByOrder[item.OrderID] = append(itemsByOrder[item.OrderID], item)
		if !snapshotOrders[item.OrderID] && item.ProductSKUID != nil && *item.ProductSKUID != uuid.Nil {
			skuSet[*item.ProductSKUID] = struct{}{}
		}
	}
	skuIDs := make([]uuid.UUID, 0, len(skuSet))
	for id := range skuSet {
		skuIDs = append(skuIDs, id)
	}
	sort.Slice(skuIDs, func(i, j int) bool { return skuIDs[i].String() < skuIDs[j].String() })
	costs, err := repo.ListSupplierCosts(ctx, tenantID, skuIDs)
	if err != nil {
		return nil, nil, err
	}
	freight, err := repo.ListFreightFacts(ctx, tenantID, orderIDs)
	if err != nil {
		return nil, nil, err
	}
	refunds, err := repo.ListRefundFacts(ctx, tenantID, orderIDs)
	if err != nil {
		return nil, nil, err
	}
	platformFees := make(map[uuid.UUID]PlatformFeeFact)
	if s.PlatformFees != nil {
		platformFees, err = s.PlatformFees.ListPlatformFees(ctx, tenantID, orderIDs)
		if err != nil {
			return nil, nil, err
		}
	}
	warehouseFees := make(map[uuid.UUID]WarehouseFeeFact)
	if s.WarehouseFees != nil {
		warehouseFees, err = s.WarehouseFees.ListWarehouseFees(ctx, tenantID, orderIDs)
		if err != nil {
			return nil, nil, err
		}
	}
	costsBySKU := make(map[uuid.UUID][]SupplierCostFact)
	for _, row := range costs {
		costsBySKU[row.ProductSKUID] = append(costsBySKU[row.ProductSKUID], row)
	}
	snapshotsByItem := make(map[uuid.UUID][]OrderItemCostSnapshotFact)
	for _, row := range snapshots {
		snapshotsByItem[row.OrderItemID] = append(snapshotsByItem[row.OrderItemID], row)
	}
	freightByOrder := make(map[uuid.UUID][]FreightFact)
	for _, row := range freight {
		freightByOrder[row.OrderID] = append(freightByOrder[row.OrderID], row)
	}
	refundByOrder := make(map[uuid.UUID][]RefundFact)
	for _, row := range refunds {
		refundByOrder[row.OrderID] = append(refundByOrder[row.OrderID], row)
	}
	profits := make([]OrderProfit, 0, len(orders))
	linesByOrder := make(map[uuid.UUID][]ProductCostLine, len(orders))
	for _, order := range orders {
		platformFee, hasPlatformFee := platformFees[order.ID]
		var platformFeePtr *PlatformFeeFact
		if hasPlatformFee {
			platformFeePtr = &platformFee
		}
		warehouseFee, hasWarehouseFee := warehouseFees[order.ID]
		var warehouseFeePtr *WarehouseFeeFact
		if hasWarehouseFee {
			warehouseFeePtr = &warehouseFee
		}
		profit, lines := calculateOrder(order, itemsByOrder[order.ID], costsBySKU, snapshotsByItem, freightByOrder[order.ID], refundByOrder[order.ID], platformFeePtr, warehouseFeePtr, calculatedAt)
		profits = append(profits, profit)
		linesByOrder[order.ID] = lines
	}
	return profits, linesByOrder, nil
}

func calculateOrder(order OrderFact, items []OrderItemFact, costsBySKU map[uuid.UUID][]SupplierCostFact, snapshotsByItem map[uuid.UUID][]OrderItemCostSnapshotFact, freight []FreightFact, refunds []RefundFact, platformFeeFact *PlatformFeeFact, warehouseFeeFact *WarehouseFeeFact, calculatedAt time.Time) (OrderProfit, []ProductCostLine) {
	currency := strings.ToUpper(strings.TrimSpace(order.Currency))
	issues := make([]Issue, 0)
	revenue := component(currency, ComponentAvailable, "order_total", &order.UpdatedAt)
	revenueMinor, err := parseMajorToMinor(order.TotalAmountText, currency)
	if err != nil || revenueMinor < 0 || !jsonSafeInteger(revenueMinor) {
		revenue.Status, revenue.ReasonCode, revenue.Reason = ComponentBlocked, "revenue_invalid", "订单收入无法按币种精确转换为最小货币单位"
		issues = append(issues, Issue{Code: revenue.ReasonCode, Component: "revenue", Message: revenue.Reason})
	} else {
		revenue.AmountMinor = int64Pointer(revenueMinor)
		revenue.KnownAmountMinor = revenueMinor
	}

	productCost, costLines, costIssues := calculateProductCost(order, currency, items, costsBySKU, snapshotsByItem)
	issues = append(issues, costIssues...)
	freightComponent, freightWaveID, freightIssues := calculateFreight(currency, freight)
	issues = append(issues, freightIssues...)
	refundComponent, refundIDs, refundIssues := calculateRefunds(currency, refunds)
	issues = append(issues, refundIssues...)
	platformFee, platformFeeIssues := calculatePlatformFee(currency, platformFeeFact)
	issues = append(issues, platformFeeIssues...)
	advertisingFee := missingFee(currency, "advertising_ledger", "advertising_fee_missing", "缺少订单归属广告费用")
	warehouseFee, warehouseFeeIssues := calculateWarehouseFee(currency, warehouseFeeFact)
	issues = append(issues, Issue{Code: advertisingFee.ReasonCode, Component: "advertisingFee", Message: advertisingFee.Reason})
	issues = append(issues, warehouseFeeIssues...)

	components := ProfitComponents{Revenue: revenue, ProductCost: productCost, Freight: freightComponent, Refund: refundComponent, PlatformFee: platformFee, Advertising: advertisingFee, WarehouseFee: warehouseFee}
	status := statusFromComponents(components)
	var knownContribution *int64
	var estimatedProfit *int64
	var marginBps *int64
	if revenue.AmountMinor != nil {
		known, safe := subtractKnown(*revenue.AmountMinor, productCost.KnownAmountMinor, freightComponent.KnownAmountMinor, refundComponent.KnownAmountMinor, platformFee.KnownAmountMinor, advertisingFee.KnownAmountMinor, warehouseFee.KnownAmountMinor)
		if !safe {
			issues = append(issues, Issue{Code: "profit_overflow", Component: "profit", Message: "预估利润计算超出整数范围"})
			status = StatusBlocked
		} else {
			knownContribution = &known
			if status == StatusComplete {
				estimatedProfit = &known
				if *revenue.AmountMinor > 0 {
					margin, marginSafe := calculateMarginBps(known, *revenue.AmountMinor)
					if marginSafe {
						marginBps = &margin
					} else {
						issues = append(issues, Issue{Code: "profit_margin_overflow", Component: "profit", Message: "预估利润率超出安全展示范围"})
						status, estimatedProfit = StatusBlocked, nil
					}
				}
			}
		}
	}
	supplierIDs := uniqueSupplierIDs(costLines)
	snapshotIDs := uniqueSnapshotIDs(costLines)
	return OrderProfit{
		OrderID: order.ID, OrderNo: order.OrderNo, Platform: order.Platform, ShopID: order.ShopID, ShopName: order.ShopName,
		WarehouseID: order.WarehouseID, WarehouseCode: order.WarehouseCode, WarehouseName: order.WarehouseName,
		Currency: currency, Status: status, OrderStatus: order.Status, PaymentStatus: order.PaymentStatus,
		FulfillmentStatus: order.FulfillmentStatus, KnownContributionMinor: knownContribution,
		EstimatedProfitMinor: estimatedProfit, EstimatedMarginBps: marginBps, Components: components,
		Issues: issues, Related: RelatedFacts{FulfillmentWaveID: freightWaveID, SupplierIDs: supplierIDs, ProductCostSnapshotIDs: snapshotIDs, RefundExecutionIDs: refundIDs,
			SettlementReconciliationID: platformFeeReconciliationID(platformFeeFact), SettlementTransactionIDs: platformFeeTransactionIDs(platformFeeFact),
			WarehouseFeeSnapshotID: warehouseFeeSnapshotID(warehouseFeeFact), WarehouseFeeAdjustmentIDs: warehouseFeeAdjustmentIDs(warehouseFeeFact)},
		OrderedAt: order.OrderedAt, CalculatedAt: calculatedAt, FormulaVersion: FormulaVersion,
	}, costLines
}

func calculateWarehouseFee(currency string, fact *WarehouseFeeFact) (MoneyComponent, []Issue) {
	if fact == nil {
		row := missingFee(currency, "warehouse_fee_ledger", "warehouse_fee_missing", "缺少已确认的仓库操作费")
		return row, []Issue{{Code: row.ReasonCode, Component: "warehouseFee", Message: row.Reason}}
	}
	row := component(currency, ComponentAvailable, "warehouse_fee_ledger", &fact.SourceAt)
	if fact.Status != "confirmed" {
		row.Status, row.ReasonCode, row.Reason = ComponentBlocked, nonEmpty(fact.ReasonCode, "warehouse_fee_blocked"), nonEmpty(fact.Reason, "仓库操作费事实已阻断")
	} else if strings.ToUpper(strings.TrimSpace(fact.Currency)) != currency {
		row.Status, row.ReasonCode, row.Reason = ComponentMismatch, "warehouse_fee_currency_mismatch", "仓库操作费币种与订单币种不一致"
	} else if fact.AmountMinor < 0 || !jsonSafeInteger(fact.AmountMinor) {
		row.Status, row.ReasonCode, row.Reason = ComponentBlocked, "warehouse_fee_amount_invalid", "仓库操作费金额无效"
	} else {
		row.AmountMinor, row.KnownAmountMinor = int64Pointer(fact.AmountMinor), fact.AmountMinor
		return row, nil
	}
	return row, []Issue{{Code: row.ReasonCode, Component: "warehouseFee", Message: row.Reason}}
}

func warehouseFeeSnapshotID(fact *WarehouseFeeFact) *uuid.UUID {
	if fact == nil || fact.SnapshotID == uuid.Nil {
		return nil
	}
	id := fact.SnapshotID
	return &id
}

func warehouseFeeAdjustmentIDs(fact *WarehouseFeeFact) []uuid.UUID {
	if fact == nil || len(fact.AdjustmentIDs) == 0 {
		return []uuid.UUID{}
	}
	return append([]uuid.UUID(nil), fact.AdjustmentIDs...)
}

func calculatePlatformFee(currency string, fact *PlatformFeeFact) (MoneyComponent, []Issue) {
	if fact == nil {
		row := missingFee(currency, "platform_settlement_ledger", "platform_fee_missing", "缺少平台结算费用")
		return row, []Issue{{Code: row.ReasonCode, Component: "platformFee", Message: row.Reason}}
	}
	row := component(currency, ComponentAvailable, "platform_settlement_ledger", &fact.SourceAt)
	switch fact.Status {
	case "matched":
		if strings.ToUpper(strings.TrimSpace(fact.Currency)) != currency {
			row.Status, row.ReasonCode, row.Reason = ComponentMismatch, "platform_fee_currency_mismatch", "平台结算费用币种与订单币种不一致"
		} else if !jsonSafeInteger(fact.AmountMinor) {
			row.Status, row.ReasonCode, row.Reason = ComponentBlocked, "platform_fee_amount_invalid", "平台结算费用超出安全整数范围"
		} else {
			row.AmountMinor, row.KnownAmountMinor = int64Pointer(fact.AmountMinor), fact.AmountMinor
			return row, nil
		}
	case "pending":
		row.Status, row.ReasonCode, row.Reason = ComponentPending, nonEmpty(fact.ReasonCode, "platform_fee_pending"), nonEmpty(fact.Reason, "平台结算费用尚未完成对账")
	case "mismatch":
		row.Status, row.ReasonCode, row.Reason = ComponentMismatch, nonEmpty(fact.ReasonCode, "platform_fee_mismatch"), nonEmpty(fact.Reason, "平台结算费用对账不一致")
	default:
		row.Status, row.ReasonCode, row.Reason = ComponentBlocked, nonEmpty(fact.ReasonCode, "platform_fee_blocked"), nonEmpty(fact.Reason, "平台结算费用对账已阻断")
	}
	return row, []Issue{{Code: row.ReasonCode, Component: "platformFee", Message: row.Reason}}
}

func platformFeeReconciliationID(fact *PlatformFeeFact) *uuid.UUID {
	if fact == nil || fact.ReconciliationID == uuid.Nil {
		return nil
	}
	id := fact.ReconciliationID
	return &id
}

func platformFeeTransactionIDs(fact *PlatformFeeFact) []uuid.UUID {
	if fact == nil || len(fact.SettlementTransactionIDs) == 0 {
		return []uuid.UUID{}
	}
	return append([]uuid.UUID(nil), fact.SettlementTransactionIDs...)
}

func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func component(currency, status, source string, sourceAt *time.Time) MoneyComponent {
	return MoneyComponent{Currency: currency, Status: status, Source: source, SourceAt: sourceAt}
}

func missingFee(currency, source, code, reason string) MoneyComponent {
	row := component(currency, ComponentMissing, source, nil)
	row.ReasonCode, row.Reason = code, reason
	return row
}

func usesFulfillmentCostSnapshot(order OrderFact) bool {
	fulfillmentStatus := strings.ToLower(strings.TrimSpace(order.FulfillmentStatus))
	if fulfillmentStatus == "fulfilled" || fulfillmentStatus == "partial" || fulfillmentStatus == "returned" {
		return true
	}
	orderStatus := strings.ToLower(strings.TrimSpace(order.Status))
	return orderStatus == "shipped" || orderStatus == "delivered"
}

func calculateProductCost(order OrderFact, currency string, items []OrderItemFact, costs map[uuid.UUID][]SupplierCostFact, snapshots map[uuid.UUID][]OrderItemCostSnapshotFact) (MoneyComponent, []ProductCostLine, []Issue) {
	if usesFulfillmentCostSnapshot(order) {
		return calculateSnapshotProductCost(currency, items, snapshots)
	}
	return calculateCatalogProductCost(currency, items, costs)
}

func calculateCatalogProductCost(currency string, items []OrderItemFact, costs map[uuid.UUID][]SupplierCostFact) (MoneyComponent, []ProductCostLine, []Issue) {
	result := component(currency, ComponentAvailable, "current_supplier_catalog_estimate", nil)
	lines := make([]ProductCostLine, 0, len(items))
	issues := make([]Issue, 0)
	var known int64
	if len(items) == 0 {
		result.Status, result.ReasonCode, result.Reason = ComponentMissing, "order_items_missing", "订单缺少可计算商品成本的明细"
		return result, lines, []Issue{{Code: result.ReasonCode, Component: "productCost", Message: result.Reason}}
	}
	for _, item := range items {
		line := ProductCostLine{OrderItemID: item.ID, ProductSKUID: item.ProductSKUID, ProductTitle: item.ProductTitle, SKUCode: item.SKUCode, Quantity: item.Quantity, Currency: currency, Status: ComponentAvailable, Source: "current_supplier_catalog_estimate", CostBasis: "estimate"}
		switch {
		case item.Quantity < 1:
			line.Status, line.ReasonCode, line.Reason = ComponentBlocked, "item_quantity_invalid", "订单明细数量无效"
		case item.ProductSKUID == nil || *item.ProductSKUID == uuid.Nil:
			line.Status, line.ReasonCode, line.Reason = ComponentMissing, "product_sku_unbound", "订单明细尚未绑定内部规格"
		default:
			bindings := costs[*item.ProductSKUID]
			line.CandidateCount = len(bindings)
			switch len(bindings) {
			case 0:
				line.Status, line.ReasonCode, line.Reason = ComponentMissing, "supplier_cost_missing", "规格缺少有效供应商采购价"
			case 1:
				binding := bindings[0]
				line.SupplierID, line.SupplierSKUID, line.SupplierName, line.SourceAt = &binding.SupplierID, &binding.SupplierSKUID, binding.SupplierName, &binding.UpdatedAt
				if strings.ToUpper(strings.TrimSpace(binding.Currency)) != currency {
					line.Status, line.ReasonCode, line.Reason = ComponentMismatch, "supplier_cost_currency_mismatch", "供应商采购价币种与订单币种不一致"
				} else if binding.UnitCostMinor < 0 || !jsonSafeInteger(binding.UnitCostMinor) || (item.Quantity > 0 && binding.UnitCostMinor > math.MaxInt64/int64(item.Quantity)) {
					line.Status, line.ReasonCode, line.Reason = ComponentBlocked, "product_cost_overflow", "商品成本计算超出整数范围"
				} else {
					lineCost := binding.UnitCostMinor * int64(item.Quantity)
					line.UnitCostMinor, line.LineCostMinor = int64Pointer(binding.UnitCostMinor), int64Pointer(lineCost)
					if next, ok := safeAdd(known, lineCost); ok && jsonSafeInteger(lineCost) && jsonSafeInteger(next) {
						known = next
					} else {
						line.Status, line.ReasonCode, line.Reason = ComponentBlocked, "product_cost_overflow", "商品成本合计超出整数范围"
						line.UnitCostMinor, line.LineCostMinor = nil, nil
					}
				}
			default:
				line.Status, line.ReasonCode, line.Reason = ComponentMismatch, "multiple_supplier_costs", "规格存在多个有效供应商采购价，无法自动选择"
			}
		}
		if line.Status != ComponentAvailable {
			issues = append(issues, Issue{Code: line.ReasonCode, Component: "productCost", Message: line.Reason})
		}
		promoteLatestTime(&result.SourceAt, line.SourceAt)
		lines = append(lines, line)
	}
	result.KnownAmountMinor = known
	result.Status = worstComponentStatus(lines)
	if result.Status == ComponentAvailable {
		result.AmountMinor = int64Pointer(known)
	} else {
		result.ReasonCode, result.Reason = summarizeLineIssue(lines)
	}
	return result, lines, issues
}

func calculateSnapshotProductCost(currency string, items []OrderItemFact, snapshots map[uuid.UUID][]OrderItemCostSnapshotFact) (MoneyComponent, []ProductCostLine, []Issue) {
	result := component(currency, ComponentAvailable, "fulfillment_cost_snapshot", nil)
	lines := make([]ProductCostLine, 0, len(items))
	issues := make([]Issue, 0)
	var known int64
	if len(items) == 0 {
		result.Status, result.ReasonCode, result.Reason = ComponentMissing, "order_items_missing", "订单缺少可计算商品成本的明细"
		return result, lines, []Issue{{Code: result.ReasonCode, Component: "productCost", Message: result.Reason}}
	}
	for _, item := range items {
		line := ProductCostLine{
			OrderItemID: item.ID, ProductSKUID: item.ProductSKUID, ProductTitle: item.ProductTitle,
			SKUCode: item.SKUCode, Quantity: item.Quantity, Currency: currency, Status: ComponentMissing,
			Source: "fulfillment_cost_snapshot", CostBasis: "snapshot",
		}
		facts := snapshots[item.ID]
		switch len(facts) {
		case 0:
			line.ReasonCode, line.Reason = "historical_cost_snapshot_missing", "已履约订单缺少当次成本快照，禁止使用当前采购价回填"
		case 1:
			fact := facts[0]
			line.SnapshotID, line.ResolutionStatus = uuidFactPointer(fact.ID), fact.ResolutionStatus
			line.CapturedAt, line.SourceAt = timeFactPointer(fact.CapturedAt), fact.SourceUpdatedAt
			line.SupplierID, line.SupplierSKUID = fact.SupplierID, fact.SupplierSKUID
			line.SupplierName, line.SupplierSKUCode, line.CandidateCount = fact.SupplierName, fact.SupplierSKUCode, fact.CandidateCount
			if strings.TrimSpace(fact.CostCurrency) != "" {
				line.Currency = strings.ToUpper(strings.TrimSpace(fact.CostCurrency))
			}
			line.UnitCostMinor, line.LineCostMinor = fact.UnitCostMinor, fact.LineCostMinor
			promoteLatestTime(&result.SourceAt, line.CapturedAt)
			applySnapshotResolution(&line, item, fact, currency, &known)
			sanitizeSnapshotLineAmounts(&line)
		default:
			line.Status, line.ReasonCode, line.Reason = ComponentMismatch, "duplicate_cost_snapshots", "订单明细存在重复履约成本快照，需要人工核对"
		}
		if line.Status != ComponentAvailable {
			issues = append(issues, Issue{Code: line.ReasonCode, Component: "productCost", Message: line.Reason})
		}
		lines = append(lines, line)
	}
	result.KnownAmountMinor = known
	result.Status = worstComponentStatus(lines)
	if result.Status == ComponentAvailable {
		result.AmountMinor = int64Pointer(known)
	} else {
		result.ReasonCode, result.Reason = summarizeLineIssue(lines)
	}
	return result, lines, issues
}

func sanitizeSnapshotLineAmounts(line *ProductCostLine) {
	if line == nil {
		return
	}
	if line.UnitCostMinor != nil && !jsonSafeInteger(*line.UnitCostMinor) {
		line.UnitCostMinor = nil
	}
	if line.LineCostMinor != nil && !jsonSafeInteger(*line.LineCostMinor) {
		line.LineCostMinor = nil
	}
}

func applySnapshotResolution(line *ProductCostLine, item OrderItemFact, fact OrderItemCostSnapshotFact, currency string, known *int64) {
	if line == nil || known == nil {
		return
	}
	reasonCode := strings.TrimSpace(fact.ReasonCode)
	switch fact.ResolutionStatus {
	case "missing":
		line.Status, line.ReasonCode, line.Reason = ComponentMissing, nonEmpty(reasonCode, "supplier_cost_missing"), "履约时规格缺少有效供应商采购价"
	case "ambiguous":
		line.Status, line.ReasonCode, line.Reason = ComponentMismatch, nonEmpty(reasonCode, "multiple_supplier_costs"), "履约时规格存在多个有效供应商采购价，未自动选择"
	case "currency_mismatch":
		line.Status, line.ReasonCode, line.Reason = ComponentMismatch, nonEmpty(reasonCode, "supplier_cost_currency_mismatch"), "履约成本快照币种与订单币种不一致"
	case "invalid":
		line.Status, line.ReasonCode, line.Reason = ComponentBlocked, nonEmpty(reasonCode, "snapshot_cost_invalid"), "履约成本快照金额无效"
	case "resolved":
		if fact.OrderID != item.OrderID || fact.OrderItemID != item.ID || item.ProductSKUID == nil || *item.ProductSKUID == uuid.Nil || fact.ProductSKUID != *item.ProductSKUID || fact.Quantity != item.Quantity || strings.ToUpper(strings.TrimSpace(fact.OrderCurrency)) != currency {
			line.Status, line.ReasonCode, line.Reason = ComponentMismatch, "cost_snapshot_source_mismatch", "履约成本快照与订单明细不一致"
			return
		}
		if strings.ToUpper(strings.TrimSpace(fact.CostCurrency)) != currency {
			line.Status, line.ReasonCode, line.Reason = ComponentMismatch, "supplier_cost_currency_mismatch", "履约成本快照币种与订单币种不一致"
			return
		}
		if fact.UnitCostMinor == nil || fact.LineCostMinor == nil || *fact.UnitCostMinor < 0 || !jsonSafeInteger(*fact.UnitCostMinor) || fact.Quantity < 1 || *fact.UnitCostMinor > math.MaxInt64/int64(fact.Quantity) || *fact.LineCostMinor != *fact.UnitCostMinor*int64(fact.Quantity) || !jsonSafeInteger(*fact.LineCostMinor) {
			line.Status, line.ReasonCode, line.Reason = ComponentBlocked, "cost_snapshot_amount_invalid", "履约成本快照金额无法安全计算"
			return
		}
		next, ok := safeAdd(*known, *fact.LineCostMinor)
		if !ok || !jsonSafeInteger(next) {
			line.Status, line.ReasonCode, line.Reason = ComponentBlocked, "product_cost_overflow", "商品成本合计超出整数范围"
			return
		}
		*known = next
		line.Status, line.ReasonCode, line.Reason = ComponentAvailable, "", ""
	default:
		line.Status, line.ReasonCode, line.Reason = ComponentBlocked, "cost_snapshot_status_invalid", "履约成本快照状态无效"
	}
}

func promoteLatestTime(current **time.Time, candidate *time.Time) {
	if candidate == nil {
		return
	}
	if *current == nil || candidate.After(**current) {
		value := candidate.UTC()
		*current = &value
	}
}

func uuidFactPointer(value uuid.UUID) *uuid.UUID {
	if value == uuid.Nil {
		return nil
	}
	return &value
}

func timeFactPointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	value = value.UTC()
	return &value
}

func calculateFreight(currency string, rows []FreightFact) (MoneyComponent, *uuid.UUID, []Issue) {
	result := component(currency, ComponentMissing, "confirmed_local_freight_quote", nil)
	result.ReasonCode, result.Reason = "freight_quote_missing", "缺少已确认的本地运费试算"
	if len(rows) == 0 {
		return result, nil, []Issue{{Code: result.ReasonCode, Component: "freight", Message: result.Reason}}
	}
	latestByWave := make(map[uuid.UUID]FreightFact)
	for _, row := range rows {
		current, ok := latestByWave[row.WaveID]
		if !ok || row.Version > current.Version || (row.Version == current.Version && row.CreatedAt.After(current.CreatedAt)) {
			latestByWave[row.WaveID] = row
		}
	}
	if len(latestByWave) != 1 {
		result.Status, result.ReasonCode, result.Reason = ComponentMismatch, "multiple_freight_waves", "订单存在多个未取消波次的已确认运费，需要人工核对"
		return result, nil, []Issue{{Code: result.ReasonCode, Component: "freight", Message: result.Reason}}
	}
	var row FreightFact
	for _, value := range latestByWave {
		row = value
	}
	result.SourceAt = &row.CreatedAt
	if strings.ToUpper(strings.TrimSpace(row.Currency)) != currency {
		result.Status, result.ReasonCode, result.Reason = ComponentMismatch, "freight_currency_mismatch", "已确认运费币种与订单币种不一致"
		return result, &row.WaveID, []Issue{{Code: result.ReasonCode, Component: "freight", Message: result.Reason}}
	}
	if row.AmountMinor < 0 || !jsonSafeInteger(row.AmountMinor) {
		result.Status, result.ReasonCode, result.Reason = ComponentBlocked, "freight_amount_invalid", "已确认运费金额无效"
		return result, &row.WaveID, []Issue{{Code: result.ReasonCode, Component: "freight", Message: result.Reason}}
	}
	result.Status, result.AmountMinor, result.KnownAmountMinor = ComponentAvailable, int64Pointer(row.AmountMinor), row.AmountMinor
	result.ReasonCode, result.Reason = "", ""
	return result, &row.WaveID, nil
}

func calculateRefunds(currency string, rows []RefundFact) (MoneyComponent, []uuid.UUID, []Issue) {
	result := component(currency, ComponentAvailable, "refund_execution_ledger", nil)
	ids := make([]uuid.UUID, 0)
	issues := make([]Issue, 0)
	var known int64
	for _, row := range rows {
		if row.SalesReturnAmount <= 0 {
			continue
		}
		if strings.ToUpper(strings.TrimSpace(row.SalesReturnCurrency)) != currency {
			promoteComponentIssue(&result, ComponentMismatch, "sales_return_currency_mismatch", "已完成售后币种与订单币种不一致")
			continue
		}
		if row.ExecutionID == nil || *row.ExecutionID == uuid.Nil {
			promoteComponentIssue(&result, ComponentPending, "refund_execution_missing", "已完成售后尚未形成退款执行事实")
			continue
		}
		ids = append(ids, *row.ExecutionID)
		if row.ExecutionUpdatedAt != nil && (result.SourceAt == nil || row.ExecutionUpdatedAt.After(*result.SourceAt)) {
			result.SourceAt = row.ExecutionUpdatedAt
		}
		if row.ExecutionStatus != "succeeded" {
			promoteComponentIssue(&result, ComponentPending, "refund_execution_unresolved", "退款执行尚未成功终结")
			continue
		}
		if strings.ToUpper(strings.TrimSpace(row.ExecutionCurrency)) != currency || row.ExecutionAmount != row.SalesReturnAmount {
			promoteComponentIssue(&result, ComponentMismatch, "refund_execution_mismatch", "退款执行金额或币种与售后事实不一致")
			continue
		}
		if row.ExecutionAmount < 0 || !jsonSafeInteger(row.ExecutionAmount) {
			promoteComponentIssue(&result, ComponentBlocked, "refund_amount_invalid", "退款金额无法安全展示")
			continue
		}
		if next, ok := safeAdd(known, row.ExecutionAmount); ok && jsonSafeInteger(next) {
			known = next
		} else {
			promoteComponentIssue(&result, ComponentBlocked, "refund_amount_overflow", "退款金额合计超出整数范围")
		}
	}
	result.KnownAmountMinor = known
	if result.Status == ComponentAvailable {
		result.AmountMinor = int64Pointer(known)
	} else {
		issues = append(issues, Issue{Code: result.ReasonCode, Component: "refund", Message: result.Reason})
	}
	return result, ids, issues
}

func promoteComponentIssue(component *MoneyComponent, status, code, reason string) {
	if componentStatusPriority(status) <= componentStatusPriority(component.Status) {
		return
	}
	component.Status, component.ReasonCode, component.Reason = status, code, reason
}

func componentStatusPriority(status string) int {
	switch status {
	case ComponentBlocked:
		return 3
	case ComponentMismatch:
		return 2
	case ComponentMissing, ComponentPending:
		return 1
	default:
		return 0
	}
}

func statusFromComponents(c ProfitComponents) string {
	statuses := []string{c.Revenue.Status, c.ProductCost.Status, c.Freight.Status, c.Refund.Status, c.PlatformFee.Status, c.Advertising.Status, c.WarehouseFee.Status}
	for _, status := range statuses {
		if status == ComponentBlocked {
			return StatusBlocked
		}
	}
	for _, status := range statuses {
		if status == ComponentMismatch {
			return StatusMismatch
		}
	}
	for _, status := range statuses {
		if status != ComponentAvailable {
			return StatusPending
		}
	}
	return StatusComplete
}

func worstComponentStatus(lines []ProductCostLine) string {
	status := ComponentAvailable
	for _, line := range lines {
		if line.Status == ComponentBlocked {
			return ComponentBlocked
		}
		if line.Status == ComponentMismatch {
			status = ComponentMismatch
		} else if line.Status != ComponentAvailable && status == ComponentAvailable {
			status = ComponentMissing
		}
	}
	return status
}

func summarizeLineIssue(lines []ProductCostLine) (string, string) {
	for _, status := range []string{ComponentBlocked, ComponentMismatch, ComponentMissing} {
		for _, line := range lines {
			if line.Status == status {
				return line.ReasonCode, line.Reason
			}
		}
	}
	return "product_cost_incomplete", "商品成本不完整"
}

func uniqueSupplierIDs(lines []ProductCostLine) []uuid.UUID {
	set := make(map[uuid.UUID]struct{})
	for _, line := range lines {
		if line.SupplierID != nil && *line.SupplierID != uuid.Nil {
			set[*line.SupplierID] = struct{}{}
		}
	}
	out := make([]uuid.UUID, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

func uniqueSnapshotIDs(lines []ProductCostLine) []uuid.UUID {
	set := make(map[uuid.UUID]struct{})
	for _, line := range lines {
		if line.SnapshotID != nil && *line.SnapshotID != uuid.Nil {
			set[*line.SnapshotID] = struct{}{}
		}
	}
	out := make([]uuid.UUID, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

func parseMajorToMinor(raw, currency string) (int64, error) {
	if !validCurrency(currency) {
		return 0, fmt.Errorf("unsupported currency")
	}
	value, ok := new(big.Rat).SetString(strings.TrimSpace(raw))
	if !ok {
		return 0, fmt.Errorf("invalid decimal amount")
	}
	scale := int64(1)
	for i := 0; i < currencyExponent(currency); i++ {
		scale *= 10
	}
	value.Mul(value, big.NewRat(scale, 1))
	if value.Denom().Cmp(big.NewInt(1)) != 0 || !value.Num().IsInt64() {
		return 0, fmt.Errorf("amount precision exceeds currency exponent")
	}
	return value.Num().Int64(), nil
}

func validCurrency(currency string) bool {
	if len(currency) != 3 {
		return false
	}
	for _, char := range currency {
		if char < 'A' || char > 'Z' {
			return false
		}
	}
	return true
}

func currencyExponent(currency string) int {
	switch strings.ToUpper(currency) {
	case "BIF", "CLP", "DJF", "GNF", "JPY", "KMF", "KRW", "MGA", "PYG", "RWF", "UGX", "VND", "VUV", "XAF", "XOF", "XPF":
		return 0
	case "BHD", "IQD", "JOD", "KWD", "LYD", "OMR", "TND":
		return 3
	case "CLF", "UYW":
		return 4
	default:
		return 2
	}
}

func safeAdd(left, right int64) (int64, bool) {
	if (right > 0 && left > math.MaxInt64-right) || (right < 0 && left < math.MinInt64-right) {
		return 0, false
	}
	return left + right, true
}

func subtractKnown(revenue int64, costs ...int64) (int64, bool) {
	result := revenue
	for _, cost := range costs {
		if cost == math.MinInt64 {
			return 0, false
		}
		next, ok := safeAdd(result, -cost)
		if !ok {
			return 0, false
		}
		result = next
	}
	return result, jsonSafeInteger(result)
}

func calculateMarginBps(profit, revenue int64) (int64, bool) {
	if revenue <= 0 || profit > math.MaxInt64/10000 || profit < math.MinInt64/10000 {
		return 0, false
	}
	margin := profit * 10000 / revenue
	return margin, jsonSafeInteger(margin)
}

func int64Pointer(value int64) *int64 { return &value }

func jsonSafeInteger(value int64) bool {
	return value >= -maxJSONSafeInteger && value <= maxJSONSafeInteger
}

func pagesOf(total int64, pageSize int) int {
	if total == 0 || pageSize < 1 {
		return 0
	}
	return int((total + int64(pageSize) - 1) / int64(pageSize))
}
