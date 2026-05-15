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
type UpdateBody struct {
	Source        *string         `json:"source"`
	SourceURL     *string         `json:"sourceUrl"`
	OriginalTitle *string         `json:"originalTitle"`
	Title         *string         `json:"title"`
	AITitle       *string         `json:"aiTitle"`
	Description   *string         `json:"description"`
	AIDescription *string         `json:"aiDescription"`
	Currency      *string         `json:"currency"`
	Status        *string         `json:"status"`
	RawData       json.RawMessage `json:"rawData"`
}

// ListQuery binds GET /products.
type ListQuery struct {
	Page     int
	PageSize int
	Status   string
	Source   string
	Keyword  string
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
	ID            uuid.UUID       `json:"id"`
	TenantID      int64           `json:"tenantId"`
	CreatedBy     *uuid.UUID      `json:"createdBy,omitempty"`
	Source        string          `json:"source"`
	SourceURL     string          `json:"sourceUrl"`
	OriginalTitle string          `json:"originalTitle"`
	Title         string          `json:"title"`
	AITitle       string          `json:"aiTitle"`
	Description   string          `json:"description"`
	AIDescription string          `json:"aiDescription"`
	Currency      string          `json:"currency"`
	Status        string          `json:"status"`
	RawData       json.RawMessage `json:"rawData,omitempty"`
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
	Images        []ProductImage  `json:"images"`
	SKUs          []ProductSKU    `json:"skus"`
}

// ImportDraftParams converts collector output into a product draft (no collect package import).
type ImportDraftParams struct {
	Source             string
	SourceURL          string
	Title              string
	Currency           string
	MainImages         []string
	DescriptionImages  []string
	SKUs               []ImportSKUParams
	FullNormalizedJSON json.RawMessage
}

// ImportSKUParams is one SKU line from a normalized product.
type ImportSKUParams struct {
	SKUCode  string
	SKUName  string
	Attrs    json.RawMessage
	Price    *float64
	Stock    *int
	ImageURL string
	RawSKU   json.RawMessage
}
