package shopee

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
	ErrIncompleteCred    = platformp.ErrIncompleteShopeeAppConfig
	errMissingAuthBase   = "shopee: platform config incomplete: set auth_base_url in settings (group platform_shopee)"
	errMissingAPIBase    = "shopee: platform config incomplete: set api_base_url in settings (group platform_shopee)"
	errMissingTimeoutSec = "shopee: platform config incomplete: set timeout_sec in settings"
	errInvalidBaseURL    = "shopee: invalid base url"
)

// RuntimeConfig holds merged Partner + endpoint settings (no logging).
type RuntimeConfig struct {
	SandboxEnabled bool
	Region         string

	AuthBaseURL string // scheme+host for auth_partner
	APIBaseURL  string // scheme+host for API calls
	RedirectURI string

	PartnerID  int64
	PartnerKey string

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

// RuntimeFromMergedMap validates lowercase-key settings map for platform_shopee.
func RuntimeFromMergedMap(m map[string]string) (RuntimeConfig, error) {
	var cfg RuntimeConfig
	if len(m) == 0 {
		return cfg, errors.New(errMissingAuthBase)
	}
	cfg.Region = mapGetCI(m, "region")
	cfg.SandboxEnabled = strings.EqualFold(mapGetCI(m, "sandbox_enabled"), "true")

	pidStr := mapGetCI(m, "partner_id")
	if pidStr == "" {
		return cfg, errors.New(ErrIncompleteCred)
	}
	pid, err := strconv.ParseInt(pidStr, 10, 64)
	if err != nil || pid <= 0 {
		return cfg, fmt.Errorf("shopee: invalid partner_id")
	}
	cfg.PartnerID = pid

	pkey := mapGetCI(m, "partner_key")
	if pkey == "" {
		return cfg, errors.New(ErrIncompleteCred)
	}
	cfg.PartnerKey = pkey

	authBase := mapGetCI(m, "auth_base_url")
	if authBase == "" {
		return cfg, errors.New(errMissingAuthBase)
	}
	if u, err := url.Parse(authBase); err != nil || u.Scheme == "" || u.Host == "" {
		return cfg, fmt.Errorf("%s: %w", errInvalidBaseURL, err)
	}
	cfg.AuthBaseURL = strings.TrimSuffix(authBase, "/")

	apiBase := mapGetCI(m, "api_base_url")
	if apiBase == "" {
		return cfg, errors.New(errMissingAPIBase)
	}
	if u, err := url.Parse(apiBase); err != nil || u.Scheme == "" || u.Host == "" {
		return cfg, fmt.Errorf("%s: %w", errInvalidBaseURL, err)
	}
	cfg.APIBaseURL = strings.TrimSuffix(apiBase, "/")

	cfg.RedirectURI = mapGetCI(m, "redirect_uri")
	if cfg.RedirectURI == "" {
		return cfg, errors.New(ErrIncompleteCred)
	}

	tsecRaw := mapGetCI(m, "timeout_sec")
	if tsecRaw == "" {
		return cfg, errors.New(errMissingTimeoutSec)
	}
	tsec, err := strconv.Atoi(tsecRaw)
	if err != nil || tsec < 5 || tsec > 600 {
		return cfg, fmt.Errorf("shopee: timeout_sec must be integer 5–600 (got %q)", tsecRaw)
	}
	cfg.HTTPTimeout = time.Duration(tsec) * time.Second

	return cfg, nil
}

// ResolveRuntime merges platform_shopee settings with optional per-shop overrides (AppKey=partner_id, AppSecret=partner_key).
func ResolveRuntime(auth platformp.TestConnectionRequest) (RuntimeConfig, error) {
	global := map[string]string{}
	if bridges != nil {
		g, err := bridges.ShopeeGlobalSettings(context.Background())
		if err != nil {
			var zero RuntimeConfig
			return zero, fmt.Errorf("shopee: load settings group platform_shopee: %w", err)
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

	if ak := strings.TrimSpace(auth.AppKey); ak != "" {
		merged = mergeStringMaps(true, map[string]string{"partner_id": ak}, merged)
	}
	if sec := strings.TrimSpace(auth.AppSecret); sec != "" {
		merged = mergeStringMaps(true, map[string]string{"partner_key": sec}, merged)
	}

	return RuntimeFromMergedMap(merged)
}

// BuildAuthPartnerURL returns the authorization URL (no network I/O).
func BuildAuthPartnerURL(cfg RuntimeConfig, redirectOverride string) (string, error) {
	if cfg.AuthBaseURL == "" {
		return "", errors.New(errMissingAuthBase)
	}
	rd := strings.TrimSpace(redirectOverride)
	if rd == "" {
		rd = cfg.RedirectURI
	}
	if rd == "" {
		return "", errors.New(ErrIncompleteCred)
	}
	ts := time.Now().Unix()
	baseStr := BaseStringPublic(cfg.PartnerID, PathAuthPartner, ts)
	sign := SignHMAC(cfg.PartnerKey, baseStr)
	u, err := url.Parse(cfg.AuthBaseURL + PathAuthPartner)
	if err != nil {
		return "", err
	}
	q := url.Values{}
	q.Set("partner_id", strconv.FormatInt(cfg.PartnerID, 10))
	q.Set("timestamp", strconv.FormatInt(ts, 10))
	q.Set("sign", sign)
	q.Set("redirect", rd)
	u.RawQuery = q.Encode()
	return u.String(), nil
}
