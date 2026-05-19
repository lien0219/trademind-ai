package image

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/providers/image/dashscopeimage"
	"github.com/trademind-ai/trademind/backend/internal/providers/image/openaiimage"
)

// compatImageKeys maps provider name to settings key prefix (e.g. openai_image -> openai_image).
func compatImageKeys(prefix string, m map[string]string) (openaiimage.Options, error) {
	key := strings.TrimSpace(m[prefix+"_api_key"])
	if key == "" {
		return openaiimage.Options{}, ErrAPIKeyMissing
	}
	model := strings.TrimSpace(m[prefix+"_model"])
	size := strings.TrimSpace(m[prefix+"_size"])
	quality := strings.TrimSpace(m[prefix+"_quality"])
	background := strings.TrimSpace(m[prefix+"_background"])
	base := strings.TrimSpace(m[prefix+"_base_url"])
	sec := timeoutSecFromImageMap(m)
	return openaiimage.Options{
		BaseURL:    base,
		APIKey:     key,
		Model:      model,
		Size:       size,
		Quality:    quality,
		Background: background,
		Timeout:    time.Duration(sec) * time.Second,
	}, nil
}

func dashscopeOpts(m map[string]string) (dashscopeimage.Options, error) {
	key := strings.TrimSpace(m["dashscope_image_api_key"])
	if key == "" {
		return dashscopeimage.Options{}, ErrAPIKeyMissing
	}
	model := strings.TrimSpace(m["dashscope_image_model"])
	size := strings.TrimSpace(m["dashscope_image_size"])
	base := strings.TrimSpace(m["dashscope_image_base_url"])
	sec := timeoutSecFromImageMap(m)
	if sec < 30 {
		sec = 120
	}
	return dashscopeimage.Options{
		BaseURL: base,
		APIKey:  key,
		Model:   model,
		Size:    size,
		Timeout: time.Duration(sec) * time.Second,
	}, nil
}

// ConfigStatus summarizes whether required settings exist (no secret values).
func ConfigStatus(provider string, m map[string]string) string {
	p := strings.TrimSpace(strings.ToLower(provider))
	switch p {
	case "noop":
		return "ready"
	case "removebg":
		if strings.TrimSpace(m["removebg_api_key"]) == "" {
			return "missing_api_key"
		}
		return "ready"
	case "openai_image":
		if strings.TrimSpace(m["openai_image_api_key"]) == "" {
			return "missing_api_key"
		}
		return "ready"
	case "comfyui":
		if strings.TrimSpace(m["comfyui_base_url"]) == "" {
			return "missing_base_url"
		}
		if wf := strings.TrimSpace(m["comfyui_workflow_json"]); wf == "" || wf == "{}" {
			return "missing_workflow"
		}
		return "ready"
	case "dashscope_image":
		if strings.TrimSpace(m["dashscope_image_api_key"]) == "" {
			return "missing_api_key"
		}
		return "ready"
	case "volcengine_image":
		if strings.TrimSpace(m["volcengine_image_api_key"]) == "" {
			return "missing_api_key"
		}
		return "ready"
	case "siliconflow_image":
		if strings.TrimSpace(m["siliconflow_image_api_key"]) == "" {
			return "missing_api_key"
		}
		return "ready"
	case "hunyuan_image":
		return "planned"
	default:
		return "unknown"
	}
}

// ValidateSettingsForProvider checks runnable provider config.
func ValidateSettingsForProvider(provider string, m map[string]string) error {
	if !IsRunnableProvider(provider) {
		cap := CapabilityByProvider(provider)
		if cap != nil && cap.Status == ProviderStatusPlanned {
			return fmt.Errorf("「%s」尚未开放，请更换其他图片 AI 服务或在后续版本再试", cap.DisplayName)
		}
		return ErrConfigIncomplete
	}
	switch ConfigStatus(provider, m) {
	case "ready":
		return nil
	case "missing_api_key":
		return ErrAPIKeyMissing
	default:
		return ErrConfigIncomplete
	}
}

func intSetting(m map[string]string, key string, def, minV, maxV int) int {
	s := strings.TrimSpace(m[key])
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < minV {
		return def
	}
	if n > maxV {
		return maxV
	}
	return n
}
