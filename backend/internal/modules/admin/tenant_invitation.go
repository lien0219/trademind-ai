package admin

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TenantInvitation stores only a digest of the single-use secret. Delivery is
// performed by an authenticated administrator; tokens never enter audit logs.
type TenantInvitation struct {
	ID              uuid.UUID  `gorm:"type:char(36);primaryKey" json:"id"`
	TenantID        int64      `gorm:"not null;index:idx_tenant_invites,priority:1" json:"tenantId"`
	Email           string     `gorm:"size:128;not null;index" json:"email"`
	Role            string     `gorm:"size:32;not null" json:"role"`
	StoreGrantsJSON string     `gorm:"type:text;not null;default:'[]'" json:"-"`
	TokenHash       string     `gorm:"size:64;not null;uniqueIndex" json:"-"`
	Status          string     `gorm:"size:24;not null;index:idx_tenant_invites,priority:2" json:"status"`
	ExpiresAt       time.Time  `gorm:"not null;index" json:"expiresAt"`
	CreatedBy       uuid.UUID  `gorm:"type:char(36);not null" json:"createdBy"`
	AcceptedBy      *uuid.UUID `gorm:"type:char(36)" json:"acceptedBy,omitempty"`
	AcceptedAt      *time.Time `json:"acceptedAt,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

func (TenantInvitation) TableName() string { return "tenant_invitations" }
func (r *TenantInvitation) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}
