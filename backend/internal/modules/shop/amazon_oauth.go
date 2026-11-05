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
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	platformamazon "github.com/trademind-ai/trademind/backend/internal/providers/platform/amazon"
)

const amazonOAuthRedisPrefix = "oauth:amazon:state:"

// AmazonAuthorizeURLResult is GET /shops/:id/oauth/amazon/authorize-url payload.
type AmazonAuthorizeURLResult struct {
	AuthorizeURL string `json:"authorizeUrl"`
	State        string `json:"state"`
}

// AmazonOAuthCallbackBody POST /shops/:id/oauth/amazon/callback input.
type AmazonOAuthCallbackBody struct {
	Code             string `json:"code"`
	State            string `json:"state"`
	SellingPartnerID string `json:"sellingPartnerId"`
	MarketplaceID    string `json:"marketplaceId"`
}

func (s *Service) AmazonOAuthAuthorizeURL(c *gin.Context, shopID uuid.UUID, redirectOverride string, adminID *uuid.UUID) (*AmazonAuthorizeURLResult, error) {
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
	if strings.TrimSpace(row.Platform) != "amazon" {
		return nil, fmt.Errorf("shop platform must be amazon")
	}
	st, err := randomOAuthState()
	if err != nil {
		return nil, err
	}
	_, _, auth, err := s.decryptedAuth(c, shopID)
	if err != nil {
		return nil, err
	}
	cfg, err := platformamazon.ResolveRuntime(auth)
	if err != nil {
		return nil, err
	}
	u, err := platformamazon.BuildAuthorizeURL(cfg, st, redirectOverride)
	if err != nil {
		return nil, err
	}
	if err := s.saveOAuthState(c.Request.Context(), amazonOAuthRedisPrefix, st, binding); err != nil {
		return nil, err
	}
	return &AmazonAuthorizeURLResult{AuthorizeURL: u, State: st}, nil
}

func (s *Service) AmazonOAuthCallback(c *gin.Context, shopID uuid.UUID, body AmazonOAuthCallbackBody, adminID *uuid.UUID) (*ShopDetailDTO, error) {
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
	binding, err := s.consumeOAuthState(c.Request.Context(), amazonOAuthRedisPrefix, st)
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
	if strings.TrimSpace(row.Platform) != "amazon" {
		return nil, fmt.Errorf("shop platform must be amazon")
	}

	c2 := c.Copy()
	c2.Request = c.Request.WithContext(ctx)
	_, _, auth, err := s.decryptedAuth(c2, shopID)
	if err != nil {
		return nil, err
	}
	cfg, err := platformamazon.ResolveRuntime(auth)
	if err != nil {
		return nil, err
	}
	rd := strings.TrimSpace(cfg.RedirectURI)

	tok, err := platformamazon.ExchangeAuthCode(ctx, cfg, code, rd)
	if err != nil {
		s.oauthAmazonLog(c, adminID, shopID, "failed", err.Error())
		if statusErr := s.setAuthStatusCtx(ctx, shopID, AuthError); statusErr != nil {
			return nil, fmt.Errorf("OAuth exchange failed: %w (persist auth status: %v)", err, statusErr)
		}
		return nil, err
	}

	marketplaceID := strings.TrimSpace(body.MarketplaceID)
	if marketplaceID == "" {
		marketplaceID = strings.TrimSpace(cfg.MarketplaceID)
	}
	spid := strings.TrimSpace(body.SellingPartnerID)

	acfg := map[string]any{
		"marketplaceId": marketplaceID,
	}
	if r := strings.TrimSpace(cfg.Region); r != "" {
		acfg["region"] = r
	}
	if spid != "" {
		acfg["sellingPartnerId"] = spid
	}

	ub := UpdateAuthBody{
		AuthType:      "oauth2",
		AccessToken:   tok.AccessToken,
		RefreshToken:  tok.RefreshToken,
		ExpiresAt:     tok.AccessExpiresAt,
		SellerID:      spid,
		MarketplaceID: marketplaceID,
		AuthConfig:    acfg,
	}
	if tok.RefreshExpiresAt != nil {
		ub.RefreshExpiresAt = tok.RefreshExpiresAt
	}

	shopFields := map[string]any{}
	if spid != "" {
		shopFields["external_shop_id"] = spid
		shopFields["shop_code"] = spid
	}

	req2 := platformp.TestConnectionRequest{
		AuthType:             "oauth2",
		AccessToken:          tok.AccessToken,
		RefreshToken:         tok.RefreshToken,
		SellerID:             spid,
		MarketplaceID:        marketplaceID,
		AccessTokenExpiresAt: tok.AccessExpiresAt,
	}
	cfg2, errC := platformamazon.ResolveRuntime(req2)
	if errC == nil {
		if pr, perr := platformamazon.MarketplaceParticipationsProbe(ctx, cfg2, tok.AccessToken); perr == nil && pr != nil && pr.OK {
			shopFields["auth_status"] = AuthAuthorized
			if pr.Currency != "" {
				shopFields["currency"] = pr.Currency
			}
			if pr.Region != "" {
				shopFields["region"] = pr.Region
			}
		}
	}

	raw := map[string]any{
		"provider":        "amazon",
		"saved_at":        time.Now().UTC().Format(time.RFC3339),
		"marketplace_ref": marketplaceID,
		"seller_ref":      spid,
	}
	rawB, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	return s.persistOAuthAuthorization(c, ctx, shopID, ub, adminID, shopFields, map[string]any{"raw_data": datatypes.JSON(rawB)}, "shop.oauth.amazon.success")
}

func (s *Service) oauthAmazonLog(c *gin.Context, adminID *uuid.UUID, shopID uuid.UUID, st, msg string) {
	if s.OpLog == nil {
		return
	}
	action := "shop.oauth.amazon.success"
	if st != "success" {
		action = "shop.oauth.amazon.failed"
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
