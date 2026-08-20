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

// WarehouseStockService owns procurement warehouse balance and movement writes.
// Scalar SKU stock remains the aggregate authority during the staged migration.
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

	var balanceCount int64
	if err := tx.WithContext(ctx).Model(&WarehouseStockBalance{}).
		Where("tenant_id = ? AND product_sku_id = ?", in.TenantID, in.ProductSKUID).
		Count(&balanceCount).Error; err != nil {
		return nil, fmt.Errorf("inventory warehouse stock: count balances: %w", err)
	}

	var balance WarehouseStockBalance
	err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND warehouse_id = ? AND product_sku_id = ?", in.TenantID, in.WarehouseID, in.ProductSKUID).
		First(&balance).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		legacyOnHand := 0
		if balanceCount == 0 && sku.Stock != nil && *sku.Stock > 0 {
			legacyOnHand = *sku.Stock
		}
		balance = WarehouseStockBalance{
			TenantID: in.TenantID, WarehouseID: in.WarehouseID, ProductSKUID: in.ProductSKUID,
			OnHand: legacyOnHand, Version: 1,
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

	// Preserve legacy adjustments and order deductions: until those writers are
	// warehouse-aware, advance the compatibility aggregate by this receipt delta
	// instead of rebuilding it from a partial warehouse ledger.
	aggregate := beforeAggregate + in.Quantity
	if err := tx.WithContext(ctx).Model(&product.ProductSKU{}).Where("id = ?", sku.ID).Update("stock", aggregate).Error; err != nil {
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
