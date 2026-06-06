package douyinshop

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

const (
	errMissingTimeoutSec  = "douyin_shop platform config is incomplete: set timeout_sec in settings"
	errInvalidAuthBaseURL = "douyin_shop: invalid auth_base_url"
	errInvalidAPIBaseURL  = "douyin_shop: invalid api_base_url"
)

// RuntimeConfig is the deploy-level Douyin Shop app configuration.
// Phase 1 deliberately stores only officially provided base URLs and credentials.
// Concrete OAuth/OpenAPI paths are added in later phases after checking the
// current Douyin Open Platform docs for each API.
type RuntimeConfig struct {
	AppKey              string
	AppSecret           string
	ServiceID           string
	RedirectURI         string
	Environment         string
	AuthBaseURL         string
	APIBaseURL          string
	RealAPIEnabled      bool
	OrderSyncEnabled    bool
	InventoryEnabled    bool
	ProductDraftEnabled bool
	HTTPTimeout         time.Duration
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

func boolFromConfig(m map[string]string, key string) bool {
	return strings.EqualFold(mapGetCI(m, key), "true")
}

func validateOptionalBaseURL(raw, errPrefix string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		if err == nil {
			err = errors.New("missing scheme or host")
		}
		return "", fmt.Errorf("%s: %w", errPrefix, err)
	}
	return strings.TrimSuffix(raw, "/"), nil
}

// RuntimeFromMergedMap validates a lowercase-key merged settings map.
func RuntimeFromMergedMap(m map[string]string) (RuntimeConfig, error) {
	var cfg RuntimeConfig

	cfg.AppKey = strings.TrimSpace(mapGetCI(m, "app_key", "client_key"))
	cfg.AppSecret = strings.TrimSpace(mapGetCI(m, "app_secret", "client_secret"))
	cfg.ServiceID = strings.TrimSpace(mapGetCI(m, "service_id"))
	cfg.RedirectURI = strings.TrimSpace(mapGetCI(m, "redirect_uri", "callback_url"))
	if cfg.AppKey == "" || cfg.AppSecret == "" || cfg.RedirectURI == "" {
		return cfg, errors.New(platformp.ErrIncompleteDouyinShopAppConfig)
	}
	ru, err := url.Parse(cfg.RedirectURI)
	if err != nil || ru.Scheme == "" || ru.Host == "" {
		if err == nil {
			err = errors.New("missing scheme or host")
		}
		return cfg, fmt.Errorf("douyin_shop: invalid redirect_uri: %w", err)
	}

	env := strings.TrimSpace(strings.ToLower(mapGetCI(m, "environment")))
	if env == "" {
		env = "production"
	}
	switch env {
	case "production", "sandbox":
		cfg.Environment = env
	default:
		return cfg, fmt.Errorf("douyin_shop: environment must be production or sandbox (got %q)", env)
	}

	authBase, err := validateOptionalBaseURL(mapGetCI(m, "auth_base_url"), errInvalidAuthBaseURL)
	if err != nil {
		return cfg, err
	}
	apiBase, err := validateOptionalBaseURL(mapGetCI(m, "api_base_url"), errInvalidAPIBaseURL)
	if err != nil {
		return cfg, err
	}
	cfg.AuthBaseURL = authBase
	cfg.APIBaseURL = apiBase
	if cfg.AuthBaseURL == "" {
		cfg.AuthBaseURL = "https://fuwu.jinritemai.com"
	}
	if cfg.APIBaseURL == "" {
		cfg.APIBaseURL = "https://openapi-fxg.jinritemai.com"
	}

	tsecRaw := strings.TrimSpace(mapGetCI(m, "timeout_sec"))
	if tsecRaw == "" {
		return cfg, errors.New(errMissingTimeoutSec)
	}
	tsec, err := strconv.Atoi(tsecRaw)
	if err != nil || tsec < 5 || tsec > 600 {
		return cfg, fmt.Errorf("douyin_shop: timeout_sec must be integer 5-600 (got %q)", tsecRaw)
	}
	cfg.HTTPTimeout = time.Duration(tsec) * time.Second

	cfg.RealAPIEnabled = boolFromConfig(m, "real_api_enabled")
	cfg.OrderSyncEnabled = boolFromConfig(m, "order_sync_enabled")
	cfg.InventoryEnabled = boolFromConfig(m, "inventory_sync_enabled")
	cfg.ProductDraftEnabled = boolFromConfig(m, "product_publish_enabled")

	return cfg, nil
}
