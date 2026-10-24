package shop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
)

const oauthStateTTL = 10 * time.Minute

// oauthExpectedTokenVersionContextKey is copied into the request context when
// an OAuth state is validated. Persistence code must read the captured version
// again under a database row lock after the provider exchange completes.
const oauthExpectedTokenVersionContextKey = "shop.oauth.expected_token_version"

type oauthExpectedTokenVersionValueKey struct{}

func setOAuthExpectedTokenVersion(c *gin.Context, version int64) {
	if c == nil {
		return
	}
	c.Set(oauthExpectedTokenVersionContextKey, version)
	// Preserve the original Douyin key while all OAuth callbacks converge on
	// this shared setter and persistence-time validator.
	c.Set(douyinOAuthExpectedTokenVersionContextKey, version)
	if c.Request != nil {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), oauthExpectedTokenVersionValueKey{}, version))
	}
}

func oauthExpectedTokenVersion(c *gin.Context) (int64, bool) {
	if c == nil {
		return 0, false
	}
	if raw, ok := c.Get(oauthExpectedTokenVersionContextKey); ok {
		switch v := raw.(type) {
		case int64:
			return v, true
		case int:
			return int64(v), true
		}
	}
	if raw, ok := c.Get(douyinOAuthExpectedTokenVersionContextKey); ok {
		switch v := raw.(type) {
		case int64:
			return v, true
		case int:
			return int64(v), true
		}
	}
	return oauthExpectedTokenVersionFromContext(c.Request.Context())
}

func oauthExpectedTokenVersionFromContext(ctx context.Context) (int64, bool) {
	if ctx == nil {
		return 0, false
	}
	version, ok := ctx.Value(oauthExpectedTokenVersionValueKey{}).(int64)
	return version, ok
}

// oauthStateBinding is the minimum identity and security snapshot required to
// complete an authenticated OAuth callback. Redis TTL provides expiry; the
// token version and user binding make role/password revocation effective even
// while an authorization window is still open.
type oauthStateBinding struct {
	TenantID         int64     `json:"tenantId"`
	UserID           uuid.UUID `json:"userId"`
	TokenVersion     int       `json:"tokenVersion"`
	ShopID           uuid.UUID `json:"shopId"`
	ShopTokenVersion int64     `json:"shopTokenVersion"`
}

func (s *Service) newOAuthStateBinding(c *gin.Context, shopID uuid.UUID) (oauthStateBinding, error) {
	if s == nil || s.DB == nil {
		return oauthStateBinding{}, fmt.Errorf("shop: db unavailable")
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return oauthStateBinding{}, err
	}
	p, err := adminperm.LoadPrincipal(c, s.DB)
	if err != nil {
		return oauthStateBinding{}, err
	}
	if p == nil || p.UserID == uuid.Nil || p.Disabled {
		return oauthStateBinding{}, errors.New("shop: OAuth actor unavailable")
	}
	var actor struct {
		TenantID     int64
		TokenVersion int
		Status       string
	}
	if err := s.DB.WithContext(c.Request.Context()).Table("admin_users").
		Select("tenant_id, token_version, status").
		Where("id = ? AND tenant_id = ?", p.UserID, tenantID).
		Take(&actor).Error; err != nil {
		return oauthStateBinding{}, fmt.Errorf("shop: resolve OAuth actor: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(actor.Status), "active") {
		return oauthStateBinding{}, errors.New("shop: OAuth actor is disabled")
	}
	var token ShopAuthToken
	if err := s.DB.WithContext(c.Request.Context()).Select("token_version").Where("shop_id = ?", shopID).First(&token).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return oauthStateBinding{}, fmt.Errorf("shop: resolve OAuth shop token: %w", err)
	}
	return oauthStateBinding{
		TenantID:         tenantID,
		UserID:           p.UserID,
		TokenVersion:     actor.TokenVersion,
		ShopID:           shopID,
		ShopTokenVersion: token.TokenVersion,
	}, nil
}

func (s *Service) saveOAuthState(ctx context.Context, prefix, state string, binding oauthStateBinding) error {
	if s == nil || s.Redis == nil || s.Redis.Client == nil {
		return fmt.Errorf("shop: redis unavailable")
	}
	raw, err := json.Marshal(binding)
	if err != nil {
		return err
	}
	return s.Redis.Set(ctx, prefix+state, raw, oauthStateTTL).Err()
}

func (s *Service) consumeOAuthState(ctx context.Context, prefix, state string) (oauthStateBinding, error) {
	if s == nil || s.Redis == nil || s.Redis.Client == nil {
		return oauthStateBinding{}, fmt.Errorf("shop: redis unavailable")
	}
	raw, err := s.Redis.GetDel(ctx, prefix+state).Result()
	if err != nil || strings.TrimSpace(raw) == "" {
		return oauthStateBinding{}, fmt.Errorf("invalid or expired oauth state")
	}
	var binding oauthStateBinding
	if err := json.Unmarshal([]byte(raw), &binding); err != nil {
		return oauthStateBinding{}, fmt.Errorf("invalid oauth state")
	}
	if binding.UserID == uuid.Nil || binding.ShopID == uuid.Nil || binding.TokenVersion <= 0 || binding.ShopTokenVersion < 0 || binding.TenantID < 0 {
		return oauthStateBinding{}, fmt.Errorf("invalid oauth state")
	}
	return binding, nil
}

func (s *Service) validateOAuthStateBinding(c *gin.Context, binding oauthStateBinding, shopID uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("shop: db unavailable")
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return err
	}
	if binding.TenantID != tenantID || binding.ShopID != shopID {
		return fmt.Errorf("state does not match shop or tenant")
	}
	p, err := adminperm.LoadPrincipal(c, s.DB)
	if err != nil {
		return err
	}
	if p == nil || p.Disabled || p.UserID == uuid.Nil || p.UserID != binding.UserID {
		return fmt.Errorf("OAuth state actor is no longer authorized")
	}
	var actor struct {
		TenantID     int64
		TokenVersion int
		Status       string
	}
	if err := s.DB.WithContext(c.Request.Context()).Table("admin_users").
		Select("tenant_id, token_version, status").
		Where("id = ?", binding.UserID).
		Take(&actor).Error; err != nil || actor.TenantID != binding.TenantID ||
		!strings.EqualFold(strings.TrimSpace(actor.Status), "active") || actor.TokenVersion != binding.TokenVersion {
		return fmt.Errorf("OAuth state actor is no longer authorized")
	}
	var token ShopAuthToken
	if err := s.DB.WithContext(c.Request.Context()).Select("token_version").Where("shop_id = ?", shopID).First(&token).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("resolve OAuth shop token: %w", err)
	}
	if token.TokenVersion != binding.ShopTokenVersion {
		return fmt.Errorf("OAuth state shop authorization is stale")
	}
	setOAuthExpectedTokenVersion(c, int64(binding.TokenVersion))
	setOAuthExpectedShopTokenVersion(c, binding.ShopTokenVersion)
	return nil
}
