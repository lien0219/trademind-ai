package tenantconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"strings"
)

const (
	ModuleGeneral       = "general"
	ModuleAI            = "ai"
	ModuleCollect       = "collect"
	ModuleInventory     = "inventory"
	ModuleNotifications = "notifications"
	ModuleOrders        = "orders"
	ModuleBranding      = "branding"
	maxSettingsBytes    = 8192
)

type General struct {
	Language string `json:"language"`
	Timezone string `json:"timezone"`
	Currency string `json:"currency"`
}
type AI struct {
	DefaultLanguage string `json:"defaultLanguage"`
}
type Inventory struct {
	LowStockThreshold int `json:"lowStockThreshold"`
}
type Orders struct {
	DefaultCurrency string `json:"defaultCurrency"`
}
type Notifications struct {
	BusinessAlertsEnabled bool     `json:"businessAlertsEnabled"`
	MailRecipients        []string `json:"mailRecipients"`
}
type Branding struct {
	DisplayName string   `json:"displayName"`
	LogoURL     string   `json:"logoUrl"`
	Labels      []string `json:"labels"`
}
type Collect struct {
	AllowedSources []string `json:"allowedSources"`
	MaxBatchURLs   int      `json:"maxBatchURLs"`
}

func Defaults(module string) (any, error) {
	switch module {
	case ModuleGeneral:
		return General{Language: "zh-CN", Timezone: "Asia/Shanghai", Currency: "CNY"}, nil
	case ModuleAI:
		return AI{DefaultLanguage: "zh-CN"}, nil
	case ModuleCollect:
		return Collect{AllowedSources: []string{}, MaxBatchURLs: 0}, nil
	case ModuleInventory:
		return Inventory{LowStockThreshold: 5}, nil
	case ModuleNotifications:
		return Notifications{BusinessAlertsEnabled: true, MailRecipients: []string{}}, nil
	case ModuleOrders:
		return Orders{DefaultCurrency: "USD"}, nil
	case ModuleBranding:
		return Branding{Labels: []string{}}, nil
	default:
		return nil, errors.New("unknown tenant settings module")
	}
}

func decodeStrict(raw []byte, target any) error {
	if len(raw) == 0 || len(raw) > maxSettingsBytes {
		return errors.New("invalid settings document size")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return errors.New("settings contain trailing data")
	}
	return nil
}

func Validate(module string, raw []byte) ([]byte, error) {
	if !json.Valid(raw) || !bytes.HasPrefix(bytes.TrimSpace(raw), []byte("{")) {
		return nil, errors.New("settings must be a JSON object")
	}
	var v any
	switch module {
	case ModuleGeneral:
		cfg := General{}
		if err := decodeStrict(raw, &cfg); err != nil {
			return nil, err
		}
		if !oneOf(cfg.Language, "zh-CN", "en-US", "ja-JP") ||
			!oneOf(cfg.Timezone, "Asia/Shanghai", "Asia/Tokyo", "UTC") ||
			!oneOf(cfg.Currency, "CNY", "USD", "JPY") {
			return nil, errors.New("unsupported language, timezone or currency")
		}
		v = cfg
	case ModuleAI:
		cfg := AI{}
		if err := decodeStrict(raw, &cfg); err != nil {
			return nil, err
		}
		if !oneOf(cfg.DefaultLanguage, "zh-CN", "en-US", "ja-JP") {
			return nil, errors.New("unsupported AI language")
		}
		v = cfg
	case ModuleInventory:
		cfg := Inventory{}
		if err := decodeStrict(raw, &cfg); err != nil {
			return nil, err
		}
		if cfg.LowStockThreshold < 0 || cfg.LowStockThreshold > 1000000 {
			return nil, errors.New("invalid inventory threshold")
		}
		v = cfg
	case ModuleOrders:
		cfg := Orders{}
		if err := decodeStrict(raw, &cfg); err != nil {
			return nil, err
		}
		if !oneOf(cfg.DefaultCurrency, "CNY", "USD", "JPY", "EUR", "GBP") {
			return nil, errors.New("unsupported default order currency")
		}
		v = cfg
	case ModuleNotifications:
		cfg := Notifications{}
		if err := decodeStrict(raw, &cfg); err != nil {
			return nil, err
		}
		if len(cfg.MailRecipients) > 10 {
			return nil, errors.New("too many notification recipients")
		}
		seen := map[string]bool{}
		for _, address := range cfg.MailRecipients {
			if address != strings.TrimSpace(address) || len(address) > 254 {
				return nil, errors.New("invalid mail recipient")
			}
			parsed, err := mail.ParseAddress(address)
			if err != nil || parsed.Address != address || parsed.Name != "" || seen[strings.ToLower(address)] {
				return nil, errors.New("invalid or duplicate mail recipient")
			}
			seen[strings.ToLower(address)] = true
		}
		if cfg.MailRecipients == nil {
			cfg.MailRecipients = []string{}
		}
		v = cfg
	case ModuleBranding:
		cfg := Branding{}
		if err := decodeStrict(raw, &cfg); err != nil {
			return nil, err
		}
		if len([]rune(cfg.DisplayName)) > 120 || len(cfg.Labels) > 20 || len([]byte(cfg.LogoURL)) > 2048 {
			return nil, errors.New("invalid branding size")
		}
		if cfg.LogoURL != "" && !strings.HasPrefix(cfg.LogoURL, "https://") {
			return nil, errors.New("logo must be HTTPS")
		}
		for _, label := range cfg.Labels {
			if strings.TrimSpace(label) != label || len([]rune(label)) > 32 {
				return nil, errors.New("invalid enterprise label")
			}
		}
		v = cfg
	case ModuleCollect:
		cfg := Collect{}
		if err := decodeStrict(raw, &cfg); err != nil {
			return nil, err
		}
		if cfg.MaxBatchURLs < 0 || cfg.MaxBatchURLs > 100 {
			return nil, errors.New("batch limit must be 0-100")
		}
		if len(cfg.AllowedSources) > 10 {
			return nil, errors.New("too many collect sources")
		}
		seen := map[string]bool{}
		for _, source := range cfg.AllowedSources {
			if strings.TrimSpace(source) != source || !oneOf(source, "1688", "pinduoduo", "taobao_tmall", "aliexpress", "custom") || seen[source] {
				return nil, fmt.Errorf("invalid or duplicate collect source: %q", source)
			}
			seen[source] = true
		}
		if cfg.AllowedSources == nil {
			cfg.AllowedSources = []string{}
		}
		v = cfg
	default:
		return nil, errors.New("unknown tenant settings module")
	}
	return json.Marshal(v)
}
func oneOf(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if candidate == value {
			return true
		}
	}
	return false
}
