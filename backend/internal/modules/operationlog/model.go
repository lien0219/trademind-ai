package operationlog

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/id"
	"gorm.io/gorm"
)

// OperationLog records auditable admin actions (immutable; no soft delete).
type OperationLog struct {
	ID          uuid.UUID  `gorm:"type:char(36);primaryKey" json:"id"`
	AdminUserID *uuid.UUID `gorm:"type:char(36);index" json:"adminUserId,omitempty"`
	AdminRole   string     `gorm:"size:32;index" json:"adminRole,omitempty"`
	Username    string     `gorm:"size:64;index" json:"username"`
	Action      string     `gorm:"size:64;index;not null" json:"action"`
	Resource    string     `gorm:"size:64;index" json:"resource"`
	ResourceID  string     `gorm:"size:128" json:"resourceId,omitempty"`
	ShopID      *uuid.UUID `gorm:"type:char(36);index" json:"shopId,omitempty"`
	Platform    string     `gorm:"size:32;index" json:"platform,omitempty"`
	Method      string     `gorm:"size:16" json:"method"`
	Path        string     `gorm:"size:512" json:"path"`
	IP          string     `gorm:"size:64" json:"ip"`
	UserAgent   string     `gorm:"size:512" json:"userAgent,omitempty"`
	RequestID   string     `gorm:"size:64;index" json:"requestId"`
	Status      string     `gorm:"size:32;index" json:"status"`
	Message     string     `gorm:"type:text" json:"message,omitempty"`
	CreatedAt   time.Time  `gorm:"index" json:"createdAt"`
}

// TableName keeps a stable table name for migrations.
func (OperationLog) TableName() string {
	return "operation_logs"
}

// BeforeCreate assigns a UUID when id is zero.
func (o *OperationLog) BeforeCreate(tx *gorm.DB) error {
	id.Ensure(&o.ID)
	return nil
}
