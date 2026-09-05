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
	DB    *gorm.DB
	Repo  Repository
	Clock func() time.Time
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
	skuSet := make(map[uuid.UUID]struct{})
	itemsByOrder := make(map[uuid.UUID][]OrderItemFact)
	for _, item := range items {
		itemsByOrder[item.OrderID] = append(itemsByOrder[item.OrderID], item)
		if item.ProductSKUID != nil && *item.ProductSKUID != uuid.Nil {
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
	costsBySKU := make(map[uuid.UUID][]SupplierCostFact)
	for _, row := range costs {
		costsBySKU[row.ProductSKUID] = append(costsBySKU[row.ProductSKUID], row)
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
		profit, lines := calculateOrder(order, itemsByOrder[order.ID], costsBySKU, freightByOrder[order.ID], refundByOrder[order.ID], calculatedAt)
		profits = append(profits, profit)
		linesByOrder[order.ID] = lines
	}
	return profits, linesByOrder, nil
}

func calculateOrder(order OrderFact, items []OrderItemFact, costsBySKU map[uuid.UUID][]SupplierCostFact, freight []FreightFact, refunds []RefundFact, calculatedAt time.Time) (OrderProfit, []ProductCostLine) {
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

	productCost, costLines, costIssues := calculateProductCost(currency, items, costsBySKU)
	issues = append(issues, costIssues...)
	freightComponent, freightWaveID, freightIssues := calculateFreight(currency, freight)
	issues = append(issues, freightIssues...)
	refundComponent, refundIDs, refundIssues := calculateRefunds(currency, refunds)
	issues = append(issues, refundIssues...)
	platformFee := missingFee(currency, "platform_settlement", "platform_fee_missing", "缺少平台结算费用")
	advertisingFee := missingFee(currency, "advertising_ledger", "advertising_fee_missing", "缺少订单归属广告费用")
	warehouseFee := missingFee(currency, "warehouse_fee_ledger", "warehouse_fee_missing", "缺少仓储作业费用")
	issues = append(issues,
		Issue{Code: platformFee.ReasonCode, Component: "platformFee", Message: platformFee.Reason},
		Issue{Code: advertisingFee.ReasonCode, Component: "advertisingFee", Message: advertisingFee.Reason},
		Issue{Code: warehouseFee.ReasonCode, Component: "warehouseFee", Message: warehouseFee.Reason},
	)

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
	return OrderProfit{
		OrderID: order.ID, OrderNo: order.OrderNo, Platform: order.Platform, ShopID: order.ShopID, ShopName: order.ShopName,
		WarehouseID: order.WarehouseID, WarehouseCode: order.WarehouseCode, WarehouseName: order.WarehouseName,
		Currency: currency, Status: status, OrderStatus: order.Status, PaymentStatus: order.PaymentStatus,
		FulfillmentStatus: order.FulfillmentStatus, KnownContributionMinor: knownContribution,
		EstimatedProfitMinor: estimatedProfit, EstimatedMarginBps: marginBps, Components: components,
		Issues: issues, Related: RelatedFacts{FulfillmentWaveID: freightWaveID, SupplierIDs: supplierIDs, RefundExecutionIDs: refundIDs},
		OrderedAt: order.OrderedAt, CalculatedAt: calculatedAt, FormulaVersion: FormulaVersion,
	}, costLines
}

func component(currency, status, source string, sourceAt *time.Time) MoneyComponent {
	return MoneyComponent{Currency: currency, Status: status, Source: source, SourceAt: sourceAt}
}

func missingFee(currency, source, code, reason string) MoneyComponent {
	row := component(currency, ComponentMissing, source, nil)
	row.ReasonCode, row.Reason = code, reason
	return row
}

func calculateProductCost(currency string, items []OrderItemFact, costs map[uuid.UUID][]SupplierCostFact) (MoneyComponent, []ProductCostLine, []Issue) {
	result := component(currency, ComponentAvailable, "current_supplier_catalog", nil)
	lines := make([]ProductCostLine, 0, len(items))
	issues := make([]Issue, 0)
	var known int64
	if len(items) == 0 {
		result.Status, result.ReasonCode, result.Reason = ComponentMissing, "order_items_missing", "订单缺少可计算商品成本的明细"
		return result, lines, []Issue{{Code: result.ReasonCode, Component: "productCost", Message: result.Reason}}
	}
	for _, item := range items {
		line := ProductCostLine{OrderItemID: item.ID, ProductSKUID: item.ProductSKUID, ProductTitle: item.ProductTitle, SKUCode: item.SKUCode, Quantity: item.Quantity, Currency: currency, Status: ComponentAvailable, Source: "current_supplier_catalog"}
		switch {
		case item.Quantity < 1:
			line.Status, line.ReasonCode, line.Reason = ComponentBlocked, "item_quantity_invalid", "订单明细数量无效"
		case item.ProductSKUID == nil || *item.ProductSKUID == uuid.Nil:
			line.Status, line.ReasonCode, line.Reason = ComponentMissing, "product_sku_unbound", "订单明细尚未绑定内部规格"
		default:
			bindings := costs[*item.ProductSKUID]
			switch len(bindings) {
			case 0:
				line.Status, line.ReasonCode, line.Reason = ComponentMissing, "supplier_cost_missing", "规格缺少有效供应商采购价"
			case 1:
				binding := bindings[0]
				line.SupplierID, line.SupplierName, line.SourceAt = &binding.SupplierID, binding.SupplierName, &binding.UpdatedAt
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
