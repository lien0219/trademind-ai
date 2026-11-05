package shop

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	platformshopee "github.com/trademind-ai/trademind/backend/internal/providers/platform/shopee"
)

const shopeeOAuthRedisPrefix = "oauth:shopee:state:"

// ShopeeAuthorizeURLResult is GET /shops/:id/oauth/shopee/authorize-url payload.
type ShopeeAuthorizeURLResult struct {
	AuthorizeURL string `json:"authorizeUrl"`
	State        string `json:"state"`
}

// ShopeeOAuthCallbackBody POST /shops/:id/oauth/shopee/callback input.
type ShopeeOAuthCallbackBody struct {
	Code          string `json:"code"`
	State         string `json:"state"`
	ShopID        string `json:"shopId"`
	MainAccountID string `json:"mainAccountId"`
}

func (s *Service) ShopeeOAuthAuthorizeURL(c *gin.Context, shopID uuid.UUID, redirectOverride string, adminID *uuid.UUID) (*ShopeeAuthorizeURLResult, error) {
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
	if strings.TrimSpace(row.Platform) != "shopee" {
		return nil, fmt.Errorf("shop platform must be shopee")
	}
	st, err := randomOAuthState()
	if err != nil {
		return nil, err
	}

	_, _, auth, err := s.decryptedAuth(c, shopID)
	if err != nil {
		return nil, err
	}
	cfg, err := platformshopee.ResolveRuntime(auth)
	if err != nil {
		return nil, err
	}
	u, err := platformshopee.BuildAuthPartnerURL(cfg, redirectOverride)
	if err != nil {
		return nil, err
	}

	if err := s.saveOAuthState(c.Request.Context(), shopeeOAuthRedisPrefix, st, binding); err != nil {
		return nil, err
	}
	return &ShopeeAuthorizeURLResult{AuthorizeURL: u, State: st}, nil
}

func (s *Service) ShopeeOAuthCallback(c *gin.Context, shopID uuid.UUID, body ShopeeOAuthCallbackBody, adminID *uuid.UUID) (*ShopDetailDTO, error) {
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
	binding, err := s.consumeOAuthState(c.Request.Context(), shopeeOAuthRedisPrefix, st)
	if err != nil {
		return nil, err
	}
	if err := s.validateOAuthStateBinding(c, binding, shopID); err != nil {
		return nil, err
	}

	shopIDIntStr := strings.TrimSpace(body.ShopID)
	if shopIDIntStr == "" {
		s.oauthShopeeLog(c, adminID, shopID, "failed", "shopId required in callback body")
		if err := s.setAuthStatusCtx(c.Request.Context(), shopID, AuthError); err != nil {
			return nil, fmt.Errorf("shopId is required (paste from Shopee redirect); persist auth status: %w", err)
		}
		return nil, fmt.Errorf("shopId is required (paste from Shopee redirect)")
	}
	shopeeShopID, err := strconv.ParseInt(shopIDIntStr, 10, 64)
	if err != nil || shopeeShopID <= 0 {
		return nil, fmt.Errorf("invalid shopId")
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
	defer cancel()

	var row Shop
	if err := s.DB.WithContext(ctx).Where("id = ? AND tenant_id = ?", shopID, tenantID).First(&row).Error; err != nil {
		return nil, err
	}
	if strings.TrimSpace(row.Platform) != "shopee" {
		return nil, fmt.Errorf("shop platform must be shopee")
	}

	c2 := c.Copy()
	c2.Request = c.Request.WithContext(ctx)

	_, _, auth, err := s.decryptedAuth(c2, shopID)
	if err != nil {
		return nil, err
	}

	tokEnv, err := platformshopee.ExchangeAuthCode(ctx, auth, code, shopeeShopID)
	if err != nil {
		s.oauthShopeeLog(c, adminID, shopID, "failed", err.Error())
		if statusErr := s.setAuthStatusCtx(ctx, shopID, AuthError); statusErr != nil {
			return nil, fmt.Errorf("OAuth exchange failed: %w (persist auth status: %v)", err, statusErr)
		}
		return nil, err
	}

	ub := UpdateAuthBody{
		AuthType:     "oauth2",
		AccessToken:  tokEnv.AccessToken,
		RefreshToken: tokEnv.RefreshToken,
		ExpiresAt:    tokEnv.AccessExpiresAt,
		SellerID:     shopIDIntStr,
	}
	if mid := strings.TrimSpace(body.MainAccountID); mid != "" {
		ub.MerchantID = mid
	}
	auth2 := auth
	auth2.AccessToken = ub.AccessToken
	auth2.RefreshToken = ub.RefreshToken
	auth2.SellerID = ub.SellerID
	shopFields := map[string]any{}
	cfg, errC := platformshopee.ResolveRuntime(auth2)
	if errC == nil {
		if nm, region, currency, ext, perr := platformshopee.GetShopInfo(ctx, cfg, shopeeShopID, strings.TrimSpace(auth2.AccessToken)); perr == nil {
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
	}

	raw := map[string]any{
		"provider":        "shopee",
		"shop_id":         shopIDIntStr,
		"main_account_id": strings.TrimSpace(body.MainAccountID),
		"saved_at":        time.Now().UTC().Format(time.RFC3339),
	}
	rawB, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	return s.persistOAuthAuthorization(c, ctx, shopID, ub, adminID, shopFields, map[string]any{"raw_data": datatypes.JSON(rawB)}, "shop.oauth.shopee.success")
}

func (s *Service) oauthShopeeLog(c *gin.Context, adminID *uuid.UUID, shopID uuid.UUID, st, msg string) {
	if s.OpLog == nil {
		return
	}
	action := "shop.oauth.shopee.success"
	if st != "success" {
		action = "shop.oauth.shopee.failed"
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
