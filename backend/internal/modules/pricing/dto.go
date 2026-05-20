package pricing

import (
	"github.com/google/uuid"
)

// CalculateBody binds POST /api/v1/pricing/calculate.
type CalculateBody struct {
	ProductSkuID *uuid.UUID `json:"productSkuId"`
	BasePrice    *float64   `json:"basePrice"`
	CostPrice    *float64   `json:"costPrice"`
	Platform     string     `json:"platform"`
	Currency     string     `json:"currency"`
	Rule         Rule       `json:"rule"`
}

// CalculateResponse is returned by calculate endpoint.
type CalculateResponse struct {
	BasePrice       float64  `json:"basePrice"`
	CostPrice       *float64 `json:"costPrice,omitempty"`
	CurrentPrice    *float64 `json:"currentPrice,omitempty"`
	CalculatedPrice float64  `json:"calculatedPrice"`
	Currency        string   `json:"currency"`
}

// ProductApplyBody binds POST /api/v1/products/:id/pricing/apply.
type ProductApplyBody struct {
	Platform string      `json:"platform"`
	Rule     Rule        `json:"rule"`
	SkuIDs   []uuid.UUID `json:"skuIds"`
	Confirm  bool        `json:"confirm"`
}

// BatchApplyFilters narrows products for batch pricing.
type BatchApplyFilters struct {
	Status  string `json:"status"`
	Source  string `json:"source"`
	Keyword string `json:"keyword"`
}

// BatchApplyBody binds POST /api/v1/products/pricing/batch-apply.
type BatchApplyBody struct {
	ProductIDs []uuid.UUID       `json:"productIds"`
	Filters    BatchApplyFilters `json:"filters"`
	Platform   string            `json:"platform"`
	Rule       Rule              `json:"rule"`
	Confirm    bool              `json:"confirm"`
	ConfirmAll bool              `json:"confirmAll"`
}

// PreviewLine is one SKU row in apply preview.
type PreviewLine struct {
	ProductSkuID    string   `json:"productSkuId"`
	ProductID       string   `json:"productId"`
	SKUCode         string   `json:"skuCode"`
	SKUName         string   `json:"skuName"`
	CostPrice       *float64 `json:"costPrice,omitempty"`
	CurrentPrice    *float64 `json:"currentPrice,omitempty"`
	CalculatedPrice float64  `json:"calculatedPrice"`
	Delta           float64  `json:"delta"`
}

// ApplySummary is returned after apply operations.
type ApplySummary struct {
	ProductCount int           `json:"productCount"`
	SkuCount     int           `json:"skuCount"`
	Updated      int           `json:"updated"`
	Skipped      int           `json:"skipped"`
	Preview      []PreviewLine `json:"preview,omitempty"`
}
