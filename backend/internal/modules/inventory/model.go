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
	ProductID      uuid.UUID  `gorm:"type:char(36);index;not null" json:"productId"`
	ProductSKUID   uuid.UUID  `gorm:"type:char(36);index;not null" json:"productSkuId"`
	ChangeType     string     `gorm:"size:48;index;not null" json:"changeType"`
	BeforeStock    int        `gorm:"not null" json:"beforeStock"`
	AfterStock     int        `gorm:"not null" json:"afterStock"`
	Delta          int        `gorm:"not null" json:"delta"`
	Reason         string     `gorm:"size:128" json:"reason,omitempty"`
	Remark         string     `gorm:"size:520" json:"remark,omitempty"`
	CreatedBy      *uuid.UUID `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	RefOrderID     *uuid.UUID `gorm:"type:char(36);index" json:"refOrderId,omitempty"`
	RefOrderItemID *uuid.UUID `gorm:"type:char(36);index" json:"refOrderItemId,omitempty"`
}

func (InventoryChangeLog) TableName() string { return "inventory_change_logs" }

// InventorySyncTask is one outbound stock push to a marketplace listing SKU.
type InventorySyncTask struct {
	model.HardDeleteBase
	ProductID        uuid.UUID      `gorm:"type:char(36);index;not null" json:"productId"`
	ProductSKUID     *uuid.UUID     `gorm:"type:char(36);index" json:"productSkuId,omitempty"`
	PublicationID    *uuid.UUID     `gorm:"type:char(36);index" json:"publicationId,omitempty"`
	PublicationSkuID *uuid.UUID     `gorm:"type:char(36);index" json:"publicationSkuId,omitempty"`
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
	LockVersion      int            `gorm:"default:0;not null" json:"lockVersion"`
}

func (InventorySyncTask) TableName() string { return "inventory_sync_tasks" }
