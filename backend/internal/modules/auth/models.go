package auth

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/id"
	"gorm.io/gorm"
)

// Session status values.
const (
	SessionStatusActive  = "active"
	SessionStatusRevoked = "revoked"
	SessionStatusExpired = "expired"
)

// Refresh token status values.
const (
	RefreshStatusActive        = "active"
	RefreshStatusRotated       = "rotated"
	RefreshStatusRevoked       = "revoked"
	RefreshStatusExpired       = "expired"
	RefreshStatusReuseDetected = "reuse_detected"
	RefreshStatusCompromised   = "compromised"
)

// AuthSession is a server-side admin session bound to refresh token families.
type AuthSession struct {
	ID               uuid.UUID  `gorm:"type:char(36);primaryKey" json:"id"`
	TenantID         int64      `gorm:"not null;default:0;index" json:"tenantId"`
	UserID           uuid.UUID  `gorm:"type:char(36);not null;index" json:"userId"`
	TokenVersion     int        `gorm:"not null;default:0" json:"-"`
	Status           string     `gorm:"size:32;not null;default:'active';index" json:"status"`
	DeviceSummary    string     `gorm:"size:128" json:"deviceSummary,omitempty"`
	BrowserSummary   string     `gorm:"size:128" json:"browserSummary,omitempty"`
	IPHash           string     `gorm:"size:64;index" json:"-"`
	UserAgentSummary string     `gorm:"size:256" json:"userAgentSummary,omitempty"`
	LastActivityAt   time.Time  `gorm:"index" json:"lastActivityAt"`
	RevokedAt        *time.Time `json:"revokedAt,omitempty"`
	RevokeReason     string     `gorm:"size:128" json:"revokeReason,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

func (AuthSession) TableName() string { return "auth_sessions" }

func (s *AuthSession) BeforeCreate(tx *gorm.DB) error {
	id.Ensure(&s.ID)
	if s.LastActivityAt.IsZero() {
		s.LastActivityAt = time.Now().UTC()
	}
	return nil
}

// AuthRefreshToken stores hashed refresh tokens with rotation lineage.
type AuthRefreshToken struct {
	ID                uuid.UUID  `gorm:"type:char(36);primaryKey" json:"id"`
	TenantID          int64      `gorm:"not null;default:0;index" json:"tenantId"`
	UserID            uuid.UUID  `gorm:"type:char(36);not null;index" json:"userId"`
	SessionID         uuid.UUID  `gorm:"type:char(36);not null;index" json:"sessionId"`
	TokenFamilyID     uuid.UUID  `gorm:"type:char(36);not null;index" json:"tokenFamilyId"`
	TokenHash         string     `gorm:"size:128;not null;uniqueIndex" json:"-"`
	ParentTokenID     *uuid.UUID `gorm:"type:char(36);index" json:"parentTokenId,omitempty"`
	ReplacedByTokenID *uuid.UUID `gorm:"type:char(36);index" json:"replacedByTokenId,omitempty"`
	Status            string     `gorm:"size:32;not null;default:'active';index" json:"status"`
	ExpiresAt         time.Time  `gorm:"not null;index" json:"expiresAt"`
	LastUsedAt        *time.Time `json:"lastUsedAt,omitempty"`
	RevokedAt         *time.Time `json:"revokedAt,omitempty"`
	RevokeReason      string     `gorm:"size:128" json:"revokeReason,omitempty"`
	IPHash            string     `gorm:"size:64" json:"-"`
	UserAgentSummary  string     `gorm:"size:256" json:"userAgentSummary,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

func (AuthRefreshToken) TableName() string { return "auth_refresh_tokens" }

func (t *AuthRefreshToken) BeforeCreate(tx *gorm.DB) error {
	id.Ensure(&t.ID)
	return nil
}

// AuthLoginAttempt tracks failed login counters for rate limiting and lockout.
type AuthLoginAttempt struct {
	ID           uuid.UUID  `gorm:"type:char(36);primaryKey" json:"id"`
	TenantID     int64      `gorm:"not null;default:0;index" json:"tenantId"`
	AccountKey   string     `gorm:"size:256;not null;index" json:"accountKey"`
	IPHash       string     `gorm:"size:64;index" json:"ipHash"`
	FailedCount  int        `gorm:"not null;default:0" json:"failedCount"`
	LockedUntil  *time.Time `gorm:"index" json:"lockedUntil,omitempty"`
	LastFailedAt *time.Time `json:"lastFailedAt,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

func (AuthLoginAttempt) TableName() string { return "auth_login_attempts" }

func (a *AuthLoginAttempt) BeforeCreate(tx *gorm.DB) error {
	id.Ensure(&a.ID)
	return nil
}

// AuthReauthToken is a short-lived token for high-risk operation confirmation.
type AuthReauthToken struct {
	ID        uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	TenantID  int64     `gorm:"not null;default:0;index" json:"tenantId"`
	UserID    uuid.UUID `gorm:"type:char(36);not null;index" json:"userId"`
	SessionID uuid.UUID `gorm:"type:char(36);not null;index" json:"sessionId"`
	TokenHash string    `gorm:"size:128;not null;uniqueIndex" json:"-"`
	Operation string    `gorm:"size:64;not null;index" json:"operation"`
	Used      bool      `gorm:"not null;default:false;index" json:"used"`
	ExpiresAt time.Time `gorm:"not null;index" json:"expiresAt"`
	CreatedAt time.Time `json:"createdAt"`
}

func (AuthReauthToken) TableName() string { return "auth_reauth_tokens" }

func (r *AuthReauthToken) BeforeCreate(tx *gorm.DB) error {
	id.Ensure(&r.ID)
	return nil
}
