package douyinshop

import (
	"encoding/json"
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
	AppKey                        string
	AppSecret                     string
	ServiceID                     string
	RedirectURI                   string
	Environment                   string
	AuthBaseURL                   string
	APIBaseURL                    string
	RealAPIEnabled                bool
	OrderSyncEnabled              bool
	OrderSyncMaxPages             int
	InventoryEnabled              bool
	ProductDraftEnabled           bool
	RuntimeStatus                 string
	RuntimeStatusReason           string
	GrayReleaseEnabled            bool
	GrayShopIDs                   []string
	WriteOperationsEnabled        bool
	ScheduledOrderSyncEnabled     bool
	ScheduledInventorySyncEnabled bool
	HTTPTimeout                   time.Duration
}

const (
	defaultOrderSyncMaxPages   = 5
	maxOrderSyncRecordsPerTask = 500
)

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
	cfg.OrderSyncMaxPages = parseOrderSyncMaxPages(mapGetCI(m, "order_sync_max_pages"))
	cfg.InventoryEnabled = boolFromConfig(m, "inventory_sync_enabled")
	cfg.ProductDraftEnabled = boolFromConfig(m, "product_publish_enabled")
	cfg.RuntimeStatus = parseRuntimeStatus(mapGetCI(m, "platform_runtime_status"))
	cfg.RuntimeStatusReason = strings.TrimSpace(mapGetCI(m, "platform_runtime_status_reason"))
	cfg.GrayReleaseEnabled = boolFromConfig(m, "gray_release_enabled")
	cfg.GrayShopIDs = parseGrayShopIDs(mapGetCI(m, "gray_shop_ids"))
	cfg.WriteOperationsEnabled = boolFromConfig(m, "write_operations_enabled")
	cfg.ScheduledOrderSyncEnabled = boolFromConfig(m, "scheduled_order_sync_enabled")
	cfg.ScheduledInventorySyncEnabled = boolFromConfig(m, "scheduled_inventory_sync_enabled")

	return cfg, nil
}

func parseGrayShopIDs(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return nil
	}
	var ids []string
	if err := json.Unmarshal([]byte(raw), &ids); err == nil {
		return normalizeShopIDs(ids)
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\n'
	})
	return normalizeShopIDs(parts)
}

func normalizeShopIDs(ids []string) []string {
	out := make([]string, 0, len(ids))
	seen := map[string]struct{}{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		key := strings.ToLower(id)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, id)
	}
	return out
}

// ShopInGrayList reports whether shopID is allowed when gray release is enabled.
func (cfg RuntimeConfig) ShopInGrayList(shopID string) bool {
	shopID = strings.TrimSpace(shopID)
	if shopID == "" {
		return false
	}
	for _, id := range cfg.GrayShopIDs {
		if strings.EqualFold(strings.TrimSpace(id), shopID) {
			return true
		}
	}
	return false
}

func parseOrderSyncMaxPages(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultOrderSyncMaxPages
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return defaultOrderSyncMaxPages
	}
	if n > 50 {
		return 50
	}
	return n
}

// ResolveOrderSyncMaxPages picks task override, platform config, or default.
func ResolveOrderSyncMaxPages(taskMax int, cfg RuntimeConfig) int {
	if taskMax > 0 {
		if taskMax > 50 {
			return 50
		}
		return taskMax
	}
	if cfg.OrderSyncMaxPages > 0 {
		return cfg.OrderSyncMaxPages
	}
	return defaultOrderSyncMaxPages
}
