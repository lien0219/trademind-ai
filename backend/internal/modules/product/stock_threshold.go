package product

import "fmt"

// Stock status values for local SKU (computed; optional persisted on product_skus.stock_status).
const (
	StockStatusNormal           = "normal"
	StockStatusLowStock         = "low_stock"
	StockStatusOutOfStock       = "out_of_stock"
	StockStatusBelowSafetyStock = "below_safety_stock"
)

// CalculateSKUStockStatus applies priority: out_of_stock → below_safety_stock → low_stock → normal.
func CalculateSKUStockStatus(stock, warningStock, safetyStock int) string {
	if stock <= 0 {
		return StockStatusOutOfStock
	}
	if safetyStock > 0 && stock <= safetyStock {
		return StockStatusBelowSafetyStock
	}
	if stock <= warningStock {
		return StockStatusLowStock
	}
	return StockStatusNormal
}

// ValidateSKUStockThresholds enforces non-negative lines and safety <= warning.
func ValidateSKUStockThresholds(warningStock, safetyStock int) error {
	if warningStock < 0 || safetyStock < 0 {
		return fmt.Errorf("warningStock and safetyStock must be >= 0")
	}
	if safetyStock > warningStock {
		return fmt.Errorf("safety_stock must not be greater than warning_stock")
	}
	return nil
}
