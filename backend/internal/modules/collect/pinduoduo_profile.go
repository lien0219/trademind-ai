package collect

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
)

// PinduoduoProfileKey is the dedicated collector persistent profile (not 1688/custom).
const PinduoduoProfileKey = "pinduoduo"

func isPifaPinduoduoURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.TrimSpace(u.Hostname()))
	return host == "pifa.pinduoduo.com" || strings.HasSuffix(host, ".pifa.pinduoduo.com")
}

func shouldUsePinduoduoDedicatedProfile(sourceURL string, useBrowserProfile bool) bool {
	return useBrowserProfile || isPifaPinduoduoURL(sourceURL)
}

func (s *Service) buildPinduoduoRequestOptions(ctx context.Context, sourceURL string, useBrowserProfile bool) []byte {
	opts := map[string]any{}
	if shouldUsePinduoduoDedicatedProfile(sourceURL, useBrowserProfile) {
		opts["useBrowserProfile"] = true
		opts["profileKey"] = PinduoduoProfileKey
	}
	if s != nil && s.Settings != nil {
		m, err := s.Settings.PlainByGroup(ctx, 0, "collector")
		if err == nil && len(m) > 0 {
			if v := strings.TrimSpace(m["collect_pinduoduo_timeout_ms"]); v != "" {
				if n, err := strconv.Atoi(v); err == nil && n > 0 {
					opts["gotoTimeoutMs"] = n
				}
			} else if v := strings.TrimSpace(m["goto_timeout_ms"]); v != "" {
				if n, err := strconv.Atoi(v); err == nil && n > 0 {
					opts["gotoTimeoutMs"] = n
				}
			}
			if v, ok := settingsBool(m, "collect_pinduoduo_access_check_enabled"); ok {
				opts["accessCheckEnabled"] = v
			} else {
				opts["accessCheckEnabled"] = true
			}
		}
	}
	if len(opts) == 0 {
		return nil
	}
	blob, _ := json.Marshal(opts)
	return blob
}

func mergeJSONIntoCollectorOpts(base map[string]any, blob []byte) map[string]any {
	if len(blob) == 0 {
		return base
	}
	var extra map[string]any
	if err := json.Unmarshal(blob, &extra); err != nil || len(extra) == 0 {
		return base
	}
	if base == nil {
		return extra
	}
	for k, v := range extra {
		base[k] = v
	}
	return base
}
