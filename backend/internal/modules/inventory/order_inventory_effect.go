package inventory

import (
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

// Order inventory linkage uses a sentinel UUID because composite unique constraints do not treat nil as equal.
var NilInventorySKUUID = uuid.Nil

// OrderInventoryEffect records one deduct/restore attempt for an order line (idempotent ledger).
type OrderInventoryEffect struct {
	model.HardDeleteBase
	OrderID      uuid.UUID  `gorm:"type:char(36);index;not null" json:"orderId"`
	OrderItemID  uuid.UUID  `gorm:"type:char(36);not null;uniqueIndex:ux_oie_item_sku_effect" json:"orderItemId"`
	ProductID    *uuid.UUID `gorm:"type:char(36);index" json:"productId,omitempty"`
	ProductSKUID uuid.UUID  `gorm:"type:char(36);not null;uniqueIndex:ux_oie_item_sku_effect" json:"productSkuId"` // NilInventorySKUUID when SKU missing
	EffectType   string     `gorm:"size:16;not null;uniqueIndex:ux_oie_item_sku_effect" json:"effectType"`
	Quantity     int        `gorm:"not null" json:"quantity"`
	Status       string     `gorm:"size:16;index;not null" json:"status"`
	BeforeStock  *int       `json:"beforeStock,omitempty"`
	AfterStock   *int       `json:"afterStock,omitempty"`
	Reason       string     `gorm:"size:128" json:"reason,omitempty"`
	ErrorMessage string     `gorm:"type:text" json:"errorMessage,omitempty"`
	LogID        *uuid.UUID `gorm:"type:char(36);index" json:"inventoryChangeLogId,omitempty"`
	CreatedBy    *uuid.UUID `gorm:"type:char(36);index" json:"createdBy,omitempty"`
}

func (OrderInventoryEffect) TableName() string { return "order_inventory_effects" }

const (
	EffectTypeDeduct  = "deduct"
	EffectTypeRestore = "restore"
)

const (
	InventoryEffectPending = "pending"
	InventoryEffectSuccess = "success"
	InventoryEffectFailed  = "failed"
	InventoryEffectSkipped = "skipped"
)
