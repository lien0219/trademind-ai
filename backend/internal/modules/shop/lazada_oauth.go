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
	var row Shop
	if err := s.DB.WithContext(c.Request.Context()).First(&row, "id = ?", shopID).Error; err != nil {
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
	key := lazadaOAuthRedisPrefix + st
	if err := s.Redis.Set(c.Request.Context(), key, shopID.String(), 10*time.Minute).Err(); err != nil {
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
	key := lazadaOAuthRedisPrefix + st
	saved, err := s.Redis.Get(c.Request.Context(), key).Result()
	if err != nil || strings.TrimSpace(saved) == "" {
		return nil, fmt.Errorf("invalid or expired oauth state")
	}
	_ = s.Redis.Del(c.Request.Context(), key)
	if strings.TrimSpace(saved) != shopID.String() {
		return nil, fmt.Errorf("state does not match shop")
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
	defer cancel()

	var row Shop
	if err := s.DB.WithContext(ctx).First(&row, "id = ?", shopID).Error; err != nil {
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
		_ = s.setAuthStatusCtx(ctx, shopID, AuthError)
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

	if _, err := s.UpdateAuth(c, shopID, ub, adminID); err != nil {
		s.oauthLazadaLog(c, adminID, shopID, "failed", err.Error())
		return nil, err
	}

	_, _, auth2, err := s.decryptedAuth(c2, shopID)
	if err == nil {
		cfg, errC := platformlazada.ResolveRuntime(auth2)
		if errC == nil {
			acc := strings.TrimSpace(auth2.AccessToken)
			if nm, sellerID, region, shortCode, perr := platformlazada.GetSellerInfo(ctx, cfg, acc); perr == nil {
				updates := map[string]any{"auth_status": AuthAuthorized}
				if nm != "" {
					updates["shop_name"] = nm
				}
				if ext := firstNonEmpty(shortCode, sellerID); ext != "" {
					updates["external_shop_id"] = ext
				}
				if region != "" {
					updates["region"] = region
				}
				_ = s.DB.WithContext(ctx).Model(&Shop{}).Where("id = ?", shopID).Updates(updates).Error
			}
		}
	}

	raw := map[string]any{
		"provider":   "lazada",
		"saved_at":   time.Now().UTC().Format(time.RFC3339),
		"country":    tok.Country,
		"seller_ref": strings.TrimSpace(tok.SellerID),
	}
	rawB, _ := json.Marshal(raw)
	_ = s.DB.WithContext(ctx).Model(&ShopAuthToken{}).Where("shop_id = ?", shopID).
		Updates(map[string]any{"raw_data": datatypes.JSON(rawB)}).Error

	s.oauthLazadaLog(c, adminID, shopID, "success", "lazada oauth tokens saved")
	return s.GetDetail(c, shopID)
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
