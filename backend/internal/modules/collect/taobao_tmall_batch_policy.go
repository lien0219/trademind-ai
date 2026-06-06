package collect

import (
	"context"
	"strings"
)

const (
	taobaoTmallBatchMaxItemsDefault    = 20
	taobaoTmallBatchConcurrencyDefault = 1
	taobaoTmallBatchConcurrencyMax     = 2
	taobaoTmallBatchDelayMinDefault    = 3500
	taobaoTmallBatchDelayMaxDefault    = 6000
	taobaoTmallBatchMaxRetriesDefault  = 2
)

func (s *Service) envTaobaoTmallBatchPolicy() BatchSourcePolicy {
	return BatchSourcePolicy{
		Source:          "taobao_tmall",
		Concurrency:     taobaoTmallBatchConcurrencyDefault,
		DelayMinMs:      taobaoTmallBatchDelayMinDefault,
		DelayMaxMs:      taobaoTmallBatchDelayMaxDefault,
		RetryOnBlocked:  false,
		RetryOnTimeout:  true,
		MaxRetries:      taobaoTmallBatchMaxRetriesDefault,
		BatchRetryBoost: true,
	}
}

func (s *Service) taobaoTmallBatchEnabled(ctx context.Context) bool {
	if s == nil || s.Settings == nil {
		return true
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "collector")
	if err != nil || len(m) == 0 {
		return true
	}
	v, ok := settingsBool(m, "collect_taobao_tmall_batch_enabled")
	if !ok {
		return true
	}
	return v
}

func (s *Service) taobaoTmallBatchMaxItems(ctx context.Context) int {
	def := taobaoTmallBatchMaxItemsDefault
	if s == nil || s.Settings == nil {
		return def
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "collector")
	if err != nil || len(m) == 0 {
		return def
	}
	if v := settingsInt(m, "collect_taobao_tmall_batch_max_items"); v > 0 {
		if v > 50 {
			return 50
		}
		return v
	}
	return def
}

func (s *Service) taobaoTmallBatchPauseOnLogin(ctx context.Context) bool {
	if s == nil || s.Settings == nil {
		return true
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "collector")
	if err != nil || len(m) == 0 {
		return true
	}
	v, ok := settingsBool(m, "collect_taobao_tmall_batch_pause_on_login")
	if !ok {
		return true
	}
	return v
}

func (s *Service) taobaoTmallBatchPauseOnVerify(ctx context.Context) bool {
	if s == nil || s.Settings == nil {
		return true
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "collector")
	if err != nil || len(m) == 0 {
		return true
	}
	v, ok := settingsBool(m, "collect_taobao_tmall_batch_pause_on_verify")
	if !ok {
		return true
	}
	return v
}

func (s *Service) taobaoTmallBatchPolicyFromSettings(ctx context.Context) BatchSourcePolicy {
	p := s.envTaobaoTmallBatchPolicy()
	if s == nil || s.Settings == nil {
		return p
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "collector")
	if err != nil || len(m) == 0 {
		return p
	}
	if v := settingsInt(m, "collect_taobao_tmall_batch_concurrency"); v > 0 {
		p.Concurrency = v
		if p.Concurrency > taobaoTmallBatchConcurrencyMax {
			p.Concurrency = taobaoTmallBatchConcurrencyMax
		}
	}
	if v := settingsInt(m, "collect_taobao_tmall_batch_delay_min_ms"); v >= 0 {
		p.DelayMinMs = v
	}
	if v := settingsInt(m, "collect_taobao_tmall_batch_delay_max_ms"); v > 0 {
		p.DelayMaxMs = v
	}
	if p.DelayMinMs > p.DelayMaxMs {
		p.DelayMinMs, p.DelayMaxMs = p.DelayMaxMs, p.DelayMinMs
	}
	if v, ok := settingsBool(m, "collect_taobao_tmall_batch_retry_on_timeout"); ok {
		p.RetryOnTimeout = v
	}
	if v := settingsInt(m, "collect_taobao_tmall_batch_max_retries"); v > 0 {
		p.MaxRetries = v
	}
	return p
}

func isTaobaoTmallAuthStatusLoggedIn(st string) bool {
	switch strings.TrimSpace(strings.ToLower(st)) {
	case "logged_in", "ok":
		return true
	default:
		return false
	}
}

func isTaobaoTmallAuthStatusVerifyRequired(st string) bool {
	switch strings.TrimSpace(strings.ToLower(st)) {
	case "verify_required", "verification_required":
		return true
	default:
		return false
	}
}

func isTaobaoTmallAuthStatusLoginRequired(st string) bool {
	switch strings.TrimSpace(strings.ToLower(st)) {
	case "login_required", "not_logged_in":
		return true
	default:
		return false
	}
}
