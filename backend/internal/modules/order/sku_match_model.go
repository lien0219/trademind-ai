package order

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

// SKU match audit row (one current row per order_item_id).
type OrderItemSKUMatch struct {
	model.Base
	OrderID         uuid.UUID      `gorm:"type:char(36);index;not null" json:"orderId"`
	OrderItemID     uuid.UUID      `gorm:"type:char(36);uniqueIndex;not null" json:"orderItemId"`
	Platform        string         `gorm:"size:64;index;not null" json:"platform"`
	ExternalOrderID *string        `gorm:"size:255;index" json:"externalOrderId,omitempty"`
	ExternalItemID  *string        `gorm:"size:255" json:"externalItemId,omitempty"`
	ExternalSKUID   *string        `gorm:"size:256" json:"externalSkuId,omitempty"`
	SellerSKU       string         `gorm:"size:128" json:"sellerSku,omitempty"`
	SKUCode         string         `gorm:"size:128" json:"skuCode,omitempty"`
	ProductID       *uuid.UUID     `gorm:"type:char(36);index" json:"productId,omitempty"`
	ProductSKUID    *uuid.UUID     `gorm:"type:char(36);index" json:"productSkuId,omitempty"`
	MatchType       string         `gorm:"size:64;index;not null" json:"matchType"`
	MatchStatus     string         `gorm:"size:32;index;not null" json:"matchStatus"`
	Confidence      int            `gorm:"default:0;not null" json:"confidence"`
	Reason          string         `gorm:"type:text" json:"reason,omitempty"`
	RawData         datatypes.JSON `gorm:"type:jsonb" json:"rawData,omitempty"`
	CreatedBy       *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
}

func (OrderItemSKUMatch) TableName() string { return "order_item_sku_matches" }

const (
	MatchTypePublicationSKUExternalID = "publication_sku_external_id"
	MatchTypePublicationSKUCode       = "publication_sku_code"
	MatchTypeLocalSKUCode             = "local_sku_code"
	MatchTypeManual                   = "manual"
	MatchTypeNone                     = "none"
)

const (
	MatchStatusMatched     = "matched"
	MatchStatusUnmatched   = "unmatched"
	MatchStatusAmbiguous   = "ambiguous"
	MatchStatusManualBound = "manual_bound"
	MatchStatusSkipped     = "skipped"
)

// OrderSKUSettings mirrors relevant inventory / bind settings (read from settings.inventory).
type OrderSKUSettings struct {
	AutoMatchOrderSKUs                bool
	AutoDeductAfterSKUMatch           bool
	AutoSyncInventoryAfterOrderDeduct bool
	AllowManualSkuBindAfterDeduct     bool
}

// MatchOrderItemResult is the outcome of MatchOrderItemToSKU before persistence.
type MatchOrderItemResult struct {
	MatchType        string
	MatchStatus      string
	Confidence       int
	Reason           string
	ProductID        *uuid.UUID
	ProductSKUID     *uuid.UUID
	RawData          map[string]any
	UpdateOrderLines bool
}

// MatchOrderSummary aggregates one order-level match pass.
type MatchOrderSummary struct {
	OrderID     uuid.UUID `json:"orderId"`
	ItemsTotal  int       `json:"itemsTotal"`
	Matched     int       `json:"matched"`
	Unmatched   int       `json:"unmatched"`
	Ambiguous   int       `json:"ambiguous"`
	Skipped     int       `json:"skipped"`
	ManualBound int       `json:"manualBound"`
	Preserved   int       `json:"preserved,omitempty"`
	Errors      []string  `json:"errors,omitempty"`
}

// SKUMatchListQuery filters global listing.
type SKUMatchListQuery struct {
	Page         int
	PageSize     int
	Platform     string
	ShopID       *uuid.UUID
	MatchStatus  string
	MatchType    string
	OrderID      *uuid.UUID
	ProductSKUID *uuid.UUID
	Start        *time.Time
	End          *time.Time
}

// SKUMatchListRow is API row with denormalized labels.
type SKUMatchListRow struct {
	OrderItemSKUMatch
	ShopName         string `json:"shopName,omitempty"`
	OrderNo          string `json:"orderNo,omitempty"`
	LineProductTitle string `json:"productTitle,omitempty"`
	LocalSkuCode     string `json:"localSkuCode,omitempty"`
}

// SKUMatchDetailDTO extends match rows with optional candidate SKUs (ambiguous).
type SKUMatchDetailDTO struct {
	OrderItemSKUMatch
	ProductTitle  string            `json:"productTitle,omitempty"`
	LocalSkuCode  string            `json:"localSkuCode,omitempty"`
	CandidateSKUs []SKUCandidateDTO `json:"candidateSkus,omitempty"`
}

// SKUCandidateDTO is a lightweight ambiguous match hint.
type SKUCandidateDTO struct {
	ProductSKUID uuid.UUID `json:"productSkuId"`
	ProductID    uuid.UUID `json:"productId"`
	SKUCode      string    `json:"skuCode"`
	SKUName      string    `json:"skuName,omitempty"`
	ProductTitle string    `json:"productTitle,omitempty"`
}
