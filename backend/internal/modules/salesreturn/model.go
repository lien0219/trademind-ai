package salesreturn

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

const (
	TypeRefundOnly   = "refund_only"
	TypeReturnRefund = "return_refund"

	StatusDraft           = "draft"
	StatusPendingApproval = "pending_approval"
	StatusApproved        = "approved"
	StatusCompleted       = "completed"
	StatusCancelled       = "cancelled"
)

// SalesReturn is a tenant-scoped after-sales case. Refund amounts are
// informational accounting facts; no payment-provider write is performed.
type SalesReturn struct {
	model.Base
	TenantID          int64             `gorm:"not null;uniqueIndex:ux_sales_return_tenant_no;uniqueIndex:ux_sales_return_idempotency;index" json:"tenantId"`
	ReturnNo          string            `gorm:"size:64;not null;uniqueIndex:ux_sales_return_tenant_no" json:"returnNo"`
	IdempotencyKey    string            `gorm:"size:128;not null;uniqueIndex:ux_sales_return_idempotency" json:"idempotencyKey"`
	PayloadHash       string            `gorm:"size:64;not null" json:"-"`
	OrderID           uuid.UUID         `gorm:"type:char(36);not null;index" json:"orderId"`
	WarehouseID       uuid.UUID         `gorm:"type:char(36);not null;index" json:"warehouseId"`
	Type              string            `gorm:"size:32;not null;index" json:"type"`
	Status            string            `gorm:"size:32;not null;index" json:"status"`
	Currency          string            `gorm:"size:16;not null" json:"currency"`
	RefundAmountMinor int64             `gorm:"not null;default:0" json:"refundAmountMinor"`
	Revision          int               `gorm:"not null;default:1" json:"revision"`
	Reason            string            `gorm:"size:128;not null" json:"reason"`
	Remark            string            `gorm:"size:520" json:"remark,omitempty"`
	CreatedBy         *uuid.UUID        `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	SubmittedBy       *uuid.UUID        `gorm:"type:char(36);index" json:"submittedBy,omitempty"`
	SubmittedAt       *time.Time        `json:"submittedAt,omitempty"`
	ApprovedBy        *uuid.UUID        `gorm:"type:char(36);index" json:"approvedBy,omitempty"`
	ApprovedAt        *time.Time        `json:"approvedAt,omitempty"`
	CompletedBy       *uuid.UUID        `gorm:"type:char(36);index" json:"completedBy,omitempty"`
	CompletedAt       *time.Time        `json:"completedAt,omitempty"`
	CancelledBy       *uuid.UUID        `gorm:"type:char(36);index" json:"cancelledBy,omitempty"`
	CancelledAt       *time.Time        `json:"cancelledAt,omitempty"`
	OrderNo           string            `gorm:"-" json:"orderNo,omitempty"`
	WarehouseName     string            `gorm:"-" json:"warehouseName,omitempty"`
	Items             []SalesReturnItem `gorm:"foreignKey:SalesReturnID" json:"items,omitempty"`
}

func (SalesReturn) TableName() string { return "sales_returns" }

type SalesReturnItem struct {
	model.HardDeleteBase
	TenantID          int64     `gorm:"not null;index" json:"tenantId"`
	SalesReturnID     uuid.UUID `gorm:"type:char(36);not null;index;uniqueIndex:ux_sales_return_order_item" json:"salesReturnId"`
	OrderItemID       uuid.UUID `gorm:"type:char(36);not null;index;uniqueIndex:ux_sales_return_order_item" json:"orderItemId"`
	ProductSKUID      uuid.UUID `gorm:"column:product_sku_id;type:char(36);not null;index" json:"productSkuId"`
	Quantity          int       `gorm:"not null" json:"quantity"`
	Disposition       string    `gorm:"size:32" json:"disposition,omitempty"`
	RefundAmountMinor int64     `gorm:"not null;default:0" json:"refundAmountMinor"`
	ProductTitle      string    `gorm:"-" json:"productTitle,omitempty"`
	SKUCode           string    `gorm:"-" json:"skuCode,omitempty"`
	SKUName           string    `gorm:"-" json:"skuName,omitempty"`
	DeductedQuantity  int       `gorm:"-" json:"deductedQuantity"`
}

func (SalesReturnItem) TableName() string { return "sales_return_items" }

// SalesReturnAction is an immutable action fact used for replay protection and audit.
type SalesReturnAction struct {
	model.HardDeleteBase
	TenantID       int64      `gorm:"not null;uniqueIndex:ux_sales_return_action_event;uniqueIndex:ux_sales_return_action_key;index" json:"tenantId"`
	SalesReturnID  uuid.UUID  `gorm:"type:char(36);not null;uniqueIndex:ux_sales_return_action_event;uniqueIndex:ux_sales_return_action_key;index" json:"salesReturnId"`
	Action         string     `gorm:"size:32;not null;uniqueIndex:ux_sales_return_action_event" json:"action"`
	IdempotencyKey string     `gorm:"size:128;not null;uniqueIndex:ux_sales_return_action_key" json:"idempotencyKey"`
	RequestHash    string     `gorm:"size:64;not null" json:"-"`
	ActorID        *uuid.UUID `gorm:"type:char(36);index" json:"actorId,omitempty"`
	FromStatus     string     `gorm:"size:32;not null" json:"fromStatus"`
	ToStatus       string     `gorm:"size:32;not null" json:"toStatus"`
	Reason         string     `gorm:"size:128" json:"reason,omitempty"`
}

func (SalesReturnAction) TableName() string { return "sales_return_actions" }

// SalesReturnInventoryEffect links a return item to the immutable inventory facts.
type SalesReturnInventoryEffect struct {
	model.HardDeleteBase
	TenantID          int64     `gorm:"not null;index" json:"tenantId"`
	SalesReturnID     uuid.UUID `gorm:"type:char(36);not null;index" json:"salesReturnId"`
	SalesReturnItemID uuid.UUID `gorm:"type:char(36);not null;uniqueIndex" json:"salesReturnItemId"`
	OrderID           uuid.UUID `gorm:"type:char(36);not null;index" json:"orderId"`
	OrderItemID       uuid.UUID `gorm:"type:char(36);not null;index" json:"orderItemId"`
	WarehouseID       uuid.UUID `gorm:"type:char(36);not null;index" json:"warehouseId"`
	ProductSKUID      uuid.UUID `gorm:"column:product_sku_id;type:char(36);not null;index" json:"productSkuId"`
	Disposition       string    `gorm:"size:32;not null" json:"disposition"`
	Quantity          int       `gorm:"not null" json:"quantity"`
	BeforeOnHand      int       `gorm:"not null" json:"beforeOnHand"`
	AfterOnHand       int       `gorm:"not null" json:"afterOnHand"`
	BeforeDamaged     int       `gorm:"not null" json:"beforeDamaged"`
	AfterDamaged      int       `gorm:"not null" json:"afterDamaged"`
	MovementID        uuid.UUID `gorm:"type:char(36);not null;uniqueIndex" json:"movementId"`
	ChangeLogID       uuid.UUID `gorm:"type:char(36);not null;uniqueIndex" json:"changeLogId"`
	BusinessEventKey  string    `gorm:"size:255;not null;uniqueIndex" json:"businessEventKey"`
}

func (SalesReturnInventoryEffect) TableName() string { return "sales_return_inventory_effects" }
