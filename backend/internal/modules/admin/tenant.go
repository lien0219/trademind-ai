package admin

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Tenant is a durable registry record, not an authorization grant.
// Platform operators and tenant administrators are independent identities.
type Tenant struct {
	ID            int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	Code          string     `gorm:"size:48;not null;uniqueIndex" json:"code"`
	Name          string     `gorm:"size:128;not null" json:"name"`
	Status        string     `gorm:"size:24;not null;default:'active';index" json:"status"`
	Description   string     `gorm:"size:512" json:"description,omitempty"`
	LegalName     string     `gorm:"size:256" json:"legalName,omitempty"`
	OwnerUserID   *uuid.UUID `gorm:"type:char(36);index" json:"ownerUserId,omitempty"`
	ContactEmail  string     `gorm:"size:128" json:"contactEmail,omitempty"`
	ContactPhone  string     `gorm:"size:32" json:"contactPhone,omitempty"`
	SettingsJSON  string     `gorm:"type:text;not null;default:'{}'" json:"settings"`
	Version       int64      `gorm:"not null;default:1" json:"version"`
	ProvisionKey  *string    `gorm:"size:64;uniqueIndex" json:"-"`
	ProvisionHash string     `gorm:"size:64" json:"-"`
	ActivatedAt   *time.Time `json:"activatedAt,omitempty"`
	SuspendedAt   *time.Time `json:"suspendedAt,omitempty"`
	ArchivedAt    *time.Time `json:"archivedAt,omitempty"`
	DeletedAt     *time.Time `gorm:"index" json:"deletedAt,omitempty"`
	CreatedBy     *uuid.UUID `gorm:"type:char(36)" json:"createdBy,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

func (Tenant) TableName() string { return "tenants" }

const (
	// RootWorkspaceCode is the protected, internal business data scope for the
	// singleton Root identity. It is never a customer-managed enterprise.
	RootWorkspaceCode   = "root-workspace"
	RootWorkspaceName   = "系统工作台"
	TenantPending       = "pending"
	TenantActive        = "active"
	TenantSuspended     = "suspended"
	TenantArchived      = "archived"
	TenantClosed        = "closed"
	TenantDeleted       = "deleted"
	MembershipInvited   = "invited"
	MembershipActive    = "active"
	MembershipDisabled  = "disabled"
	MembershipRemoved   = "removed"
	InvitationPending   = "pending"
	InvitationAccepted  = "accepted"
	InvitationCancelled = "cancelled"
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
