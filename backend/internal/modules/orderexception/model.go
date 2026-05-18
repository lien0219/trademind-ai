package orderexception

import (
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

// OrderExceptionMark is a workbench-only overlay (does not mutate business tasks).
type OrderExceptionMark struct {
	model.HardDeleteBase
	ExceptionType string     `gorm:"size:64;not null;uniqueIndex:ux_order_exception_mark_quad" json:"exceptionType"`
	SourceType    string     `gorm:"size:64;not null;uniqueIndex:ux_order_exception_mark_quad" json:"sourceType"`
	SourceID      string     `gorm:"size:64;not null;uniqueIndex:ux_order_exception_mark_quad" json:"sourceId"`
	MarkType      string     `gorm:"size:16;not null;uniqueIndex:ux_order_exception_mark_quad" json:"markType"`
	OrderID       *uuid.UUID `gorm:"type:char(36);index" json:"orderId,omitempty"`
	OrderItemID   *uuid.UUID `gorm:"type:char(36);index" json:"orderItemId,omitempty"`
	Remark        string     `gorm:"type:text" json:"remark,omitempty"`
	CreatedBy     *uuid.UUID `gorm:"type:char(36);index" json:"createdBy,omitempty"`
}

func (OrderExceptionMark) TableName() string { return "order_exception_marks" }
