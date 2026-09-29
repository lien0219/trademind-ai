package profitability

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type OrderFact struct {
	ID                uuid.UUID  `gorm:"column:id"`
	Platform          string     `gorm:"column:platform"`
	ShopID            *uuid.UUID `gorm:"column:shop_id"`
	ShopName          string     `gorm:"column:shop_name"`
	WarehouseID       *uuid.UUID `gorm:"column:warehouse_id"`
	WarehouseCode     string     `gorm:"column:warehouse_code"`
	WarehouseName     string     `gorm:"column:warehouse_name"`
	OrderNo           string     `gorm:"column:order_no"`
	Status            string     `gorm:"column:status"`
	PaymentStatus     string     `gorm:"column:payment_status"`
	FulfillmentStatus string     `gorm:"column:fulfillment_status"`
	Currency          string     `gorm:"column:currency"`
	TotalAmountText   string     `gorm:"column:total_amount_text"`
	OrderedAt         *time.Time `gorm:"column:ordered_at"`
	CreatedAt         time.Time  `gorm:"column:created_at"`
	UpdatedAt         time.Time  `gorm:"column:updated_at"`
}

type OrderItemFact struct {
	ID           uuid.UUID  `gorm:"column:id"`
	OrderID      uuid.UUID  `gorm:"column:order_id"`
	ProductSKUID *uuid.UUID `gorm:"column:product_sku_id"`
	ProductTitle string     `gorm:"column:product_title"`
	SKUCode      string     `gorm:"column:sku_code"`
	Quantity     int        `gorm:"column:quantity"`
}

type SupplierCostFact struct {
	SupplierSKUID uuid.UUID `gorm:"column:supplier_sku_id"`
	SupplierID    uuid.UUID `gorm:"column:supplier_id"`
	ProductSKUID  uuid.UUID `gorm:"column:product_sku_id"`
	SupplierName  string    `gorm:"column:supplier_name"`
	UnitCostMinor int64     `gorm:"column:unit_cost_minor"`
	Currency      string    `gorm:"column:currency"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

type OrderItemCostSnapshotFact struct {
	ID               uuid.UUID  `gorm:"column:id"`
	OrderID          uuid.UUID  `gorm:"column:order_id"`
	OrderItemID      uuid.UUID  `gorm:"column:order_item_id"`
	ProductSKUID     uuid.UUID  `gorm:"column:product_sku_id"`
	Quantity         int        `gorm:"column:quantity"`
	OrderCurrency    string     `gorm:"column:order_currency"`
	UnitCostMinor    *int64     `gorm:"column:unit_cost_minor"`
	LineCostMinor    *int64     `gorm:"column:line_cost_minor"`
	CostCurrency     string     `gorm:"column:cost_currency"`
	ResolutionStatus string     `gorm:"column:resolution_status"`
	ReasonCode       string     `gorm:"column:reason_code"`
	SupplierID       *uuid.UUID `gorm:"column:supplier_id"`
	SupplierSKUID    *uuid.UUID `gorm:"column:supplier_sku_id"`
	SupplierName     string     `gorm:"column:supplier_name"`
	SupplierSKUCode  string     `gorm:"column:supplier_sku_code"`
	SourceUpdatedAt  *time.Time `gorm:"column:source_updated_at"`
	CandidateCount   int        `gorm:"column:candidate_count"`
	CapturedAt       time.Time  `gorm:"column:captured_at"`
}

type FreightFact struct {
	ID          uuid.UUID `gorm:"column:id"`
	OrderID     uuid.UUID `gorm:"column:order_id"`
	WaveID      uuid.UUID `gorm:"column:wave_id"`
	Version     int       `gorm:"column:version"`
	AmountMinor int64     `gorm:"column:amount_minor"`
	Currency    string    `gorm:"column:currency"`
	CreatedAt   time.Time `gorm:"column:created_at"`
}

type RefundFact struct {
	SalesReturnID       uuid.UUID  `gorm:"column:sales_return_id"`
	OrderID             uuid.UUID  `gorm:"column:order_id"`
	SalesReturnAmount   int64      `gorm:"column:sales_return_amount"`
	SalesReturnCurrency string     `gorm:"column:sales_return_currency"`
	ExecutionID         *uuid.UUID `gorm:"column:execution_id"`
	ExecutionStatus     string     `gorm:"column:execution_status"`
	ExecutionAmount     int64      `gorm:"column:execution_amount"`
	ExecutionCurrency   string     `gorm:"column:execution_currency"`
	ExecutionUpdatedAt  *time.Time `gorm:"column:execution_updated_at"`
}

type Repository interface {
	ListOrders(context.Context, int64, Scope, ListQuery, int, int) ([]OrderFact, int64, error)
	GetOrder(context.Context, int64, Scope, uuid.UUID) (*OrderFact, error)
	ListOrderItems(context.Context, int64, []uuid.UUID) ([]OrderItemFact, error)
	ListOrderItemCostSnapshots(context.Context, int64, []uuid.UUID) ([]OrderItemCostSnapshotFact, error)
	ListSupplierCosts(context.Context, int64, []uuid.UUID) ([]SupplierCostFact, error)
	ListFreightFacts(context.Context, int64, []uuid.UUID) ([]FreightFact, error)
	ListRefundFacts(context.Context, int64, []uuid.UUID) ([]RefundFact, error)
}

type GormRepository struct {
	DB *gorm.DB
}

func (r *GormRepository) orderQuery(ctx context.Context, tenantID int64, scope Scope, q ListQuery) *gorm.DB {
	tx := r.DB.WithContext(ctx).Table("orders").
		Where("orders.tenant_id = ? AND orders.deleted_at IS NULL", tenantID)
	if scope.RestrictStoreScope {
		if len(scope.AllowedShopIDs) == 0 {
			tx = tx.Where("1 = 0")
		} else {
			tx = tx.Where("orders.shop_id IN ?", scope.AllowedShopIDs)
		}
	}
	if q.ShopID != nil && *q.ShopID != uuid.Nil {
		tx = tx.Where("orders.shop_id = ?", *q.ShopID)
	}
	if q.WarehouseID != nil && *q.WarehouseID != uuid.Nil {
		tx = tx.Where("orders.warehouse_id = ?", *q.WarehouseID)
	}
	if value := strings.TrimSpace(q.OrderNo); value != "" {
		tx = tx.Where("LOWER(orders.order_no) LIKE ?", "%"+strings.ToLower(value)+"%")
	}
	if value := strings.TrimSpace(q.Platform); value != "" {
		tx = tx.Where("LOWER(orders.platform) = ?", strings.ToLower(value))
	}
	if value := strings.TrimSpace(q.Currency); value != "" {
		tx = tx.Where("UPPER(orders.currency) = ?", strings.ToUpper(value))
	}
	if q.Start != nil {
		tx = tx.Where("COALESCE(orders.ordered_at, orders.created_at) >= ?", *q.Start)
	}
	if q.End != nil {
		tx = tx.Where("COALESCE(orders.ordered_at, orders.created_at) <= ?", *q.End)
	}
	return tx
}

func (r *GormRepository) ListOrders(ctx context.Context, tenantID int64, scope Scope, q ListQuery, offset, limit int) ([]OrderFact, int64, error) {
	if r == nil || r.DB == nil {
		return nil, 0, fmt.Errorf("profitability repository: database unavailable")
	}
	base := r.orderQuery(ctx, tenantID, scope, q)
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count order profit candidates: %w", err)
	}
	rows := make([]OrderFact, 0)
	if total == 0 || limit < 1 {
		return rows, total, nil
	}
	err := base.
		Select(`orders.id, orders.platform, orders.shop_id, orders.warehouse_id, orders.order_no,
			orders.status, orders.payment_status, orders.fulfillment_status, orders.currency,
			orders.total_amount AS total_amount_text, orders.ordered_at, orders.created_at, orders.updated_at,
			COALESCE(shops.shop_name, '') AS shop_name,
			COALESCE(warehouses.code, '') AS warehouse_code,
			COALESCE(warehouses.name, '') AS warehouse_name`).
		Joins("LEFT JOIN shops ON shops.id = orders.shop_id AND shops.tenant_id = orders.tenant_id AND shops.deleted_at IS NULL").
		Joins("LEFT JOIN warehouses ON warehouses.id = orders.warehouse_id AND warehouses.tenant_id = orders.tenant_id AND warehouses.deleted_at IS NULL").
		Order("orders.created_at DESC, orders.id DESC").
		Offset(offset).Limit(limit).Scan(&rows).Error
	if err != nil {
		return nil, 0, fmt.Errorf("list order profit candidates: %w", err)
	}
	return rows, total, nil
}

func (r *GormRepository) GetOrder(ctx context.Context, tenantID int64, scope Scope, orderID uuid.UUID) (*OrderFact, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("profitability repository: database unavailable")
	}
	q := ListQuery{}
	var row OrderFact
	err := r.orderQuery(ctx, tenantID, scope, q).
		Select(`orders.id, orders.platform, orders.shop_id, orders.warehouse_id, orders.order_no,
			orders.status, orders.payment_status, orders.fulfillment_status, orders.currency,
			orders.total_amount AS total_amount_text, orders.ordered_at, orders.created_at, orders.updated_at,
			COALESCE(shops.shop_name, '') AS shop_name,
			COALESCE(warehouses.code, '') AS warehouse_code,
			COALESCE(warehouses.name, '') AS warehouse_name`).
		Joins("LEFT JOIN shops ON shops.id = orders.shop_id AND shops.tenant_id = orders.tenant_id AND shops.deleted_at IS NULL").
		Joins("LEFT JOIN warehouses ON warehouses.id = orders.warehouse_id AND warehouses.tenant_id = orders.tenant_id AND warehouses.deleted_at IS NULL").
		Where("orders.id = ?", orderID).Take(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *GormRepository) ListOrderItems(ctx context.Context, tenantID int64, orderIDs []uuid.UUID) ([]OrderItemFact, error) {
	rows := make([]OrderItemFact, 0)
	if len(orderIDs) == 0 {
		return rows, nil
	}
	err := r.DB.WithContext(ctx).Table("order_items AS item").
		Select("item.id, item.order_id, item.product_sku_id, item.product_title, item.sku_code, item.quantity").
		Joins("JOIN orders ON orders.id = item.order_id AND orders.tenant_id = ? AND orders.deleted_at IS NULL", tenantID).
		Where("item.order_id IN ?", orderIDs).
		Order("item.order_id ASC, item.created_at ASC, item.id ASC").Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list order profit items: %w", err)
	}
	return rows, nil
}

func (r *GormRepository) ListOrderItemCostSnapshots(ctx context.Context, tenantID int64, orderIDs []uuid.UUID) ([]OrderItemCostSnapshotFact, error) {
	rows := make([]OrderItemCostSnapshotFact, 0)
	if len(orderIDs) == 0 {
		return rows, nil
	}
	err := r.DB.WithContext(ctx).Table("order_item_cost_snapshots AS snapshot").
		Select(`snapshot.id, snapshot.order_id, snapshot.order_item_id, snapshot.product_sku_id,
			snapshot.quantity, snapshot.order_currency, snapshot.unit_cost_minor, snapshot.line_cost_minor,
			snapshot.cost_currency, snapshot.resolution_status, snapshot.reason_code,
			snapshot.supplier_id, snapshot.supplier_sku_id, snapshot.supplier_name, snapshot.supplier_sku_code,
			snapshot.source_updated_at, snapshot.candidate_count, snapshot.captured_at`).
		Joins("JOIN orders ON orders.id = snapshot.order_id AND orders.tenant_id = ? AND orders.deleted_at IS NULL", tenantID).
		Where("snapshot.tenant_id = ? AND snapshot.order_id IN ?", tenantID, orderIDs).
		Order("snapshot.order_id ASC, snapshot.order_item_id ASC, snapshot.id ASC").Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list order item cost snapshots: %w", err)
	}
	return rows, nil
}

func (r *GormRepository) ListSupplierCosts(ctx context.Context, tenantID int64, skuIDs []uuid.UUID) ([]SupplierCostFact, error) {
	rows := make([]SupplierCostFact, 0)
	if len(skuIDs) == 0 {
		return rows, nil
	}
	err := r.DB.WithContext(ctx).Table("supplier_skus AS supplier_sku").
		Select(`supplier_sku.id AS supplier_sku_id, supplier_sku.supplier_id, supplier_sku.product_sku_id,
			supplier_sku.unit_cost_minor, supplier_sku.currency, supplier_sku.updated_at,
			supplier.name AS supplier_name`).
		Joins("JOIN suppliers AS supplier ON supplier.id = supplier_sku.supplier_id AND supplier.tenant_id = ? AND supplier.status = ? AND supplier.deleted_at IS NULL", tenantID, "active").
		Where("supplier_sku.tenant_id = ? AND supplier_sku.deleted_at IS NULL AND supplier_sku.product_sku_id IN ?", tenantID, skuIDs).
		Order("supplier_sku.product_sku_id ASC, supplier.name ASC, supplier_sku.id ASC").Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list supplier cost estimates: %w", err)
	}
	return rows, nil
}

func (r *GormRepository) ListFreightFacts(ctx context.Context, tenantID int64, orderIDs []uuid.UUID) ([]FreightFact, error) {
	rows := make([]FreightFact, 0)
	if len(orderIDs) == 0 {
		return rows, nil
	}
	err := r.DB.WithContext(ctx).Table("fulfillment_wave_freight_quotes AS quote").
		Select("quote.id, quote.order_id, quote.wave_id, quote.version, quote.amount_minor, quote.currency, quote.created_at").
		Joins("JOIN fulfillment_waves AS wave ON wave.id = quote.wave_id AND wave.tenant_id = ? AND wave.deleted_at IS NULL", tenantID).
		Where("quote.tenant_id = ? AND quote.order_id IN ? AND wave.status <> ?", tenantID, orderIDs, "cancelled").
		Order("quote.order_id ASC, quote.wave_id ASC, quote.version DESC, quote.created_at DESC, quote.id DESC").Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list confirmed freight facts: %w", err)
	}
	return rows, nil
}

func (r *GormRepository) ListRefundFacts(ctx context.Context, tenantID int64, orderIDs []uuid.UUID) ([]RefundFact, error) {
	rows := make([]RefundFact, 0)
	if len(orderIDs) == 0 {
		return rows, nil
	}
	err := r.DB.WithContext(ctx).Table("sales_returns AS sales_return").
		Select(`sales_return.id AS sales_return_id, sales_return.order_id,
			sales_return.refund_amount_minor AS sales_return_amount, sales_return.currency AS sales_return_currency,
			execution.id AS execution_id, COALESCE(execution.status, '') AS execution_status,
			COALESCE(execution.refund_amount_minor, 0) AS execution_amount,
			COALESCE(execution.currency, '') AS execution_currency, execution.updated_at AS execution_updated_at`).
		Joins("LEFT JOIN refund_executions AS execution ON execution.sales_return_id = sales_return.id AND execution.tenant_id = sales_return.tenant_id AND execution.deleted_at IS NULL").
		Where("sales_return.tenant_id = ? AND sales_return.deleted_at IS NULL AND sales_return.status = ? AND sales_return.order_id IN ?", tenantID, "completed", orderIDs).
		Order("sales_return.order_id ASC, sales_return.created_at ASC, sales_return.id ASC").Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list refund facts: %w", err)
	}
	return rows, nil
}
