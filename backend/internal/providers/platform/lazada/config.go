package lazada

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

// RuntimeConfig holds merged Lazada Open Platform + endpoint settings (no logging).
type RuntimeConfig struct {
	AppKey         string
	AppSecret      string
	SandboxEnabled bool
	Region         string
	AuthPageBase   string // e.g. https://auth.lazada.com (no /rest)
	AuthRESTBase   string // e.g. https://auth.lazada.com/rest
	APIRESTBase    string // e.g. https://api.lazada.sg/rest
	RedirectURI    string
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

// RuntimeFromMergedMap validates lowercase-key settings map for platform_lazada.
func RuntimeFromMergedMap(m map[string]string) (RuntimeConfig, error) {
	var cfg RuntimeConfig
	if len(m) == 0 {
		return cfg, errors.New(platformp.ErrIncompleteLazadaAppConfig)
	}
	cfg.AppKey = mapGetCI(m, "app_key")
	cfg.AppSecret = mapGetCI(m, "app_secret")
	if cfg.AppKey == "" || cfg.AppSecret == "" {
		return cfg, errors.New(platformp.ErrIncompleteLazadaAppConfig)
	}
	cfg.Region = mapGetCI(m, "region")
	cfg.SandboxEnabled = strings.EqualFold(mapGetCI(m, "sandbox_enabled"), "true")

	authBase := mapGetCI(m, "auth_base_url")
	if authBase == "" {
		return cfg, fmt.Errorf("lazada: set auth_base_url in settings (group platform_lazada)")
	}
	if u, err := url.Parse(authBase); err != nil || u.Scheme == "" || u.Host == "" {
		return cfg, fmt.Errorf("lazada: invalid auth_base_url")
	}
	authBase = strings.TrimSuffix(strings.TrimSpace(authBase), "/")
	cfg.AuthPageBase = stripRESTSuffix(authBase)
	cfg.AuthRESTBase = ensureRESTBase(authBase)

	apiBase := mapGetCI(m, "api_base_url")
	if apiBase == "" {
		return cfg, fmt.Errorf("lazada: set api_base_url in settings (group platform_lazada)")
	}
	if u, err := url.Parse(apiBase); err != nil || u.Scheme == "" || u.Host == "" {
		return cfg, fmt.Errorf("lazada: invalid api_base_url")
	}
	apiBase = strings.TrimSuffix(strings.TrimSpace(apiBase), "/")
	cfg.APIRESTBase = ensureRESTBase(apiBase)

	cfg.RedirectURI = mapGetCI(m, "redirect_uri")
	if cfg.RedirectURI == "" {
		return cfg, errors.New(platformp.ErrIncompleteLazadaAppConfig)
	}

	tsecRaw := mapGetCI(m, "timeout_sec")
	if tsecRaw == "" {
		return cfg, fmt.Errorf("lazada: set timeout_sec in settings")
	}
	tsec, err := strconv.Atoi(tsecRaw)
	if err != nil || tsec < 5 || tsec > 600 {
		return cfg, fmt.Errorf("lazada: timeout_sec must be integer 5–600 (got %q)", tsecRaw)
	}
	cfg.HTTPTimeout = time.Duration(tsec) * time.Second

	return cfg, nil
}

func ensureRESTBase(base string) string {
	b := strings.TrimSuffix(strings.TrimSpace(base), "/")
	if strings.HasSuffix(strings.ToLower(b), "/rest") {
		return b
	}
	return b + "/rest"
}

func stripRESTSuffix(base string) string {
	b := strings.TrimSuffix(strings.TrimSpace(base), "/")
	if strings.HasSuffix(strings.ToLower(b), "/rest") {
		b = strings.TrimSuffix(b, "/rest")
		b = strings.TrimSuffix(b, "/REST")
	}
	return strings.TrimSuffix(b, "/")
}

// ResolveRuntime merges settings.platform_lazada with optional per-shop overrides.
func ResolveRuntime(auth platformp.TestConnectionRequest) (RuntimeConfig, error) {
	global := map[string]string{}
	if bridges != nil {
		g, err := bridges.LazadaGlobalSettings(context.Background())
		if err != nil {
			var zero RuntimeConfig
			return zero, fmt.Errorf("lazada: load settings group platform_lazada: %w", err)
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
		merged = mergeStringMaps(true, map[string]string{"app_key": ak}, merged)
	}
	if sec := strings.TrimSpace(auth.AppSecret); sec != "" {
		merged = mergeStringMaps(true, map[string]string{"app_secret": sec}, merged)
	}

	return RuntimeFromMergedMap(merged)
}

// BuildAuthorizeURL returns the Lazada OAuth authorize URL (no network I/O).
func BuildAuthorizeURL(cfg RuntimeConfig, state string, redirectOverride string, regionHint string) (string, error) {
	if cfg.AuthPageBase == "" {
		return "", fmt.Errorf("lazada: missing auth_base_url")
	}
	rd := strings.TrimSpace(redirectOverride)
	if rd == "" {
		rd = cfg.RedirectURI
	}
	if rd == "" {
		return "", errors.New(platformp.ErrIncompleteLazadaAppConfig)
	}
	st := strings.TrimSpace(state)
	if st == "" {
		return "", fmt.Errorf("lazada: state required")
	}
	u, err := url.Parse(cfg.AuthPageBase + "/oauth/authorize")
	if err != nil {
		return "", err
	}
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("force_auth", "true")
	q.Set("redirect_uri", rd)
	q.Set("client_id", cfg.AppKey)
	q.Set("state", st)
	if r := strings.TrimSpace(regionHint); r != "" {
		q.Set("country", r)
	} else if r := strings.TrimSpace(cfg.Region); r != "" {
		q.Set("country", r)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}
