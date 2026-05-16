package order

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

// Order is manually managed internal draft order (no marketplace sync).
type Order struct {
	model.Base
	TenantID          int64          `gorm:"default:0;index" json:"tenantId"`
	Platform          string         `gorm:"size:64;index;not null" json:"platform"`
	ShopID            *uuid.UUID     `gorm:"type:char(36);index" json:"shopId,omitempty"`
	ExternalOrderID   *string        `gorm:"size:255;index" json:"externalOrderId,omitempty"`
	OrderNo           string         `gorm:"size:128;uniqueIndex;not null" json:"orderNo"`
	CustomerName      string         `gorm:"size:255;index;not null" json:"customerName"`
	CustomerEmail     string         `gorm:"size:255" json:"customerEmail,omitempty"`
	CustomerPhone     string         `gorm:"size:64" json:"customerPhone,omitempty"`
	Status            string         `gorm:"size:32;index;not null" json:"status"`
	PaymentStatus     string         `gorm:"size:32;index;not null" json:"paymentStatus"`
	FulfillmentStatus string         `gorm:"size:32;index;not null" json:"fulfillmentStatus"`
	Currency          string         `gorm:"size:16;not null" json:"currency"`
	TotalAmount       float64        `gorm:"type:decimal(18,4);default:0" json:"totalAmount"`
	PaidAt            *time.Time     `json:"paidAt,omitempty"`
	OrderedAt         *time.Time     `json:"orderedAt,omitempty"`
	ShippedAt         *time.Time     `json:"shippedAt,omitempty"`
	DeliveredAt       *time.Time     `json:"deliveredAt,omitempty"`
	Remark            string         `gorm:"type:text" json:"remark,omitempty"`
	RawData           datatypes.JSON `gorm:"type:jsonb" json:"rawData,omitempty"`
	CreatedBy         *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`

	Items     []OrderItem     `gorm:"foreignKey:OrderID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"items,omitempty"`
	Shipments []OrderShipment `gorm:"foreignKey:OrderID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"shipments,omitempty"`
}

func (Order) TableName() string { return "orders" }

// OrderItem is a line item (optional link to product draft).
type OrderItem struct {
	model.HardDeleteBase
	OrderID        uuid.UUID      `gorm:"type:char(36);index;not null" json:"orderId"`
	ProductID      *uuid.UUID     `gorm:"type:char(36);index" json:"productId,omitempty"`
	ProductSKUID   *uuid.UUID     `gorm:"type:char(36);index" json:"productSkuId,omitempty"`
	ExternalItemID *string        `gorm:"size:255" json:"externalItemId,omitempty"`
	ProductTitle   string         `gorm:"size:512;not null" json:"productTitle"`
	SKUName        string         `gorm:"size:512" json:"skuName,omitempty"`
	SKUCode        string         `gorm:"size:128" json:"skuCode,omitempty"`
	Quantity       int            `gorm:"not null" json:"quantity"`
	UnitPrice      float64        `gorm:"type:decimal(18,4);default:0" json:"unitPrice"`
	TotalPrice     float64        `gorm:"type:decimal(18,4);default:0" json:"totalPrice"`
	ImageURL       string         `gorm:"type:text" json:"imageUrl,omitempty"`
	Attrs          datatypes.JSON `gorm:"type:jsonb" json:"attrs,omitempty"`
	RawData        datatypes.JSON `gorm:"type:jsonb" json:"rawData,omitempty"`
}

func (OrderItem) TableName() string { return "order_items" }

// OrderShipment is logistics info for one package / tracking segment.
type OrderShipment struct {
	model.HardDeleteBase
	OrderID     uuid.UUID      `gorm:"type:char(36);index;not null" json:"orderId"`
	Carrier     string         `gorm:"size:128;not null" json:"carrier"`
	TrackingNo  string         `gorm:"size:255;not null;index" json:"trackingNo"`
	TrackingURL string         `gorm:"type:text" json:"trackingUrl,omitempty"`
	Status      string         `gorm:"size:32;index;not null" json:"status"`
	ShippedAt   *time.Time     `json:"shippedAt,omitempty"`
	DeliveredAt *time.Time     `json:"deliveredAt,omitempty"`
	RawData     datatypes.JSON `gorm:"type:jsonb" json:"rawData,omitempty"`
}

func (OrderShipment) TableName() string { return "order_shipments" }
