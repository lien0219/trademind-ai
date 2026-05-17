package platform

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

// PlatformProductDraft is a normalized publish payload consumed by adapters.
type PlatformProductDraft struct {
	ProductID        uuid.UUID
	Title            string
	Description      string
	Currency         string
	Images           []PlatformProductImage
	SKUs             []PlatformProductSKU
	Attributes       map[string]any
	RawData          map[string]any
	SourceProductRow map[string]any // trimmed summary only — no tokens in workers
}

// PlatformProductImage is one gallery/sku-linked image.
type PlatformProductImage struct {
	URL       string
	Type      string // main | detail | sku
	SortOrder int
}

// PlatformProductSKU is one variant line.
type PlatformProductSKU struct {
	LocalSKUID uuid.UUID
	SKUCode    string
	SKUName    string
	Attrs      map[string]any
	Price      float64
	Stock      int
	ImageURL   string
}

// PublishProductRequest is passed to adapters (never log Auth secrets).
type PublishProductRequest struct {
	ShopID    uuid.UUID
	Platform  string
	Auth      TestConnectionRequest
	Product   PlatformProductDraft
	// PublishConfig is merged defaults (settings.platform_publish_*) plus per-task overrides — stringified scalars plus bools encoded as strings.
	PublishConfig map[string]any
	Options       map[string]any
}

// PlatformSKUMapping links local SKU to remote listing SKU.
type PlatformSKUMapping struct {
	LocalSKUID    uuid.UUID
	ExternalSKUID string
	SKUCode       string
}

// PublishProductResult is persisted as a trimmed summary in task output / publication rows (no secrets).
type PublishProductResult struct {
	ExternalProductID string
	ExternalSPUID     string
	ExternalURL       string
	Status            string
	SKUMappings       []PlatformSKUMapping
	RawSummary        map[string]any
}

// ProductPublishProvider publishes a unified draft via a marketplace API (optional per adapter).
type ProductPublishProvider interface {
	PublishProduct(ctx context.Context, req PublishProductRequest) (*PublishProductResult, error)
}

// AsProductPublish type-asserts to ProductPublishProvider.
func AsProductPublish(p Provider) (ProductPublishProvider, bool) {
	if p == nil {
		return nil, false
	}
	pp, ok := p.(ProductPublishProvider)
	return pp, ok
}

// ProductPublishImplementationStatus summarizes listings publish rollout for /platform/providers (product_publish capability).
func ProductPublishImplementationStatus(p Provider) string {
	if p == nil {
		return StatusDisabled
	}
	switch strings.TrimSpace(strings.ToLower(p.Platform())) {
	case "mock":
		return StatusAvailable
	case "manual":
		return StatusDisabled
	case "tiktok", "shopee", "lazada", "amazon":
		return StatusPlanned
	default:
		if !HasCapability(p, CapProductPublish) {
			return StatusDisabled
		}
		return StatusPlanned
	}
}
