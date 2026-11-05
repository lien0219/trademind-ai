package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/textproto"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"gorm.io/gorm"
)

const (
	CodeDouyinWebhookShopNotResolved      = "DOUYIN_WEBHOOK_SHOP_NOT_RESOLVED"
	CodeDouyinWebhookShopAmbiguous        = "DOUYIN_WEBHOOK_SHOP_AMBIGUOUS"
	CodeDouyinWebhookShopBindingMismatch  = "DOUYIN_WEBHOOK_SHOP_BINDING_MISMATCH"
	CodeDouyinWebhookTenantMismatch       = "DOUYIN_WEBHOOK_TENANT_MISMATCH"
	CodeDouyinWebhookAppBindingMismatch   = "DOUYIN_WEBHOOK_APP_BINDING_MISMATCH"
	CodeDouyinWebhookAuthorizationExpired = "DOUYIN_WEBHOOK_AUTHORIZATION_EXPIRED"
	CodeDouyinWebhookBindingRevoked       = "DOUYIN_WEBHOOK_BINDING_REVOKED"
	CodeDouyinWebhookUntrustedShopID      = "DOUYIN_WEBHOOK_UNTRUSTED_SHOP_IDENTIFIER"
	CodeDouyinWebhookClaimConflict        = "DOUYIN_WEBHOOK_SIGNED_CLAIM_CONFLICT"
)

// WebhookShopResolver is the single entry for assigning a verified platform
// webhook to one internal tenant/shop binding.
type WebhookShopResolver interface {
	Resolve(ctx context.Context, input ResolveWebhookShopInput) (*ResolvedWebhookShop, error)
}

type ResolveWebhookShopInput struct {
	Platform               string
	AppID                  string
	PlatformShopID         string
	EventType              string
	VerifiedHeaders        map[string]string
	WebhookSecretBindingID *uuid.UUID
	RequestID              string
	AppEnv                 string
}

type ResolvedWebhookShop struct {
	TenantID            int64
	InternalShopID      uuid.UUID
	Platform            string
	PlatformShopID      string
	AppID               string
	BindingID           uuid.UUID
	AuthorizationStatus string
	ContractStatus      string
	TestFallback        bool
}

type DBWebhookShopResolver struct {
	DB     *gorm.DB
	AppEnv string
}

type webhookShopCandidate struct {
	TenantID       int64
	ShopID         uuid.UUID
	Platform       string
	PlatformShopID string
	ShopStatus     string
	AuthStatus     string
	AppID          string
	BindingID      uuid.UUID
	TokenExpiresAt *time.Time
}

func (r *DBWebhookShopResolver) Resolve(ctx context.Context, input ResolveWebhookShopInput) (*ResolvedWebhookShop, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("webhook shop resolver unavailable")
	}
	platform := normalizeDouyinPlatform(input.Platform)
	if platform == "" {
		return nil, newCodeError(CodeDouyinWebhookShopNotResolved, http.StatusBadRequest, CodeDouyinWebhookShopNotResolved)
	}
	appEnv := config.NormalizeEnv(firstNonEmptyString(input.AppEnv, r.AppEnv))
	platformShopID := strings.TrimSpace(input.PlatformShopID)
	appID := strings.TrimSpace(input.AppID)

	if platformShopID == "" {
		if fb, ok, err := r.resolveExplicitFallback(ctx, appEnv, platform); ok || err != nil {
			return fb, err
		}
		return nil, newCodeError(CodeDouyinWebhookUntrustedShopID, http.StatusBadRequest, CodeDouyinWebhookUntrustedShopID)
	}

	candidates, err := r.findCandidates(ctx, platform, appID, platformShopID, input.WebhookSecretBindingID)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		if input.WebhookSecretBindingID != nil && *input.WebhookSecretBindingID != uuid.Nil {
			withoutBinding, bindErr := r.findCandidates(ctx, platform, appID, platformShopID, nil)
			if bindErr != nil {
				return nil, bindErr
			}
			if len(withoutBinding) > 0 {
				return nil, newCodeError(CodeDouyinWebhookShopBindingMismatch, http.StatusForbidden, CodeDouyinWebhookShopBindingMismatch)
			}
		}
		if appID != "" {
			withoutApp, appErr := r.findCandidates(ctx, platform, "", platformShopID, input.WebhookSecretBindingID)
			if appErr != nil {
				return nil, appErr
			}
			if len(withoutApp) > 0 {
				return nil, newCodeError(CodeDouyinWebhookAppBindingMismatch, http.StatusForbidden, CodeDouyinWebhookAppBindingMismatch)
			}
		}
		return nil, newCodeError(CodeDouyinWebhookShopNotResolved, http.StatusNotFound, CodeDouyinWebhookShopNotResolved)
	}
	if len(candidates) > 1 {
		return nil, newCodeError(CodeDouyinWebhookShopAmbiguous, http.StatusConflict, CodeDouyinWebhookShopAmbiguous)
	}
	return candidateToResolved(candidates[0], false)
}

func (r *DBWebhookShopResolver) findCandidates(ctx context.Context, platform, appID, platformShopID string, bindingID *uuid.UUID) ([]webhookShopCandidate, error) {
	q := r.DB.WithContext(ctx).
		Table("shops").
		Select("shops.tenant_id, shops.id AS shop_id, shops.platform, shops.external_shop_id AS platform_shop_id, shops.status AS shop_status, shops.auth_status, shop_auth_tokens.app_key AS app_id, shop_auth_tokens.id AS binding_id, shop_auth_tokens.expires_at AS token_expires_at").
		Joins("JOIN shop_auth_tokens ON shop_auth_tokens.shop_id = shops.id AND shop_auth_tokens.deleted_at IS NULL").
		Where("shops.deleted_at IS NULL").
		Where("shops.platform IN ?", douyinResolverPlatforms(platform)).
		Where("shops.external_shop_id = ?", strings.TrimSpace(platformShopID))
	if appID != "" {
		q = q.Where("shop_auth_tokens.app_key = ?", appID)
	}
	if bindingID != nil && *bindingID != uuid.Nil {
		q = q.Where("shop_auth_tokens.id = ?", *bindingID)
	}
	var rows []webhookShopCandidate
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *DBWebhookShopResolver) resolveExplicitFallback(ctx context.Context, appEnv, platform string) (*ResolvedWebhookShop, bool, error) {
	testBindingID := strings.TrimSpace(os.Getenv("DOUYIN_WEBHOOK_TEST_SHOP_BINDING_ID"))
	demoFallback := strings.EqualFold(strings.TrimSpace(os.Getenv("ENABLE_DOUYIN_WEBHOOK_DEMO_FALLBACK")), "true")
	switch appEnv {
	case config.EnvDevelopment, config.EnvTest:
		if testBindingID == "" {
			return nil, false, nil
		}
	case config.EnvDemo:
		if testBindingID == "" || !demoFallback {
			return nil, false, nil
		}
	default:
		return nil, false, nil
	}

	id, err := uuid.Parse(testBindingID)
	if err != nil {
		return nil, true, newCodeError(CodeDouyinWebhookShopNotResolved, http.StatusBadRequest, CodeDouyinWebhookShopNotResolved)
	}
	var rows []webhookShopCandidate
	err = r.DB.WithContext(ctx).
		Table("shops").
		Select("shops.tenant_id, shops.id AS shop_id, shops.platform, shops.external_shop_id AS platform_shop_id, shops.status AS shop_status, shops.auth_status, shop_auth_tokens.app_key AS app_id, shop_auth_tokens.id AS binding_id, shop_auth_tokens.expires_at AS token_expires_at").
		Joins("JOIN shop_auth_tokens ON shop_auth_tokens.shop_id = shops.id AND shop_auth_tokens.deleted_at IS NULL").
		Where("shops.deleted_at IS NULL").
		Where("shops.platform IN ?", douyinResolverPlatforms(platform)).
		Where("(shop_auth_tokens.id = ? OR shops.id = ?)", id, id).
		Find(&rows).Error
	if err != nil {
		return nil, true, err
	}
	if len(rows) == 0 {
		return nil, true, newCodeError(CodeDouyinWebhookShopNotResolved, http.StatusNotFound, CodeDouyinWebhookShopNotResolved)
	}
	if len(rows) > 1 {
		return nil, true, newCodeError(CodeDouyinWebhookShopAmbiguous, http.StatusConflict, CodeDouyinWebhookShopAmbiguous)
	}
	res, err := candidateToResolved(rows[0], true)
	return res, true, err
}

func candidateToResolved(c webhookShopCandidate, testFallback bool) (*ResolvedWebhookShop, error) {
	if c.ShopID == uuid.Nil || strings.TrimSpace(c.PlatformShopID) == "" {
		return nil, newCodeError(CodeDouyinWebhookShopNotResolved, http.StatusNotFound, CodeDouyinWebhookShopNotResolved)
	}
	if strings.TrimSpace(c.ShopStatus) != shop.StatusActive {
		return nil, newCodeError(CodeDouyinWebhookBindingRevoked, http.StatusForbidden, CodeDouyinWebhookBindingRevoked)
	}
	switch strings.TrimSpace(c.AuthStatus) {
	case shop.AuthAuthorized:
	default:
		if c.AuthStatus == shop.AuthExpired {
			return nil, newCodeError(CodeDouyinWebhookAuthorizationExpired, http.StatusForbidden, CodeDouyinWebhookAuthorizationExpired)
		}
		return nil, newCodeError(CodeDouyinWebhookBindingRevoked, http.StatusForbidden, CodeDouyinWebhookBindingRevoked)
	}
	if c.TokenExpiresAt != nil && time.Now().UTC().After(c.TokenExpiresAt.UTC()) {
		return nil, newCodeError(CodeDouyinWebhookAuthorizationExpired, http.StatusForbidden, CodeDouyinWebhookAuthorizationExpired)
	}
	return &ResolvedWebhookShop{
		TenantID:            c.TenantID,
		InternalShopID:      c.ShopID,
		Platform:            strings.TrimSpace(c.Platform),
		PlatformShopID:      strings.TrimSpace(c.PlatformShopID),
		AppID:               strings.TrimSpace(c.AppID),
		BindingID:           c.BindingID,
		AuthorizationStatus: strings.TrimSpace(c.AuthStatus),
		ContractStatus:      "fixture_verified",
		TestFallback:        testFallback,
	}, nil
}

func ExtractResolveWebhookShopInput(platform, eventType string, headers http.Header, raw []byte) (ResolveWebhookShopInput, error) {
	input := ResolveWebhookShopInput{
		Platform:        platform,
		EventType:       eventType,
		VerifiedHeaders: safeHeaders(headers),
	}
	headerAppID := firstHeader(headers, "X-Douyin-Client-Key", "X-Douyin-App-Id", "X-Douyin-App-Key", "X-Client-Key")
	headerShopID := firstHeader(headers, "X-Douyin-Shop-Id", "X-Platform-Shop-Id", "X-Shop-Id")
	if id := firstHeader(headers, "X-Webhook-Secret-Binding-Id", "X-TradeMind-Secret-Binding-Id"); id != "" {
		if parsed, err := uuid.Parse(id); err == nil {
			input.WebhookSecretBindingID = &parsed
		}
	}
	var body any
	if err := json.Unmarshal(raw, &body); err != nil {
		return input, err
	}
	bodyAppID := firstJSONPathString(body, "client_key", "clientKey", "app_id", "appId", "app_key", "appKey",
		"content.client_key", "content.clientKey", "content.app_id", "content.appId", "content.app_key", "content.appKey",
		"data.client_key", "data.clientKey", "data.app_id", "data.appId", "data.app_key", "data.appKey")
	bodyShopID := firstJSONPathString(body,
		"shop_id", "shopId", "store_id", "storeId", "platform_shop_id", "platformShopId",
		"content.shop_id", "content.shopId", "content.store_id", "content.storeId", "content.platform_shop_id", "content.platformShopId",
		"data.shop_id", "data.shopId", "data.store_id", "data.storeId", "data.platform_shop_id", "data.platformShopId",
		"0.data.shop_id", "0.data.shopId", "0.data.store_id", "0.data.storeId", "0.data.platform_shop_id", "0.data.platformShopId",
	)
	if bodyAppID != "" && headerAppID != "" && bodyAppID != headerAppID {
		return input, newCodeError(CodeDouyinWebhookClaimConflict, http.StatusBadRequest, CodeDouyinWebhookClaimConflict)
	}
	if bodyShopID != "" && headerShopID != "" && bodyShopID != headerShopID {
		return input, newCodeError(CodeDouyinWebhookClaimConflict, http.StatusBadRequest, CodeDouyinWebhookClaimConflict)
	}
	input.AppID = firstNonEmptyString(bodyAppID, headerAppID)
	input.PlatformShopID = firstNonEmptyString(bodyShopID, headerShopID)
	return input, nil
}
func normalizeDouyinPlatform(platform string) string {
	p := strings.ToLower(strings.TrimSpace(platform))
	switch p {
	case "douyin_shop", "douyin":
		return p
	default:
		return ""
	}
}

func douyinResolverPlatforms(platform string) []string {
	switch normalizeDouyinPlatform(platform) {
	case "douyin":
		return []string{"douyin", "douyin_shop"}
	default:
		return []string{"douyin_shop", "douyin"}
	}
}

func isDouyinWebhookPlatform(platform string) bool {
	return normalizeDouyinPlatform(platform) != ""
}

func safeHeaders(h http.Header) map[string]string {
	out := map[string]string{}
	for _, key := range []string{"X-Douyin-Client-Key", "X-Douyin-App-Id", "X-Douyin-Shop-Id", "X-Platform-Shop-Id", "X-Webhook-Secret-Binding-Id"} {
		if v := strings.TrimSpace(h.Get(key)); v != "" {
			out[textproto.CanonicalMIMEHeaderKey(key)] = v
		}
	}
	return out
}

func firstHeader(h http.Header, keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(h.Get(key)); v != "" {
			return v
		}
	}
	return ""
}

func firstJSONPathString(root any, paths ...string) string {
	for _, path := range paths {
		if v := strings.TrimSpace(jsonPathString(root, strings.Split(path, "."))); v != "" {
			return v
		}
	}
	return ""
}

func jsonPathString(v any, path []string) string {
	if len(path) == 0 {
		switch t := v.(type) {
		case string:
			return t
		case float64:
			return strings.TrimSuffix(strings.TrimSuffix(fmt.Sprintf("%.0f", t), ".0"), ".")
		case json.Number:
			return t.String()
		default:
			return strings.TrimSpace(fmt.Sprint(t))
		}
	}
	head := path[0]
	switch cur := v.(type) {
	case map[string]any:
		next, ok := cur[head]
		if !ok {
			return ""
		}
		return jsonPathString(next, path[1:])
	case []any:
		if head != "0" || len(cur) == 0 {
			return ""
		}
		return jsonPathString(cur[0], path[1:])
	default:
		return ""
	}
}

func firstNonEmptyString(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func IsWebhookResolverBindingError(err error) bool {
	var ce *CodeError
	if errors.As(err, &ce) {
		switch ce.Code {
		case CodeDouyinWebhookShopNotResolved, CodeDouyinWebhookShopAmbiguous,
			CodeDouyinWebhookShopBindingMismatch, CodeDouyinWebhookTenantMismatch,
			CodeDouyinWebhookAppBindingMismatch, CodeDouyinWebhookAuthorizationExpired,
			CodeDouyinWebhookBindingRevoked, CodeDouyinWebhookUntrustedShopID:
			return true
		}
	}
	return false
}
