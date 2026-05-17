package inventory

import (
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

// orderMirror / orderLineMirror mirror `orders` / `order_items` without importing `modules/order`,
// avoiding a cycle (order.Handler already depends on inventory.Service).
type orderMirror struct {
	model.Base
	OrderNo       string `gorm:"size:128;not null"`
	Status        string `gorm:"size:32;not null"`
	PaymentStatus string `gorm:"size:32;not null"`
}

func (orderMirror) TableName() string { return "orders" }

type orderLineMirror struct {
	model.HardDeleteBase
	OrderID        uuid.UUID  `gorm:"type:char(36);index;not null"`
	ProductID      *uuid.UUID `gorm:"type:char(36);index"`
	ProductSKUID   *uuid.UUID `gorm:"type:char(36);index"`
	ExternalItemID *string    `gorm:"size:255"`
	Quantity       int        `gorm:"not null"`
}

func (orderLineMirror) TableName() string { return "order_items" }
