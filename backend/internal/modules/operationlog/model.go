package operationlog

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/id"
	"gorm.io/gorm"
)

// OperationLog records auditable admin actions (immutable; no soft delete).
type OperationLog struct {
	ID               uuid.UUID  `gorm:"type:char(36);primaryKey" json:"id"`
	TenantID         int64      `gorm:"not null;default:0;index" json:"tenantId"`
	AdminUserID      *uuid.UUID `gorm:"type:char(36);index" json:"adminUserId,omitempty"`
	SessionID        *uuid.UUID `gorm:"type:char(36);index" json:"sessionId,omitempty"`
	AdminRole        string     `gorm:"size:32;index" json:"adminRole,omitempty"`
	Username         string     `gorm:"size:128;index" json:"username"`
	Action           string     `gorm:"size:64;index;not null" json:"action"`
	Resource         string     `gorm:"size:64;index" json:"resource"`
	ResourceID       string     `gorm:"size:128" json:"resourceId,omitempty"`
	ShopID           *uuid.UUID `gorm:"type:char(36);index" json:"shopId,omitempty"`
	Platform         string     `gorm:"size:32;index" json:"platform,omitempty"`
	Permission       string     `gorm:"size:64" json:"permission,omitempty"`
	Method           string     `gorm:"size:16" json:"method"`
	Path             string     `gorm:"size:512" json:"path"`
	IPHash           string     `gorm:"size:64" json:"ipHash,omitempty"`
	UserAgentSummary string     `gorm:"size:256" json:"userAgentSummary,omitempty"`
	RequestID        string     `gorm:"size:64;index" json:"requestId"`
	Status           string     `gorm:"size:32;index" json:"status"`
	Message          string     `gorm:"type:text" json:"message,omitempty"`
	PrevHash         string     `gorm:"size:128;index" json:"prevHash,omitempty"`
	EntryHash        string     `gorm:"size:128;index" json:"entryHash,omitempty"`
	HashVersion      int        `gorm:"not null;default:1" json:"hashVersion"`
	ChainPartition   string     `gorm:"size:64;index" json:"chainPartition,omitempty"`
	CreatedAt        time.Time  `gorm:"index" json:"createdAt"`
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
