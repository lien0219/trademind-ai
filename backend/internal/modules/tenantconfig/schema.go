package tenantconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	ModuleGeneral = "general"
	ModuleAI = "ai"
	ModuleCollect = "collect"
	maxSettingsBytes = 8192
)

type General struct {
	Language string `json:"language"`
	Timezone string `json:"timezone"`
	Currency string `json:"currency"`
}
type AI struct {
	DefaultLanguage string `json:"defaultLanguage"`
}
type Collect struct {
	AllowedSources []string `json:"allowedSources"`
	MaxBatchURLs int `json:"maxBatchURLs"`
}

func Defaults(module string) (any, error) {
	switch module {
	case ModuleGeneral:
		return General{Language: "zh-CN", Timezone: "Asia/Shanghai", Currency: "CNY"}, nil
	case ModuleAI:
		return AI{DefaultLanguage: "zh-CN"}, nil
	case ModuleCollect:
		return Collect{AllowedSources: []string{}, MaxBatchURLs: 0}, nil
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
		if err := decodeStrict(raw, &cfg); err != nil { return nil, err }
		if !oneOf(cfg.Language, "zh-CN", "en-US", "ja-JP") ||
			!oneOf(cfg.Timezone, "Asia/Shanghai", "Asia/Tokyo", "UTC") ||
			!oneOf(cfg.Currency, "CNY", "USD", "JPY") {
			return nil, errors.New("unsupported language, timezone or currency")
		}
		v = cfg
	case ModuleAI:
		cfg := AI{}
		if err := decodeStrict(raw, &cfg); err != nil { return nil, err }
		if !oneOf(cfg.DefaultLanguage, "zh-CN", "en-US", "ja-JP") {
			return nil, errors.New("unsupported AI language")
		}
		v = cfg
	case ModuleCollect:
		cfg := Collect{}
		if err := decodeStrict(raw, &cfg); err != nil { return nil, err }
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
		if cfg.AllowedSources == nil { cfg.AllowedSources = []string{} }
		v = cfg
	default:
		return nil, errors.New("unknown tenant settings module")
	}
	return json.Marshal(v)
}
func oneOf(value string, candidates ...string) bool {
	for _, candidate := range candidates { if candidate == value { return true } }
	return false
}
