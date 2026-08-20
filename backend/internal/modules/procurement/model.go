package procurement

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

const (
	StatusDraft             = "draft"
	StatusPendingApproval   = "pending_approval"
	StatusApproved          = "approved"
	StatusPartiallyReceived = "partially_received"
	StatusReceived          = "received"
	StatusClosed            = "closed"
	StatusCancelled         = "cancelled"
)

type PurchaseOrder struct {
	model.Base
	TenantID         int64               `gorm:"not null;uniqueIndex:ux_purchase_order_tenant_no;uniqueIndex:ux_purchase_order_idempotency;index" json:"tenantId"`
	PurchaseOrderNo  string              `gorm:"size:64;not null;uniqueIndex:ux_purchase_order_tenant_no" json:"purchaseOrderNo"`
	IdempotencyKey   string              `gorm:"size:128;not null;uniqueIndex:ux_purchase_order_idempotency" json:"idempotencyKey"`
	PayloadHash      string              `gorm:"size:64;not null" json:"-"`
	SupplierID       uuid.UUID           `gorm:"type:char(36);not null;index" json:"supplierId"`
	WarehouseID      uuid.UUID           `gorm:"type:char(36);not null;index" json:"warehouseId"`
	Status           string              `gorm:"size:32;not null;index" json:"status"`
	Currency         string              `gorm:"size:8;not null" json:"currency"`
	TotalAmountMinor int64               `gorm:"not null;default:0" json:"totalAmountMinor"`
	Revision         int                 `gorm:"not null;default:1" json:"revision"`
	Remark           string              `gorm:"size:1000" json:"remark,omitempty"`
	CreatedBy        *uuid.UUID          `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	ApprovedBy       *uuid.UUID          `gorm:"type:char(36);index" json:"approvedBy,omitempty"`
	ApprovedAt       *time.Time          `json:"approvedAt,omitempty"`
	ClosedAt         *time.Time          `json:"closedAt,omitempty"`
	CancelledAt      *time.Time          `json:"cancelledAt,omitempty"`
	Items            []PurchaseOrderItem `gorm:"foreignKey:PurchaseOrderID" json:"items,omitempty"`
}

func (PurchaseOrder) TableName() string { return "purchase_orders" }

type PurchaseOrderItem struct {
	model.HardDeleteBase
	TenantID         int64      `gorm:"not null;index" json:"tenantId"`
	PurchaseOrderID  uuid.UUID  `gorm:"type:char(36);not null;index;uniqueIndex:ux_purchase_order_item_sku" json:"purchaseOrderId"`
	ProductSKUID     uuid.UUID  `gorm:"column:product_sku_id;type:char(36);not null;index;uniqueIndex:ux_purchase_order_item_sku" json:"productSkuId"`
	SupplierSKUID    *uuid.UUID `gorm:"column:supplier_sku_id;type:char(36);index" json:"supplierSkuId,omitempty"`
	Quantity         int        `gorm:"not null" json:"quantity"`
	ReceivedQuantity int        `gorm:"not null;default:0" json:"receivedQuantity"`
	UnitCostMinor    int64      `gorm:"not null" json:"unitCostMinor"`
	LineAmountMinor  int64      `gorm:"not null" json:"lineAmountMinor"`
	ProductTitle     string     `gorm:"-" json:"productTitle,omitempty"`
	SKUCode          string     `gorm:"-" json:"skuCode,omitempty"`
	SKUName          string     `gorm:"-" json:"skuName,omitempty"`
}

func (PurchaseOrderItem) TableName() string { return "purchase_order_items" }

type GoodsReceipt struct {
	model.HardDeleteBase
	TenantID        int64              `gorm:"not null;uniqueIndex:ux_goods_receipt_idempotency;uniqueIndex:ux_goods_receipt_tenant_no;index" json:"tenantId"`
	ReceiptNo       string             `gorm:"size:64;not null;uniqueIndex:ux_goods_receipt_tenant_no" json:"receiptNo"`
	PurchaseOrderID uuid.UUID          `gorm:"type:char(36);not null;uniqueIndex:ux_goods_receipt_idempotency;index" json:"purchaseOrderId"`
	WarehouseID     uuid.UUID          `gorm:"type:char(36);not null;index" json:"warehouseId"`
	IdempotencyKey  string             `gorm:"size:128;not null;uniqueIndex:ux_goods_receipt_idempotency" json:"idempotencyKey"`
	PayloadHash     string             `gorm:"size:64;not null" json:"-"`
	CreatedBy       *uuid.UUID         `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	ReceivedAt      time.Time          `gorm:"not null;index" json:"receivedAt"`
	Items           []GoodsReceiptItem `gorm:"foreignKey:GoodsReceiptID" json:"items,omitempty"`
}

func (GoodsReceipt) TableName() string { return "goods_receipts" }

type GoodsReceiptItem struct {
	model.HardDeleteBase
	TenantID            int64     `gorm:"not null;index" json:"tenantId"`
	GoodsReceiptID      uuid.UUID `gorm:"type:char(36);not null;index;uniqueIndex:ux_goods_receipt_item" json:"goodsReceiptId"`
	PurchaseOrderItemID uuid.UUID `gorm:"type:char(36);not null;index;uniqueIndex:ux_goods_receipt_item" json:"purchaseOrderItemId"`
	ProductSKUID        uuid.UUID `gorm:"column:product_sku_id;type:char(36);not null;index" json:"productSkuId"`
	Quantity            int       `gorm:"not null" json:"quantity"`
}

func (GoodsReceiptItem) TableName() string { return "goods_receipt_items" }
