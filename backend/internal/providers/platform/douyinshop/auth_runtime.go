package douyinshop

import (
	"context"
	"strings"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

func mergeStringMaps(overlay map[string]string, bases ...map[string]string) map[string]string {
	out := map[string]string{}
	for _, base := range bases {
		if base == nil {
			continue
		}
		for k, v := range base {
			if strings.TrimSpace(v) == "" {
				continue
			}
			out[strings.TrimSpace(strings.ToLower(k))] = strings.TrimSpace(v)
		}
	}
	for k, v := range overlay {
		if strings.TrimSpace(v) == "" {
			continue
		}
		out[strings.TrimSpace(strings.ToLower(k))] = strings.TrimSpace(v)
	}
	return out
}

func authOverlay(auth platformp.TestConnectionRequest) map[string]string {
	overlay := map[string]string{}
	if v := strings.TrimSpace(auth.AppKey); v != "" {
		overlay["app_key"] = v
	}
	if v := strings.TrimSpace(auth.AppSecret); v != "" {
		overlay["app_secret"] = v
	}
	if auth.Extra != nil {
		for k, v := range auth.Extra {
			if strings.TrimSpace(v) == "" {
				continue
			}
			overlay[strings.TrimSpace(strings.ToLower(k))] = strings.TrimSpace(v)
		}
	}
	return overlay
}

// ResolveRuntime merges shop auth overrides with platform_douyin_shop settings.
func ResolveRuntime(ctx context.Context, auth platformp.TestConnectionRequest) (RuntimeConfig, error) {
	var global map[string]string
	if bridges != nil {
		g, err := bridges.DouyinGlobalSettings(ctx)
		if err != nil {
			return RuntimeConfig{}, err
		}
		global = g
	}
	merged := mergeStringMaps(authOverlay(auth), global)
	return RuntimeFromMergedMap(merged)
}
