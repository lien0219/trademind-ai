package procurement

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/supplier"
	"github.com/trademind-ai/trademind/backend/internal/modules/warehouse"
	"gorm.io/gorm"
)

const maxReplenishmentRows = 5000

var (
	ErrReplenishmentWarehouseRequired = errors.New("replenishment warehouse is required")
	ErrReplenishmentWarehouseInvalid  = errors.New("replenishment warehouse is not active")
	ErrReplenishmentInvalidStatus     = errors.New("invalid replenishment status")
	ErrReplenishmentTooManyRows       = errors.New("replenishment result exceeds export limit")
)

type ReplenishmentQuery struct {
	WarehouseID uuid.UUID
	Keyword     string
	Status      string
	Page        int
	PageSize    int
	Export      bool
}

type ReplenishmentSuggestion struct {
	WarehouseID           uuid.UUID  `json:"warehouseId"`
	WarehouseCode         string     `json:"warehouseCode"`
	WarehouseName         string     `json:"warehouseName"`
	ProductID             uuid.UUID  `json:"productId"`
	ProductTitle          string     `json:"productTitle"`
	ProductSKUID          uuid.UUID  `json:"productSkuId"`
	SKUCode               string     `json:"skuCode"`
	SKUName               string     `json:"skuName"`
	AvailableStock        int        `json:"availableStock"`
	InTransitTransfer     int        `json:"inTransitTransfer"`
	PendingPurchase       int        `json:"pendingPurchase"`
	WarningStock          int        `json:"warningStock"`
	SafetyStock           int        `json:"safetyStock"`
	Deficit               int        `json:"deficit"`
	SuggestedQuantity     int        `json:"suggestedQuantity"`
	MinOrderQty           int        `json:"minOrderQty"`
	UnitCostMinor         int64      `json:"unitCostMinor"`
	Currency              string     `json:"currency"`
	LeadTimeDays          int        `json:"leadTimeDays"`
	SupplierID            *uuid.UUID `json:"supplierId,omitempty"`
	SupplierName          string     `json:"supplierName,omitempty"`
	Status                string     `json:"status"`
	BlockReasonCode       string     `json:"blockReasonCode,omitempty"`
	BlockReason           string     `json:"blockReason,omitempty"`
	InventoryOnHandTotal  int        `json:"inventoryOnHandTotal"`
	InventoryBalanceCount int        `json:"inventoryBalanceCount"`
}

type ReplenishmentResult struct {
	WarehouseID   uuid.UUID                 `json:"warehouseId"`
	WarehouseCode string                    `json:"warehouseCode"`
	WarehouseName string                    `json:"warehouseName"`
	List          []ReplenishmentSuggestion `json:"list"`
	Page          int                       `json:"page"`
	PageSize      int                       `json:"pageSize"`
	Total         int                       `json:"total"`
	TotalPages    int                       `json:"totalPages"`
}

type replenishmentSKU struct {
	ProductID             uuid.UUID `gorm:"column:product_id"`
	ProductTitle          string    `gorm:"column:product_title"`
	ProductSKUID          uuid.UUID `gorm:"column:product_sku_id"`
	SKUCode               string    `gorm:"column:sku_code"`
	SKUName               string    `gorm:"column:sku_name"`
	AggregateStock        int       `gorm:"column:aggregate_stock"`
	WarningStock          int       `gorm:"column:warning_stock"`
	SafetyStock           int       `gorm:"column:safety_stock"`
	TargetOnHand          int       `gorm:"column:target_on_hand"`
	TargetReserved        int       `gorm:"column:target_reserved"`
	TargetDamaged         int       `gorm:"column:target_damaged"`
	InventoryOnHandTotal  int       `gorm:"column:inventory_on_hand_total"`
	InventoryBalanceCount int       `gorm:"column:inventory_balance_count"`
}

type replenishmentSupplier struct {
	ID            uuid.UUID `gorm:"column:id"`
	ProductSKUID  uuid.UUID `gorm:"column:product_sku_id"`
	SupplierName  string    `gorm:"column:supplier_name"`
	UnitCostMinor int64     `gorm:"column:unit_cost_minor"`
	Currency      string    `gorm:"column:currency"`
	MinOrderQty   int       `gorm:"column:min_order_qty"`
	LeadTimeDays  int       `gorm:"column:lead_time_days"`
}

func (s *Service) ListReplenishmentSuggestions(ctx context.Context, tenantID int64, q ReplenishmentQuery) (*ReplenishmentResult, error) {
	if s == nil || s.DB == nil || tenantID < 0 {
		return nil, fmt.Errorf("replenishment: database unavailable")
	}
	if q.WarehouseID == uuid.Nil {
		return nil, ErrReplenishmentWarehouseRequired
	}
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 {
		q.PageSize = 20
	}
	if q.Export {
		q.Page = 1
		q.PageSize = maxReplenishmentRows
	} else if q.PageSize > 100 {
		q.PageSize = 100
	}
	q.Status = strings.TrimSpace(strings.ToLower(q.Status))
	if q.Status != "" && q.Status != "all" && q.Status != "actionable" && q.Status != "not_needed" &&
		q.Status != "blocked_inventory_mismatch" && q.Status != "blocked_inventory_unmigrated" &&
		q.Status != "blocked_supplier_missing" && q.Status != "blocked_supplier_selection" {
		return nil, ErrReplenishmentInvalidStatus
	}
	var wh warehouse.Warehouse
	if err := s.DB.WithContext(ctx).Where("id = ? AND tenant_id = ?", q.WarehouseID, tenantID).First(&wh).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrReplenishmentWarehouseInvalid
		}
		return nil, fmt.Errorf("load replenishment warehouse: %w", err)
	}
	if wh.Status != warehouse.StatusActive {
		return nil, ErrReplenishmentWarehouseInvalid
	}

	globalBalances := s.DB.WithContext(ctx).Table("warehouse_stock_balances AS all_balances").
		Select("all_balances.product_sku_id, COALESCE(SUM(all_balances.on_hand), 0) AS inventory_on_hand_total, COUNT(*) AS inventory_balance_count").
		Where("all_balances.tenant_id = ?", tenantID).
		Group("all_balances.product_sku_id")
	base := s.DB.WithContext(ctx).Table("product_skus AS sk").
		Select(`sk.product_id, p.title AS product_title, sk.id AS product_sku_id, sk.sku_code, sk.sku_name,
			COALESCE(sk.stock, 0) AS aggregate_stock, sk.warning_stock, sk.safety_stock,
			COALESCE(target.on_hand, 0) AS target_on_hand, COALESCE(target.reserved, 0) AS target_reserved,
			COALESCE(target.damaged, 0) AS target_damaged,
			COALESCE(global_balances.inventory_on_hand_total, 0) AS inventory_on_hand_total,
			COALESCE(global_balances.inventory_balance_count, 0) AS inventory_balance_count`).
		Joins("JOIN products AS p ON p.id = sk.product_id AND p.tenant_id = ? AND p.deleted_at IS NULL", tenantID).
		Joins("LEFT JOIN warehouse_stock_balances AS target ON target.tenant_id = ? AND target.warehouse_id = ? AND target.product_sku_id = sk.id", tenantID, q.WarehouseID).
		Joins("LEFT JOIN (?) AS global_balances ON global_balances.product_sku_id = sk.id", globalBalances)
	if keyword := strings.TrimSpace(q.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("p.title LIKE ? OR sk.sku_code LIKE ? OR sk.sku_name LIKE ?", like, like, like)
	}
	base = base.Order("p.title ASC, sk.sku_code ASC, sk.id ASC")
	var rows []replenishmentSKU
	if err := base.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list replenishment SKUs: %w", err)
	}
	if len(rows) == 0 {
		return &ReplenishmentResult{WarehouseID: wh.ID, WarehouseCode: wh.Code, WarehouseName: wh.Name, List: []ReplenishmentSuggestion{}, Page: q.Page, PageSize: q.PageSize, TotalPages: 0}, nil
	}
	skuIDs := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		skuIDs = append(skuIDs, row.ProductSKUID)
	}
	transferPending, err := s.pendingTransferQuantities(ctx, tenantID, q.WarehouseID, skuIDs)
	if err != nil {
		return nil, err
	}
	purchasePending, err := s.pendingPurchaseQuantities(ctx, tenantID, q.WarehouseID, skuIDs)
	if err != nil {
		return nil, err
	}
	suppliers, err := s.replenishmentSuppliers(ctx, tenantID, skuIDs)
	if err != nil {
		return nil, err
	}
	all := make([]ReplenishmentSuggestion, 0, len(rows))
	for _, row := range rows {
		available := row.TargetOnHand - row.TargetReserved - row.TargetDamaged
		if available < 0 {
			available = 0
		}
		inTransit := transferPending[row.ProductSKUID]
		pendingPurchase := purchasePending[row.ProductSKUID]
		deficit := row.WarningStock - (available + inTransit + pendingPurchase)
		if deficit < 0 {
			deficit = 0
		}
		item := ReplenishmentSuggestion{
			WarehouseID: q.WarehouseID, WarehouseCode: wh.Code, WarehouseName: wh.Name,
			ProductID: row.ProductID, ProductTitle: row.ProductTitle, ProductSKUID: row.ProductSKUID,
			SKUCode: row.SKUCode, SKUName: row.SKUName, AvailableStock: available,
			InTransitTransfer: inTransit, PendingPurchase: pendingPurchase, WarningStock: row.WarningStock,
			SafetyStock: row.SafetyStock, Deficit: deficit, InventoryOnHandTotal: row.InventoryOnHandTotal,
			InventoryBalanceCount: row.InventoryBalanceCount, Status: "not_needed",
		}
		if row.InventoryBalanceCount == 0 {
			item.Status, item.BlockReasonCode, item.BlockReason = "blocked_inventory_unmigrated", "inventory_unmigrated", "尚未建立完整仓库库存账，不能计算补货建议"
		} else if row.InventoryOnHandTotal != row.AggregateStock {
			item.Status, item.BlockReasonCode, item.BlockReason = "blocked_inventory_mismatch", "inventory_mismatch", "仓库库存账与聚合库存不一致，需人工对账"
		} else if deficit > 0 {
			bound := suppliers[row.ProductSKUID]
			switch len(bound) {
			case 0:
				item.Status, item.BlockReasonCode, item.BlockReason = "blocked_supplier_missing", "supplier_missing", "未找到有效供应商，请先维护供应商绑定"
			case 1:
				item.Status = "actionable"
				item.SupplierID, item.SupplierName = &bound[0].ID, bound[0].SupplierName
				item.MinOrderQty, item.UnitCostMinor, item.Currency, item.LeadTimeDays = bound[0].MinOrderQty, bound[0].UnitCostMinor, bound[0].Currency, bound[0].LeadTimeDays
				if item.MinOrderQty < 1 {
					item.MinOrderQty = 1
				}
				item.SuggestedQuantity = int(math.Ceil(float64(deficit)/float64(item.MinOrderQty))) * item.MinOrderQty
			default:
				item.Status, item.BlockReasonCode, item.BlockReason = "blocked_supplier_selection", "multiple_suppliers", "存在多个有效供应商，请人工选择后再采购"
			}
		}
		if q.Status == "" || q.Status == "all" || q.Status == item.Status {
			all = append(all, item)
		}
	}
	if q.Export && len(all) > maxReplenishmentRows {
		return nil, ErrReplenishmentTooManyRows
	}
	total := len(all)
	offset := (q.Page - 1) * q.PageSize
	if offset > total {
		offset = total
	}
	end := offset + q.PageSize
	if end > total {
		end = total
	}
	list := all[offset:end]
	return &ReplenishmentResult{WarehouseID: wh.ID, WarehouseCode: wh.Code, WarehouseName: wh.Name, List: list, Page: q.Page, PageSize: q.PageSize, Total: total, TotalPages: replenishmentPagesOf(total, q.PageSize)}, nil
}

func (s *Service) pendingTransferQuantities(ctx context.Context, tenantID int64, warehouseID uuid.UUID, skuIDs []uuid.UUID) (map[uuid.UUID]int, error) {
	type row struct {
		ProductSKUID uuid.UUID `gorm:"column:product_sku_id"`
		Quantity     int       `gorm:"column:quantity"`
	}
	var rows []row
	err := s.DB.WithContext(ctx).Table("warehouse_transfer_items AS item").
		Select("item.product_sku_id, COALESCE(SUM(item.quantity - item.received_quantity), 0) AS quantity").
		Joins("JOIN warehouse_transfers AS transfer ON transfer.id = item.transfer_id AND transfer.tenant_id = ?", tenantID).
		Where("item.tenant_id = ? AND transfer.target_warehouse_id = ? AND transfer.status = ? AND item.product_sku_id IN ? AND item.received_quantity < item.quantity", tenantID, warehouseID, inventory.TransferInTransit, skuIDs).
		Group("item.product_sku_id").Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("load in-transit transfers: %w", err)
	}
	out := make(map[uuid.UUID]int, len(rows))
	for _, r := range rows {
		out[r.ProductSKUID] = r.Quantity
	}
	return out, nil
}

func (s *Service) pendingPurchaseQuantities(ctx context.Context, tenantID int64, warehouseID uuid.UUID, skuIDs []uuid.UUID) (map[uuid.UUID]int, error) {
	type row struct {
		ProductSKUID uuid.UUID `gorm:"column:product_sku_id"`
		Quantity     int       `gorm:"column:quantity"`
	}
	var rows []row
	err := s.DB.WithContext(ctx).Table("purchase_order_items AS item").
		Select("item.product_sku_id, COALESCE(SUM(item.quantity - item.received_quantity), 0) AS quantity").
		Joins("JOIN purchase_orders AS po ON po.id = item.purchase_order_id AND po.tenant_id = ?", tenantID).
		Where("item.tenant_id = ? AND po.warehouse_id = ? AND po.status IN ? AND item.product_sku_id IN ? AND item.received_quantity < item.quantity", tenantID, warehouseID, []string{StatusApproved, StatusPartiallyReceived}, skuIDs).
		Group("item.product_sku_id").Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("load pending purchase quantities: %w", err)
	}
	out := make(map[uuid.UUID]int, len(rows))
	for _, r := range rows {
		out[r.ProductSKUID] = r.Quantity
	}
	return out, nil
}

func (s *Service) replenishmentSuppliers(ctx context.Context, tenantID int64, skuIDs []uuid.UUID) (map[uuid.UUID][]replenishmentSupplier, error) {
	var rows []replenishmentSupplier
	err := s.DB.WithContext(ctx).Table("supplier_skus AS ss").
		Select("ss.id, ss.product_sku_id, suppliers.name AS supplier_name, ss.unit_cost_minor, ss.currency, ss.min_order_qty, ss.lead_time_days").
		Joins("JOIN suppliers ON suppliers.id = ss.supplier_id AND suppliers.tenant_id = ? AND suppliers.status = ?", tenantID, supplier.StatusActive).
		Where("ss.tenant_id = ? AND ss.product_sku_id IN ?", tenantID, skuIDs).
		Order("ss.product_sku_id ASC, suppliers.name ASC, ss.id ASC").Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("load supplier bindings: %w", err)
	}
	out := make(map[uuid.UUID][]replenishmentSupplier, len(rows))
	for _, r := range rows {
		out[r.ProductSKUID] = append(out[r.ProductSKUID], r)
	}
	return out, nil
}

func WriteReplenishmentCSV(w io.Writer, rows []ReplenishmentSuggestion) error {
	c := csv.NewWriter(w)
	if err := c.Write([]string{"仓库", "商品", "规格编码", "可用库存", "在途调拨", "待收采购", "预警库存", "安全库存", "缺口", "建议采购量", "MOQ", "采购价(最小单位)", "币种", "交期(天)", "供应商", "状态", "阻断原因"}); err != nil {
		return err
	}
	for _, row := range rows {
		if err := c.Write([]string{row.WarehouseCode, row.ProductTitle, row.SKUCode, strconv.Itoa(row.AvailableStock), strconv.Itoa(row.InTransitTransfer), strconv.Itoa(row.PendingPurchase), strconv.Itoa(row.WarningStock), strconv.Itoa(row.SafetyStock), strconv.Itoa(row.Deficit), strconv.Itoa(row.SuggestedQuantity), strconv.Itoa(row.MinOrderQty), strconv.FormatInt(row.UnitCostMinor, 10), row.Currency, strconv.Itoa(row.LeadTimeDays), row.SupplierName, row.Status, row.BlockReason}); err != nil {
			return err
		}
	}
	c.Flush()
	return c.Error()
}

func replenishmentPagesOf(total, pageSize int) int {
	if total == 0 {
		return 0
	}
	return (total + pageSize - 1) / pageSize
}
