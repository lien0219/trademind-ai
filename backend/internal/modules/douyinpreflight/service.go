package douyinpreflight

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/encrypt"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/modules/storagepublic"
	"github.com/trademind-ai/trademind/backend/internal/pkg/httppublic"
	storagepub "github.com/trademind-ai/trademind/backend/internal/pkg/storagepublic"
	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
	"gorm.io/gorm"
)

const (
	douyinPlatform       = "douyin_shop"
	expectedCallbackPath = "/shops/oauth/douyin/callback"
)

// Service runs Douyin production preflight checks.
type Service struct {
	DB       *gorm.DB
	Settings *settings.Service
	Shops    *shop.Service
	Storage  *storagepublic.Service
}

// Run executes all preflight checks and persists the latest result.
func (s *Service) Run(c *gin.Context, req RunRequest) (*Result, error) {
	if s == nil || s.Settings == nil || s.DB == nil {
		return nil, fmt.Errorf("douyin preflight unavailable")
	}
	ctx := c.Request.Context()
	checks := make([]CheckItem, 0, 32)

	checks = append(checks, s.checkAppConfig(ctx)...)
	checks = append(checks, s.checkShopAuth(ctx)...)
	checks = append(checks, s.checkFeatureSwitches(ctx)...)
	checks = append(checks, s.checkStorage(ctx)...)
	checks = append(checks, s.checkDataState(ctx)...)

	blocked := s.needsRealCredentials(ctx)
	if req.LiveTest && !blocked {
		checks = append(checks, s.checkLiveAuth(c)...)
	} else if req.LiveTest && blocked {
		checks = append(checks, checkWarning(
			"live.auth_test",
			"真实接口联调",
			"当前环境缺少真实凭证，已跳过 Token 刷新与店铺信息联调",
			"配置真实 App Key / Secret 并完成店铺 OAuth 后重新运行预检（开启 liveTest）",
			map[string]any{"skipped": true},
		))
	}

	status, passed, warning, failed := aggregateStatus(checks)
	out := &Result{
		Status:        status,
		Checks:        checks,
		PassedCount:   passed,
		WarningCount:  warning,
		FailedCount:   failed,
		CheckedAt:     nowRFC3339(),
		LiveTest:      req.LiveTest,
		BlockedByReal: blocked && !req.LiveTest,
	}
	if blocked {
		out.BlockedByReal = true
	}

	if err := s.saveLatest(ctx, out); err != nil {
		return out, fmt.Errorf("save preflight result: %w", err)
	}
	return out, nil
}

// GetLatest returns the most recent persisted preflight result.
func (s *Service) GetLatest(ctx context.Context) (*Result, error) {
	if s == nil || s.Settings == nil {
		return nil, fmt.Errorf("douyin preflight unavailable")
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, settingsGroup)
	if err != nil {
		return nil, err
	}
	raw := strings.TrimSpace(m[settingsKey])
	if raw == "" {
		return nil, nil
	}
	var out Result
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("parse preflight result: %w", err)
	}
	return &out, nil
}

func (s *Service) saveLatest(ctx context.Context, res *Result) error {
	b, err := json.Marshal(res)
	if err != nil {
		return err
	}
	return s.Settings.PutBulk(ctx, []settings.PutItem{{
		TenantID:  0,
		GroupKey:  settingsGroup,
		ItemKey:   settingsKey,
		ItemValue: string(b),
		ValueType: "json",
	}})
}

func (s *Service) needsRealCredentials(ctx context.Context) bool {
	plain, err := s.Settings.PlainByGroup(ctx, 0, "platform_douyin_shop")
	if err != nil {
		return true
	}
	if strings.TrimSpace(plain["app_key"]) == "" || strings.TrimSpace(plain["app_secret"]) == "" {
		return true
	}
	var n int64
	_ = s.DB.WithContext(ctx).Model(&shop.Shop{}).
		Where("platform = ? AND auth_status = ?", douyinPlatform, shop.AuthAuthorized).
		Count(&n).Error
	return n == 0
}

func (s *Service) douyinConfig(ctx context.Context) (platformdouyin.RuntimeConfig, map[string]string, error) {
	plain, err := s.Settings.PlainByGroup(ctx, 0, "platform_douyin_shop")
	if err != nil {
		return platformdouyin.RuntimeConfig{}, nil, err
	}
	mm := map[string]string{}
	for k, v := range plain {
		mm[strings.TrimSpace(strings.ToLower(k))] = strings.TrimSpace(v)
	}
	cfg, err := platformdouyin.RuntimeFromMergedMap(mm)
	return cfg, mm, err
}

func (s *Service) checkAppConfig(ctx context.Context) []CheckItem {
	out := make([]CheckItem, 0, 12)
	rows, err := s.Settings.List(ctx)
	if err != nil {
		out = append(out, checkFailed("app.config", "应用配置", "无法读取平台配置", "检查数据库与 settings 服务", map[string]any{"error": err.Error()}))
		return out
	}
	var appKey, appSecretEnc, serviceID, redirectURI, environment, apiBase, timeoutSec string
	for _, r := range rows {
		if r.GroupKey != "platform_douyin_shop" {
			continue
		}
		switch strings.TrimSpace(strings.ToLower(r.ItemKey)) {
		case "app_key", "client_key":
			appKey = strings.TrimSpace(r.ItemValue)
		case "app_secret", "client_secret":
			appSecretEnc = r.ItemValue
		case "service_id":
			serviceID = strings.TrimSpace(r.ItemValue)
		case "redirect_uri", "callback_url":
			redirectURI = strings.TrimSpace(r.ItemValue)
		case "environment":
			environment = strings.TrimSpace(strings.ToLower(r.ItemValue))
		case "api_base_url":
			apiBase = strings.TrimSpace(r.ItemValue)
		case "timeout_sec":
			timeoutSec = strings.TrimSpace(r.ItemValue)
		}
	}

	if appKey == "" {
		out = append(out, checkFailed("app.app_key", "App Key", "未配置 App Key", "在平台开放配置中填写抖店 App Key", nil))
	} else {
		out = append(out, checkPassed("app.app_key", "App Key", "App Key 已配置", map[string]any{"masked": encrypt.MaskSecret(appKey)}))
	}

	secretOK := false
	if strings.TrimSpace(appSecretEnc) == "" {
		out = append(out, checkFailed("app.app_secret", "App Secret", "未配置 App Secret", "在平台开放配置中填写并保存 App Secret", nil))
	} else if encrypt.LooksMasked(appSecretEnc) {
		// masked in list output — verify decrypt via PlainByGroup
		plain, err := s.Settings.PlainByGroup(ctx, 0, "platform_douyin_shop")
		if err != nil || strings.TrimSpace(plain["app_secret"]) == "" {
			out = append(out, checkFailed("app.app_secret", "App Secret", "App Secret 无法解密或为空", "确认 APP_MASTER_KEY 正确并重新保存 Secret", nil))
		} else {
			secretOK = true
			out = append(out, checkPassed("app.app_secret", "App Secret", "App Secret 已配置且可解密", map[string]any{"masked": encrypt.MaskSecret(plain["app_secret"])}))
		}
	} else {
		secretOK = true
		out = append(out, checkPassed("app.app_secret", "App Secret", "App Secret 已配置", nil))
	}
	_ = secretOK

	if serviceID == "" {
		out = append(out, checkWarning("app.service_id", "Service ID", "未配置 Service ID", "抖店 OAuth 授权通常需要 Service ID", nil))
	} else {
		out = append(out, checkPassed("app.service_id", "Service ID", "Service ID 已配置", nil))
	}

	if apiBase == "" {
		apiBase = "https://openapi-fxg.jinritemai.com"
	}
	if u, err := url.Parse(apiBase); err != nil || u.Scheme == "" || u.Host == "" {
		out = append(out, checkFailed("app.api_endpoint", "API 地址", "API Endpoint 无效", "填写合法的 https:// 开放平台地址", map[string]any{"apiBaseUrl": apiBase}))
	} else {
		out = append(out, checkPassed("app.api_endpoint", "API 地址", "API Endpoint 格式合法", map[string]any{"apiBaseUrl": apiBase}))
	}

	if redirectURI == "" {
		out = append(out, checkFailed("app.redirect_uri", "OAuth 回调地址", "未配置 Redirect URI", "填写与抖店开放平台登记一致的回调 URL", nil))
	} else {
		ru, err := url.Parse(redirectURI)
		if err != nil || ru.Scheme == "" || ru.Host == "" {
			out = append(out, checkFailed("app.redirect_uri", "OAuth 回调地址", "Redirect URI 格式无效", "使用完整 https:// 域名 + 路径", nil))
		} else {
			if environment == "" {
				environment = "production"
			}
			if environment == "production" && strings.ToLower(ru.Scheme) != "https" {
				out = append(out, checkFailed("app.redirect_https", "OAuth HTTPS", "生产环境回调地址须为 HTTPS", "将 Redirect URI 改为 https://", map[string]any{"scheme": ru.Scheme}))
			} else {
				out = append(out, checkPassed("app.redirect_https", "OAuth HTTPS", "回调地址使用 HTTPS", nil))
			}
			if !strings.Contains(ru.Path, expectedCallbackPath) {
				out = append(out, checkWarning("app.redirect_path", "OAuth 回调路径", "回调路径可能与系统默认不一致", "确认路径包含 "+expectedCallbackPath, map[string]any{"path": ru.Path}))
			} else {
				out = append(out, checkPassed("app.redirect_path", "OAuth 回调路径", "回调路径与系统路由一致", map[string]any{"path": ru.Path}))
			}
		}
	}

	if environment == "" {
		environment = "production"
	}
	if environment != "production" && environment != "sandbox" {
		out = append(out, checkFailed("app.environment", "运行环境", "环境配置无效", "environment 须为 production 或 sandbox", map[string]any{"environment": environment}))
	} else {
		msg := "当前为生产环境配置"
		if environment == "sandbox" {
			msg = "当前为沙箱环境配置"
		}
		out = append(out, checkPassed("app.environment", "运行环境", msg, map[string]any{"environment": environment}))
	}

	cfg, _, cfgErr := s.douyinConfig(ctx)
	if cfgErr != nil {
		out = append(out, checkFailed("app.config_valid", "配置完整性", "平台配置校验未通过", cfgErr.Error(), nil))
	} else {
		out = append(out, checkPassed("app.config_valid", "配置完整性", "平台配置字段校验通过", nil))
		if !cfg.RealAPIEnabled {
			out = append(out, checkWarning("app.real_api_enabled", "真实接口开关", "真实接口开关未开启", "生产联调前请在平台配置中开启 real_api_enabled", nil))
		} else {
			out = append(out, checkPassed("app.real_api_enabled", "真实接口开关", "真实接口开关已开启", nil))
		}
	}

	if timeoutSec == "" {
		out = append(out, checkFailed("app.timeout", "请求超时", "未配置 timeout_sec", "设置 5–600 秒之间的超时", nil))
	} else {
		out = append(out, checkPassed("app.timeout", "请求超时", "超时配置已设置", map[string]any{"timeoutSec": timeoutSec}))
	}

	return out
}

func (s *Service) checkShopAuth(ctx context.Context) []CheckItem {
	out := make([]CheckItem, 0, 8)
	var shops []shop.Shop
	if err := s.DB.WithContext(ctx).Where("platform = ?", douyinPlatform).Find(&shops).Error; err != nil {
		out = append(out, checkFailed("shop.list", "店铺授权", "无法读取抖店店铺列表", err.Error(), nil))
		return out
	}
	if len(shops) == 0 {
		out = append(out, checkFailed("shop.authorized", "已授权店铺", "尚无抖店店铺记录", "完成 OAuth 授权后再运行预检", nil))
		return out
	}

	var authorized, needCheck, expired, invalid int
	var soonExpire int
	for _, sh := range shops {
		switch sh.AuthStatus {
		case shop.AuthAuthorized:
			authorized++
		case shop.AuthNeedCheck:
			needCheck++
		case shop.AuthExpired:
			expired++
		case shop.AuthInvalid:
			invalid++
		}
	}
	if authorized == 0 {
		out = append(out, checkFailed("shop.authorized", "已授权店铺", "没有处于已授权状态的抖店店铺", "在店铺管理中完成授权或刷新 Token", map[string]any{
			"needCheck": needCheck, "expired": expired, "invalid": invalid,
		}))
	} else {
		out = append(out, checkPassed("shop.authorized", "已授权店铺", fmt.Sprintf("已有 %d 家已授权抖店店铺", authorized), map[string]any{"count": authorized}))
	}
	if needCheck+expired+invalid > 0 {
		out = append(out, checkWarning("shop.auth_status", "授权异常店铺", fmt.Sprintf("存在需处理的授权状态（待检查 %d / 过期 %d / 无效 %d）", needCheck, expired, invalid), "在店铺管理中重新授权或测试连接", map[string]any{
			"needCheck": needCheck, "expired": expired, "invalid": invalid,
		}))
	}

	// Token presence for first authorized shop
	var probeShop *shop.Shop
	for i := range shops {
		if shops[i].AuthStatus == shop.AuthAuthorized {
			probeShop = &shops[i]
			break
		}
	}
	if probeShop == nil {
		return out
	}
	var tok shop.ShopAuthToken
	if err := s.DB.WithContext(ctx).Where("shop_id = ?", probeShop.ID).First(&tok).Error; err != nil {
		out = append(out, checkFailed("shop.token", "店铺 Token", "已授权店铺缺少 Token 记录", "重新发起 OAuth 授权", map[string]any{"shopId": probeShop.ID.String()}))
		return out
	}
	hasAccess := strings.TrimSpace(tok.AccessTokenEnc) != ""
	hasRefresh := strings.TrimSpace(tok.RefreshTokenEnc) != ""
	if !hasAccess {
		out = append(out, checkFailed("shop.access_token", "Access Token", "Access Token 不存在", "刷新或重新授权", nil))
	} else {
		out = append(out, checkPassed("shop.access_token", "Access Token", "Access Token 已保存（加密）", nil))
	}
	if !hasRefresh {
		out = append(out, checkWarning("shop.refresh_token", "Refresh Token", "Refresh Token 不存在", "Token 过期后可能无法自动刷新", nil))
	} else {
		out = append(out, checkPassed("shop.refresh_token", "Refresh Token", "Refresh Token 已保存（加密）", nil))
	}
	if tok.ExpiresAt != nil {
		if tok.ExpiresAt.Before(time.Now()) {
			out = append(out, checkWarning("shop.token_expiry", "Token 有效期", "Access Token 已过期", "在店铺管理中刷新授权", map[string]any{"expiresAt": tok.ExpiresAt.Format(time.RFC3339)}))
		} else if tok.ExpiresAt.Before(time.Now().Add(24 * time.Hour)) {
			soonExpire++
			out = append(out, checkWarning("shop.token_expiry", "Token 有效期", "Access Token 将在 24 小时内过期", "建议提前刷新授权", map[string]any{"expiresAt": tok.ExpiresAt.Format(time.RFC3339)}))
		} else {
			out = append(out, checkPassed("shop.token_expiry", "Token 有效期", "Access Token 仍在有效期内", map[string]any{"expiresAt": tok.ExpiresAt.Format(time.RFC3339)}))
		}
	} else {
		out = append(out, checkWarning("shop.token_expiry", "Token 有效期", "未记录 Token 过期时间", "运行店铺连接测试以校准", nil))
	}
	_ = soonExpire
	return out
}

func (s *Service) checkFeatureSwitches(ctx context.Context) []CheckItem {
	out := make([]CheckItem, 0, 6)
	cfg, _, err := s.douyinConfig(ctx)
	if err != nil {
		out = append(out, checkWarning("feature.config", "功能开关", "无法读取功能开关（配置不完整）", "先完成应用配置", map[string]any{"error": err.Error()}))
		return out
	}
	if cfg.ProductDraftEnabled {
		out = append(out, checkPassed("feature.product_draft", "商品草稿创建", "商品草稿创建开关已开启", nil))
	} else {
		out = append(out, checkWarning("feature.product_draft", "商品草稿创建", "商品草稿创建开关未开启", "如需创建抖店草稿请在平台配置中开启", nil))
	}
	if cfg.OrderSyncEnabled {
		out = append(out, checkPassed("feature.order_sync", "订单同步", "订单同步开关已开启", nil))
	} else {
		out = append(out, checkWarning("feature.order_sync", "订单同步", "订单同步开关未开启", "生产灰度建议手动开启并小范围验证", nil))
	}
	if cfg.InventoryEnabled {
		out = append(out, checkPassed("feature.inventory_sync", "库存同步", "库存同步开关已开启", nil))
	} else {
		out = append(out, checkWarning("feature.inventory_sync", "库存同步", "库存同步开关未开启", "生产灰度建议手动开启并仅选测试 SKU", nil))
	}
	maxPages := cfg.OrderSyncMaxPages
	if maxPages < 1 || maxPages > 50 {
		out = append(out, checkFailed("feature.order_sync_max_pages", "订单同步页数", "order_sync_max_pages 不在合法范围", "设置为 1–50 之间的整数", map[string]any{"maxPages": maxPages}))
	} else {
		out = append(out, checkPassed("feature.order_sync_max_pages", "订单同步页数", fmt.Sprintf("每任务最多同步 %d 页", maxPages), map[string]any{"maxPages": maxPages}))
	}
	if cfg.HTTPTimeout < 5*time.Second || cfg.HTTPTimeout > 600*time.Second {
		out = append(out, checkFailed("feature.timeout", "请求超时", "timeout_sec 配置不合法", "设置为 5–600 秒", map[string]any{"timeoutSec": cfg.HTTPTimeout.Seconds()}))
	} else {
		out = append(out, checkPassed("feature.timeout", "请求超时", "HTTP 超时配置合法", map[string]any{"timeoutSec": int(cfg.HTTPTimeout.Seconds())}))
	}
	return out
}

func (s *Service) checkStorage(ctx context.Context) []CheckItem {
	out := make([]CheckItem, 0, 4)
	if s.Storage == nil {
		out = append(out, checkFailed("storage.public_access", "图片公网访问", "Storage 公网检测服务不可用", "联系管理员检查服务配置", nil))
		return out
	}
	plain, err := s.Settings.PlainByGroup(ctx, 0, "storage")
	if err != nil {
		out = append(out, checkFailed("storage.config", "存储配置", "无法读取存储配置", err.Error(), nil))
		return out
	}
	pubBase := storagepub.ResolvePublicBase(plain)
	if pubBase == "" {
		out = append(out, checkFailed("storage.public_base", "公开访问域名", "未配置 public_base", "在存储设置中配置 HTTPS 公网域名", nil))
		return out
	}
	if !strings.Contains(pubBase, "://") {
		out = append(out, checkFailed("storage.public_base", "公开访问域名", "当前为相对路径，外部平台无法访问", "生产环境须配置完整 HTTPS URL（抖店无法访问 /static）", map[string]any{"publicBase": pubBase}))
		return out
	}
	if !httppublic.IsPublicHTTPURL(pubBase) {
		out = append(out, checkFailed("storage.public_base", "公开访问域名", "公开地址指向本机或私网", "配置公网可访问的 HTTPS 域名", map[string]any{"publicBase": pubBase}))
		return out
	}
	if !strings.HasPrefix(strings.ToLower(pubBase), "https://") {
		out = append(out, checkFailed("storage.public_https", "HTTPS", "公开地址未使用 HTTPS", "抖店等平台要求 HTTPS 图片地址", map[string]any{"publicBase": pubBase}))
	} else {
		out = append(out, checkPassed("storage.public_https", "HTTPS", "公开地址使用 HTTPS", nil))
	}

	e2e, err := storagepub.TestEndToEnd(ctx, plain)
	if err != nil {
		out = append(out, checkFailed("storage.public_access", "图片公网访问", "公网访问测试执行失败", err.Error(), nil))
		return out
	}
	if e2e.OK {
		out = append(out, checkPassed("storage.public_access", "图片公网访问", "图片存储可以被外部平台正常访问", e2e.TechnicalDetails))
	} else {
		suggestion := "请检查公开访问域名、HTTPS 证书和 Bucket 读权限"
		out = append(out, checkFailed("storage.public_access", "图片公网访问", e2e.Message, suggestion, map[string]any{
			"errorCode":        e2e.ErrorCode,
			"technicalDetails": e2e.TechnicalDetails,
		}))
	}
	return out
}

func (s *Service) checkDataState(ctx context.Context) []CheckItem {
	out := make([]CheckItem, 0, 10)

	var productCount int64
	q := s.DB.WithContext(ctx).Model(&product.Product{}).Where("status IN ?", []string{product.StatusDraft, product.StatusReady})
	if err := q.Count(&productCount).Error; err != nil {
		out = append(out, checkWarning("data.products", "测试商品", "无法统计商品草稿", err.Error(), nil))
	} else if productCount == 0 {
		out = append(out, checkWarning("data.products", "测试商品", "尚无可用商品草稿", "采集或手工创建至少 1 个商品用于 E2E", map[string]any{"count": 0}))
	} else {
		out = append(out, checkPassed("data.products", "测试商品", fmt.Sprintf("已有 %d 个可编辑商品草稿", productCount), map[string]any{"count": productCount}))
	}

	var withMainImage int64
	_ = s.DB.WithContext(ctx).Model(&product.ProductImage{}).
		Where("image_type IN ?", []string{product.ImageTypeMain}).
		Distinct("product_id").Count(&withMainImage).Error
	if withMainImage == 0 {
		out = append(out, checkWarning("data.main_images", "商品主图", "没有商品包含主图", "上传或采集主图后再创建抖店草稿", nil))
	} else {
		out = append(out, checkPassed("data.main_images", "商品主图", fmt.Sprintf("%d 个商品已有主图", withMainImage), map[string]any{"count": withMainImage}))
	}

	var skuCount int64
	_ = s.DB.WithContext(ctx).Model(&product.ProductSKU{}).Count(&skuCount).Error
	if skuCount == 0 {
		out = append(out, checkWarning("data.skus", "商品规格", "尚无 SKU 规格数据", "维护至少一个带规格的商品", nil))
	} else {
		out = append(out, checkPassed("data.skus", "商品规格", fmt.Sprintf("已有 %d 条 SKU 记录", skuCount), map[string]any{"count": skuCount}))
	}

	var pubCount int64
	_ = s.DB.WithContext(ctx).Model(&productpublish.ProductPublication{}).
		Where("platform = ?", douyinPlatform).
		Count(&pubCount).Error
	if pubCount == 0 {
		out = append(out, checkWarning("data.publications", "平台商品草稿", "尚无已创建的抖店平台草稿", "完成映射与图片上传后创建草稿", nil))
	} else {
		out = append(out, checkPassed("data.publications", "平台商品草稿", fmt.Sprintf("已有 %d 条抖店刊登记录", pubCount), map[string]any{"count": pubCount}))
	}

	var unmatched int64
	_ = s.DB.WithContext(ctx).Model(&productpublish.ProductPublicationSKU{}).
		Joins("JOIN product_publications ON product_publications.id = product_publication_skus.publication_id").
		Where("product_publications.platform = ? AND product_publication_skus.bind_status IN ?", douyinPlatform, []string{productpublish.BindStatusUnmatched, productpublish.BindStatusAmbiguous}).
		Count(&unmatched).Error
	if unmatched > 0 {
		out = append(out, checkWarning("data.sku_binding", "SKU 绑定", fmt.Sprintf("存在 %d 条未匹配或待确认的 SKU 绑定", unmatched), "在商品详情中完成 SKU 手动绑定", map[string]any{"count": unmatched}))
	} else {
		out = append(out, checkPassed("data.sku_binding", "SKU 绑定", "未发现未匹配或冲突的抖店 SKU 绑定", nil))
	}

	return out
}

func (s *Service) checkLiveAuth(c *gin.Context) []CheckItem {
	out := make([]CheckItem, 0, 4)
	if s.Shops == nil {
		out = append(out, checkFailed("live.auth", "真实联调", "店铺服务不可用", "", nil))
		return out
	}
	ctx := c.Request.Context()
	var sh shop.Shop
	if err := s.DB.WithContext(ctx).Where("platform = ? AND auth_status = ?", douyinPlatform, shop.AuthAuthorized).First(&sh).Error; err != nil {
		out = append(out, checkFailed("live.auth", "真实联调", "没有可用于联调的已授权店铺", "先完成 OAuth", nil))
		return out
	}
	var adminID *uuid.UUID
	res, err := s.Shops.DouyinOAuthTest(c, sh.ID, adminID)
	if err != nil {
		out = append(out, checkFailed("live.token_refresh", "Token 刷新联调", "店铺连接测试失败", "查看店铺管理中的错误提示并重新授权", map[string]any{
			"shopId": sh.ID.String(),
			"error":  err.Error(),
		}))
		return out
	}
	if res != nil && res.OK {
		out = append(out, checkPassed("live.token_refresh", "Token 刷新联调", "Token 刷新与店铺信息读取成功", map[string]any{
			"shopName":       res.ShopName,
			"externalShopId": res.ExternalShopID,
			"expiresAt":      res.ExpiresAt,
		}))
	} else {
		out = append(out, checkFailed("live.token_refresh", "Token 刷新联调", "店铺连接测试未通过", "重新授权或检查 App 权限", nil))
	}
	return out
}
