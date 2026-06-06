package shop

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

// Shop is a unified storefront record (channels are not duplicated per table).
type Shop struct {
	model.Base
	TenantID        int64          `gorm:"default:0;index" json:"tenantId"`
	Platform        string         `gorm:"size:32;index;not null" json:"platform"`
	ShopName        string         `gorm:"size:255;not null" json:"shopName"`
	ShopCode        string         `gorm:"size:128;index" json:"shopCode,omitempty"`
	ExternalShopID  string         `gorm:"size:255;index" json:"externalShopId,omitempty"`
	Status          string         `gorm:"size:32;index;not null" json:"status"`
	AuthStatus      string         `gorm:"size:32;index;not null" json:"authStatus"`
	Region          string         `gorm:"size:64" json:"region,omitempty"`
	Currency        string         `gorm:"size:16" json:"currency,omitempty"`
	Timezone        string         `gorm:"size:128" json:"timezone,omitempty"`
	DefaultLanguage string         `gorm:"size:32" json:"defaultLanguage,omitempty"`
	Capabilities    datatypes.JSON `gorm:"type:jsonb" json:"capabilities,omitempty"`
	PlatformConfig  datatypes.JSON `gorm:"type:jsonb" json:"platformConfig,omitempty"`
	Remark          string         `gorm:"type:text" json:"remark,omitempty"`
	CreatedBy       *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
}

func (Shop) TableName() string { return "shops" }

// ShopAuthToken stores secrets for one shop (single row per shop in v1).
type ShopAuthToken struct {
	model.Base
	ShopID           uuid.UUID      `gorm:"type:char(36);uniqueIndex;not null" json:"shopId"`
	Platform         string         `gorm:"size:32;index;not null" json:"platform"`
	AuthType         string         `gorm:"size:32;index;not null" json:"authType"`
	AppKey           string         `gorm:"size:512" json:"appKey,omitempty"`
	AppSecretEnc     string         `gorm:"type:text" json:"-"`
	AccessTokenEnc   string         `gorm:"type:text" json:"-"`
	RefreshTokenEnc  string         `gorm:"type:text" json:"-"`
	SellerID         string         `gorm:"size:255" json:"sellerId,omitempty"`
	MerchantID       string         `gorm:"size:255" json:"merchantId,omitempty"`
	MarketplaceID    string         `gorm:"size:255" json:"marketplaceId,omitempty"`
	ExpiresAt        *time.Time     `json:"expiresAt,omitempty"`
	RefreshExpiresAt *time.Time     `json:"refreshExpiresAt,omitempty"`
	Scopes           datatypes.JSON `gorm:"type:jsonb" json:"scopes,omitempty"`
	AuthConfig       datatypes.JSON `gorm:"type:jsonb" json:"authConfig,omitempty"`
	RawData          datatypes.JSON `gorm:"type:jsonb" json:"rawData,omitempty"`
}

func (ShopAuthToken) TableName() string { return "shop_auth_tokens" }

// PlatformCategory caches marketplace categories for listing preparation.
type PlatformCategory struct {
	model.Base
	Platform   string         `gorm:"size:32;uniqueIndex:idx_platform_category;index;not null" json:"platform"`
	CategoryID string         `gorm:"size:128;uniqueIndex:idx_platform_category;not null" json:"categoryId"`
	ParentID   string         `gorm:"size:128;index" json:"parentId"`
	Name       string         `gorm:"size:512;index" json:"name"`
	Level      int            `gorm:"index" json:"level"`
	IsLeaf     bool           `gorm:"index;not null;default:false" json:"isLeaf"`
	Status     string         `gorm:"size:64;index" json:"status,omitempty"`
	Raw        datatypes.JSON `gorm:"type:jsonb" json:"raw,omitempty"`
	SyncedAt   *time.Time     `gorm:"index" json:"syncedAt,omitempty"`
}

func (PlatformCategory) TableName() string { return "platform_categories" }

// PlatformCategoryAttribute caches marketplace-required category attributes.
type PlatformCategoryAttribute struct {
	model.Base
	Platform    string         `gorm:"size:32;uniqueIndex:idx_platform_category_attr;index;not null" json:"platform"`
	CategoryID  string         `gorm:"size:128;uniqueIndex:idx_platform_category_attr;index;not null" json:"categoryId"`
	AttrID      string         `gorm:"size:128;uniqueIndex:idx_platform_category_attr;not null" json:"attrId"`
	Name        string         `gorm:"size:512;index" json:"name"`
	Required    bool           `gorm:"index;not null;default:false" json:"required"`
	ValueType   string         `gorm:"size:128" json:"valueType,omitempty"`
	Options     datatypes.JSON `gorm:"type:jsonb" json:"options,omitempty"`
	UnitOptions datatypes.JSON `gorm:"type:jsonb" json:"unitOptions,omitempty"`
	Raw         datatypes.JSON `gorm:"type:jsonb" json:"raw,omitempty"`
	SyncedAt    *time.Time     `gorm:"index" json:"syncedAt,omitempty"`
}

func (PlatformCategoryAttribute) TableName() string { return "platform_category_attributes" }
