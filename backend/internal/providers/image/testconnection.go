package image

import (
	"context"
	"strings"
	"time"
)

// TestConnectionResult is returned by POST /settings/test-image.
type TestConnectionResult struct {
	Provider       string   `json:"provider"`
	OK             bool     `json:"ok"`
	Message        string   `json:"message"`
	LatencyMs      int64    `json:"latencyMs"`
	SupportedTasks []string `json:"supportedTasks"`
	ConfigStatus   string   `json:"configStatus"`
	TestMode       string   `json:"testMode,omitempty"`
}

// TestConnection validates image provider settings from decrypted settings.image map.
func TestConnection(ctx context.Context, imageSettings map[string]string, providerName, testMode string) *TestConnectionResult {
	p := strings.TrimSpace(strings.ToLower(providerName))
	if p == "" && imageSettings != nil {
		p = strings.TrimSpace(strings.ToLower(imageSettings["provider"]))
	}
	if p == "" {
		p = "noop"
	}
	cap := CapabilityByProvider(p)
	res := &TestConnectionResult{
		Provider: p,
		TestMode: strings.TrimSpace(strings.ToLower(testMode)),
	}
	if res.TestMode == "" {
		res.TestMode = "config_only"
	}
	if cap != nil {
		res.SupportedTasks = cap.SupportedTasks
	}
	m := imageSettings
	if m == nil {
		m = map[string]string{}
	}
	res.ConfigStatus = ConfigStatus(p, m)
	start := time.Now()
	if cap != nil && cap.Status == ProviderStatusPlanned {
		res.OK = false
		res.Message = "该 Provider 为预留项，后续版本开放"
		res.LatencyMs = time.Since(start).Milliseconds()
		return res
	}
	if err := ValidateSettingsForProvider(p, m); err != nil {
		res.OK = false
		res.Message = err.Error()
		res.LatencyMs = time.Since(start).Milliseconds()
		return res
	}
	if res.TestMode == "live" && p != "noop" {
		res.Message = "配置可用（live 模式仅做配置校验，未实际生成图片，避免产生费用；请在商品图任务中试跑）"
	} else {
		res.Message = "配置检查通过"
	}
	res.OK = true
	res.LatencyMs = time.Since(start).Milliseconds()
	_ = ctx
	return res
}
