package product

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// CreateBody binds POST /products.
type CreateBody struct {
	TenantID      int64           `json:"tenantId"`
	Source        string          `json:"source"`
	SourceURL     string          `json:"sourceUrl"`
	OriginalTitle string          `json:"originalTitle"`
	Title         string          `json:"title"`
	Description   string          `json:"description"`
	Currency      string          `json:"currency"`
	Status        string          `json:"status"`
	RawData       json.RawMessage `json:"rawData"`
}

// UpdateBody binds PUT /products/:id.
// Source / source URL / rawData are not editable here (采集来源与原始归一数据只读).
// JSON accepts camelCase and snake_case for the text fields (e.g. original_title, ai_title).
type UpdateBody struct {
	OriginalTitle *string `json:"originalTitle"`
	Title         *string `json:"title"`
	AITitle       *string `json:"aiTitle"`
	Description   *string `json:"description"`
	AIDescription *string `json:"aiDescription"`
	Currency      *string `json:"currency"`
	Status        *string `json:"status"`
}

// PlatformPublishConfigBody binds PUT /products/:id/platform-configs/:platform.
type PlatformPublishConfigBody struct {
	ShopID             string          `json:"shopId"`
	CategoryID         string          `json:"categoryId"`
	CategoryPath       string          `json:"categoryPath"`
	PlatformAttributes json.RawMessage `json:"platformAttributes"`
}

type PlatformPublishConfigDTO struct {
	ProductID          uuid.UUID           `json:"productId"`
	Platform           string              `json:"platform"`
	ShopID             *uuid.UUID          `json:"shopId,omitempty"`
	CategoryID         string              `json:"categoryId,omitempty"`
	CategoryPath       string              `json:"categoryPath,omitempty"`
	PlatformAttributes json.RawMessage     `json:"platformAttributes,omitempty"`
	Mapping            *DouyinDraftMapping `json:"mapping,omitempty"`
	LastMappedAt       *time.Time          `json:"lastMappedAt,omitempty"`
	CreatedAt          time.Time           `json:"createdAt"`
	UpdatedAt          time.Time           `json:"updatedAt"`
}

// UnmarshalJSON merges alternate snake_case keys with camelCase.
func (b *UpdateBody) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	pickStr := func(keys ...string) *string {
		for _, k := range keys {
			v, ok := raw[k]
			if !ok {
				continue
			}
			if string(v) == "null" {
				empty := ""
				return &empty
			}
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				continue
			}
			return &s
		}
		return nil
	}
	b.OriginalTitle = pickStr("originalTitle", "original_title")
	b.Title = pickStr("title")
	b.AITitle = pickStr("aiTitle", "ai_title")
	b.Description = pickStr("description")
	b.AIDescription = pickStr("aiDescription", "ai_description")
	b.Currency = pickStr("currency")
	b.Status = pickStr("status")
	return nil
}

// SKUBody binds POST /products/:id/skus.
type SKUBody struct {
	SKUCode         string          `json:"skuCode"`
	SKUName         string          `json:"skuName"`
	Attrs           json.RawMessage `json:"attrs"`
	Price           *float64        `json:"price"`
	CostPrice       *float64        `json:"costPrice"`
	CompareAtPrice  *float64        `json:"compareAtPrice"`
	MinPublishPrice *float64        `json:"minPublishPrice"`
	Stock           *int            `json:"stock"`
	ImageURL        string          `json:"imageUrl"`
}

// SKUUpdateBody binds PUT /products/:id/skus/:skuId (partial).
type SKUUpdateBody struct {
	SKUCode         *string          `json:"skuCode"`
	SKUName         *string          `json:"skuName"`
	Attrs           *json.RawMessage `json:"attrs"`
	Price           *float64         `json:"price"`
	CostPrice       *float64         `json:"costPrice"`
	CompareAtPrice  *float64         `json:"compareAtPrice"`
	MinPublishPrice *float64         `json:"minPublishPrice"`
	Stock           *int             `json:"stock"`
	ImageURL        *string          `json:"imageUrl"`
}

// SKUStockSettingsBody binds PUT /products/:id/skus/:skuId/stock-settings.
type SKUStockSettingsBody struct {
	WarningStock int `json:"warningStock"`
	SafetyStock  int `json:"safetyStock"`
}

// ImageCreateBody binds POST /products/:id/images.
type ImageCreateBody struct {
	FileID          *uuid.UUID `json:"fileId"`
	ObjectKey       string     `json:"objectKey"`
	StorageKey      string     `json:"storageKey"`
	OriginURL       string     `json:"originUrl"`
	PublicURL       string     `json:"publicUrl"`
	ImageType       string     `json:"imageType"`
	Source          string     `json:"source"`
	SourceTaskID    *uuid.UUID `json:"sourceTaskId"`
	OriginalImageID *uuid.UUID `json:"originalImageId"`
	Score           *float64   `json:"score"`
	IsBestMain      *bool      `json:"isBestMain"`
	SortOrder       *int       `json:"sortOrder"`
}

// ImageUpdateBody binds PUT /products/:id/images/:imageId.
type ImageUpdateBody struct {
	ImageType  *string  `json:"imageType"`
	ObjectKey  *string  `json:"objectKey"`
	StorageKey *string  `json:"storageKey"`
	OriginURL  *string  `json:"originUrl"`
	PublicURL  *string  `json:"publicUrl"`
	Score      *float64 `json:"score"`
	IsBestMain *bool    `json:"isBestMain"`
	SortOrder  *int     `json:"sortOrder"`
}

// ImageReorderBody binds POST /products/:id/images/reorder.
type ImageReorderBody struct {
	ImageIDs []uuid.UUID `json:"imageIds"`
}

// ListQuery binds GET /products.
type ListQuery struct {
	Page     int
	PageSize int
	Status   string
	Source   string
	Keyword  string
	// Dashboard deep-link filters (optional).
	MissingAiTitle       bool
	MissingAiDescription bool
	ReadinessBlocked     bool
	Publishable          bool
}

// ListItem is one row for draft list (includes optional cover).
type ListItem struct {
	ID        uuid.UUID  `json:"id"`
	TenantID  int64      `json:"tenantId"`
	CreatedBy *uuid.UUID `json:"createdBy,omitempty"`
	Source    string     `json:"source"`
	SourceURL string     `json:"sourceUrl"`
	Title     string     `json:"title"`
	Status    string     `json:"status"`
	Currency  string     `json:"currency"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	CoverURL  string     `json:"coverUrl,omitempty"`
}

// ListResult paginates products.
type ListResult struct {
	Items      []ListItem `json:"list"`
	Total      int64      `json:"total"`
	Page       int        `json:"page"`
	PageSize   int        `json:"pageSize"`
	TotalPages int        `json:"totalPages"`
}

// DetailDTO is product detail with nested images and SKUs.
type DetailDTO struct {
	ID                uuid.UUID       `json:"id"`
	TenantID          int64           `json:"tenantId"`
	CreatedBy         *uuid.UUID      `json:"createdBy,omitempty"`
	Source            string          `json:"source"`
	SourceURL         string          `json:"sourceUrl"`
	OriginalTitle     string          `json:"originalTitle"`
	Title             string          `json:"title"`
	AITitle           string          `json:"aiTitle"`
	Description       string          `json:"description"`
	AIDescription     string          `json:"aiDescription"`
	Currency          string          `json:"currency"`
	Status            string          `json:"status"`
	RawData           json.RawMessage `json:"rawData,omitempty"`
	Raw               json.RawMessage `json:"raw,omitempty"`
	MainImages        []string        `json:"mainImages"`
	DescriptionImages []string        `json:"descriptionImages"`
	Attributes        json.RawMessage `json:"attributes,omitempty"`
	SKUGroups         json.RawMessage `json:"skuGroups,omitempty"`
	CostPrice         *float64        `json:"costPrice,omitempty"`
	SalePrice         *float64        `json:"salePrice,omitempty"`
	Stock             *int            `json:"stock,omitempty"`
	CollectWarnings   []string        `json:"collectWarnings"`
	PublishStatus     string          `json:"publishStatus"`
	CreatedAt         time.Time       `json:"createdAt"`
	UpdatedAt         time.Time       `json:"updatedAt"`
	Images            []ProductImage  `json:"images"`
	SKUs              []ProductSKU    `json:"skus"`
}

// ImportDraftParams converts collector output into a product draft (no collect package import).
type ImportDraftParams struct {
	Source             string
	SourceURL          string
	Title              string
	Currency           string
	Description        string
	MainImages         []string
	DescriptionImages  []string
	SKUs               []ImportSKUParams
	FullNormalizedJSON json.RawMessage
}

// ImportSKUParams is one SKU line from a normalized product.
type ImportSKUParams struct {
	SKUCode   string
	SKUName   string
	Attrs     json.RawMessage
	Price     *float64
	CostPrice *float64
	Stock     *int
	ImageURL  string
	RawSKU    json.RawMessage
}

// OptimizeTitleBody binds POST /products/:id/ai/optimize-title.
type OptimizeTitleBody struct {
	Language  string `json:"language"`
	Platform  string `json:"platform"`
	MaxLength int    `json:"maxLength"`
	Tone      string `json:"tone"` // optional; included in prompt vars when set
}

// AITitleRunExtra links a title optimization to a bulk batch and optional field writes.
type AITitleRunExtra struct {
	BatchID         *uuid.UUID
	BatchNo         string
	SkipSingleOpLog bool
	SaveAIField     bool // updates products.ai_title only; never products.title
}

// OptimizeTitleResult is returned after an AI title optimization call.
type OptimizeTitleResult struct {
	OptimizedTitle string   `json:"optimizedTitle"`
	Keywords       []string `json:"keywords"`
	Reason         string   `json:"reason"`
	TaskID         string   `json:"taskId"`
}

// ApplyAITitleBody binds POST /products/:id/apply-ai-title.
type ApplyAITitleBody struct {
	AITitle string `json:"aiTitle"`
	TaskID  string `json:"taskId"`
}

// GenerateDescriptionBody binds POST /products/:id/ai/generate-description.
type GenerateDescriptionBody struct {
	Language string `json:"language"`
	Platform string `json:"platform"`
	Tone     string `json:"tone"`
}

// GenerateDescriptionResult is returned after an AI description generation call.
type GenerateDescriptionResult struct {
	Description     string   `json:"description"`
	Highlights      []string `json:"highlights"`
	Specifications  []string `json:"specifications"`
	PackageIncludes []string `json:"packageIncludes"`
	Notes           string   `json:"notes"`
	Reason          string   `json:"reason"`
	TaskID          string   `json:"taskId"`
}

// AIDescriptionRunExtra links description generation to a bulk batch.
type AIDescriptionRunExtra struct {
	BatchID         *uuid.UUID
	BatchNo         string
	SkipSingleOpLog bool
	SaveAIField     bool // updates products.ai_description only
}

// ApplyAIDescriptionBody binds POST /products/:id/apply-ai-description.
type ApplyAIDescriptionBody struct {
	AIDescription string `json:"aiDescription"`
	TaskID        string `json:"taskId"`
}
