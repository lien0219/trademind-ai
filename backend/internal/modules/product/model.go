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
	// ImageTypeMarketing marks promotional / listing marketing images.
	ImageTypeMarketing = "marketing"
	// ImageTypeAIGenerated marks AI-processed images saved to the product library.
	ImageTypeAIGenerated = "ai_generated"
	// ImageTypeSKU marks images associated with SKU variants.
	ImageTypeSKU = "sku"
	// ImageTypeDescription is a legacy value kept for rows imported before "detail" was introduced.
	ImageTypeDescription = "description"
)

const (
	ImageSourceCollect = "collect"
	ImageSourceUpload  = "upload"
	ImageSourceAI      = "ai"
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
	AITitle       string         `gorm:"column:ai_title;size:512" json:"aiTitle"`
	Description   string         `gorm:"type:text" json:"description"`
	AIDescription string         `gorm:"column:ai_description;type:text" json:"aiDescription"`
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
	ProductID       uuid.UUID  `gorm:"type:char(36);index;not null" json:"productId"`
	ImageType       string     `gorm:"size:32;index;not null" json:"imageType"`
	Source          string     `gorm:"size:32;index" json:"source,omitempty"`
	SourceTaskID    *uuid.UUID `gorm:"type:char(36);index" json:"sourceTaskId,omitempty"`
	OriginalImageID *uuid.UUID `gorm:"type:char(36);index" json:"originalImageId,omitempty"`
	OriginURL       string     `gorm:"size:2048" json:"originUrl"`
	ObjectKey       string     `gorm:"size:512" json:"objectKey"`
	StorageKey      string     `gorm:"size:512" json:"storageKey,omitempty"`
	PublicURL       string     `gorm:"size:2048" json:"publicUrl"`
	Score           *float64   `json:"score,omitempty"`
	IsBestMain      bool       `gorm:"default:false" json:"isBestMain"`
	SortOrder       int        `gorm:"index" json:"sortOrder"`
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

// ProductPlatformPublishConfig stores per-product marketplace listing prep data.
// It is separate from collect raw_data so operator selections never mutate source raw.
type ProductPlatformPublishConfig struct {
	model.HardDeleteBase
	ProductID          uuid.UUID      `gorm:"type:char(36);uniqueIndex:idx_product_platform_publish_config;index;not null" json:"productId"`
	Platform           string         `gorm:"size:64;uniqueIndex:idx_product_platform_publish_config;index;not null" json:"platform"`
	ShopID             *uuid.UUID     `gorm:"type:char(36);index" json:"shopId,omitempty"`
	CategoryID         string         `gorm:"size:128;index" json:"categoryId,omitempty"`
	CategoryPath       string         `gorm:"size:1024" json:"categoryPath,omitempty"`
	PlatformAttributes datatypes.JSON `gorm:"type:jsonb" json:"platformAttributes,omitempty"`
	MappedTitle        string         `gorm:"size:512" json:"mappedTitle,omitempty"`
	MappedDescription  string         `gorm:"type:text" json:"mappedDescription,omitempty"`
	MappedImages       datatypes.JSON `gorm:"type:jsonb" json:"mappedImages,omitempty"`
	MappedSKUs         datatypes.JSON `gorm:"column:mapped_skus;type:jsonb" json:"mappedSkus,omitempty"`
	MappedPrice        datatypes.JSON `gorm:"type:jsonb" json:"mappedPrice,omitempty"`
	MappedStock        datatypes.JSON `gorm:"type:jsonb" json:"mappedStock,omitempty"`
	MappingWarnings    datatypes.JSON `gorm:"type:jsonb" json:"mappingWarnings,omitempty"`
	MappingErrors      datatypes.JSON `gorm:"type:jsonb" json:"mappingErrors,omitempty"`
	LastMappedAt       *time.Time     `gorm:"index" json:"lastMappedAt,omitempty"`
}

func (ProductPlatformPublishConfig) TableName() string { return "product_platform_publish_configs" }

const (
	AIContentFieldTitle         = "ai_title"
	AIContentFieldDescription   = "ai_description"
	AIContentApplyStatusApplied = "applied"
	AIContentApplyStatusUndone  = "undone"
)

// ProductAIContentApplication stores the minimal snapshot needed to safely undo
// an AI suggestion application without overwriting later manual edits.
type ProductAIContentApplication struct {
	model.HardDeleteBase
	ProductID          uuid.UUID  `gorm:"type:char(36);index;index:idx_product_ai_content_recent,priority:1;not null" json:"productId"`
	FieldType          string     `gorm:"size:32;index;index:idx_product_ai_content_recent,priority:2;not null" json:"fieldType"`
	AITaskID           *uuid.UUID `gorm:"type:char(36);index" json:"aiTaskId,omitempty"`
	PreviousValue      string     `gorm:"type:text" json:"previousValue,omitempty"`
	AppliedValue       string     `gorm:"type:text" json:"appliedValue,omitempty"`
	SourceSnapshotHash string     `gorm:"size:128;index" json:"sourceSnapshotHash,omitempty"`
	ExpectedUpdatedAt  *time.Time `json:"expectedUpdatedAt,omitempty"`
	AppliedBy          *uuid.UUID `gorm:"type:char(36);index" json:"appliedBy,omitempty"`
	AppliedAt          time.Time  `gorm:"index;index:idx_product_ai_content_recent,priority:4" json:"appliedAt"`
	UndoneBy           *uuid.UUID `gorm:"type:char(36);index" json:"undoneBy,omitempty"`
	UndoneAt           *time.Time `json:"undoneAt,omitempty"`
	Status             string     `gorm:"size:32;index;index:idx_product_ai_content_recent,priority:3;not null" json:"status"`
}

func (ProductAIContentApplication) TableName() string { return "product_ai_content_applications" }

const (
	ImageApplyStatusApplied = "applied"
	ImageApplyStatusUndone  = "undone"
)

// ProductImageApplication stores snapshot for safely undoing AI image apply.
type ProductImageApplication struct {
	model.HardDeleteBase
	ProductID          uuid.UUID      `gorm:"type:char(36);index;not null" json:"productId"`
	ProductImageID     uuid.UUID      `gorm:"type:char(36);index;not null" json:"productImageId"`
	ApplyMode          string         `gorm:"size:32;index;not null" json:"applyMode"`
	ImageTaskID        *uuid.UUID     `gorm:"type:char(36);index" json:"imageTaskId,omitempty"`
	BatchItemID        *uuid.UUID     `gorm:"type:char(36);index" json:"batchItemId,omitempty"`
	PreviousSnapshot   datatypes.JSON `gorm:"type:jsonb" json:"previousSnapshot,omitempty"`
	SourceSnapshotHash string         `gorm:"size:128;index" json:"sourceSnapshotHash,omitempty"`
	AppliedBy          *uuid.UUID     `gorm:"type:char(36);index" json:"appliedBy,omitempty"`
	AppliedAt          time.Time      `gorm:"index" json:"appliedAt"`
	UndoneBy           *uuid.UUID     `gorm:"type:char(36);index" json:"undoneBy,omitempty"`
	UndoneAt           *time.Time     `json:"undoneAt,omitempty"`
	Status             string         `gorm:"size:32;index;not null" json:"status"`
}

func (ProductImageApplication) TableName() string { return "product_image_applications" }
