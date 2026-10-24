package shop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
	"gorm.io/gorm/clause"
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

var douyinOAuthShopLocks sync.Map

func douyinLockForShop(shopID uuid.UUID) *sync.Mutex {
	key := shopID.String()
	actual, _ := douyinOAuthShopLocks.LoadOrStore(key, &sync.Mutex{})
	mu, _ := actual.(*sync.Mutex)
	if mu == nil {
		return &sync.Mutex{}
	}
	return mu
}

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
	case DouyinCategorySyncFailed:
		return "抖店类目同步失败，请检查店铺授权或稍后重试。"
	case DouyinCategoryEmpty:
		return "暂无抖店类目数据，请先点击「刷新类目」。"
	case DouyinCategoryNotSelected:
		return "请先选择抖店商品类目。"
	case DouyinCategoryNotLeaf:
		return "请选择抖店叶子类目。"
	case DouyinCategoryAttrSyncFailed:
		return "抖店类目属性同步失败，请检查店铺授权或稍后重试。"
	case DouyinRequiredAttrMissing:
		return "请补全抖店要求的商品属性。"
	case DouyinCategoryCacheStale:
		return "抖店类目缓存较旧，建议刷新。"
	case DouyinCategoryPermissionDenied:
		return "当前抖店应用没有类目接口权限，请到抖店开放平台检查权限。"
	default:
		return "抖店授权异常，请稍后重试或重新发起授权。"
	}
}

type douyinOAuthStatePayload struct {
	Platform string `json:"platform"`
	TenantID int64  `json:"tenantId"`
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
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	if adminID == nil || *adminID == uuid.Nil {
		return nil, fmt.Errorf("shop: authenticated OAuth actor required")
	}
	principal, err := adminperm.LoadPrincipal(c, s.DB)
	if err != nil {
		return nil, err
	}
	if shopID == nil || *shopID == uuid.Nil {
		if !principal.IsAdmin() {
			return nil, fmt.Errorf("shop: only tenant administrators may start OAuth without an existing shop")
		}
	} else if !principal.IsAdmin() && (!principal.Can(adminperm.PermStoreOperate) || !principal.CanOperateStore(*shopID)) {
		return nil, gorm.ErrRecordNotFound
	}
	if shopID != nil && *shopID != uuid.Nil {
		var row Shop
		if err := s.DB.WithContext(ctx).Where("id = ? AND tenant_id = ?", *shopID, tenantID).First(&row).Error; err != nil {
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
	p := douyinOAuthStatePayload{Platform: "douyin_shop", TenantID: tenantID, Created: time.Now().UTC().Unix()}
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
	// Store a durable one-time state hash in the database alongside the Redis fast path.
	if err := s.persistDouyinOAuthState(ctx, st, tenantID, adminID, shopID, cfg.RedirectURI); err != nil {
		return nil, err
	}
	s.douyinLog(c, adminID, shopID, "douyin.auth.start", "success", "", "state created")
	return &DouyinAuthorizeURLResult{RedirectURL: u, AuthorizeURL: u, State: st}, nil
}

func (s *Service) DouyinOAuthCallback(c *gin.Context, q DouyinOAuthCallbackQuery) (*ShopDetailDTO, *DouyinAuthError) {
	st := strings.TrimSpace(q.State)
	if st == "" || s == nil {
		return nil, douyinErr(platformdouyin.CodeDouyinOAuthStateMissing, "抖店授权 state 缺失，请重新发起授权。", nil)
	}
	ctxCB := c.Request.Context()
	stateRow, err := s.consumeDouyinOAuthState(ctxCB, st)
	if err != nil {
		return nil, asDouyinAuthError(err, DouyinOAuthStateInvalid)
	}
	if s.Redis != nil && s.Redis.Client != nil {
		if raw, redisErr := s.Redis.Get(ctxCB, douyinOAuthRedisPrefix+st).Result(); redisErr == nil {
			if err := validateDouyinOAuthStateCache(raw, stateRow); err != nil {
				return nil, douyinErr(DouyinOAuthStateInvalid, douyinFriendlyMessage(DouyinOAuthStateInvalid), err)
			}
			_ = s.Redis.Del(ctxCB, douyinOAuthRedisPrefix+st).Err()
		}
	}
	adminID := stateRow.UserID
	shopID := parseUUIDPtr(stateRow.ShopBindingContext)
	if adminID == nil {
		return nil, douyinErr(DouyinOAuthStateInvalid, douyinFriendlyMessage(DouyinOAuthStateInvalid), fmt.Errorf("oauth state binding mismatch"))
	}
	if stateRow.ShopBindingContext != "" && shopID == nil {
		return nil, douyinErr(DouyinOAuthStateInvalid, douyinFriendlyMessage(DouyinOAuthStateInvalid), fmt.Errorf("oauth state binding mismatch"))
	}
	c.Set(ctxkey.TenantID, stateRow.TenantID)
	c.Set(ctxkey.AdminID, adminID.String())
	setOAuthExpectedTokenVersion(c, stateRow.TokenVersion)
	tenantContext := &security.TenantContext{
		TenantID:   stateRow.TenantID,
		UserID:     *adminID,
		AuthSource: security.AuthSourceAccessToken,
	}
	c.Request = c.Request.WithContext(security.WithTenantContext(c.Request.Context(), tenantContext))
	ctxCB = c.Request.Context()
	principal, err := adminperm.LoadPrincipal(c, s.DB)
	if err != nil {
		return nil, douyinErr(DouyinOAuthStateInvalid, douyinFriendlyMessage(DouyinOAuthStateInvalid), err)
	}
	if shopID == nil || *shopID == uuid.Nil {
		if !principal.IsAdmin() {
			return nil, douyinErr(DouyinOAuthStateInvalid, douyinFriendlyMessage(DouyinOAuthStateInvalid), fmt.Errorf("OAuth actor no longer has shop-create permission"))
		}
	} else {
		if !principal.IsAdmin() && (!principal.Can(adminperm.PermStoreOperate) || !principal.CanOperateStore(*shopID)) {
			return nil, douyinErr(DouyinOAuthStateInvalid, douyinFriendlyMessage(DouyinOAuthStateInvalid), fmt.Errorf("OAuth actor no longer has shop authorization"))
		}
		var count int64
		if err := s.DB.WithContext(ctxCB).Model(&Shop{}).Where("id = ? AND tenant_id = ?", *shopID, stateRow.TenantID).Count(&count).Error; err != nil || count != 1 {
			return nil, douyinErr(DouyinOAuthStateInvalid, douyinFriendlyMessage(DouyinOAuthStateInvalid), err)
		}
	}
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
	tok, err := (&platformdouyin.Client{Config: cfg}).ExchangeCode(ctx, code)
	if err != nil {
		authErr := douyinErr(DouyinTokenExchangeFailed, douyinFriendlyMessage(DouyinTokenExchangeFailed), err)
		s.douyinLog(c, adminID, shopID, "douyin.auth.failed", "failed", authErr.Code, authErr.Message)
		if shopID != nil {
			if statusErr := s.setAuthStatusCtx(ctx, *shopID, AuthInvalid); statusErr != nil {
				return nil, asDouyinAuthError(fmt.Errorf("token exchange failed and auth status could not be persisted: %w", statusErr), UnknownDouyinAuthError)
			}
		}
		return nil, authErr
	}
	detail, authErr := s.persistDouyinOAuthBundle(c, ctx, stateRow.TenantID, shopID, adminID, cfg, tok)
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

func (s *Service) persistDouyinOAuthBundle(c *gin.Context, ctx context.Context, tenantID int64, shopID *uuid.UUID, adminID *uuid.UUID, cfg platformdouyin.RuntimeConfig, tok *platformdouyin.TokenBundle) (*ShopDetailDTO, *DouyinAuthError) {
	if s == nil || s.DB == nil {
		return nil, douyinErr(UnknownDouyinAuthError, douyinFriendlyMessage(UnknownDouyinAuthError), fmt.Errorf("shop: no db"))
	}
	var detail *ShopDetailDTO
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := withOAuthPersistenceTx(ctx, tx)
		txGin := c
		if c != nil {
			txGin = c.Copy()
			if c.Request != nil {
				txGin.Request = c.Request.WithContext(txCtx)
			}
		}
		var authErr *DouyinAuthError
		detail, authErr = s.persistDouyinOAuthBundleTx(txGin, txCtx, tenantID, shopID, adminID, cfg, tok)
		if authErr != nil {
			return authErr
		}
		return nil
	})
	if err != nil {
		return nil, asDouyinAuthError(err, UnknownDouyinAuthError)
	}
	if detail != nil {
		if detail.AuthStatus == AuthNeedCheck {
			s.douyinLog(c, adminID, &detail.ID, "douyin.shop.info.sync", "failed", DouyinShopInfoFailed, douyinFriendlyMessage(DouyinShopInfoFailed))
		} else {
			s.douyinLog(c, adminID, &detail.ID, "douyin.shop.info.sync", "success", "", "shop info saved")
		}
	}
	return detail, nil
}

func (s *Service) persistDouyinOAuthBundleTx(c *gin.Context, ctx context.Context, tenantID int64, shopID *uuid.UUID, adminID *uuid.UUID, cfg platformdouyin.RuntimeConfig, tok *platformdouyin.TokenBundle) (*ShopDetailDTO, *DouyinAuthError) {
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
		if err := oauthPersistenceDB(ctx, s.DB).Where("id = ? AND tenant_id = ?", *shopID, tenantID).First(&row).Error; err != nil {
			return nil, asDouyinAuthError(err, UnknownDouyinAuthError)
		}
	} else if platformShopID != "" {
		err := oauthPersistenceDB(ctx, s.DB).Where("tenant_id = ? AND platform = ? AND external_shop_id = ?", tenantID, "douyin_shop", platformShopID).First(&row).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, asDouyinAuthError(err, UnknownDouyinAuthError)
		}
	}
	if row.ID == uuid.Nil {
		row = Shop{
			TenantID:       tenantID,
			Platform:       "douyin_shop",
			ShopName:       shopName,
			ShopCode:       platformShopID,
			ExternalShopID: platformShopID,
			Status:         StatusActive,
			AuthStatus:     status,
			Currency:       "CNY",
			CreatedBy:      adminID,
		}
		if err := oauthPersistenceDB(ctx, s.DB).Create(&row).Error; err != nil {
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
		if err := s.updateShopOAuthFields(ctx, row.ID, updates); err != nil {
			return nil, asDouyinAuthError(err, UnknownDouyinAuthError)
		}
		if err := oauthPersistenceDB(ctx, s.DB).Where("id = ? AND tenant_id = ?", row.ID, tenantID).First(&row).Error; err != nil {
			return nil, asDouyinAuthError(err, UnknownDouyinAuthError)
		}
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
	if err := s.setAuthStatusCtx(ctx, row.ID, finalStatus); err != nil {
		return nil, asDouyinAuthError(err, UnknownDouyinAuthError)
	}
	rawB, err := json.Marshal(raw)
	if err != nil {
		return nil, asDouyinAuthError(err, UnknownDouyinAuthError)
	}
	if err := s.updateShopAuthTokenOAuthFields(ctx, row.ID, map[string]any{"raw_data": datatypes.JSON(rawB)}); err != nil {
		return nil, asDouyinAuthError(err, UnknownDouyinAuthError)
	}
	var persistedToken ShopAuthToken
	if err := oauthPersistenceDB(ctx, s.DB).Where("shop_id = ?", row.ID).First(&persistedToken).Error; err != nil {
		return nil, asDouyinAuthError(err, UnknownDouyinAuthError)
	}
	row.AuthStatus = finalStatus
	detail := &ShopDetailDTO{Shop: row, Auth: s.buildAuthPublic(&persistedToken)}
	return detail, nil
}

func (s *Service) douyinClientForShop(c *gin.Context, ctx context.Context, shopID uuid.UUID, adminID *uuid.UUID) (*platformdouyin.Client, *Shop, *ShopAuthToken, error) {
	tenant, err := security.RequireTenantContext(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	shopRow, tok, auth, err := s.decryptedAuthTenantCtx(ctx, shopID, tenant.TenantID)
	if err != nil {
		return nil, nil, nil, err
	}
	if shopRow == nil || strings.TrimSpace(shopRow.Platform) != "douyin_shop" {
		return nil, nil, nil, fmt.Errorf("shop platform must be douyin_shop")
	}
	if tok == nil || strings.TrimSpace(auth.RefreshToken) == "" {
		// A locally revoked shop must remain unauthorized. A stale refresh
		// attempt must not turn the revocation into an expired status.
		if !strings.EqualFold(strings.TrimSpace(shopRow.AuthStatus), AuthUnauthorized) {
			var statusErr error
			if tok != nil {
				statusErr = s.setAuthStatusVersioned(ctx, shopID, AuthExpired, tok.TokenVersion)
			} else {
				statusErr = s.setAuthStatusCtx(ctx, shopID, AuthExpired)
			}
			if statusErr != nil {
				return nil, shopRow, tok, fmt.Errorf("douyin auth expired and status could not be persisted: %w", statusErr)
			}
		}
		return nil, shopRow, tok, douyinErr(DouyinAuthExpired, douyinFriendlyMessage(DouyinAuthExpired), nil)
	}
	cfg, err := s.douyinGlobalConfig(ctx)
	if err != nil {
		return nil, shopRow, tok, err
	}
	var refreshVersionMu sync.Mutex
	refreshVersion := tok.TokenVersion
	client := &platformdouyin.Client{
		ShopID:                shopID.String(),
		Config:                cfg,
		AccessToken:           auth.AccessToken,
		RefreshTokenValue:     auth.RefreshToken,
		AccessTokenExpiresAt:  auth.AccessTokenExpiresAt,
		RefreshTokenExpiresAt: auth.RefreshTokenExpiresAt,
		TokenVersion:          tok.TokenVersion,
		PersistRefreshedToken: func(ctx context.Context, bundle *platformdouyin.TokenBundle) error {
			if bundle == nil {
				return nil
			}
			refresh := bundle.RefreshToken
			if strings.TrimSpace(refresh) == "" {
				refresh = auth.RefreshToken
			}
			refreshVersionMu.Lock()
			expectedVersion := refreshVersion
			refreshVersionMu.Unlock()
			refreshCtx := withOAuthExpectedShopTokenVersion(ctx, expectedVersion)
			refreshCtx = withOAuthRefreshAuditContext(refreshCtx, c, adminID, "douyin.auth.refresh")
			if err := s.persistOAuthTokenRefreshVersioned(refreshCtx, shopID, bundle.AccessToken, refresh, bundle.AccessExpiresAt, bundle.RefreshExpiresAt); err != nil {
				return err
			}
			refreshVersionMu.Lock()
			if refreshVersion == expectedVersion {
				refreshVersion++
			}
			refreshVersionMu.Unlock()
			return nil
		},
		MarkAuthStatus: func(ctx context.Context, status string) error {
			refreshVersionMu.Lock()
			expectedVersion := refreshVersion
			refreshVersionMu.Unlock()
			return s.setAuthStatusVersioned(ctx, shopID, status, expectedVersion)
		},
		Logger: platformdouyin.SafeLoggerFunc(func(ctx context.Context, entry platformdouyin.SafeRequestLog) {
			action := "douyin.client.request"
			status := "success"
			code := ""
			if !entry.Success {
				action = "douyin.client.failed"
				status = "failed"
				code = entry.ErrorCode
			}
			msg := fmt.Sprintf("method=%s requestId=%s traceId=%s elapsedMs=%d platformCode=%s",
				entry.Method, entry.RequestID, entry.TraceID, entry.ElapsedMs, entry.PlatformCode)
			s.douyinLog(c, adminID, &shopID, action, status, code, msg)
		}),
	}
	return client, shopRow, tok, nil
}

// DouyinClientForShop returns the centralized Douyin OpenAPI client for
// business modules that need platform calls without duplicating token handling.
func (s *Service) DouyinClientForShop(c *gin.Context, ctx context.Context, shopID uuid.UUID, adminID *uuid.UUID) (*platformdouyin.Client, *Shop, error) {
	client, shopRow, _, err := s.douyinClientForShop(c, ctx, shopID, adminID)
	return client, shopRow, err
}

// DouyinClientForShopContext returns the Douyin client without requiring a gin context (for workers).
func (s *Service) DouyinClientForShopContext(ctx context.Context, shopID uuid.UUID, adminID *uuid.UUID) (*platformdouyin.Client, *Shop, error) {
	return s.DouyinClientForShop(nil, ctx, shopID, adminID)
}

func (s *Service) persistDouyinShopInfo(ctx context.Context, shopID uuid.UUID, info *platformdouyin.ShopInfo) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("shop: no db")
	}
	if info == nil {
		return fmt.Errorf("douyin shop info is empty")
	}
	status := AuthAuthorized
	if strings.TrimSpace(info.PlatformShopID) == "" || strings.TrimSpace(info.ShopName) == "" {
		status = AuthNeedCheck
	}
	updates := map[string]any{
		"auth_status": status,
		"currency":    "CNY",
	}
	if v := strings.TrimSpace(info.ShopName); v != "" {
		updates["shop_name"] = v
	}
	if v := strings.TrimSpace(info.PlatformShopID); v != "" {
		updates["external_shop_id"] = v
		updates["shop_code"] = v
	}
	if err := s.updateShopOAuthFields(ctx, shopID, updates); err != nil {
		return err
	}

	var tok ShopAuthToken
	if err := oauthPersistenceDB(ctx, s.DB).Where("shop_id = ?", shopID).First(&tok).Error; err != nil {
		return err
	}
	authConfig := map[string]any{}
	if len(tok.AuthConfig) > 0 {
		_ = json.Unmarshal(tok.AuthConfig, &authConfig)
	}
	authConfig["shopLogo"] = info.ShopLogo
	authConfig["shopStatus"] = info.ShopStatus
	authConfig["authorityId"] = info.AuthorityID
	authConfig["shopBizType"] = info.ShopBizType
	authConfig["lastShopInfoSyncAt"] = time.Now().UTC().Format(time.RFC3339)
	authConfigB, err := json.Marshal(authConfig)
	if err != nil {
		return err
	}

	raw := info.Raw
	if raw == nil {
		raw = map[string]any{}
	}
	raw["provider"] = "douyin_shop"
	raw["shop_info_status"] = "ok"
	if status == AuthNeedCheck {
		raw["shop_info_status"] = "need_check"
	}
	raw["last_shop_info_sync_at"] = time.Now().UTC().Format(time.RFC3339)
	rawB, err := json.Marshal(raw)
	if err != nil {
		return err
	}

	tokenUpdates := map[string]any{
		"auth_config": datatypes.JSON(authConfigB),
		"raw_data":    datatypes.JSON(rawB),
	}
	if info.ExpiresAt != nil {
		tokenUpdates["expires_at"] = info.ExpiresAt
	}
	if info.RefreshExpiresAt != nil {
		tokenUpdates["refresh_expires_at"] = info.RefreshExpiresAt
	}
	if info.AuthorizedScopes != nil {
		scopesB, err := json.Marshal(info.AuthorizedScopes)
		if err != nil {
			return err
		}
		tokenUpdates["scopes"] = datatypes.JSON(scopesB)
	}
	return s.updateShopAuthTokenOAuthFields(ctx, shopID, tokenUpdates)
}

// persistDouyinShopInfoVersioned applies provider metadata only when the
// refresh that produced it still owns the captured token version. Revoke and
// refresh use the same shop-then-token lock order, so a stale callback cannot
// restore authorization or overwrite revoked metadata.
func (s *Service) persistDouyinShopInfoVersioned(ctx context.Context, shopID uuid.UUID, info *platformdouyin.ShopInfo, expectedVersion int64) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("shop: no db")
	}
	if info == nil {
		return fmt.Errorf("douyin shop info is empty")
	}
	tenant, err := security.RequireTenantContext(ctx)
	if err != nil {
		return err
	}
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var shopRow Shop
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND tenant_id = ?", shopID, tenant.TenantID).First(&shopRow).Error; err != nil {
			return err
		}
		var tok ShopAuthToken
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("shop_id = ?", shopID).First(&tok).Error; err != nil {
			return err
		}
		if tok.TokenVersion != expectedVersion || strings.EqualFold(strings.TrimSpace(shopRow.AuthStatus), AuthUnauthorized) {
			return fmt.Errorf("shop: Douyin shop info refresh is stale or revoked (expectedVersion=%d actualVersion=%d authStatus=%s)", expectedVersion, tok.TokenVersion, strings.TrimSpace(shopRow.AuthStatus))
		}
		return s.persistDouyinShopInfo(withOAuthPersistenceTx(ctx, tx), shopID, info)
	})
}

func (s *Service) markDouyinShopInfoFailed(ctx context.Context, shopID uuid.UUID, code, msg, status string) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("shop: no db")
	}
	tenant, err := security.RequireTenantContext(ctx)
	if err != nil {
		return err
	}
	var shopRow Shop
	if err := s.DB.WithContext(ctx).Where("id = ? AND tenant_id = ?", shopID, tenant.TenantID).First(&shopRow).Error; err != nil {
		return err
	}
	st := strings.TrimSpace(status)
	if st != "" {
		if err := s.setAuthStatusCtx(ctx, shopID, st); err != nil {
			return err
		}
	}
	var tok ShopAuthToken
	if err := s.DB.WithContext(ctx).Where("shop_id = ?", shopRow.ID).First(&tok).Error; err != nil {
		return err
	}
	raw := map[string]any{}
	if len(tok.RawData) > 0 {
		_ = json.Unmarshal(tok.RawData, &raw)
	}
	raw["provider"] = "douyin_shop"
	raw["shop_info_status"] = "failed"
	raw["last_error_code"] = strings.TrimSpace(code)
	raw["last_error_message"] = douyinSafeText(msg)
	raw["last_error_at"] = time.Now().UTC().Format(time.RFC3339)
	rawB, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return s.updateShopAuthTokenOAuthFields(ctx, shopID, map[string]any{"raw_data": datatypes.JSON(rawB)})
}

// markDouyinShopInfoFailedVersioned records a refresh failure only while the
// captured token version remains current. This keeps a revoked shop in the
// unauthorized state when an older provider request finishes late.
func (s *Service) markDouyinShopInfoFailedVersioned(ctx context.Context, shopID uuid.UUID, code, msg, status string, expectedVersion int64) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("shop: no db")
	}
	tenant, err := security.RequireTenantContext(ctx)
	if err != nil {
		return err
	}
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var shopRow Shop
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND tenant_id = ?", shopID, tenant.TenantID).First(&shopRow).Error; err != nil {
			return err
		}
		var tok ShopAuthToken
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("shop_id = ?", shopID).First(&tok).Error; err != nil {
			return err
		}
		if tok.TokenVersion != expectedVersion || strings.EqualFold(strings.TrimSpace(shopRow.AuthStatus), AuthUnauthorized) {
			return fmt.Errorf("shop: Douyin shop info failure is stale or revoked (expectedVersion=%d actualVersion=%d authStatus=%s)", expectedVersion, tok.TokenVersion, strings.TrimSpace(shopRow.AuthStatus))
		}
		if st := strings.TrimSpace(status); st != "" {
			result := tx.Model(&Shop{}).Where("id = ? AND tenant_id = ?", shopID, tenant.TenantID).Update("auth_status", st)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return gorm.ErrRecordNotFound
			}
		}
		raw := map[string]any{}
		if len(tok.RawData) > 0 {
			_ = json.Unmarshal(tok.RawData, &raw)
		}
		raw["provider"] = "douyin_shop"
		raw["shop_info_status"] = "failed"
		raw["last_error_code"] = strings.TrimSpace(code)
		raw["last_error_message"] = douyinSafeText(msg)
		raw["last_error_at"] = time.Now().UTC().Format(time.RFC3339)
		rawB, err := json.Marshal(raw)
		if err != nil {
			return err
		}
		result := tx.Model(&ShopAuthToken{}).Where("id = ? AND shop_id = ?", tok.ID, shopID).Update("raw_data", datatypes.JSON(rawB))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func (s *Service) DouyinOAuthRefresh(c *gin.Context, shopID uuid.UUID, adminID *uuid.UUID) (*ShopDetailDTO, error) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
	defer cancel()
	mu := douyinLockForShop(shopID)
	mu.Lock()
	defer mu.Unlock()

	client, _, initialToken, err := s.douyinClientForShop(c, ctx, shopID, adminID)
	if err != nil {
		return nil, err
	}
	tok, err := client.RefreshAccessToken(ctx)
	if err != nil {
		authErr := douyinAuthErrFromProvider(err, DouyinTokenRefreshFailed)
		status := AuthExpired
		if authErr.Code == DouyinPermissionDenied {
			status = AuthInvalid
		}
		if initialToken == nil {
			return nil, fmt.Errorf("shop: OAuth token version unavailable")
		}
		if persistErr := s.markDouyinShopInfoFailedVersioned(ctx, shopID, authErr.Code, authErr.Message, status, initialToken.TokenVersion); persistErr != nil {
			return nil, persistErr
		}
		s.douyinLog(c, adminID, &shopID, "douyin.auth.refresh_failed", "failed", authErr.Code, authErr.Message)
		return nil, authErr
	}
	if initialToken == nil {
		return nil, fmt.Errorf("shop: OAuth token version unavailable")
	}
	expectedVersion := initialToken.TokenVersion + 1
	if info := platformdouyin.ShopInfoFromTokenBundle(tok); info != nil {
		if err := s.persistDouyinShopInfoVersioned(ctx, shopID, info, expectedVersion); err != nil {
			return nil, err
		}
	}
	if err := s.setAuthStatusVersioned(ctx, shopID, AuthAuthorized, expectedVersion); err != nil {
		return nil, err
	}
	s.douyinLog(c, adminID, &shopID, "douyin.auth.refresh", "success", "", "token refreshed")
	return s.GetDetail(c, shopID)
}

func (s *Service) DouyinOAuthRevoke(c *gin.Context, shopID uuid.UUID, adminID *uuid.UUID) (*ShopDetailDTO, error) {
	ctx := c.Request.Context()
	tenantID, err := requireShopOperateContext(c, s.DB, shopID)
	if err != nil {
		return nil, err
	}
	if err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row Shop
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND tenant_id = ?", shopID, tenantID).First(&row).Error; err != nil {
			return err
		}
		if strings.TrimSpace(row.Platform) != "douyin_shop" {
			return fmt.Errorf("shop platform must be douyin_shop")
		}
		var token ShopAuthToken
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("shop_id = ?", shopID).First(&token).Error; err != nil {
			return err
		}
		token.AccessTokenEnc = ""
		token.RefreshTokenEnc = ""
		token.ExpiresAt = nil
		token.RefreshExpiresAt = nil
		token.AuthConfig = datatypes.JSON([]byte(`{"revokedLocally":true}`))
		token.TokenVersion++
		if err := tx.Save(&token).Error; err != nil {
			return err
		}
		statusResult := tx.Model(&Shop{}).Where("id = ? AND tenant_id = ?", shopID, tenantID).Update("auth_status", AuthUnauthorized)
		if statusResult.Error != nil {
			return statusResult.Error
		}
		if statusResult.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return s.writeShopAuditTx(tx, c, adminID, "douyin.auth.revoke", shopID.String(), "local authorization revoked")
	}); err != nil {
		return nil, err
	}
	platformdouyin.ClearTokenRefreshState(shopID.String())
	return s.GetDetail(c, shopID)
}

func (s *Service) DouyinOAuthTest(c *gin.Context, shopID uuid.UUID, adminID *uuid.UUID) (*TestShopConnectionResult, error) {
	res, err := s.testDouyinShopConnection(c, shopID, adminID)
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
	ExpiresAt      string `json:"expiresAt,omitempty"`
	ShopStatus     string `json:"shopStatus,omitempty"`
	LastTestAt     string `json:"lastTestAt,omitempty"`
	ScopesSummary  string `json:"scopesSummary,omitempty"`
}

func (s *Service) testDouyinShopConnection(c *gin.Context, shopID uuid.UUID, adminID *uuid.UUID) (*platformTestResult, error) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
	defer cancel()
	mu := douyinLockForShop(shopID)
	mu.Lock()
	defer mu.Unlock()

	client, row, _, err := s.douyinClientForShop(c, ctx, shopID, adminID)
	if err != nil {
		return nil, err
	}
	info, err := client.GetShopInfo(ctx, row.ExternalShopID)
	if err != nil {
		authErr := douyinAuthErrFromProvider(err, DouyinShopInfoFailed)
		status := AuthNeedCheck
		if authErr.Code == DouyinAuthExpired {
			status = AuthExpired
		}
		if authErr.Code == DouyinPermissionDenied {
			status = AuthInvalid
		}
		if persistErr := s.markDouyinShopInfoFailed(ctx, shopID, authErr.Code, authErr.Message, status); persistErr != nil {
			return nil, persistErr
		}
		s.douyinLog(c, adminID, &shopID, "douyin.shop.info.failed", "failed", authErr.Code, authErr.Message)
		return nil, authErr
	}
	if err := s.persistDouyinShopInfo(ctx, shopID, info); err != nil {
		return nil, err
	}
	s.douyinLog(c, adminID, &shopID, "douyin.shop.info.sync", "success", "", "shop info synced")
	detail, _ := s.GetDetail(c, shopID)
	shopName := row.ShopName
	externalID := row.ExternalShopID
	currency := row.Currency
	if detail != nil {
		shopName = detail.ShopName
		externalID = detail.ExternalShopID
		currency = detail.Currency
	}
	if shopName == "" {
		shopName = info.ShopName
	}
	if externalID == "" {
		externalID = info.PlatformShopID
	}
	expiresAt := ""
	if info.ExpiresAt != nil {
		expiresAt = info.ExpiresAt.Format(time.RFC3339)
	}
	return &platformTestResult{
		OK:             true,
		Message:        "店铺连接正常",
		ShopName:       shopName,
		ExternalShopID: externalID,
		Currency:       firstNonEmptyDouyin(currency, "CNY"),
		ExpiresAt:      expiresAt,
		ShopStatus:     info.ShopStatus,
		LastTestAt:     time.Now().UTC().Format(time.RFC3339),
		ScopesSummary:  summarizeDouyinScopes(info.AuthorizedScopes),
	}, nil
}

func (s *Service) DouyinSyncShopInfo(c *gin.Context, shopID uuid.UUID, adminID *uuid.UUID) (*ShopDetailDTO, error) {
	if _, err := s.testDouyinShopConnection(c, shopID, adminID); err != nil {
		return nil, err
	}
	return s.GetDetail(c, shopID)
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

func douyinAuthErrFromProvider(err error, fallback string) *DouyinAuthError {
	var de *DouyinAuthError
	if errors.As(err, &de) {
		return de
	}
	var pe *platformdouyin.Error
	if errors.As(err, &pe) {
		switch pe.Code {
		case platformdouyin.CodeDouyinAuthExpired:
			return douyinErr(DouyinAuthExpired, douyinFriendlyMessage(DouyinAuthExpired), err)
		case platformdouyin.CodeDouyinPermissionDenied:
			return douyinErr(DouyinPermissionDenied, douyinFriendlyMessage(DouyinPermissionDenied), err)
		case platformdouyin.CodeDouyinTokenRefreshFailed:
			return douyinErr(DouyinTokenRefreshFailed, douyinFriendlyMessage(DouyinTokenRefreshFailed), err)
		case platformdouyin.CodeDouyinShopInfoFailed:
			return douyinErr(DouyinShopInfoFailed, douyinFriendlyMessage(DouyinShopInfoFailed), err)
		}
	}
	code := strings.TrimSpace(fallback)
	if code == "" {
		code = UnknownDouyinAuthError
	}
	return douyinErr(code, douyinFriendlyMessage(code), err)
}

func douyinSafeText(raw string) string {
	msg := strings.TrimSpace(raw)
	if msg == "" {
		return ""
	}
	for _, marker := range []string{"access_token", "refresh_token", "app_secret", "secret", "token"} {
		if strings.Contains(strings.ToLower(msg), strings.ToLower(marker)) {
			return douyinFriendlyMessage(UnknownDouyinAuthError)
		}
	}
	if len(msg) > 500 {
		msg = msg[:500] + "..."
	}
	return msg
}

func summarizeDouyinScopes(scopes []any) string {
	if len(scopes) == 0 {
		return ""
	}
	parts := make([]string, 0, len(scopes))
	for _, s := range scopes {
		v := strings.TrimSpace(fmt.Sprint(s))
		if v != "" {
			parts = append(parts, v)
		}
		if len(parts) >= 5 {
			break
		}
	}
	if len(scopes) > len(parts) {
		parts = append(parts, fmt.Sprintf("+%d", len(scopes)-len(parts)))
	}
	return strings.Join(parts, ", ")
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
