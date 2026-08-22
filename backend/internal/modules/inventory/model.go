package inventory

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

// InventoryChangeLog is an append-only local stock / sync audit trail (hard-deleted rows only via admin tooling).
type InventoryChangeLog struct {
	model.HardDeleteBase
	TenantID         int64      `gorm:"not null;default:0;index" json:"tenantId"`
	ProductID        uuid.UUID  `gorm:"type:char(36);index;not null" json:"productId"`
	ProductSKUID     uuid.UUID  `gorm:"column:product_sku_id;type:char(36);index;not null" json:"productSkuId"`
	ChangeType       string     `gorm:"size:48;index;not null" json:"changeType"`
	BeforeStock      int        `gorm:"not null" json:"beforeStock"`
	AfterStock       int        `gorm:"not null" json:"afterStock"`
	Delta            int        `gorm:"not null" json:"delta"`
	Reason           string     `gorm:"size:128" json:"reason,omitempty"`
	Remark           string     `gorm:"size:520" json:"remark,omitempty"`
	CreatedBy        *uuid.UUID `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	RefOrderID       *uuid.UUID `gorm:"type:char(36);index" json:"refOrderId,omitempty"`
	RefOrderItemID   *uuid.UUID `gorm:"type:char(36);index" json:"refOrderItemId,omitempty"`
	BusinessEventKey string     `gorm:"size:255;uniqueIndex" json:"businessEventKey,omitempty"`
}

func (InventoryChangeLog) TableName() string { return "inventory_change_logs" }

// WarehouseStockBalance is the tenant-scoped warehouse ledger introduced for
// ERP flows. During the compatibility migration product_skus.stock remains the
// aggregate authority until every legacy stock writer has moved to this ledger.
type WarehouseStockBalance struct {
	model.HardDeleteBase
	TenantID     int64     `gorm:"not null;uniqueIndex:ux_warehouse_stock_balance;index" json:"tenantId"`
	WarehouseID  uuid.UUID `gorm:"type:char(36);not null;uniqueIndex:ux_warehouse_stock_balance;index" json:"warehouseId"`
	ProductSKUID uuid.UUID `gorm:"column:product_sku_id;type:char(36);not null;uniqueIndex:ux_warehouse_stock_balance;index" json:"productSkuId"`
	OnHand       int       `gorm:"not null;default:0" json:"onHand"`
	Reserved     int       `gorm:"not null;default:0" json:"reserved"`
	InTransit    int       `gorm:"not null;default:0" json:"inTransit"`
	Damaged      int       `gorm:"not null;default:0" json:"damaged"`
	Version      int       `gorm:"not null;default:1" json:"version"`
}

func (WarehouseStockBalance) TableName() string { return "warehouse_stock_balances" }

// Available returns stock that may be promised to an order or marketplace.
func (b WarehouseStockBalance) Available() int {
	available := b.OnHand - b.Reserved - b.Damaged
	if available < 0 {
		return 0
	}
	return available
}

// InventoryMovement is an append-only warehouse stock fact.
type InventoryMovement struct {
	model.HardDeleteBase
	TenantID         int64      `gorm:"not null;index" json:"tenantId"`
	WarehouseID      uuid.UUID  `gorm:"type:char(36);not null;index" json:"warehouseId"`
	ProductID        uuid.UUID  `gorm:"type:char(36);not null;index" json:"productId"`
	ProductSKUID     uuid.UUID  `gorm:"column:product_sku_id;type:char(36);not null;index" json:"productSkuId"`
	MovementType     string     `gorm:"size:48;not null;index" json:"movementType"`
	Quantity         int        `gorm:"not null" json:"quantity"`
	BeforeOnHand     int        `gorm:"not null" json:"beforeOnHand"`
	AfterOnHand      int        `gorm:"not null" json:"afterOnHand"`
	BeforeReserved   int        `gorm:"not null;default:0" json:"beforeReserved"`
	AfterReserved    int        `gorm:"not null;default:0" json:"afterReserved"`
	SourceType       string     `gorm:"size:48;not null;index" json:"sourceType"`
	SourceID         uuid.UUID  `gorm:"type:char(36);not null;index" json:"sourceId"`
	BusinessEventKey string     `gorm:"size:255;not null;uniqueIndex" json:"businessEventKey"`
	RequestHash      string     `gorm:"size:64" json:"-"`
	Reason           string     `gorm:"size:128" json:"reason,omitempty"`
	Remark           string     `gorm:"size:520" json:"remark,omitempty"`
	CreatedBy        *uuid.UUID `gorm:"type:char(36);index" json:"createdBy,omitempty"`
}

func (InventoryMovement) TableName() string { return "inventory_movements" }

// WarehouseTransfer is the tenant-scoped warehouse relocation aggregate.
type WarehouseTransfer struct {
	model.Base
	TenantID          int64                   `gorm:"not null;uniqueIndex:ux_transfer_tenant_no;uniqueIndex:ux_transfer_tenant_idempotency;index" json:"tenantId"`
	TransferNo        string                  `gorm:"size:64;not null;uniqueIndex:ux_transfer_tenant_no" json:"transferNo"`
	SourceWarehouseID uuid.UUID               `gorm:"type:char(36);not null;index" json:"sourceWarehouseId"`
	TargetWarehouseID uuid.UUID               `gorm:"type:char(36);not null;index" json:"targetWarehouseId"`
	Status            string                  `gorm:"size:32;not null;index" json:"status"`
	Revision          int                     `gorm:"not null;default:1" json:"revision"`
	IdempotencyKey    string                  `gorm:"size:128;not null;uniqueIndex:ux_transfer_tenant_idempotency" json:"idempotencyKey"`
	PayloadHash       string                  `gorm:"size:64;not null" json:"-"`
	Reason            string                  `gorm:"size:128" json:"reason,omitempty"`
	Remark            string                  `gorm:"size:520" json:"remark,omitempty"`
	CreatedBy         *uuid.UUID              `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	ApprovedBy        *uuid.UUID              `gorm:"type:char(36);index" json:"approvedBy,omitempty"`
	ApprovedAt        *time.Time              `json:"approvedAt,omitempty"`
	DispatchedAt      *time.Time              `json:"dispatchedAt,omitempty"`
	ReceivedAt        *time.Time              `json:"receivedAt,omitempty"`
	CancelledAt       *time.Time              `json:"cancelledAt,omitempty"`
	Items             []WarehouseTransferItem `gorm:"foreignKey:TransferID" json:"items,omitempty"`
}

func (WarehouseTransfer) TableName() string { return "warehouse_transfers" }

type WarehouseTransferItem struct {
	model.HardDeleteBase
	TenantID         int64     `gorm:"not null;index" json:"tenantId"`
	TransferID       uuid.UUID `gorm:"type:char(36);not null;index;uniqueIndex:ux_transfer_item_sku" json:"transferId"`
	ProductID        uuid.UUID `gorm:"type:char(36);not null;index" json:"productId"`
	ProductSKUID     uuid.UUID `gorm:"column:product_sku_id;type:char(36);not null;index;uniqueIndex:ux_transfer_item_sku" json:"productSkuId"`
	Quantity         int       `gorm:"not null" json:"quantity"`
	ReceivedQuantity int       `gorm:"not null;default:0" json:"receivedQuantity"`
	ProductTitle     string    `gorm:"-" json:"productTitle,omitempty"`
	SKUCode          string    `gorm:"-" json:"skuCode,omitempty"`
	SKUName          string    `gorm:"-" json:"skuName,omitempty"`
}

func (WarehouseTransferItem) TableName() string { return "warehouse_transfer_items" }

type WarehouseTransferAction struct {
	model.HardDeleteBase
	TenantID       int64     `gorm:"not null;uniqueIndex:ux_transfer_action_event;index" json:"tenantId"`
	TransferID     uuid.UUID `gorm:"type:char(36);not null;uniqueIndex:ux_transfer_action_event;index" json:"transferId"`
	Action         string    `gorm:"size:32;not null;uniqueIndex:ux_transfer_action_event" json:"action"`
	IdempotencyKey string    `gorm:"size:128;not null" json:"idempotencyKey"`
	RequestHash    string    `gorm:"size:64;not null" json:"-"`
}

func (WarehouseTransferAction) TableName() string { return "warehouse_transfer_actions" }

// InventoryStocktake records one warehouse physical count. The counted
// quantity is intentionally limited to on-hand stock in the first increment;
// reserved, damaged, and in-transit quantities remain ledger-controlled facts.
type InventoryStocktake struct {
	model.Base
	TenantID       int64                    `gorm:"not null;uniqueIndex:ux_stocktake_tenant_no;uniqueIndex:ux_stocktake_tenant_idempotency;index" json:"tenantId"`
	StocktakeNo    string                   `gorm:"size:64;not null;uniqueIndex:ux_stocktake_tenant_no" json:"stocktakeNo"`
	WarehouseID    uuid.UUID                `gorm:"type:char(36);not null;index" json:"warehouseId"`
	Status         string                   `gorm:"size:32;not null;index" json:"status"`
	Revision       int                      `gorm:"not null;default:1" json:"revision"`
	IdempotencyKey string                   `gorm:"size:128;not null;uniqueIndex:ux_stocktake_tenant_idempotency" json:"idempotencyKey"`
	PayloadHash    string                   `gorm:"size:64;not null" json:"-"`
	Reason         string                   `gorm:"size:128" json:"reason,omitempty"`
	Remark         string                   `gorm:"size:520" json:"remark,omitempty"`
	CreatedBy      *uuid.UUID               `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	SubmittedBy    *uuid.UUID               `gorm:"type:char(36);index" json:"submittedBy,omitempty"`
	SubmittedAt    *time.Time               `json:"submittedAt,omitempty"`
	ApprovedBy     *uuid.UUID               `gorm:"type:char(36);index" json:"approvedBy,omitempty"`
	ApprovedAt     *time.Time               `json:"approvedAt,omitempty"`
	PostedBy       *uuid.UUID               `gorm:"type:char(36);index" json:"postedBy,omitempty"`
	PostedAt       *time.Time               `json:"postedAt,omitempty"`
	CancelledAt    *time.Time               `json:"cancelledAt,omitempty"`
	Items          []InventoryStocktakeItem `gorm:"foreignKey:StocktakeID" json:"items,omitempty"`
}

func (InventoryStocktake) TableName() string { return "inventory_stocktakes" }

type InventoryStocktakeItem struct {
	model.HardDeleteBase
	TenantID          int64     `gorm:"not null;index" json:"tenantId"`
	StocktakeID       uuid.UUID `gorm:"type:char(36);not null;index;uniqueIndex:ux_stocktake_item_sku" json:"stocktakeId"`
	ProductID         uuid.UUID `gorm:"type:char(36);not null;index" json:"productId"`
	ProductSKUID      uuid.UUID `gorm:"column:product_sku_id;type:char(36);not null;index;uniqueIndex:ux_stocktake_item_sku" json:"productSkuId"`
	SnapshotOnHand    int       `gorm:"not null" json:"snapshotOnHand"`
	SnapshotReserved  int       `gorm:"not null;default:0" json:"snapshotReserved"`
	SnapshotInTransit int       `gorm:"not null;default:0" json:"snapshotInTransit"`
	SnapshotDamaged   int       `gorm:"not null;default:0" json:"snapshotDamaged"`
	SnapshotVersion   int       `gorm:"not null" json:"snapshotVersion"`
	CountedOnHand     *int      `json:"countedOnHand,omitempty"`
	Remark            string    `gorm:"size:520" json:"remark,omitempty"`
	ProductTitle      string    `gorm:"-" json:"productTitle,omitempty"`
	SKUCode           string    `gorm:"-" json:"skuCode,omitempty"`
	SKUName           string    `gorm:"-" json:"skuName,omitempty"`
}

func (InventoryStocktakeItem) TableName() string { return "inventory_stocktake_items" }

type InventoryStocktakeAction struct {
	model.HardDeleteBase
	TenantID       int64     `gorm:"not null;uniqueIndex:ux_stocktake_action_event;index" json:"tenantId"`
	StocktakeID    uuid.UUID `gorm:"type:char(36);not null;uniqueIndex:ux_stocktake_action_event;index" json:"stocktakeId"`
	Action         string    `gorm:"size:64;not null;uniqueIndex:ux_stocktake_action_event" json:"action"`
	IdempotencyKey string    `gorm:"size:128;not null;uniqueIndex:ux_stocktake_action_event" json:"idempotencyKey"`
	RequestHash    string    `gorm:"size:64;not null" json:"-"`
}

func (InventoryStocktakeAction) TableName() string { return "inventory_stocktake_actions" }

// InventorySyncBatch groups many outbound inventory_sync_tasks created in one bulk submission.
type InventorySyncBatch struct {
	model.HardDeleteBase
	TenantID      int64          `gorm:"not null;default:0;index" json:"tenantId"`
	BatchNo       string         `gorm:"size:48;uniqueIndex;not null" json:"batchNo"`
	Source        string         `gorm:"size:48;index;not null" json:"source"`
	Status        string         `gorm:"size:32;index;not null" json:"status"`
	Platform      string         `gorm:"size:64;index" json:"platform,omitempty"`
	ShopID        *uuid.UUID     `gorm:"type:char(36);index" json:"shopId,omitempty"`
	ProductID     *uuid.UUID     `gorm:"type:char(36);index" json:"productId,omitempty"`
	TotalCount    int            `gorm:"not null;default:0" json:"totalCount"`
	PendingCount  int            `gorm:"not null;default:0" json:"pendingCount"`
	RunningCount  int            `gorm:"not null;default:0" json:"runningCount"`
	SuccessCount  int            `gorm:"not null;default:0" json:"successCount"`
	FailedCount   int            `gorm:"not null;default:0" json:"failedCount"`
	SkippedCount  int            `gorm:"not null;default:0" json:"skippedCount"`
	SkippedReason string         `gorm:"type:text" json:"skippedReason,omitempty"`
	Input         datatypes.JSON `gorm:"type:jsonb" json:"input,omitempty"`
	Output        datatypes.JSON `gorm:"type:jsonb" json:"output,omitempty"`
	CreatedBy     *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	StartedAt     *time.Time     `json:"startedAt,omitempty"`
	FinishedAt    *time.Time     `json:"finishedAt,omitempty"`
}

func (InventorySyncBatch) TableName() string { return "inventory_sync_batches" }

// InventorySyncTask is one outbound stock push to a marketplace listing SKU.
type InventorySyncTask struct {
	model.HardDeleteBase
	TenantID         int64          `gorm:"not null;default:0;index" json:"tenantId"`
	BatchID          *uuid.UUID     `gorm:"type:char(36);index" json:"batchId,omitempty"`
	BatchNo          string         `gorm:"size:64;index" json:"batchNo,omitempty"`
	ProductID        uuid.UUID      `gorm:"type:char(36);index;not null" json:"productId"`
	ProductSKUID     *uuid.UUID     `gorm:"column:product_sku_id;type:char(36);index" json:"productSkuId,omitempty"`
	PublicationID    *uuid.UUID     `gorm:"type:char(36);index" json:"publicationId,omitempty"`
	PublicationSKUID *uuid.UUID     `gorm:"column:publication_sku_id;type:char(36);index:idx_inventory_sync_tasks_publication_sku_id" json:"publicationSkuId,omitempty"`
	ShopID           uuid.UUID      `gorm:"type:char(36);index;not null" json:"shopId"`
	Platform         string         `gorm:"size:64;index;not null" json:"platform"`
	TaskType         string         `gorm:"size:64;index;not null" json:"taskType"`
	Status           string         `gorm:"size:32;index;not null" json:"status"`
	Mode             string         `gorm:"size:32;index;not null" json:"mode"`
	TargetStock      int            `gorm:"not null" json:"targetStock"`
	StartedAt        *time.Time     `json:"startedAt,omitempty"`
	FinishedAt       *time.Time     `json:"finishedAt,omitempty"`
	ErrorMessage     string         `gorm:"type:text" json:"errorMessage,omitempty"`
	Input            datatypes.JSON `gorm:"type:jsonb" json:"input,omitempty"`
	Output           datatypes.JSON `gorm:"type:jsonb" json:"output,omitempty"`
	CreatedBy        *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	LockedBy         *string        `gorm:"size:220;index" json:"lockedBy,omitempty"`
	LockedUntil      *time.Time     `gorm:"index" json:"lockedUntil,omitempty"`
	LockVersion      int            `gorm:"column:lock_version;default:0;not null" json:"leaseVersion"`
	HeartbeatAt      *time.Time     `gorm:"index" json:"heartbeatAt,omitempty"`
	ExecutionID      *string        `gorm:"size:36;index" json:"executionId,omitempty"`
}

func (InventorySyncTask) TableName() string { return "inventory_sync_tasks" }
