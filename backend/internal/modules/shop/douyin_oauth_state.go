package shop

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
	"gorm.io/gorm"
)

const douyinOAuthExpectedTokenVersionContextKey = "shop.douyin.oauth.expected_token_version"

// DouyinOAuthState persists OAuth state hash for one-time-use verification.
// In addition to the Redis TTL (fast path), the DB record provides:
//   - Durability across Redis restarts
//   - One-time-consume enforcement (ConsumedAt)
//   - Audit trail
type DouyinOAuthState struct {
	model.Base
	TenantID           int64      `gorm:"index;not null;default:0" json:"tenantId"`
	TokenVersion       int64      `gorm:"not null;default:0" json:"-"`
	StateHash          string     `gorm:"size:128;uniqueIndex;not null" json:"stateHash"`
	UserID             *uuid.UUID `gorm:"type:char(36);index" json:"userId,omitempty"`
	ShopBindingContext string     `gorm:"size:255" json:"shopBindingContext,omitempty"`
	RedirectURL        string     `gorm:"type:text" json:"redirectUrl,omitempty"`
	ExpiresAt          time.Time  `gorm:"index;not null" json:"expiresAt"`
	ConsumedAt         *time.Time `gorm:"index" json:"consumedAt,omitempty"`
}

func (DouyinOAuthState) TableName() string { return "douyin_oauth_states" }

// IsExpired returns true if the state is past its expiry time.
func (s *DouyinOAuthState) IsExpired() bool {
	if s == nil {
		return true
	}
	return time.Now().UTC().After(s.ExpiresAt)
}

// IsConsumed returns true if the state has already been used.
func (s *DouyinOAuthState) IsConsumed() bool {
	return s != nil && s.ConsumedAt != nil
}

func hashDouyinOAuthState(rawState string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(rawState)))
	return hex.EncodeToString(sum[:])
}

func (s *Service) persistDouyinOAuthState(ctx context.Context, rawState string, tenantID int64, userID, shopID *uuid.UUID, redirectURL string) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("shop: db unavailable")
	}
	redirectURL = strings.TrimSpace(redirectURL)
	if redirectURL == "" {
		return douyinErr(platformdouyin.CodeDouyinOAuthRedirectNotAllowed, "抖店回调地址未配置，禁止发起授权。", nil)
	}
	if userID == nil || *userID == uuid.Nil {
		return fmt.Errorf("shop: OAuth state requires an authenticated user")
	}
	var actor struct {
		TenantID     int64
		TokenVersion int64
		Status       string
	}
	if err := s.DB.WithContext(ctx).Table("admin_users").
		Select("tenant_id, token_version, status").
		Where("id = ? AND tenant_id = ?", *userID, tenantID).
		Take(&actor).Error; err != nil {
		return fmt.Errorf("shop: resolve OAuth actor: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(actor.Status), "active") {
		return fmt.Errorf("shop: OAuth actor is disabled")
	}
	if shopID != nil && *shopID != uuid.Nil {
		var count int64
		if err := s.DB.WithContext(ctx).Model(&Shop{}).Where("id = ? AND tenant_id = ?", *shopID, tenantID).Count(&count).Error; err != nil {
			return fmt.Errorf("shop: validate OAuth shop: %w", err)
		}
		if count != 1 {
			return gorm.ErrRecordNotFound
		}
	}
	row := DouyinOAuthState{
		TenantID:           tenantID,
		TokenVersion:       actor.TokenVersion,
		StateHash:          hashDouyinOAuthState(rawState),
		UserID:             userID,
		ShopBindingContext: "",
		RedirectURL:        redirectURL,
		ExpiresAt:          time.Now().UTC().Add(10 * time.Minute),
	}
	if shopID != nil && *shopID != uuid.Nil {
		row.ShopBindingContext = shopID.String()
	}
	return s.DB.WithContext(ctx).Create(&row).Error
}

// consumeDouyinOAuthState marks state as used exactly once. Returns typed DouyinAuthError on failure.
func (s *Service) consumeDouyinOAuthState(ctx context.Context, rawState string) (*DouyinOAuthState, error) {
	if s == nil || s.DB == nil {
		return nil, douyinErr(DouyinOAuthStateInvalid, douyinFriendlyMessage(DouyinOAuthStateInvalid), nil)
	}
	hash := hashDouyinOAuthState(rawState)
	var row DouyinOAuthState
	if err := s.DB.WithContext(ctx).Where("state_hash = ?", hash).First(&row).Error; err != nil {
		return nil, douyinErr(platformdouyin.CodeDouyinOAuthStateMissing, "抖店授权状态无效或不存在，请重新发起授权。", err)
	}
	if row.IsConsumed() {
		return nil, douyinErr(platformdouyin.CodeDouyinOAuthStateAlreadyUsed, "抖店授权状态已使用，请重新发起授权。", nil)
	}
	if row.IsExpired() {
		return nil, douyinErr(platformdouyin.CodeDouyinOAuthStateExpired, "抖店授权状态已过期，请重新发起授权。", nil)
	}
	if row.UserID == nil || *row.UserID == uuid.Nil {
		return nil, douyinErr(DouyinOAuthStateInvalid, douyinFriendlyMessage(DouyinOAuthStateInvalid), nil)
	}
	var actor struct {
		TenantID     int64
		TokenVersion int64
		Status       string
	}
	if err := s.DB.WithContext(ctx).Table("admin_users").
		Select("tenant_id, token_version, status").
		Where("id = ?", *row.UserID).Take(&actor).Error; err != nil || actor.TenantID != row.TenantID || !strings.EqualFold(strings.TrimSpace(actor.Status), "active") || actor.TokenVersion != row.TokenVersion {
		return nil, douyinErr(DouyinOAuthStateInvalid, douyinFriendlyMessage(DouyinOAuthStateInvalid), err)
	}
	now := time.Now().UTC()
	res := s.DB.WithContext(ctx).Model(&DouyinOAuthState{}).
		Where("id = ? AND consumed_at IS NULL", row.ID).
		Updates(map[string]any{"consumed_at": now, "updated_at": now})
	if res.Error != nil {
		return nil, douyinErr(DouyinOAuthStateInvalid, douyinFriendlyMessage(DouyinOAuthStateInvalid), res.Error)
	}
	if res.RowsAffected != 1 {
		return nil, douyinErr(platformdouyin.CodeDouyinOAuthStateAlreadyUsed, "抖店授权状态已使用，请重新发起授权。", nil)
	}
	row.ConsumedAt = &now
	return &row, nil
}

// validateDouyinOAuthStateCache checks Redis only when a cached value exists.
// The database state hash is the durable single-use authority, so a cache miss
// or Redis outage must not invalidate a still-valid database record.
func validateDouyinOAuthStateCache(raw string, state *DouyinOAuthState) error {
	if state == nil {
		return fmt.Errorf("shop: OAuth state unavailable")
	}
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	payload, err := decodeDouyinStatePayload(raw)
	if err != nil {
		return fmt.Errorf("shop: invalid cached OAuth state: %w", err)
	}
	if state.UserID == nil || payload.TenantID != state.TenantID || payload.AdminID != state.UserID.String() {
		return fmt.Errorf("shop: cached OAuth state binding mismatch")
	}
	shopID := parseUUIDPtr(state.ShopBindingContext)
	if (state.ShopBindingContext != "" && shopID == nil) || payload.ShopID != firstUUIDString(shopID) {
		return fmt.Errorf("shop: cached OAuth state binding mismatch")
	}
	return nil
}

func firstUUIDString(id *uuid.UUID) string {
	if id == nil || *id == uuid.Nil {
		return ""
	}
	return id.String()
}
