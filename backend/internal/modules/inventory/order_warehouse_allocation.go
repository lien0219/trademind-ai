package inventory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"github.com/trademind-ai/trademind/backend/internal/modules/warehouse"
	"gorm.io/gorm"
)

const (
	WarehouseAllocationAllocated   = "allocated"
	WarehouseAllocationAllocatable = "allocatable"
	WarehouseAllocationBlocked     = "blocked"
)

var (
	ErrOrderAllocationRevisionConflict = errors.New("warehouse allocation candidate has changed; refresh and retry")
	ErrOrderAllocationBlocked          = errors.New("order is not eligible for warehouse allocation")
	ErrOrderAllocationAlreadyConfirmed = errors.New("order warehouse allocation is already confirmed")
)

// WarehouseAllocationBlock is a stable machine code plus safe operator copy.
type WarehouseAllocationBlock struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WarehouseAllocationCandidateLine explains the stock check for one aggregated SKU.
type WarehouseAllocationCandidateLine struct {
	ProductSKUID      uuid.UUID `json:"productSkuId"`
	SKUCode           string    `json:"skuCode,omitempty"`
	SKUName           string    `json:"skuName,omitempty"`
	Required          int       `json:"required"`
	Available         int       `json:"available"`
	Shortage          int       `json:"shortage"`
	BalanceVersion    int       `json:"-"`
	ProjectionStock   int       `json:"-"`
	WarehouseSellable int       `json:"-"`
}

// WarehouseAllocationCandidate is one single-warehouse fulfillment option.
type WarehouseAllocationCandidate struct {
	WarehouseID   uuid.UUID                          `json:"warehouseId"`
	WarehouseCode string                             `json:"warehouseCode"`
	WarehouseName string                             `json:"warehouseName"`
	IsDefault     bool                               `json:"isDefault"`
	Eligible      bool                               `json:"eligible"`
	ShortageCount int                                `json:"shortageCount"`
	Revision      string                             `json:"revision"`
	Lines         []WarehouseAllocationCandidateLine `json:"lines"`
}

// WarehouseAllocationEvaluation is the fail-closed allocation projection for one order.
type WarehouseAllocationEvaluation struct {
	OrderID                uuid.UUID                      `json:"orderId"`
	Status                 string                         `json:"status"`
	WarehouseID            *uuid.UUID                     `json:"warehouseId,omitempty"`
	WarehouseCode          string                         `json:"warehouseCode,omitempty"`
	WarehouseName          string                         `json:"warehouseName,omitempty"`
	RecommendedWarehouseID *uuid.UUID                     `json:"recommendedWarehouseId,omitempty"`
	CandidateCount         int                            `json:"candidateCount"`
	EligibleCandidateCount int                            `json:"eligibleCandidateCount"`
	Blocks                 []WarehouseAllocationBlock     `json:"blocks"`
	Candidates             []WarehouseAllocationCandidate `json:"candidates"`
}

type allocationSKURow struct {
	ID      uuid.UUID `gorm:"column:id"`
	SKUCode string    `gorm:"column:sku_code"`
	SKUName string    `gorm:"column:sku_name"`
	Stock   *int      `gorm:"column:stock"`
}

type allocationSnapshot struct {
	orders     map[uuid.UUID]orderMirror
	items      map[uuid.UUID][]orderLineMirror
	warehouses []warehouse.Warehouse
	skus       map[uuid.UUID]allocationSKURow
	balances   map[uuid.UUID]map[uuid.UUID]WarehouseStockBalance
	sellable   map[uuid.UUID]int
	effects    map[uuid.UUID][]OrderInventoryEffect
}

func allocationBlock(code, message string) WarehouseAllocationBlock {
	return WarehouseAllocationBlock{Code: code, Message: message}
}

func appendAllocationBlock(blocks []WarehouseAllocationBlock, block WarehouseAllocationBlock) []WarehouseAllocationBlock {
	for _, existing := range blocks {
		if existing.Code == block.Code {
			return blocks
		}
	}
	return append(blocks, block)
}

func loadAllocationSnapshot(ctx context.Context, db *gorm.DB, tenantID int64, orderIDs []uuid.UUID) (*allocationSnapshot, error) {
	if db == nil || tenantID < 0 {
		return nil, fmt.Errorf("warehouse allocation unavailable")
	}
	uniqueOrderIDs := make([]uuid.UUID, 0, len(orderIDs))
	seenOrders := map[uuid.UUID]struct{}{}
	for _, id := range orderIDs {
		if id == uuid.Nil {
			continue
		}
		if _, ok := seenOrders[id]; ok {
			continue
		}
		seenOrders[id] = struct{}{}
		uniqueOrderIDs = append(uniqueOrderIDs, id)
	}
	snapshot := &allocationSnapshot{
		orders: map[uuid.UUID]orderMirror{}, items: map[uuid.UUID][]orderLineMirror{},
		skus: map[uuid.UUID]allocationSKURow{}, balances: map[uuid.UUID]map[uuid.UUID]WarehouseStockBalance{},
		sellable: map[uuid.UUID]int{}, effects: map[uuid.UUID][]OrderInventoryEffect{},
	}
	if len(uniqueOrderIDs) == 0 {
		return snapshot, nil
	}

	var orders []orderMirror
	if err := db.WithContext(ctx).Where("tenant_id = ? AND id IN ? AND deleted_at IS NULL", tenantID, uniqueOrderIDs).
		Order("id ASC").Find(&orders).Error; err != nil {
		return nil, err
	}
	for _, row := range orders {
		snapshot.orders[row.ID] = row
	}

	var items []orderLineMirror
	if err := db.WithContext(ctx).Where("order_id IN ?", uniqueOrderIDs).
		Order("order_id ASC, product_sku_id ASC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	skuSet := map[uuid.UUID]struct{}{}
	for _, item := range items {
		snapshot.items[item.OrderID] = append(snapshot.items[item.OrderID], item)
		if item.ProductSKUID != nil && *item.ProductSKUID != uuid.Nil {
			skuSet[*item.ProductSKUID] = struct{}{}
		}
	}

	if err := db.WithContext(ctx).Where("tenant_id = ? AND status = ?", tenantID, warehouse.StatusActive).
		Order("is_default DESC, code ASC, id ASC").Find(&snapshot.warehouses).Error; err != nil {
		return nil, err
	}

	skuIDs := make([]uuid.UUID, 0, len(skuSet))
	for id := range skuSet {
		skuIDs = append(skuIDs, id)
	}
	sort.Slice(skuIDs, func(i, j int) bool { return skuIDs[i].String() < skuIDs[j].String() })
	if len(skuIDs) > 0 {
		var skuRows []allocationSKURow
		if err := db.WithContext(ctx).Table("product_skus AS sk").
			Select("sk.id, sk.sku_code, sk.sku_name, sk.stock").
			Joins("JOIN products AS p ON p.id = sk.product_id AND p.deleted_at IS NULL").
			Where("sk.id IN ? AND p.tenant_id = ?", skuIDs, tenantID).
			Order("sk.id ASC").Scan(&skuRows).Error; err != nil {
			return nil, err
		}
		for _, row := range skuRows {
			snapshot.skus[row.ID] = row
		}

		var balanceRows []WarehouseStockBalance
		if err := db.WithContext(ctx).Where("tenant_id = ? AND product_sku_id IN ?", tenantID, skuIDs).
			Order("product_sku_id ASC, warehouse_id ASC").Find(&balanceRows).Error; err != nil {
			return nil, err
		}
		for _, row := range balanceRows {
			if snapshot.balances[row.WarehouseID] == nil {
				snapshot.balances[row.WarehouseID] = map[uuid.UUID]WarehouseStockBalance{}
			}
			snapshot.balances[row.WarehouseID][row.ProductSKUID] = row
			sellable := row.OnHand - row.Damaged
			if sellable > 0 {
				snapshot.sellable[row.ProductSKUID] += sellable
			}
		}
	}

	var effects []OrderInventoryEffect
	if err := db.WithContext(ctx).Where(
		"tenant_id = ? AND order_id IN ? AND effect_type IN ? AND status = ?",
		tenantID, uniqueOrderIDs, []string{EffectTypeReserve, EffectTypeDeduct}, InventoryEffectSuccess,
	).Order("order_id ASC, order_item_id ASC, effect_type ASC, id ASC").Find(&effects).Error; err != nil {
		return nil, err
	}
	for _, effect := range effects {
		snapshot.effects[effect.OrderID] = append(snapshot.effects[effect.OrderID], effect)
	}
	return snapshot, nil
}

func aggregateAllocationItems(items []orderLineMirror) (map[uuid.UUID]int, []uuid.UUID, bool) {
	required := map[uuid.UUID]int{}
	valid := len(items) > 0
	for _, item := range items {
		if item.ProductSKUID == nil || *item.ProductSKUID == uuid.Nil || item.Quantity <= 0 {
			valid = false
			continue
		}
		required[*item.ProductSKUID] += item.Quantity
	}
	ids := make([]uuid.UUID, 0, len(required))
	for id := range required {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	return required, ids, valid
}

func allocationCandidateRevision(order orderMirror, wh warehouse.Warehouse, lines []WarehouseAllocationCandidateLine) string {
	payload, _ := json.Marshal(struct {
		OrderID           uuid.UUID                          `json:"orderId"`
		OrderUpdatedAt    string                             `json:"orderUpdatedAt"`
		OrderStatus       string                             `json:"orderStatus"`
		PaymentStatus     string                             `json:"paymentStatus"`
		FulfillmentStatus string                             `json:"fulfillmentStatus"`
		AssignedWarehouse *uuid.UUID                         `json:"assignedWarehouse,omitempty"`
		WarehouseID       uuid.UUID                          `json:"warehouseId"`
		WarehouseCode     string                             `json:"warehouseCode"`
		WarehouseDefault  bool                               `json:"warehouseDefault"`
		Lines             []WarehouseAllocationCandidateLine `json:"lines"`
	}{
		OrderID: order.ID, OrderUpdatedAt: order.UpdatedAt.UTC().Format("2006-01-02T15:04:05.000000000Z07:00"),
		OrderStatus: order.Status, PaymentStatus: order.PaymentStatus, FulfillmentStatus: order.FulfillmentStatus,
		AssignedWarehouse: order.WarehouseID, WarehouseID: wh.ID, WarehouseCode: wh.Code, WarehouseDefault: wh.IsDefault, Lines: lines,
	})
	return idempotency.HashRequest(payload)
}

func evaluateAllocation(snapshot *allocationSnapshot, order orderMirror) WarehouseAllocationEvaluation {
	out := WarehouseAllocationEvaluation{OrderID: order.ID, Status: WarehouseAllocationBlocked, Blocks: []WarehouseAllocationBlock{}, Candidates: []WarehouseAllocationCandidate{}}
	items := snapshot.items[order.ID]
	required, skuIDs, validItems := aggregateAllocationItems(items)
	if orderInventoryActionFor(order) != orderInventoryReserve {
		out.Blocks = appendAllocationBlock(out.Blocks, allocationBlock("ORDER_NOT_RESERVABLE", "订单须已付款且尚未进入发货或终态"))
	}
	if !validItems {
		out.Blocks = appendAllocationBlock(out.Blocks, allocationBlock("SKU_BINDING_REQUIRED", "全部订单行必须绑定有效的本地 SKU 和数量"))
	}
	for _, skuID := range skuIDs {
		sku, ok := snapshot.skus[skuID]
		if !ok {
			out.Blocks = appendAllocationBlock(out.Blocks, allocationBlock("SKU_TENANT_MISMATCH", "订单包含不可用或不属于当前租户的 SKU"))
			continue
		}
		projection := derefStock(sku.Stock)
		if projection != snapshot.sellable[skuID] {
			out.Blocks = appendAllocationBlock(out.Blocks, allocationBlock("INVENTORY_PROJECTION_MISMATCH", "仓库库存账与兼容库存投影不一致，请先完成库存对账"))
		}
	}

	effects := snapshot.effects[order.ID]
	effectItems := map[uuid.UUID]struct{}{}
	effectWarehouses := map[uuid.UUID]struct{}{}
	missingEffectWarehouse := false
	for _, effect := range effects {
		effectItems[effect.OrderItemID] = struct{}{}
		if effect.WarehouseID == nil || *effect.WarehouseID == uuid.Nil {
			missingEffectWarehouse = true
			continue
		}
		effectWarehouses[*effect.WarehouseID] = struct{}{}
	}
	effectsComplete := len(effectItems) == len(items)
	for _, item := range items {
		if _, ok := effectItems[item.ID]; !ok {
			effectsComplete = false
			break
		}
	}
	if len(effects) > 0 {
		if effectsComplete && len(items) > 0 && len(effectWarehouses) == 1 && !missingEffectWarehouse {
			for warehouseID := range effectWarehouses {
				out.WarehouseID = ptrUUID(warehouseID)
				for _, wh := range snapshot.warehouses {
					if wh.ID == warehouseID {
						out.WarehouseCode, out.WarehouseName = wh.Code, wh.Name
					}
				}
			}
			if order.WarehouseID != nil && out.WarehouseID != nil && *order.WarehouseID != *out.WarehouseID {
				out.Blocks = appendAllocationBlock(out.Blocks, allocationBlock("INVENTORY_EFFECT_WAREHOUSE_CONFLICT", "订单仓库与既有库存事实不一致"))
				return out
			}
			out.Status = WarehouseAllocationAllocated
			out.Blocks = []WarehouseAllocationBlock{}
			return out
		}
		out.Blocks = appendAllocationBlock(out.Blocks, allocationBlock("PARTIAL_INVENTORY_EFFECT", "订单存在不完整或跨仓的库存事实，禁止继续分仓"))
		return out
	}
	if len(out.Blocks) > 0 {
		return out
	}

	candidateWarehouses := snapshot.warehouses
	if order.WarehouseID != nil && *order.WarehouseID != uuid.Nil {
		candidateWarehouses = nil
		for _, wh := range snapshot.warehouses {
			if wh.ID == *order.WarehouseID {
				candidateWarehouses = append(candidateWarehouses, wh)
				break
			}
		}
		if len(candidateWarehouses) == 0 {
			out.Blocks = appendAllocationBlock(out.Blocks, allocationBlock("ASSIGNED_WAREHOUSE_INACTIVE", "订单已绑定的仓库不可用"))
			return out
		}
	}
	if len(candidateWarehouses) == 0 {
		out.Blocks = appendAllocationBlock(out.Blocks, allocationBlock("ACTIVE_WAREHOUSE_REQUIRED", "当前租户没有可用仓库"))
		return out
	}

	for _, wh := range candidateWarehouses {
		candidate := WarehouseAllocationCandidate{
			WarehouseID: wh.ID, WarehouseCode: wh.Code, WarehouseName: wh.Name, IsDefault: wh.IsDefault,
			Eligible: true, Lines: make([]WarehouseAllocationCandidateLine, 0, len(skuIDs)),
		}
		for _, skuID := range skuIDs {
			sku := snapshot.skus[skuID]
			balance := snapshot.balances[wh.ID][skuID]
			available := balance.Available()
			shortage := required[skuID] - available
			if shortage < 0 {
				shortage = 0
			}
			line := WarehouseAllocationCandidateLine{
				ProductSKUID: skuID, SKUCode: sku.SKUCode, SKUName: sku.SKUName, Required: required[skuID], Available: available,
				Shortage: shortage, BalanceVersion: balance.Version, ProjectionStock: derefStock(sku.Stock), WarehouseSellable: snapshot.sellable[skuID],
			}
			candidate.Lines = append(candidate.Lines, line)
			if shortage > 0 {
				candidate.Eligible = false
				candidate.ShortageCount++
			}
		}
		candidate.Revision = allocationCandidateRevision(order, wh, candidate.Lines)
		out.Candidates = append(out.Candidates, candidate)
		if candidate.Eligible {
			out.EligibleCandidateCount++
			if out.RecommendedWarehouseID == nil {
				out.RecommendedWarehouseID = ptrUUID(wh.ID)
			}
		}
	}
	out.CandidateCount = len(out.Candidates)
	if out.EligibleCandidateCount == 0 {
		out.Blocks = appendAllocationBlock(out.Blocks, allocationBlock("INSUFFICIENT_SINGLE_WAREHOUSE_STOCK", "没有单一仓库可满足整单可用库存"))
		return out
	}
	out.Status = WarehouseAllocationAllocatable
	return out
}

// EvaluateOrderWarehouseAllocations calculates candidates in batches and never creates or mutates stock facts.
func (s *Service) EvaluateOrderWarehouseAllocations(ctx context.Context, tenantID int64, orderIDs []uuid.UUID) (map[uuid.UUID]WarehouseAllocationEvaluation, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("warehouse allocation unavailable")
	}
	return evaluateOrderWarehouseAllocationsWithDB(ctx, s.DB, tenantID, orderIDs)
}

func evaluateOrderWarehouseAllocationsWithDB(ctx context.Context, db *gorm.DB, tenantID int64, orderIDs []uuid.UUID) (map[uuid.UUID]WarehouseAllocationEvaluation, error) {
	snapshot, err := loadAllocationSnapshot(ctx, db, tenantID, orderIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]WarehouseAllocationEvaluation, len(snapshot.orders))
	for _, id := range orderIDs {
		if row, ok := snapshot.orders[id]; ok {
			out[id] = evaluateAllocation(snapshot, row)
		}
	}
	return out, nil
}

func validateOrderWarehouseAllocationTx(ctx context.Context, tx *gorm.DB, tenantID int64, orderID, warehouseID uuid.UUID, revision string) (map[uuid.UUID]int, error) {
	if warehouseID == uuid.Nil || len(strings.TrimSpace(revision)) != 64 {
		return nil, ErrOrderAllocationRevisionConflict
	}
	evaluations, err := evaluateOrderWarehouseAllocationsWithDB(ctx, tx, tenantID, []uuid.UUID{orderID})
	if err != nil {
		return nil, err
	}
	evaluation, ok := evaluations[orderID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	if evaluation.Status == WarehouseAllocationAllocated {
		return nil, ErrOrderAllocationAlreadyConfirmed
	}
	if evaluation.Status != WarehouseAllocationAllocatable {
		return nil, ErrOrderAllocationBlocked
	}
	for _, candidate := range evaluation.Candidates {
		if candidate.WarehouseID != warehouseID {
			continue
		}
		if !candidate.Eligible {
			return nil, ErrOrderAllocationBlocked
		}
		if candidate.Revision != strings.TrimSpace(revision) {
			return nil, ErrOrderAllocationRevisionConflict
		}
		versions := make(map[uuid.UUID]int, len(candidate.Lines))
		for _, line := range candidate.Lines {
			versions[line.ProductSKUID] = line.BalanceVersion
		}
		return versions, nil
	}
	return nil, ErrOrderAllocationRevisionConflict
}
