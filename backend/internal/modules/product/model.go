package product

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

// Status constants for product drafts.
const (
	StatusDraft        = "draft"
	StatusAIProcessing = "ai_processing"
	StatusReady        = "ready"
	StatusPublished    = "published"
	StatusArchived     = "archived"
)

const (
	ImageTypeMain = "main"
	// ImageTypeDetail is the canonical type for gallery / detail images (API & UI use "detail").
	ImageTypeDetail = "detail"
	// ImageTypeSKU marks images associated with SKU variants.
	ImageTypeSKU = "sku"
	// ImageTypeDescription is a legacy value kept for rows imported before "detail" was introduced.
	ImageTypeDescription = "description"
)

// Product is a draft listing row (soft-deleted when removed).
type Product struct {
	model.Base
	TenantID      int64          `gorm:"default:0;index" json:"tenantId"`
	CreatedBy     *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	Source        string         `gorm:"size:64;index;not null" json:"source"`
	SourceURL     string         `gorm:"size:2048" json:"sourceUrl"`
	OriginalTitle string         `gorm:"size:512" json:"originalTitle"`
	Title         string         `gorm:"size:512;index" json:"title"`
	AITitle       string         `gorm:"size:512" json:"aiTitle"`
	Description   string         `gorm:"type:text" json:"description"`
	AIDescription string         `gorm:"type:text" json:"aiDescription"`
	Currency      string         `gorm:"size:16" json:"currency"`
	Status        string         `gorm:"size:32;index;not null" json:"status"`
	RawData       datatypes.JSON `gorm:"type:jsonb" json:"rawData,omitempty"`

	Images []ProductImage `gorm:"foreignKey:ProductID" json:"images,omitempty"`
	SKUs   []ProductSKU   `gorm:"foreignKey:ProductID" json:"skus,omitempty"`
}

func (Product) TableName() string { return "products" }

// ProductImage links remote or stored images to a product.
type ProductImage struct {
	model.HardDeleteBase
	ProductID uuid.UUID `gorm:"type:char(36);index;not null" json:"productId"`
	ImageType string    `gorm:"size:32;index;not null" json:"imageType"`
	OriginURL string    `gorm:"size:2048" json:"originUrl"`
	ObjectKey string    `gorm:"size:512" json:"objectKey"`
	PublicURL string    `gorm:"size:2048" json:"publicUrl"`
	SortOrder int       `gorm:"index" json:"sortOrder"`
}

func (ProductImage) TableName() string { return "product_images" }

// ProductSKU stores normalized SKU rows for a draft product.
type ProductSKU struct {
	model.HardDeleteBase
	ProductID uuid.UUID      `gorm:"type:char(36);index;not null" json:"productId"`
	SKUCode   string         `gorm:"size:128;index" json:"skuCode"`
	SKUName   string         `gorm:"size:512" json:"skuName"`
	Attrs     datatypes.JSON `gorm:"type:jsonb" json:"attrs,omitempty"`
	Price     *float64       `json:"price,omitempty"`
	// CostPrice: collected cost / purchase price (optional).
	CostPrice *float64 `gorm:"column:cost_price" json:"costPrice,omitempty"`
	// CompareAtPrice: optional strikethrough / list price.
	CompareAtPrice *float64 `gorm:"column:compare_at_price" json:"compareAtPrice,omitempty"`
	// MinPublishPrice: per-SKU floor when applying pricing rules.
	MinPublishPrice *float64 `gorm:"column:min_publish_price" json:"minPublishPrice,omitempty"`
	Stock           *int     `json:"stock,omitempty"`
	// WarningStock: local alert line; new rows default from settings.inventory.default_warning_stock (fallback 5).
	WarningStock int `gorm:"column:warning_stock;default:5;not null" json:"warningStock"`
	// SafetyStock: optional lower bound; must be <= WarningStock; 0 means unset for below-safety comparisons except stock<=0 still out_of_stock.
	SafetyStock int `gorm:"column:safety_stock;default:0;not null" json:"safetyStock"`
	// StockStatus optional persisted hint (normal/low_stock/below_safety_stock/out_of_stock); APIs may compute dynamically instead.
	StockStatus string `gorm:"column:stock_status;size:32;index" json:"stockStatus,omitempty"`
	// LastStockCheckedAt optional audit timestamp for future batch jobs.
	LastStockCheckedAt *time.Time     `gorm:"column:last_stock_checked_at" json:"lastStockCheckedAt,omitempty"`
	ImageURL           string         `gorm:"size:2048" json:"imageUrl"`
	RawData            datatypes.JSON `gorm:"type:jsonb" json:"rawData,omitempty"`
}

func (ProductSKU) TableName() string { return "product_skus" }
