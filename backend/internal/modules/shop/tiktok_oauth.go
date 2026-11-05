package shop

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	platformtiktok "github.com/trademind-ai/trademind/backend/internal/providers/platform/tiktok"
)

const tiktokOAuthRedisPrefix = "oauth:tiktok:state:"

func randomOAuthState() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// TikTokAuthorizeURLResult is GET /shops/:id/oauth/tiktok/authorize-url payload.
type TikTokAuthorizeURLResult struct {
	AuthorizeURL string `json:"authorizeUrl"`
	State        string `json:"state"`
}

// TikTokOAuthCallbackBody POST /shops/:id/oauth/tiktok/callback input.
type TikTokOAuthCallbackBody struct {
	Code  string `json:"code"`
	State string `json:"state"`
}

func (s *Service) TikTokOAuthAuthorizeURL(c *gin.Context, shopID uuid.UUID, redirectOverride string, adminID *uuid.UUID) (*TikTokAuthorizeURLResult, error) {
	_ = adminID
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("shop: no db")
	}
	if s.Redis == nil || s.Redis.Client == nil {
		return nil, fmt.Errorf("redis required for OAuth state")
	}
	tenantID, err := requireShopOperateContext(c, s.DB, shopID)
	if err != nil {
		return nil, err
	}
	binding, err := s.newOAuthStateBinding(c, shopID)
	if err != nil {
		return nil, err
	}
	var row Shop
	if err := s.DB.WithContext(c.Request.Context()).Where("id = ? AND tenant_id = ?", shopID, tenantID).First(&row).Error; err != nil {
		return nil, err
	}
	if strings.TrimSpace(row.Platform) != "tiktok" {
		return nil, fmt.Errorf("shop platform must be tiktok")
	}
	st, err := randomOAuthState()
	if err != nil {
		return nil, err
	}

	_, _, auth, err := s.decryptedAuth(c, shopID)
	if err != nil {
		return nil, err
	}

	cfg, err := platformtiktok.ResolveRuntime(auth)
	if err != nil {
		return nil, err
	}
	u, err := platformtiktok.BuildAuthorizeURL(cfg, st, redirectOverride)
	if err != nil {
		return nil, err
	}

	if err := s.saveOAuthState(c.Request.Context(), tiktokOAuthRedisPrefix, st, binding); err != nil {
		return nil, err
	}

	return &TikTokAuthorizeURLResult{AuthorizeURL: u, State: st}, nil
}

func (s *Service) TikTokOAuthCallback(c *gin.Context, shopID uuid.UUID, body TikTokOAuthCallbackBody, adminID *uuid.UUID) (*ShopDetailDTO, error) {
	code := strings.TrimSpace(body.Code)
	st := strings.TrimSpace(body.State)
	if code == "" || st == "" {
		return nil, fmt.Errorf("code and state required")
	}
	if s.Redis == nil || s.Redis.Client == nil {
		return nil, fmt.Errorf("redis required")
	}
	tenantID, err := requireShopOperateContext(c, s.DB, shopID)
	if err != nil {
		return nil, err
	}
	binding, err := s.consumeOAuthState(c.Request.Context(), tiktokOAuthRedisPrefix, st)
	if err != nil {
		return nil, err
	}
	if err := s.validateOAuthStateBinding(c, binding, shopID); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
	defer cancel()

	var row Shop
	if err := s.DB.WithContext(ctx).Where("id = ? AND tenant_id = ?", shopID, tenantID).First(&row).Error; err != nil {
		return nil, err
	}
	if strings.TrimSpace(row.Platform) != "tiktok" {
		return nil, fmt.Errorf("shop platform must be tiktok")
	}

	c2 := c.Copy()
	c2.Request = c.Request.WithContext(ctx)

	_, _, auth, err := s.decryptedAuth(c2, shopID)
	if err != nil {
		return nil, err
	}

	tbundle, _, _, err := platformtiktok.ExchangeAuthCode(ctx, auth, code)
	if err != nil {
		s.oauthTikTokLog(c, adminID, shopID, "failed", err.Error())
		if statusErr := s.setAuthStatusCtx(ctx, shopID, AuthError); statusErr != nil {
			return nil, fmt.Errorf("OAuth exchange failed: %w (persist auth status: %v)", err, statusErr)
		}
		return nil, err
	}

	ub := UpdateAuthBody{
		AuthType:         "oauth2",
		AccessToken:      tbundle.AccessToken,
		RefreshToken:     tbundle.RefreshToken,
		ExpiresAt:        tbundle.AccessExpiresAt,
		RefreshExpiresAt: tbundle.RefreshExpiresAt,
	}
	auth2 := auth
	auth2.AccessToken = ub.AccessToken
	auth2.RefreshToken = ub.RefreshToken
	shopFields := map[string]any{}
	tokenFields := map[string]any{}
	if cipher, nm, ext, region, currency, perr := platformtiktok.PrimaryAuthorizedShop(ctx, auth2, auth2.AccessToken); perr == nil && cipher != "" {
		tokenFields["merchant_id"] = cipher
		shopFields["auth_status"] = AuthAuthorized
		if nm != "" {
			shopFields["shop_name"] = nm
		}
		if ext != "" {
			shopFields["external_shop_id"] = ext
		}
		if region != "" {
			shopFields["region"] = region
		}
		if currency != "" {
			shopFields["currency"] = strings.ToUpper(currency)
		}
	}
	return s.persistOAuthAuthorization(c, ctx, shopID, ub, adminID, shopFields, tokenFields, "shop.oauth.tiktok.success")
}

func (s *Service) oauthTikTokLog(c *gin.Context, adminID *uuid.UUID, shopID uuid.UUID, st, msg string) {
	if s.OpLog == nil {
		return
	}
	action := "shop.oauth.tiktok.success"
	if st != "success" {
		action = "shop.oauth.tiktok.failed"
	}
	if len(msg) > 500 {
		msg = msg[:500] + "..."
	}
	_ = s.OpLog.Write(c, operationlog.WriteOpts{
		AdminUserID: adminID,
		Action:      action,
		Resource:    "shop",
		ResourceID:  shopID.String(),
		Status:      st,
		Message:     fmt.Sprintf("shopId=%s %s", shopID.String(), msg),
	})
}
