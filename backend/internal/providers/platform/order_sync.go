package platform

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// OrderSyncProvider extends Provider with marketplace order fetch (implemented per channel).
type OrderSyncProvider interface {
	Provider
	SyncOrders(ctx context.Context, req SyncOrdersRequest) (*SyncOrdersResult, error)
}

// SyncOrdersRequest is passed to a provider adapter (never log Auth secrets).
type SyncOrdersRequest struct {
	ShopID    uuid.UUID
	Platform  string
	Auth      TestConnectionRequest
	Mode      string
	StartTime *time.Time
	EndTime   *time.Time
	Cursor    string
	Limit     int
}

// SyncOrdersResult is one page of normalized orders plus paging hints.
type SyncOrdersResult struct {
	Orders     []PlatformOrder
	NextCursor string
	HasMore    bool
	RawSummary map[string]any
}

// PlatformOrder is provider-neutral order snapshot for persistence (no TikTok/Shopee-specific fields).
type PlatformOrder struct {
	ExternalOrderID   string
	OrderNo           string
	CustomerName      string
	Status            string
	PaymentStatus     string
	FulfillmentStatus string
	Currency          string
	TotalAmount       float64
	OrderedAt         *time.Time
	PaidAt            *time.Time
	ShippedAt         *time.Time
	DeliveredAt       *time.Time
	Items             []PlatformOrderItem
	Shipments         []PlatformShipment
	RawData           map[string]any
}

// PlatformOrderItem is a line item snapshot.
type PlatformOrderItem struct {
	ExternalItemID string
	ExternalSKUID  string // platform listing / SKU identifier (e.g. TikTok sku_id, Shopee model_id)
	SellerSKU      string // seller-facing SKU code when distinct from normalized SKUCode
	ProductTitle   string
	SKUName        string
	SKUCode        string
	Quantity       int
	UnitPrice      float64
	TotalPrice     float64
	ImageURL       string
	Attrs          map[string]any
	RawData        map[string]any
}

// PlatformShipment is logistics snapshot.
type PlatformShipment struct {
	Carrier     string
	TrackingNo  string
	TrackingURL string
	Status      string
	ShippedAt   *time.Time
	DeliveredAt *time.Time
	RawData     map[string]any
}

// HasCapability reports whether p advertises cap (registry meta).
func HasCapability(p Provider, want Capability) bool {
	if p == nil {
		return false
	}
	for _, c := range p.Capabilities() {
		if c == want {
			return true
		}
	}
	return false
}

// AsOrderSync type-asserts to OrderSyncProvider.
func AsOrderSync(p Provider) (OrderSyncProvider, bool) {
	if p == nil {
		return nil, false
	}
	os, ok := p.(OrderSyncProvider)
	return os, ok
}
