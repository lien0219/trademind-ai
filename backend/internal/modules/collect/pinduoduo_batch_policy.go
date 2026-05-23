package collect

import (
	"context"
	"strings"
)

func (s *Service) envPinduoduoBatchPolicy() BatchSourcePolicy {
	p := BatchSourcePolicy{
		Source:          "pinduoduo",
		Concurrency:     1,
		DelayMinMs:      4000,
		DelayMaxMs:      9000,
		RetryOnBlocked:  true,
		RetryOnTimeout:  true,
		MaxRetries:      2,
		BatchRetryBoost: true,
	}
	if s == nil {
		return p
	}
	return p
}

func (s *Service) pinduoduoBatchEnabled(ctx context.Context) bool {
	if s == nil || s.Settings == nil {
		return true
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "collector")
	if err != nil || len(m) == 0 {
		return true
	}
	v, ok := settingsBool(m, "collect_pinduoduo_batch_enabled")
	if !ok {
		return true
	}
	return v
}

func (s *Service) pinduoduoBatchPolicyFromSettings(ctx context.Context) BatchSourcePolicy {
	p := s.envPinduoduoBatchPolicy()
	if s == nil || s.Settings == nil {
		return p
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "collector")
	if err != nil || len(m) == 0 {
		return p
	}
	if v := settingsInt(m, "collect_pinduoduo_batch_concurrency"); v > 0 {
		p.Concurrency = v
		if p.Concurrency > 3 {
			p.Concurrency = 3
		}
	}
	if v := settingsInt(m, "collect_pinduoduo_batch_delay_min_ms"); v >= 0 {
		p.DelayMinMs = v
	}
	if v := settingsInt(m, "collect_pinduoduo_batch_delay_max_ms"); v > 0 {
		p.DelayMaxMs = v
	}
	if p.DelayMinMs > p.DelayMaxMs {
		p.DelayMinMs, p.DelayMaxMs = p.DelayMaxMs, p.DelayMinMs
	}
	if v, ok := settingsBool(m, "collect_pinduoduo_retry_on_blocked"); ok {
		p.RetryOnBlocked = v
	}
	if v, ok := settingsBool(m, "collect_pinduoduo_retry_on_timeout"); ok {
		p.RetryOnTimeout = v
	}
	if v := settingsInt(m, "collect_pinduoduo_batch_max_retries"); v > 0 {
		p.MaxRetries = v
	}
	return p
}

func isPinduoduoCollectSource(source string) bool {
	src := strings.TrimSpace(strings.ToLower(source))
	return src == "pinduoduo" || src == "pdd"
}
