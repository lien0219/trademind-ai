package collect

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
)

// TaobaoTmallProfileKey is the dedicated collector persistent profile (isolated from 1688/pinduoduo/custom).
const TaobaoTmallProfileKey = "taobao_tmall"

func isTaobaoTmallCollectSource(source string) bool {
	s := strings.TrimSpace(strings.ToLower(source))
	return s == "taobao_tmall" || s == "taobao"
}

func (s *Service) buildTaobaoTmallRequestOptions(ctx context.Context, _sourceURL string, useBrowserProfile bool) []byte {
	opts := map[string]any{}
	if useBrowserProfile {
		opts["useBrowserProfile"] = true
		opts["profileKey"] = TaobaoTmallProfileKey
	}
	if s != nil && s.Settings != nil {
		m, err := s.Settings.PlainByGroup(ctx, 0, "collector")
		if err == nil && len(m) > 0 {
			if v := strings.TrimSpace(m["collect_taobao_tmall_timeout_ms"]); v != "" {
				if n, err := strconv.Atoi(v); err == nil && n > 0 {
					opts["gotoTimeoutMs"] = n
				}
			} else if v := strings.TrimSpace(m["goto_timeout_ms"]); v != "" {
				if n, err := strconv.Atoi(v); err == nil && n > 0 {
					opts["gotoTimeoutMs"] = n
				}
			}
			if v, ok := settingsBool(m, "collect_taobao_tmall_access_check_enabled"); ok {
				opts["accessCheckEnabled"] = v
			} else {
				opts["accessCheckEnabled"] = true
			}
			if v, ok := settingsBool(m, "collect_taobao_tmall_scroll_wait_enabled"); ok {
				opts["scrollWaitEnabled"] = v
			} else {
				opts["scrollWaitEnabled"] = true
			}
			if v := strings.TrimSpace(m["collect_taobao_tmall_detail_image_wait_ms"]); v != "" {
				if n, err := strconv.Atoi(v); err == nil && n >= 0 {
					opts["detailImageWaitMs"] = n
				}
			} else {
				opts["detailImageWaitMs"] = 3000
			}
			if v, ok := settingsBool(m, "collect_taobao_tmall_sku_click_enabled"); ok {
				opts["skuClickCollectEnabled"] = v
			} else {
				opts["skuClickCollectEnabled"] = true
			}
			if v := strings.TrimSpace(m["collect_taobao_tmall_sku_click_max"]); v != "" {
				if n, err := strconv.Atoi(v); err == nil && n > 0 {
					opts["skuClickMaxCount"] = n
				}
			} else {
				opts["skuClickMaxCount"] = 24
			}
		}
	}
	if len(opts) == 0 {
		return nil
	}
	blob, _ := json.Marshal(opts)
	return blob
}

func (s *Service) taobaoTmallMaxRetries(ctx context.Context) int {
	def := 2
	if s != nil && s.Settings != nil {
		m, err := s.Settings.PlainByGroup(ctx, 0, "collector")
		if err == nil {
			if v := strings.TrimSpace(m["collect_taobao_tmall_max_retries"]); v != "" {
				if n, err := strconv.Atoi(v); err == nil && n >= 0 {
					return n
				}
			}
		}
	}
	return def
}

func (s *Service) taobaoTmallAutoRetryEnabled(ctx context.Context) bool {
	if s != nil && s.Settings != nil {
		m, err := s.Settings.PlainByGroup(ctx, 0, "collector")
		if err == nil {
			if v, ok := settingsBool(m, "collect_taobao_tmall_retry_on_failure"); ok {
				return v
			}
		}
	}
	return true
}
