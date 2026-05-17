package product

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
)

func stockInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// UpdateSKUStockSettings updates warning / safety lines only (not on-hand stock).
func (s *Service) UpdateSKUStockSettings(c *gin.Context, productID, skuID uuid.UUID, body SKUStockSettingsBody, adminID *uuid.UUID) (*ProductSKU, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("product: no db")
	}
	if err := ValidateSKUStockThresholds(body.WarningStock, body.SafetyStock); err != nil {
		return nil, err
	}
	var row ProductSKU
	if err := s.DB.WithContext(c.Request.Context()).First(&row, "id = ? AND product_id = ?", skuID, productID).Error; err != nil {
		return nil, err
	}
	st := CalculateSKUStockStatus(stockInt(row.Stock), body.WarningStock, body.SafetyStock)
	if err := s.DB.WithContext(c.Request.Context()).Model(&ProductSKU{}).Where("id = ? AND product_id = ?", skuID, productID).
		Updates(map[string]any{
			"warning_stock": body.WarningStock,
			"safety_stock":  body.SafetyStock,
			"stock_status":  st,
			"updated_at":    time.Now().UTC(),
		}).Error; err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "inventory.stock_alert.update",
			Resource:    "product_sku",
			ResourceID:  skuID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("productId=%s warning=%d safety=%d", productID.String(), body.WarningStock, body.SafetyStock),
		})
	}
	var out ProductSKU
	if err := s.DB.WithContext(c.Request.Context()).First(&out, "id = ?", skuID).Error; err != nil {
		return nil, err
	}
	return &out, nil
}
