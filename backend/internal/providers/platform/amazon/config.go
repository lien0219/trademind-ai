package amazon

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

// RuntimeConfig holds merged Amazon LWA + SP-API settings.
type RuntimeConfig struct {
	ClientID       string
	ClientSecret   string
	LWAAuthBaseURL string
	LWATokenURL    string
	SPAPIBaseURL   string
	RedirectURI    string
	MarketplaceID  string
	Region         string
	RoleARN        string
	SandboxEnabled bool
	HTTPTimeout    time.Duration
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

// RuntimeFromMergedMap validates settings map for platform_amazon (lowercase keys).
func RuntimeFromMergedMap(m map[string]string) (RuntimeConfig, error) {
	var cfg RuntimeConfig
	if len(m) == 0 {
		return cfg, errors.New(platformp.ErrIncompleteAmazonAppConfig)
	}
	cfg.ClientID = mapGetCI(m, "client_id")
	cfg.ClientSecret = mapGetCI(m, "client_secret")
	cfg.LWAAuthBaseURL = strings.TrimSuffix(mapGetCI(m, "lwa_auth_base_url"), "/")
	cfg.LWATokenURL = strings.TrimSpace(mapGetCI(m, "lwa_token_url"))
	cfg.SPAPIBaseURL = strings.TrimSuffix(mapGetCI(m, "sp_api_base_url"), "/")
	cfg.RedirectURI = mapGetCI(m, "redirect_uri")
	cfg.MarketplaceID = mapGetCI(m, "marketplace_id")
	cfg.Region = mapGetCI(m, "region")
	cfg.RoleARN = mapGetCI(m, "role_arn")
	cfg.SandboxEnabled = strings.EqualFold(mapGetCI(m, "sandbox_enabled"), "true")

	if cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.RedirectURI == "" ||
		cfg.MarketplaceID == "" || cfg.SPAPIBaseURL == "" {
		return cfg, errors.New(platformp.ErrIncompleteAmazonAppConfig)
	}
	if cfg.LWAAuthBaseURL == "" || cfg.LWATokenURL == "" {
		return cfg, fmt.Errorf("platform config incomplete: please configure platform_amazon.lwa_auth_base_url and lwa_token_url")
	}
	if u, err := url.Parse(cfg.LWAAuthBaseURL); err != nil || u.Scheme == "" || u.Host == "" {
		return cfg, fmt.Errorf("amazon: invalid lwa_auth_base_url")
	}
	if u, err := url.Parse(cfg.LWATokenURL); err != nil || u.Scheme == "" || u.Host == "" {
		return cfg, fmt.Errorf("amazon: invalid lwa_token_url")
	}
	if u, err := url.Parse(cfg.SPAPIBaseURL); err != nil || u.Scheme == "" || u.Host == "" {
		return cfg, fmt.Errorf("amazon: invalid sp_api_base_url")
	}

	tsecRaw := mapGetCI(m, "timeout_sec")
	if tsecRaw == "" {
		return cfg, fmt.Errorf("amazon: set timeout_sec in settings")
	}
	tsec, err := strconv.Atoi(tsecRaw)
	if err != nil || tsec < 5 || tsec > 600 {
		return cfg, fmt.Errorf("amazon: timeout_sec must be integer 5–600 (got %q)", tsecRaw)
	}
	cfg.HTTPTimeout = time.Duration(tsec) * time.Second

	return cfg, nil
}

// InferSigV4Region picks execute-api region from SP-API host when cfg.region empty.
func InferSigV4Region(host string) string {
	h := strings.ToLower(strings.TrimSpace(host))
	switch {
	case strings.Contains(h, "sellingpartnerapi-na"):
		return "us-east-1"
	case strings.Contains(h, "sellingpartnerapi-eu"):
		return "eu-west-1"
	case strings.Contains(h, "sellingpartnerapi-fe"):
		return "us-west-2"
	default:
		return "us-east-1"
	}
}

// ResolveRuntime merges settings.platform_amazon with optional values from shop auth.
func ResolveRuntime(auth platformp.TestConnectionRequest) (RuntimeConfig, error) {
	global := map[string]string{}
	if bridges != nil {
		g, err := bridges.AmazonGlobalSettings(context.Background())
		if err != nil {
			var zero RuntimeConfig
			return zero, fmt.Errorf("amazon: load settings group platform_amazon: %w", err)
		}
		if g != nil {
			global = g
		}
	}
	merged := mergeStringMaps(true, nil, global)

	exOverlay := map[string]string{}
	if auth.Extra != nil {
		for k, v := range auth.Extra {
			kk := strings.TrimSpace(strings.ToLower(k))
			if kk == "" {
				continue
			}
			exOverlay[kk] = strings.TrimSpace(fmt.Sprint(v))
		}
	}
	merged = mergeStringMaps(true, exOverlay, merged)

	if sid := strings.TrimSpace(auth.SellerID); sid != "" {
		merged = mergeStringMaps(true, map[string]string{"selling_partner_id": sid}, merged)
	}
	if mid := strings.TrimSpace(auth.MarketplaceID); mid != "" {
		merged = mergeStringMaps(true, map[string]string{"marketplace_id": mid}, merged)
	}

	return RuntimeFromMergedMap(merged)
}

// BuildAuthorizeURL returns Amazon Seller Central consent URL (no network I/O).
func BuildAuthorizeURL(cfg RuntimeConfig, state string, redirectOverride string) (string, error) {
	if cfg.LWAAuthBaseURL == "" {
		return "", fmt.Errorf("amazon: missing lwa_auth_base_url")
	}
	st := strings.TrimSpace(state)
	if st == "" {
		return "", fmt.Errorf("amazon: state required")
	}
	rd := strings.TrimSpace(redirectOverride)
	if rd == "" {
		rd = cfg.RedirectURI
	}
	if rd == "" {
		return "", errors.New(platformp.ErrIncompleteAmazonAppConfig)
	}
	u, err := url.Parse(strings.TrimSuffix(cfg.LWAAuthBaseURL, "/") + "/apps/authorize/consent")
	if err != nil {
		return "", err
	}
	q := url.Values{}
	q.Set("application_id", cfg.ClientID)
	q.Set("state", st)
	q.Set("redirect_uri", rd)
	if cfg.SandboxEnabled {
		q.Set("version", "beta")
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// EffectiveMarketplaceID prefers shop-level token marketplace over app default.
func EffectiveMarketplaceID(auth platformp.TestConnectionRequest, cfg RuntimeConfig) string {
	if v := strings.TrimSpace(auth.MarketplaceID); v != "" {
		return v
	}
	return strings.TrimSpace(cfg.MarketplaceID)
}
