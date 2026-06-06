package shop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
)

const (
	douyinOAuthRedisPrefix = "oauth:douyin_shop:state:"

	DouyinAppConfigIncomplete = "DOUYIN_APP_CONFIG_INCOMPLETE"
	DouyinOAuthStateInvalid   = "DOUYIN_OAUTH_STATE_INVALID"
	DouyinOAuthDenied         = "DOUYIN_OAUTH_DENIED"
	DouyinOAuthCodeMissing    = "DOUYIN_OAUTH_CODE_MISSING"
	DouyinTokenExchangeFailed = "DOUYIN_TOKEN_EXCHANGE_FAILED"
	DouyinTokenRefreshFailed  = "DOUYIN_TOKEN_REFRESH_FAILED"
	DouyinShopInfoFailed      = "DOUYIN_SHOP_INFO_FAILED"
	DouyinAuthExpired         = "DOUYIN_AUTH_EXPIRED"
	DouyinPermissionDenied    = "DOUYIN_PERMISSION_DENIED"
	UnknownDouyinAuthError    = "UNKNOWN_DOUYIN_AUTH_ERROR"
)

type DouyinAuthError struct {
	Code    string
	Message string
	Cause   error
}

func (e *DouyinAuthError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	return e.Code
}

func (e *DouyinAuthError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func douyinErr(code, msg string, cause error) *DouyinAuthError {
	return &DouyinAuthError{Code: code, Message: msg, Cause: cause}
}

func douyinFriendlyMessage(code string) string {
	switch code {
	case DouyinAppConfigIncomplete:
		return "抖店应用配置不完整，请先填写 App Key、App Secret、回调地址和服务 ID。"
	case DouyinOAuthStateInvalid:
		return "授权状态已失效，请重新发起抖店授权。"
	case DouyinOAuthDenied:
		return "抖店授权已取消或被拒绝，请重新发起授权。"
	case DouyinOAuthCodeMissing:
		return "抖店授权回调缺少 code，请重新发起授权。"
	case DouyinTokenExchangeFailed:
		return "抖店授权换取令牌失败，请检查应用配置或重新授权。"
	case DouyinTokenRefreshFailed:
		return "抖店刷新令牌失败，请重新授权。"
	case DouyinShopInfoFailed:
		return "抖店店铺信息读取失败，请检查应用权限。"
	case DouyinAuthExpired:
		return "抖店授权已过期，请重新连接店铺。"
	case DouyinPermissionDenied:
		return "抖店权限不足，请检查应用权限或重新授权。"
	default:
		return "抖店授权异常，请稍后重试或重新发起授权。"
	}
}

type douyinOAuthStatePayload struct {
	Platform string `json:"platform"`
	AdminID  string `json:"adminId,omitempty"`
	ShopID   string `json:"shopId,omitempty"`
	Created  int64  `json:"created"`
}

func encodeDouyinStatePayload(p douyinOAuthStatePayload) (string, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func decodeDouyinStatePayload(raw string) (douyinOAuthStatePayload, error) {
	var p douyinOAuthStatePayload
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &p); err != nil {
		return p, err
	}
	if strings.TrimSpace(p.Platform) != "douyin_shop" {
		return p, fmt.Errorf("state platform mismatch")
	}
	return p, nil
}

type DouyinAuthorizeURLResult struct {
	RedirectURL  string `json:"redirectUrl"`
	AuthorizeURL string `json:"authorizeUrl"`
	State        string `json:"state"`
}

type DouyinOAuthCallbackQuery struct {
	Code             string
	State            string
	Error            string
	ErrorDescription string
}

func (s *Service) douyinGlobalConfig(ctx context.Context) (platformdouyin.RuntimeConfig, error) {
	var zero platformdouyin.RuntimeConfig
	if s == nil || s.Settings == nil {
		return zero, errors.New("settings unavailable")
	}
	plain, err := s.Settings.PlainByGroup(ctx, 0, "platform_douyin_shop")
	if err != nil {
		return zero, err
	}
	cfg, err := platformdouyin.RuntimeFromMergedMap(plain)
	if err != nil {
		return zero, douyinErr(DouyinAppConfigIncomplete, douyinFriendlyMessage(DouyinAppConfigIncomplete), err)
	}
	if strings.TrimSpace(cfg.ServiceID) == "" {
		return zero, douyinErr(DouyinAppConfigIncomplete, douyinFriendlyMessage(DouyinAppConfigIncomplete), nil)
	}
	return cfg, nil
}

func (s *Service) DouyinOAuthStart(c *gin.Context, shopID *uuid.UUID, adminID *uuid.UUID) (*DouyinAuthorizeURLResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("shop: no db")
	}
	if s.Redis == nil || s.Redis.Client == nil {
		return nil, fmt.Errorf("redis required for OAuth state")
	}
	ctx := c.Request.Context()
	if shopID != nil && *shopID != uuid.Nil {
		var row Shop
		if err := s.DB.WithContext(ctx).First(&row, "id = ?", *shopID).Error; err != nil {
			return nil, err
		}
		if strings.TrimSpace(row.Platform) != "douyin_shop" {
			return nil, fmt.Errorf("shop platform must be douyin_shop")
		}
	}
	cfg, err := s.douyinGlobalConfig(ctx)
	if err != nil {
		s.douyinLog(c, adminID, nil, "douyin.auth.start", "failed", douyinErrCode(err), douyinSafeErr(err))
		return nil, err
	}
	st, err := randomOAuthState()
	if err != nil {
		return nil, err
	}
	u, err := platformdouyin.BuildAuthorizeURL(cfg, st)
	if err != nil {
		return nil, douyinErr(DouyinAppConfigIncomplete, douyinFriendlyMessage(DouyinAppConfigIncomplete), err)
	}
	p := douyinOAuthStatePayload{Platform: "douyin_shop", Created: time.Now().UTC().Unix()}
	if adminID != nil {
		p.AdminID = adminID.String()
	}
	if shopID != nil && *shopID != uuid.Nil {
		p.ShopID = shopID.String()
	}
	raw, err := encodeDouyinStatePayload(p)
	if err != nil {
		return nil, err
	}
	if err := s.Redis.Set(ctx, douyinOAuthRedisPrefix+st, raw, 10*time.Minute).Err(); err != nil {
		return nil, err
	}
	s.douyinLog(c, adminID, shopID, "douyin.auth.start", "success", "", "state created")
	return &DouyinAuthorizeURLResult{RedirectURL: u, AuthorizeURL: u, State: st}, nil
}

func (s *Service) DouyinOAuthCallback(c *gin.Context, q DouyinOAuthCallbackQuery) (*ShopDetailDTO, *DouyinAuthError) {
	st := strings.TrimSpace(q.State)
	if st == "" || s == nil || s.Redis == nil || s.Redis.Client == nil {
		return nil, douyinErr(DouyinOAuthStateInvalid, douyinFriendlyMessage(DouyinOAuthStateInvalid), nil)
	}
	raw, err := s.Redis.Get(c.Request.Context(), douyinOAuthRedisPrefix+st).Result()
	if err != nil || strings.TrimSpace(raw) == "" {
		return nil, douyinErr(DouyinOAuthStateInvalid, douyinFriendlyMessage(DouyinOAuthStateInvalid), err)
	}
	_ = s.Redis.Del(c.Request.Context(), douyinOAuthRedisPrefix+st)
	payload, err := decodeDouyinStatePayload(raw)
	if err != nil {
		return nil, douyinErr(DouyinOAuthStateInvalid, douyinFriendlyMessage(DouyinOAuthStateInvalid), err)
	}
	adminID := parseUUIDPtr(payload.AdminID)
	shopID := parseUUIDPtr(payload.ShopID)
	s.douyinLog(c, adminID, shopID, "douyin.auth.callback", "success", "", "callback received")

	if e := strings.TrimSpace(q.Error); e != "" {
		msg := strings.TrimSpace(q.ErrorDescription)
		if msg == "" {
			msg = douyinFriendlyMessage(DouyinOAuthDenied)
		}
		authErr := douyinErr(DouyinOAuthDenied, msg, nil)
		s.douyinLog(c, adminID, shopID, "douyin.auth.failed", "failed", authErr.Code, authErr.Message)
		return nil, authErr
	}
	code := strings.TrimSpace(q.Code)
	if code == "" {
		authErr := douyinErr(DouyinOAuthCodeMissing, douyinFriendlyMessage(DouyinOAuthCodeMissing), nil)
		s.douyinLog(c, adminID, shopID, "douyin.auth.failed", "failed", authErr.Code, authErr.Message)
		return nil, authErr
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
	defer cancel()
	cfg, err := s.douyinGlobalConfig(ctx)
	if err != nil {
		authErr := asDouyinAuthError(err, DouyinAppConfigIncomplete)
		s.douyinLog(c, adminID, shopID, "douyin.auth.failed", "failed", authErr.Code, authErr.Message)
		return nil, authErr
	}
	tok, err := platformdouyin.Client{Config: cfg}.ExchangeCode(ctx, code)
	if err != nil {
		authErr := douyinErr(DouyinTokenExchangeFailed, douyinFriendlyMessage(DouyinTokenExchangeFailed), err)
		s.douyinLog(c, adminID, shopID, "douyin.auth.failed", "failed", authErr.Code, authErr.Message)
		if shopID != nil {
			_ = s.setAuthStatusCtx(ctx, *shopID, AuthInvalid)
		}
		return nil, authErr
	}
	detail, authErr := s.persistDouyinOAuthBundle(c, ctx, shopID, adminID, cfg, tok)
	if authErr != nil {
		s.douyinLog(c, adminID, shopID, "douyin.auth.failed", "failed", authErr.Code, authErr.Message)
		return nil, authErr
	}
	s.douyinLog(c, adminID, &detail.ID, "douyin.auth.success", "success", "", "tokens saved")
	return detail, nil
}

func parseUUIDPtr(raw string) *uuid.UUID {
	if u, err := uuid.Parse(strings.TrimSpace(raw)); err == nil {
		return &u
	}
	return nil
}

func (s *Service) persistDouyinOAuthBundle(c *gin.Context, ctx context.Context, shopID *uuid.UUID, adminID *uuid.UUID, cfg platformdouyin.RuntimeConfig, tok *platformdouyin.TokenBundle) (*ShopDetailDTO, *DouyinAuthError) {
	if tok == nil {
		return nil, douyinErr(DouyinTokenExchangeFailed, douyinFriendlyMessage(DouyinTokenExchangeFailed), nil)
	}
	platformShopID := strings.TrimSpace(tok.PlatformShopID)
	shopName := strings.TrimSpace(tok.ShopName)
	status := AuthAuthorized
	if platformShopID == "" || shopName == "" {
		status = AuthNeedCheck
	}
	if shopName == "" {
		if platformShopID != "" {
			shopName = "Douyin Shop " + platformShopID
		} else {
			shopName = "Douyin Shop"
		}
	}

	var row Shop
	if shopID != nil && *shopID != uuid.Nil {
		if err := s.DB.WithContext(ctx).First(&row, "id = ?", *shopID).Error; err != nil {
			return nil, asDouyinAuthError(err, UnknownDouyinAuthError)
		}
	} else if platformShopID != "" {
		err := s.DB.WithContext(ctx).Where("platform = ? AND external_shop_id = ?", "douyin_shop", platformShopID).First(&row).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, asDouyinAuthError(err, UnknownDouyinAuthError)
		}
	}
	if row.ID == uuid.Nil {
		row = Shop{
			Platform:       "douyin_shop",
			ShopName:       shopName,
			ShopCode:       platformShopID,
			ExternalShopID: platformShopID,
			Status:         StatusActive,
			AuthStatus:     status,
			Currency:       "CNY",
			CreatedBy:      adminID,
		}
		if err := s.DB.WithContext(ctx).Create(&row).Error; err != nil {
			return nil, asDouyinAuthError(err, UnknownDouyinAuthError)
		}
	} else {
		updates := map[string]any{"auth_status": status, "currency": "CNY"}
		if shopName != "" {
			updates["shop_name"] = shopName
		}
		if platformShopID != "" {
			updates["external_shop_id"] = platformShopID
			updates["shop_code"] = platformShopID
		}
		if err := s.DB.WithContext(ctx).Model(&Shop{}).Where("id = ?", row.ID).Updates(updates).Error; err != nil {
			return nil, asDouyinAuthError(err, UnknownDouyinAuthError)
		}
		_ = s.DB.WithContext(ctx).First(&row, "id = ?", row.ID).Error
	}

	authConfig := map[string]any{
		"serviceId":  cfg.ServiceID,
		"shopLogo":   tok.ShopLogo,
		"shopStatus": tok.ShopStatus,
	}
	raw := tok.RawNonSensitiveFields
	if raw == nil {
		raw = map[string]any{}
	}
	raw["provider"] = "douyin_shop"
	raw["shop_info_status"] = "ok"
	if status == AuthNeedCheck {
		raw["shop_info_status"] = "need_check"
	}
	scopes := tok.Scopes
	if scopes == nil {
		scopes = []any{}
	}
	ub := UpdateAuthBody{
		AuthType:         "oauth2",
		AppKey:           cfg.AppKey,
		AppSecret:        cfg.AppSecret,
		AccessToken:      tok.AccessToken,
		RefreshToken:     tok.RefreshToken,
		SellerID:         platformShopID,
		ExpiresAt:        tok.AccessExpiresAt,
		RefreshExpiresAt: tok.RefreshExpiresAt,
		Scopes:           scopes,
		AuthConfig:       authConfig,
	}
	if _, err := s.UpdateAuth(c, row.ID, ub, adminID); err != nil {
		return nil, asDouyinAuthError(err, UnknownDouyinAuthError)
	}
	finalStatus := status
	if finalStatus == "" {
		finalStatus = AuthAuthorized
	}
	_ = s.setAuthStatusCtx(ctx, row.ID, finalStatus)
	rawB, _ := json.Marshal(raw)
	_ = s.DB.WithContext(ctx).Model(&ShopAuthToken{}).Where("shop_id = ?", row.ID).Updates(map[string]any{
		"raw_data": datatypes.JSON(rawB),
	}).Error
	if status == AuthNeedCheck {
		s.douyinLog(c, adminID, &row.ID, "douyin.shop.info.sync", "failed", DouyinShopInfoFailed, douyinFriendlyMessage(DouyinShopInfoFailed))
	} else {
		s.douyinLog(c, adminID, &row.ID, "douyin.shop.info.sync", "success", "", "shop info saved")
	}
	detail, err := s.GetDetail(c, row.ID)
	if err != nil {
		return nil, asDouyinAuthError(err, UnknownDouyinAuthError)
	}
	if status == AuthNeedCheck {
		detail.AuthStatus = AuthNeedCheck
	}
	return detail, nil
}

func (s *Service) DouyinOAuthRefresh(c *gin.Context, shopID uuid.UUID, adminID *uuid.UUID) (*ShopDetailDTO, error) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
	defer cancel()
	shopRow, _, auth, err := s.decryptedAuthCtx(ctx, shopID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(shopRow.Platform) != "douyin_shop" {
		return nil, fmt.Errorf("shop platform must be douyin_shop")
	}
	if strings.TrimSpace(auth.RefreshToken) == "" {
		_ = s.setAuthStatusCtx(ctx, shopID, AuthExpired)
		return nil, douyinErr(DouyinAuthExpired, douyinFriendlyMessage(DouyinAuthExpired), nil)
	}
	cfg, err := s.douyinGlobalConfig(ctx)
	if err != nil {
		return nil, err
	}
	tok, err := platformdouyin.Client{Config: cfg}.RefreshToken(ctx, auth.RefreshToken)
	if err != nil {
		_ = s.setAuthStatusCtx(ctx, shopID, AuthExpired)
		s.douyinLog(c, adminID, &shopID, "douyin.auth.refresh", "failed", DouyinTokenRefreshFailed, douyinFriendlyMessage(DouyinTokenRefreshFailed))
		return nil, douyinErr(DouyinTokenRefreshFailed, douyinFriendlyMessage(DouyinTokenRefreshFailed), err)
	}
	refresh := tok.RefreshToken
	if strings.TrimSpace(refresh) == "" {
		refresh = auth.RefreshToken
	}
	if err := s.persistOAuthTokenRefresh(ctx, shopID, tok.AccessToken, refresh, tok.AccessExpiresAt, tok.RefreshExpiresAt); err != nil {
		return nil, err
	}
	_ = s.setAuthStatusCtx(ctx, shopID, AuthAuthorized)
	s.douyinLog(c, adminID, &shopID, "douyin.auth.refresh", "success", "", "token refreshed")
	return s.GetDetail(c, shopID)
}

func (s *Service) DouyinOAuthRevoke(c *gin.Context, shopID uuid.UUID, adminID *uuid.UUID) (*ShopDetailDTO, error) {
	ctx := c.Request.Context()
	var row Shop
	if err := s.DB.WithContext(ctx).First(&row, "id = ?", shopID).Error; err != nil {
		return nil, err
	}
	if strings.TrimSpace(row.Platform) != "douyin_shop" {
		return nil, fmt.Errorf("shop platform must be douyin_shop")
	}
	updates := map[string]any{
		"access_token_enc":  "",
		"refresh_token_enc": "",
		"expires_at":        nil,
		"auth_config":       datatypes.JSON([]byte(`{"revokedLocally":true}`)),
	}
	_ = s.DB.WithContext(ctx).Model(&ShopAuthToken{}).Where("shop_id = ?", shopID).Updates(updates).Error
	_ = s.setAuthStatusCtx(ctx, shopID, AuthUnauthorized)
	s.douyinLog(c, adminID, &shopID, "douyin.auth.revoke", "success", "", "local authorization revoked")
	return s.GetDetail(c, shopID)
}

func (s *Service) DouyinOAuthTest(c *gin.Context, shopID uuid.UUID, adminID *uuid.UUID) (*TestShopConnectionResult, error) {
	res, err := s.testDouyinShopConnection(c.Request.Context(), shopID)
	st := "success"
	code := ""
	msg := "ok"
	if err != nil {
		st = "failed"
		code = douyinErrCode(err)
		msg = douyinSafeErr(err)
	}
	s.douyinLog(c, adminID, &shopID, "douyin.shop.connection.test", st, code, msg)
	return res, err
}

type TestShopConnectionResult = platformTestResult

type platformTestResult struct {
	OK             bool   `json:"ok"`
	Message        string `json:"message,omitempty"`
	ShopName       string `json:"shopName,omitempty"`
	ExternalShopID string `json:"externalShopId,omitempty"`
	Currency       string `json:"currency,omitempty"`
}

func (s *Service) testDouyinShopConnection(ctx context.Context, shopID uuid.UUID) (*platformTestResult, error) {
	var row Shop
	if err := s.DB.WithContext(ctx).First(&row, "id = ?", shopID).Error; err != nil {
		return nil, err
	}
	if strings.TrimSpace(row.Platform) != "douyin_shop" {
		return nil, fmt.Errorf("shop platform must be douyin_shop")
	}
	var tok ShopAuthToken
	if err := s.DB.WithContext(ctx).Where("shop_id = ?", shopID).First(&tok).Error; err != nil {
		return nil, douyinErr(DouyinAuthExpired, douyinFriendlyMessage(DouyinAuthExpired), err)
	}
	if strings.TrimSpace(tok.AccessTokenEnc) == "" {
		return nil, douyinErr(DouyinAuthExpired, douyinFriendlyMessage(DouyinAuthExpired), nil)
	}
	if tok.ExpiresAt != nil && tok.ExpiresAt.Before(time.Now().UTC()) {
		_ = s.setAuthStatusCtx(ctx, shopID, AuthExpired)
		return nil, douyinErr(DouyinAuthExpired, douyinFriendlyMessage(DouyinAuthExpired), nil)
	}
	return &platformTestResult{
		OK:             true,
		Message:        "店铺连接正常",
		ShopName:       row.ShopName,
		ExternalShopID: row.ExternalShopID,
		Currency:       row.Currency,
	}, nil
}

func (s *Service) DouyinOAuthCallbackRedirect(c *gin.Context) {
	_, authErr := s.DouyinOAuthCallback(c, DouyinOAuthCallbackQuery{
		Code:             c.Query("code"),
		State:            c.Query("state"),
		Error:            c.Query("error"),
		ErrorDescription: firstNonEmptyDouyin(c.Query("error_description"), c.Query("errorDescription"), c.Query("msg")),
	})
	target := "/settings/platforms?platform=douyin_shop&auth=success"
	if authErr != nil {
		target = "/settings/platforms?platform=douyin_shop&auth=failed&reason=" + url.QueryEscape(authErr.Code)
	}
	c.Redirect(http.StatusFound, target)
}

func firstNonEmptyDouyin(vals ...string) string {
	for _, v := range vals {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

func asDouyinAuthError(err error, fallback string) *DouyinAuthError {
	var de *DouyinAuthError
	if errors.As(err, &de) {
		return de
	}
	code := fallback
	if code == "" {
		code = UnknownDouyinAuthError
	}
	return douyinErr(code, douyinFriendlyMessage(code), err)
}

func douyinErrCode(err error) string {
	var de *DouyinAuthError
	if errors.As(err, &de) && de.Code != "" {
		return de.Code
	}
	return UnknownDouyinAuthError
}

func douyinSafeErr(err error) string {
	var de *DouyinAuthError
	if errors.As(err, &de) && de.Message != "" {
		return de.Message
	}
	if err == nil {
		return ""
	}
	msg := err.Error()
	for _, marker := range []string{"access_token", "refresh_token", "app_secret", "App Secret"} {
		if strings.Contains(strings.ToLower(msg), strings.ToLower(marker)) {
			return douyinFriendlyMessage(UnknownDouyinAuthError)
		}
	}
	if len(msg) > 500 {
		msg = msg[:500] + "..."
	}
	return msg
}

func (s *Service) douyinLog(c *gin.Context, adminID *uuid.UUID, shopID *uuid.UUID, action, status, code, msg string) {
	if s == nil || s.OpLog == nil {
		return
	}
	if strings.TrimSpace(action) == "" {
		action = "douyin.auth.failed"
	}
	if strings.TrimSpace(status) == "" {
		status = "failed"
	}
	if strings.TrimSpace(msg) == "" {
		msg = code
	}
	resourceID := ""
	if shopID != nil {
		resourceID = shopID.String()
	}
	if code != "" {
		msg = "code=" + code + " " + msg
	}
	if shopID != nil {
		msg = "shopId=" + shopID.String() + " " + msg
	}
	_ = s.OpLog.Write(c, operationlog.WriteOpts{
		AdminUserID: adminID,
		Action:      action,
		Resource:    "shop",
		ResourceID:  resourceID,
		Status:      status,
		Message:     msg,
	})
}
