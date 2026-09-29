package inventory

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"gorm.io/gorm"
)

// ErrSKUStockProjection is returned when the compatibility projection cannot
// be updated for the tenant-scoped SKU row.
var ErrSKUStockProjection = errors.New("sku stock projection update failed")

// writeSKUStockProjectionTx is the only inventory-module writer for the
// legacy product_skus.stock compatibility projection. Callers must calculate
// the delta-aware aggregate while holding the SKU and warehouse locks; this
// helper only validates the target row and persists the projection/status.
func writeSKUStockProjectionTx(ctx context.Context, tx *gorm.DB, sku *product.ProductSKU, stock int) error {
	if tx == nil {
		return fmt.Errorf("%w: db is nil", ErrSKUStockProjection)
	}
	if sku == nil || sku.ID == uuid.Nil || sku.ProductID == uuid.Nil {
		return fmt.Errorf("%w: sku identifiers are required", ErrSKUStockProjection)
	}
	result := tx.WithContext(ctx).
		Model(&product.ProductSKU{}).
		Where("id = ? AND product_id = ?", sku.ID, sku.ProductID).
		Updates(map[string]any{
			"stock":        stock,
			"stock_status": stockStatusForSKU(*sku, stock),
			"updated_at":   time.Now().UTC(),
		})
	if result.Error != nil {
		return fmt.Errorf("%w: %v", ErrSKUStockProjection, result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("%w: sku=%s", ErrSKUStockProjection, sku.ID)
	}
	return nil
}
