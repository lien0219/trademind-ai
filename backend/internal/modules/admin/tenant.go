package admin

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Tenant is a durable registry record, not an authorization grant.
// Platform operators and tenant administrators are independent identities.
type Tenant struct {
	ID          int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	Code        string     `gorm:"size:48;not null;uniqueIndex" json:"code"`
	Name        string     `gorm:"size:128;not null" json:"name"`
	Status      string     `gorm:"size:24;not null;default:'active';index" json:"status"`
	Description string     `gorm:"size:512" json:"description,omitempty"`
	CreatedBy   *uuid.UUID `gorm:"type:char(36)" json:"createdBy,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

func (Tenant) TableName() string { return "tenants" }

const (
	TenantActive    = "active"
	TenantSuspended = "suspended"
	TenantClosed    = "closed"
)

// TenantMembership grants a global identity a specific tenant-scoped role.
// Disabled memberships never confer access even when JWTs remain unexpired.
type TenantMembership struct {
	ID        uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	TenantID  int64     `gorm:"not null;uniqueIndex:ux_tenant_membership,priority:1;index" json:"tenantId"`
	UserID    uuid.UUID `gorm:"type:char(36);not null;uniqueIndex:ux_tenant_membership,priority:2;index" json:"userId"`
	Role      string    `gorm:"size:32;not null" json:"role"`
	Status    string    `gorm:"size:24;not null;default:'active'" json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (TenantMembership) TableName() string { return "tenant_memberships" }
func (m *TenantMembership) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// PlatformOperator is a separately persisted control-plane authorization.
// Merely having the "admin" role or tenant_id=0 does not grant this capability.
type PlatformOperator struct {
	UserID    uuid.UUID `gorm:"type:char(36);primaryKey" json:"userId"`
	Enabled   bool      `gorm:"not null;default:false" json:"enabled"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (PlatformOperator) TableName() string { return "platform_operators" }
