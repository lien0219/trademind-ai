package inventory

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ReceiptStockInput is the stable cross-module contract used by procurement.
type ReceiptStockInput struct {
	TenantID            int64
	WarehouseID         uuid.UUID
	ProductSKUID        uuid.UUID
	Quantity            int
	ReceiptID           uuid.UUID
	PurchaseOrderID     uuid.UUID
	PurchaseOrderItemID uuid.UUID
	BusinessEventKey    string
	CreatedBy           *uuid.UUID
}

// PurchaseReturnStockInput is the stable cross-module contract used when a
// completed supplier return removes stock received by procurement.
type PurchaseReturnStockInput struct {
	TenantID             int64
	WarehouseID          uuid.UUID
	ProductSKUID         uuid.UUID
	Quantity             int
	PurchaseReturnID     uuid.UUID
	PurchaseReturnItemID uuid.UUID
	GoodsReceiptItemID   uuid.UUID
	BusinessEventKey     string
	Reason               string
	CreatedBy            *uuid.UUID
}

// SalesReturnStockInput is the stable cross-module contract for receiving one
// customer return into the warehouse used by the original order deduction.
type SalesReturnStockInput struct {
	TenantID          int64
	WarehouseID       uuid.UUID
	ProductSKUID      uuid.UUID
	Quantity          int
	Disposition       string
	SalesReturnID     uuid.UUID
	SalesReturnItemID uuid.UUID
	OrderID           uuid.UUID
	OrderItemID       uuid.UUID
	BusinessEventKey  string
	Reason            string
	CreatedBy         *uuid.UUID
}

type SalesReturnStockResult struct {
	Balance   *WarehouseStockBalance
	Movement  *InventoryMovement
	ChangeLog *InventoryChangeLog
}

var ErrInsufficientWarehouseAvailable = errors.New("insufficient warehouse available stock")

// WarehouseStockService owns procurement warehouse balance and movement writes.
// Scalar SKU stock is maintained only as a compatibility sellable projection.
type WarehouseStockService struct{}

// Receive posts one positive purchase receipt inside the caller-owned transaction.
func (WarehouseStockService) Receive(ctx context.Context, tx *gorm.DB, in ReceiptStockInput) (*WarehouseStockBalance, error) {
	if tx == nil {
		return nil, fmt.Errorf("inventory warehouse stock: db is nil")
	}
	if in.WarehouseID == uuid.Nil || in.ProductSKUID == uuid.Nil || in.ReceiptID == uuid.Nil || in.PurchaseOrderItemID == uuid.Nil {
		return nil, fmt.Errorf("inventory warehouse stock: identifiers are required")
	}
	if in.Quantity <= 0 {
		return nil, fmt.Errorf("inventory warehouse stock: quantity must be positive")
	}
	if strings.TrimSpace(in.BusinessEventKey) == "" {
		return nil, fmt.Errorf("inventory warehouse stock: business event key is required")
	}

	var sku product.ProductSKU
	if err := tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Joins("JOIN products ON products.id = product_skus.product_id AND products.deleted_at IS NULL").
		Where("product_skus.id = ? AND products.tenant_id = ?", in.ProductSKUID, in.TenantID).
		First(&sku).Error; err != nil {
		return nil, fmt.Errorf("inventory warehouse stock: load tenant SKU: %w", err)
	}

	var existingMovement InventoryMovement
	if err := tx.WithContext(ctx).Where("business_event_key = ?", in.BusinessEventKey).First(&existingMovement).Error; err == nil {
		var replay WarehouseStockBalance
		if err := tx.WithContext(ctx).Where("tenant_id = ? AND warehouse_id = ? AND product_sku_id = ?", in.TenantID, in.WarehouseID, in.ProductSKUID).First(&replay).Error; err != nil {
			return nil, fmt.Errorf("inventory warehouse stock: load replay balance: %w", err)
		}
		return &replay, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("inventory warehouse stock: check movement: %w", err)
	}

	if _, err := ensureLegacyBalanceInMigrationWarehouseTx(ctx, tx, in.TenantID, sku, in.CreatedBy); err != nil {
		return nil, fmt.Errorf("inventory warehouse stock: migrate legacy stock: %w", err)
	}
	var balance WarehouseStockBalance
	err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND warehouse_id = ? AND product_sku_id = ?", in.TenantID, in.WarehouseID, in.ProductSKUID).
		First(&balance).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		balance = WarehouseStockBalance{
			TenantID: in.TenantID, WarehouseID: in.WarehouseID, ProductSKUID: in.ProductSKUID,
			Version: 1,
		}
		if err := tx.WithContext(ctx).Create(&balance).Error; err != nil {
			return nil, fmt.Errorf("inventory warehouse stock: create balance: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("inventory warehouse stock: load balance: %w", err)
	}

	beforeBalance := balance.OnHand
	beforeAggregate := 0
	if sku.Stock != nil {
		beforeAggregate = *sku.Stock
	}
	balance.OnHand += in.Quantity
	balance.Version++
	result := tx.WithContext(ctx).Model(&WarehouseStockBalance{}).Where("id = ? AND version = ?", balance.ID, balance.Version-1).
		Updates(map[string]any{"on_hand": balance.OnHand, "version": balance.Version})
	if result.Error != nil {
		return nil, fmt.Errorf("inventory warehouse stock: update balance: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return nil, fmt.Errorf("inventory warehouse stock: concurrent balance update")
	}

	movement := InventoryMovement{
		TenantID: in.TenantID, WarehouseID: in.WarehouseID, ProductID: sku.ProductID, ProductSKUID: in.ProductSKUID,
		MovementType: MovementPurchaseReceipt, Quantity: in.Quantity, BeforeOnHand: beforeBalance, AfterOnHand: balance.OnHand,
		SourceType: "purchase_receipt", SourceID: in.ReceiptID, BusinessEventKey: strings.TrimSpace(in.BusinessEventKey), CreatedBy: in.CreatedBy,
	}
	if err := tx.WithContext(ctx).Create(&movement).Error; err != nil {
		return nil, fmt.Errorf("inventory warehouse stock: create movement: %w", err)
	}

	// Until order reservations and deductions move to the warehouse ledger,
	// preserve any pre-existing compatibility delta instead of rebuilding the
	// scalar field from a still-partial set of warehouse facts.
	aggregate := beforeAggregate + in.Quantity
	if err := writeSKUStockProjectionTx(ctx, tx, &sku, aggregate); err != nil {
		return nil, fmt.Errorf("inventory warehouse stock: update SKU projection: %w", err)
	}
	logRow := InventoryChangeLog{
		TenantID: in.TenantID, ProductID: sku.ProductID, ProductSKUID: sku.ID,
		ChangeType: ChangePurchaseReceipt, BeforeStock: beforeAggregate, AfterStock: aggregate, Delta: aggregate - beforeAggregate,
		Reason: "purchase receipt", CreatedBy: in.CreatedBy, BusinessEventKey: in.BusinessEventKey,
	}
	if err := tx.WithContext(ctx).Create(&logRow).Error; err != nil {
		return nil, fmt.Errorf("inventory warehouse stock: create compatibility log: %w", err)
	}
	return &balance, nil
}

// Return removes available on-hand stock inside the caller-owned transaction.
func (WarehouseStockService) Return(ctx context.Context, tx *gorm.DB, in PurchaseReturnStockInput) (*WarehouseStockBalance, error) {
	if tx == nil {
		return nil, fmt.Errorf("inventory warehouse stock: db is nil")
	}
	if in.WarehouseID == uuid.Nil || in.ProductSKUID == uuid.Nil || in.PurchaseReturnID == uuid.Nil || in.PurchaseReturnItemID == uuid.Nil || in.GoodsReceiptItemID == uuid.Nil {
		return nil, fmt.Errorf("inventory warehouse stock: return identifiers are required")
	}
	if in.Quantity <= 0 {
		return nil, fmt.Errorf("inventory warehouse stock: return quantity must be positive")
	}
	eventKey := strings.TrimSpace(in.BusinessEventKey)
	if eventKey == "" {
		return nil, fmt.Errorf("inventory warehouse stock: business event key is required")
	}

	var sku product.ProductSKU
	if err := tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Joins("JOIN products ON products.id = product_skus.product_id AND products.deleted_at IS NULL").
		Where("product_skus.id = ? AND products.tenant_id = ?", in.ProductSKUID, in.TenantID).
		First(&sku).Error; err != nil {
		return nil, fmt.Errorf("inventory warehouse stock: load return tenant SKU: %w", err)
	}

	var existingMovement InventoryMovement
	if err := tx.WithContext(ctx).Where("business_event_key = ?", eventKey).First(&existingMovement).Error; err == nil {
		var replay WarehouseStockBalance
		if err := tx.WithContext(ctx).Where("tenant_id = ? AND warehouse_id = ? AND product_sku_id = ?", in.TenantID, in.WarehouseID, in.ProductSKUID).First(&replay).Error; err != nil {
			return nil, fmt.Errorf("inventory warehouse stock: load return replay balance: %w", err)
		}
		return &replay, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("inventory warehouse stock: check return movement: %w", err)
	}

	var balance WarehouseStockBalance
	if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND warehouse_id = ? AND product_sku_id = ?", in.TenantID, in.WarehouseID, in.ProductSKUID).
		First(&balance).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInsufficientWarehouseAvailable
		}
		return nil, fmt.Errorf("inventory warehouse stock: load return balance: %w", err)
	}
	beforeAggregate := 0
	if sku.Stock != nil {
		beforeAggregate = *sku.Stock
	}
	if balance.Available() < in.Quantity || beforeAggregate < in.Quantity {
		return nil, ErrInsufficientWarehouseAvailable
	}

	beforeOnHand := balance.OnHand
	balance.OnHand -= in.Quantity
	balance.Version++
	result := tx.WithContext(ctx).Model(&WarehouseStockBalance{}).Where("id = ? AND version = ?", balance.ID, balance.Version-1).
		Updates(map[string]any{"on_hand": balance.OnHand, "version": balance.Version})
	if result.Error != nil {
		return nil, fmt.Errorf("inventory warehouse stock: update return balance: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return nil, fmt.Errorf("inventory warehouse stock: concurrent return balance update")
	}

	reason := strings.TrimSpace(in.Reason)
	movement := InventoryMovement{
		TenantID: in.TenantID, WarehouseID: in.WarehouseID, ProductID: sku.ProductID, ProductSKUID: sku.ID,
		MovementType: MovementPurchaseReturn, Quantity: -in.Quantity, BeforeOnHand: beforeOnHand, AfterOnHand: balance.OnHand,
		BeforeReserved: balance.Reserved, AfterReserved: balance.Reserved, SourceType: "purchase_return", SourceID: in.PurchaseReturnID,
		BusinessEventKey: eventKey, Reason: reason, CreatedBy: in.CreatedBy,
	}
	if err := tx.WithContext(ctx).Create(&movement).Error; err != nil {
		return nil, fmt.Errorf("inventory warehouse stock: create return movement: %w", err)
	}

	aggregate := beforeAggregate - in.Quantity
	if err := writeSKUStockProjectionTx(ctx, tx, &sku, aggregate); err != nil {
		return nil, fmt.Errorf("inventory warehouse stock: update return SKU projection: %w", err)
	}
	logRow := InventoryChangeLog{
		TenantID: in.TenantID, ProductID: sku.ProductID, ProductSKUID: sku.ID,
		ChangeType: ChangePurchaseReturn, BeforeStock: beforeAggregate, AfterStock: aggregate, Delta: -in.Quantity,
		Reason: reason, CreatedBy: in.CreatedBy, BusinessEventKey: eventKey,
	}
	if err := tx.WithContext(ctx).Create(&logRow).Error; err != nil {
		return nil, fmt.Errorf("inventory warehouse stock: create return compatibility log: %w", err)
	}
	return &balance, nil
}

// ReceiveSalesReturn receives sellable or damaged customer stock inside the
// caller-owned transaction. Damaged stock increases both on-hand and damaged,
// so it never increases available stock or the scalar compatibility projection.
func (WarehouseStockService) ReceiveSalesReturn(ctx context.Context, tx *gorm.DB, in SalesReturnStockInput) (*SalesReturnStockResult, error) {
	if tx == nil {
		return nil, fmt.Errorf("inventory warehouse stock: db is nil")
	}
	if in.WarehouseID == uuid.Nil || in.ProductSKUID == uuid.Nil || in.SalesReturnID == uuid.Nil || in.SalesReturnItemID == uuid.Nil || in.OrderID == uuid.Nil || in.OrderItemID == uuid.Nil {
		return nil, fmt.Errorf("inventory warehouse stock: sales return identifiers are required")
	}
	if in.Quantity <= 0 {
		return nil, fmt.Errorf("inventory warehouse stock: sales return quantity must be positive")
	}
	disposition := strings.TrimSpace(in.Disposition)
	if disposition != ReturnDispositionSellable && disposition != ReturnDispositionDamaged {
		return nil, fmt.Errorf("inventory warehouse stock: invalid sales return disposition")
	}
	eventKey := strings.TrimSpace(in.BusinessEventKey)
	if eventKey == "" {
		return nil, fmt.Errorf("inventory warehouse stock: business event key is required")
	}

	var sku product.ProductSKU
	if err := tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Joins("JOIN products ON products.id = product_skus.product_id AND products.deleted_at IS NULL").
		Where("product_skus.id = ? AND products.tenant_id = ?", in.ProductSKUID, in.TenantID).
		First(&sku).Error; err != nil {
		return nil, fmt.Errorf("inventory warehouse stock: load sales return tenant SKU: %w", err)
	}

	var existingMovement InventoryMovement
	if err := tx.WithContext(ctx).Where("business_event_key = ?", eventKey).First(&existingMovement).Error; err == nil {
		var replay WarehouseStockBalance
		if err := tx.WithContext(ctx).Where("tenant_id = ? AND warehouse_id = ? AND product_sku_id = ?", in.TenantID, in.WarehouseID, in.ProductSKUID).First(&replay).Error; err != nil {
			return nil, fmt.Errorf("inventory warehouse stock: load sales return replay balance: %w", err)
		}
		var change InventoryChangeLog
		if err := tx.WithContext(ctx).Where("business_event_key = ?", eventKey).First(&change).Error; err != nil {
			return nil, fmt.Errorf("inventory warehouse stock: load sales return replay log: %w", err)
		}
		return &SalesReturnStockResult{Balance: &replay, Movement: &existingMovement, ChangeLog: &change}, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("inventory warehouse stock: check sales return movement: %w", err)
	}

	var balance WarehouseStockBalance
	if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND warehouse_id = ? AND product_sku_id = ?", in.TenantID, in.WarehouseID, in.ProductSKUID).
		First(&balance).Error; err != nil {
		return nil, fmt.Errorf("inventory warehouse stock: load sales return balance: %w", err)
	}

	beforeOnHand, beforeDamaged := balance.OnHand, balance.Damaged
	beforeAggregate := 0
	if sku.Stock != nil {
		beforeAggregate = *sku.Stock
	}
	balance.OnHand += in.Quantity
	if disposition == ReturnDispositionDamaged {
		balance.Damaged += in.Quantity
	}
	balance.Version++
	result := tx.WithContext(ctx).Model(&WarehouseStockBalance{}).Where("id = ? AND version = ?", balance.ID, balance.Version-1).
		Updates(map[string]any{"on_hand": balance.OnHand, "damaged": balance.Damaged, "version": balance.Version})
	if result.Error != nil {
		return nil, fmt.Errorf("inventory warehouse stock: update sales return balance: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return nil, fmt.Errorf("inventory warehouse stock: concurrent sales return balance update")
	}

	reason := strings.TrimSpace(in.Reason)
	movement := &InventoryMovement{
		TenantID: in.TenantID, WarehouseID: in.WarehouseID, ProductID: sku.ProductID, ProductSKUID: sku.ID,
		MovementType: MovementSalesReturn, Quantity: in.Quantity, BeforeOnHand: beforeOnHand, AfterOnHand: balance.OnHand,
		BeforeReserved: balance.Reserved, AfterReserved: balance.Reserved, BeforeDamaged: beforeDamaged, AfterDamaged: balance.Damaged,
		SourceType: "sales_return", SourceID: in.SalesReturnID, BusinessEventKey: eventKey,
		Reason: reason, Remark: disposition, CreatedBy: in.CreatedBy,
	}
	if err := tx.WithContext(ctx).Create(movement).Error; err != nil {
		return nil, fmt.Errorf("inventory warehouse stock: create sales return movement: %w", err)
	}

	aggregate := beforeAggregate
	if disposition == ReturnDispositionSellable {
		aggregate += in.Quantity
	}
	if err := writeSKUStockProjectionTx(ctx, tx, &sku, aggregate); err != nil {
		return nil, fmt.Errorf("inventory warehouse stock: update sales return SKU projection: %w", err)
	}
	change := &InventoryChangeLog{
		TenantID: in.TenantID, ProductID: sku.ProductID, ProductSKUID: sku.ID,
		ChangeType: ChangeSalesReturn, BeforeStock: beforeAggregate, AfterStock: aggregate, Delta: aggregate - beforeAggregate,
		Reason: reason, Remark: disposition, CreatedBy: in.CreatedBy, RefOrderID: &in.OrderID, RefOrderItemID: &in.OrderItemID,
		BusinessEventKey: eventKey,
	}
	if err := tx.WithContext(ctx).Create(change).Error; err != nil {
		return nil, fmt.Errorf("inventory warehouse stock: create sales return compatibility log: %w", err)
	}
	return &SalesReturnStockResult{Balance: &balance, Movement: movement, ChangeLog: change}, nil
}
