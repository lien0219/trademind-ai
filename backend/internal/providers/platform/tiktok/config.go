package tiktok

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

const (
	// ErrIncompleteCred aliases the unified platform wording for TikTok Partner app secrets / redirect_uri.
	ErrIncompleteCred     = platformp.ErrIncompleteTikTokAppConfig
	errMissingAuthBaseURL = "tiktok platform config is incomplete: set auth_base_url in settings → platform group platform_tiktok (or legacy oauth_*_url entries)"
	errMissingAPIBaseURL  = "tiktok platform config is incomplete: set api_base_url in settings (group platform_tiktok)"
	errMissingAPIVersion  = "tiktok platform config is incomplete: set api_version (e.g. 202309) in settings"
	errMissingTimeoutSec  = "tiktok platform config is incomplete: set timeout_sec in settings"
	errInvalidAuthBaseURL = "tiktok: invalid auth_base_url"
	errInvalidAPIBaseURL  = "tiktok: invalid api_base_url"
)

// RuntimeConfig merges global settings + per-shop overrides (auth Extra / AppKey/AppSecret overlays).
type RuntimeConfig struct {
	SandboxEnabled bool
	Region         string // optional marker from settings / shop (not wired into every TikTok HTTP call yet)

	OAuthAuthorizeURL    string // full URL incl. path
	OAuthTokenGetURL     string
	OAuthTokenRefreshURL string

	OpenAPIHost string // scheme + host (+ optional port); no trailing path

	APIShopCipherPath  string // path starting with /
	APIOrderSearchPath string // path starting with /
	APIVersion         string // query/version param expected by TikTok OpenAPI

	RedirectURI string
	OAuthScopes string

	AppKey    string
	AppSecret string

	ShopCipher string

	HTTPTimeout time.Duration
}

func mapGetCI(m map[string]string, aliases ...string) string {
	for _, a := range aliases {
		al := strings.TrimSpace(strings.ToLower(a))
		if m == nil {
			continue
		}
		for mk, mv := range m {
			if strings.TrimSpace(strings.ToLower(mk)) == al && strings.TrimSpace(mv) != "" {
				return strings.TrimSpace(mv)
			}
		}
	}
	return ""
}

func mergeStringMaps(lower bool, overlay map[string]string, bases ...map[string]string) map[string]string {
	out := map[string]string{}
	for _, base := range bases {
		if base == nil {
			continue
		}
		for k, v := range base {
			if strings.TrimSpace(v) == "" {
				continue
			}
			key := k
			if lower {
				key = strings.TrimSpace(strings.ToLower(k))
			}
			out[key] = strings.TrimSpace(v)
		}
	}
	for k, v := range overlay {
		if strings.TrimSpace(v) == "" {
			continue
		}
		key := k
		if lower {
			key = strings.TrimSpace(strings.ToLower(k))
		}
		out[key] = strings.TrimSpace(v)
	}
	return out
}

// RuntimeFromMergedMap builds RuntimeConfig strictly from lowercase-key merged settings (no env defaults, no TikTok-domain fallbacks).
func RuntimeFromMergedMap(m map[string]string) (RuntimeConfig, error) {
	var cfg RuntimeConfig
	if len(m) == 0 {
		return cfg, errors.New(errMissingAuthBaseURL)
	}

	cfg.Region = strings.TrimSpace(mapGetCI(m, "region"))
	cfg.OAuthScopes = strings.TrimSpace(mapGetCI(m, "oauth_scopes"))
	cfg.SandboxEnabled = strings.EqualFold(mapGetCI(m, "sandbox_enabled"), "true")

	authBase := strings.TrimSpace(mapGetCI(m, "auth_base_url"))
	authz := strings.TrimSuffix(strings.TrimSpace(mapGetCI(m, "oauth_authorize_url", "tiktok_oauth_authorize_url")), "/")
	tokGet := strings.TrimSuffix(strings.TrimSpace(mapGetCI(m, "oauth_token_get_url", "tiktok_token_get_url")), "/")
	tokRef := strings.TrimSuffix(strings.TrimSpace(mapGetCI(m, "oauth_token_refresh_url", "tiktok_token_refresh_url")), "/")

	switch {
	case authBase != "":
		if _, err := url.Parse(authBase); err != nil {
			return cfg, fmt.Errorf("%s: %w", errInvalidAuthBaseURL, err)
		}
		b := strings.TrimSuffix(authBase, "/")
		cfg.OAuthAuthorizeURL = b + "/api/v2/oauth/authorize"
		cfg.OAuthTokenGetURL = b + "/api/v2/token/get"
		cfg.OAuthTokenRefreshURL = b + "/api/v2/token/refresh"
	case authz != "" && tokGet != "" && tokRef != "":
		// Migration: legacy per-URL keys only (no bundled defaults).
		cfg.OAuthAuthorizeURL = authz
		cfg.OAuthTokenGetURL = tokGet
		cfg.OAuthTokenRefreshURL = tokRef
	default:
		return cfg, errors.New(errMissingAuthBaseURL)
	}

	apiBase := strings.TrimSpace(mapGetCI(m, "api_base_url", "openapi_base_url"))
	if apiBase == "" {
		return cfg, errors.New(errMissingAPIBaseURL)
	}
	u, err := url.Parse(apiBase)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return cfg, fmt.Errorf("%s: %w", errInvalidAPIBaseURL, err)
	}
	cfg.OpenAPIHost = strings.TrimSuffix(apiBase, "/")

	ver := strings.TrimSpace(mapGetCI(m, "api_version"))
	if ver == "" {
		return cfg, errors.New(errMissingAPIVersion)
	}
	cfg.APIVersion = ver
	cfg.APIShopCipherPath = "/authorization/" + ver + "/shops"
	cfg.APIOrderSearchPath = "/order/" + ver + "/orders/search"

	tsecRaw := strings.TrimSpace(mapGetCI(m, "timeout_sec"))
	if tsecRaw == "" {
		return cfg, errors.New(errMissingTimeoutSec)
	}
	tsec, err := strconv.Atoi(tsecRaw)
	if err != nil || tsec < 5 || tsec > 600 {
		return cfg, fmt.Errorf("tiktok: timeout_sec must be integer 5–600 (got %q)", tsecRaw)
	}
	cfg.HTTPTimeout = time.Duration(tsec) * time.Second

	cfg.RedirectURI = strings.TrimSpace(mapGetCI(m, "redirect_uri", "redirecturi"))
	cfg.AppKey = strings.TrimSpace(mapGetCI(m, "app_key"))
	cfg.AppSecret = strings.TrimSpace(mapGetCI(m, "app_secret"))

	if cfg.AppKey == "" || cfg.AppSecret == "" || cfg.RedirectURI == "" {
		return cfg, errors.New(ErrIncompleteCred)
	}

	for _, chk := range []struct{ raw, label string }{
		{cfg.OAuthAuthorizeURL, "oauth authorize url"},
		{cfg.OAuthTokenGetURL, "oauth token get url"},
		{cfg.OAuthTokenRefreshURL, "oauth token refresh url"},
	} {
		if _, err := url.Parse(chk.raw); err != nil {
			return cfg, fmt.Errorf("tiktok: invalid %s: %w", chk.label, err)
		}
	}

	for _, chk := range []struct{ raw, label string }{
		{cfg.OAuthAuthorizeURL, "oauth authorize url"},
	} {
		if !strings.Contains(strings.TrimSpace(chk.raw), "://") {
			return cfg, fmt.Errorf("%s: missing scheme", chk.label)
		}
	}

	return cfg, nil
}

// ResolveRuntime builds effective configuration from platform_tiktok settings (via ShopsBridge),
// overlays per-shop auth (Extra JSON, optional AppKey / AppSecret override), never logs secrets.
func ResolveRuntime(auth platformp.TestConnectionRequest) (RuntimeConfig, error) {
	global := map[string]string{}
	if bridges != nil {
		g, err := bridges.TikTokGlobalSettings(context.Background())
		if err != nil {
			var zero RuntimeConfig
			return zero, fmt.Errorf("tiktok: load settings group platform_tiktok: %w", err)
		}
		if g != nil {
			global = g
		}
	}

	merged := mergeStringMaps(true, nil, global)

	if auth.Extra != nil {
		exOverlay := map[string]string{}
		for k, v := range auth.Extra {
			kk := strings.TrimSpace(strings.ToLower(k))
			if kk == "" {
				continue
			}
			exOverlay[kk] = strings.TrimSpace(fmt.Sprint(v))
		}
		merged = mergeStringMaps(true, exOverlay, merged)
	}

	shopOverlay := map[string]string{}
	if ak := strings.TrimSpace(auth.AppKey); ak != "" {
		shopOverlay["app_key"] = ak
	}
	if sec := strings.TrimSpace(auth.AppSecret); sec != "" {
		shopOverlay["app_secret"] = sec
	}
	merged = mergeStringMaps(true, shopOverlay, merged)

	cfg, err := RuntimeFromMergedMap(merged)
	if err != nil {
		return cfg, err
	}

	shopCipher := strings.TrimSpace(auth.MerchantID)
	if shopCipher == "" {
		for _, k := range []string{"shop_cipher", "shopcipher"} {
			if v := mapGetCI(auth.Extra, k); strings.TrimSpace(v) != "" {
				shopCipher = strings.TrimSpace(v)
				break
			}
		}
	}
	cfg.ShopCipher = shopCipher

	return cfg, nil
}

// BuildAuthorizeURL assembles the TikTok OAuth authorize URL.
func BuildAuthorizeURL(cfg RuntimeConfig, state string, redirectURIOverride string) (string, error) {
	if strings.TrimSpace(cfg.OAuthAuthorizeURL) == "" {
		return "", fmt.Errorf("%s", errMissingAuthBaseURL)
	}
	rd := strings.TrimSpace(redirectURIOverride)
	if rd == "" {
		rd = strings.TrimSpace(cfg.RedirectURI)
	}
	if rd == "" {
		return "", errors.New(ErrIncompleteCred)
	}

	uu, err := url.Parse(cfg.OAuthAuthorizeURL)
	if err != nil {
		return "", fmt.Errorf("tiktok: invalid oauth authorize URL: %w", err)
	}
	q := url.Values{}
	q.Set("app_key", cfg.AppKey)
	q.Set("redirect_uri", rd)
	q.Set("response_type", "code")
	q.Set("state", state)
	if scopes := strings.TrimSpace(cfg.OAuthScopes); scopes != "" {
		q.Set("scope", scopes)
	}
	uu.RawQuery = q.Encode()
	return uu.String(), nil
}
