package shop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	platformlazada "github.com/trademind-ai/trademind/backend/internal/providers/platform/lazada"
)

const lazadaOAuthRedisPrefix = "oauth:lazada:state:"

// LazadaAuthorizeURLResult is GET /shops/:id/oauth/lazada/authorize-url payload.
type LazadaAuthorizeURLResult struct {
	AuthorizeURL string `json:"authorizeUrl"`
	State        string `json:"state"`
}

// LazadaOAuthCallbackBody POST /shops/:id/oauth/lazada/callback input.
type LazadaOAuthCallbackBody struct {
	Code  string `json:"code"`
	State string `json:"state"`
}

func (s *Service) LazadaOAuthAuthorizeURL(c *gin.Context, shopID uuid.UUID, redirectOverride string, adminID *uuid.UUID) (*LazadaAuthorizeURLResult, error) {
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
	if strings.TrimSpace(row.Platform) != "lazada" {
		return nil, fmt.Errorf("shop platform must be lazada")
	}
	st, err := randomOAuthState()
	if err != nil {
		return nil, err
	}
	_, _, auth, err := s.decryptedAuth(c, shopID)
	if err != nil {
		return nil, err
	}
	cfg, err := platformlazada.ResolveRuntime(auth)
	if err != nil {
		return nil, err
	}
	u, err := platformlazada.BuildAuthorizeURL(cfg, st, redirectOverride, strings.TrimSpace(row.Region))
	if err != nil {
		return nil, err
	}
	if err := s.saveOAuthState(c.Request.Context(), lazadaOAuthRedisPrefix, st, binding); err != nil {
		return nil, err
	}
	return &LazadaAuthorizeURLResult{AuthorizeURL: u, State: st}, nil
}

func (s *Service) LazadaOAuthCallback(c *gin.Context, shopID uuid.UUID, body LazadaOAuthCallbackBody, adminID *uuid.UUID) (*ShopDetailDTO, error) {
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
	binding, err := s.consumeOAuthState(c.Request.Context(), lazadaOAuthRedisPrefix, st)
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
	if strings.TrimSpace(row.Platform) != "lazada" {
		return nil, fmt.Errorf("shop platform must be lazada")
	}

	c2 := c.Copy()
	c2.Request = c.Request.WithContext(ctx)
	_, _, auth, err := s.decryptedAuth(c2, shopID)
	if err != nil {
		return nil, err
	}

	tok, err := platformlazada.ExchangeAuthCode(ctx, auth, code)
	if err != nil {
		s.oauthLazadaLog(c, adminID, shopID, "failed", err.Error())
		if statusErr := s.setAuthStatusCtx(ctx, shopID, AuthError); statusErr != nil {
			return nil, fmt.Errorf("OAuth exchange failed: %w (persist auth status: %v)", err, statusErr)
		}
		return nil, err
	}

	ub := UpdateAuthBody{
		AuthType:     "oauth2",
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		ExpiresAt:    tok.AccessExpiresAt,
	}
	if tok.RefreshExpiresAt != nil {
		ub.RefreshExpiresAt = tok.RefreshExpiresAt
	}
	if sid := strings.TrimSpace(tok.SellerID); sid != "" {
		ub.SellerID = sid
	}
	if tok.Country != "" {
		ub.AuthConfig = map[string]any{
			"country":    tok.Country,
			"account_id": tok.AccountID,
		}
	}

	auth2 := auth
	auth2.AccessToken = ub.AccessToken
	auth2.RefreshToken = ub.RefreshToken
	if ub.AuthConfig != nil {
		auth2.Extra = make(map[string]string, len(ub.AuthConfig))
		for key, value := range ub.AuthConfig {
			auth2.Extra[key] = fmt.Sprint(value)
		}
	}
	shopFields := map[string]any{}
	cfg, errC := platformlazada.ResolveRuntime(auth2)
	if errC == nil {
		acc := strings.TrimSpace(auth2.AccessToken)
		if nm, sellerID, region, shortCode, perr := platformlazada.GetSellerInfo(ctx, cfg, acc); perr == nil {
			shopFields["auth_status"] = AuthAuthorized
			if nm != "" {
				shopFields["shop_name"] = nm
			}
			if ext := firstNonEmpty(shortCode, sellerID); ext != "" {
				shopFields["external_shop_id"] = ext
			}
			if region != "" {
				shopFields["region"] = region
			}
		}
	}

	raw := map[string]any{
		"provider":   "lazada",
		"saved_at":   time.Now().UTC().Format(time.RFC3339),
		"country":    tok.Country,
		"seller_ref": strings.TrimSpace(tok.SellerID),
	}
	rawB, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	return s.persistOAuthAuthorization(c, ctx, shopID, ub, adminID, shopFields, map[string]any{"raw_data": datatypes.JSON(rawB)}, "shop.oauth.lazada.success")
}

func firstNonEmpty(a, b string) string {
	a = strings.TrimSpace(a)
	if a != "" {
		return a
	}
	return strings.TrimSpace(b)
}

func (s *Service) oauthLazadaLog(c *gin.Context, adminID *uuid.UUID, shopID uuid.UUID, st, msg string) {
	if s.OpLog == nil {
		return
	}
	action := "shop.oauth.lazada.success"
	if st != "success" {
		action = "shop.oauth.lazada.failed"
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
