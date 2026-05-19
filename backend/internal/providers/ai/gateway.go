package ai

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// SettingsReader loads decrypted settings groups (implemented by settings.Service).
type SettingsReader interface {
	PlainByGroup(ctx context.Context, tenantID int64, groupKey string) (map[string]string, error)
}

// Gateway resolves settings.ai and dispatches to a concrete Provider.
// Business code must call the gateway only, never a Provider directly.
type Gateway struct {
	Settings SettingsReader
}

// ConnectionTestResult is returned by TestConnection.
type ConnectionTestResult struct {
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	LatencyMs int64  `json:"latencyMs"`
	OK        bool   `json:"ok"`
	Message   string `json:"message"`
}

func (g *Gateway) httpTimeout(ctx context.Context, plain map[string]string) time.Duration {
	timeout := 120 * time.Second
	if plain == nil {
		return timeout
	}
	if sec := strings.TrimSpace(plain["timeout_sec"]); sec != "" {
		if n, err := parseTimeoutSec(sec); err == nil {
			timeout = n
		}
	}
	return timeout
}

func parseTimeoutSec(sec string) (time.Duration, error) {
	n, err := strconv.Atoi(strings.TrimSpace(sec))
	if err != nil || n <= 0 || n > 600 {
		return 0, fmt.Errorf("invalid timeout")
	}
	return time.Duration(n) * time.Second, nil
}

// Chat merges settings defaults with req, picks a Provider, and runs Chat with a bounded context timeout.
func (g *Gateway) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if g == nil || g.Settings == nil {
		return nil, fmt.Errorf("ai gateway: not configured")
	}
	plain, err := g.Settings.PlainByGroup(ctx, 0, "ai")
	if err != nil {
		return nil, err
	}
	pname := normalizeProviderName(plain["provider"])
	if pname == "" {
		pname = "openai_compatible"
	}

	base := resolveBaseURL(pname, plain["base_url"])
	if base == "" {
		return nil, fmt.Errorf("请配置 base_url")
	}
	apiKey := strings.TrimSpace(plain["api_key"])
	if apiKey == "" {
		return nil, fmt.Errorf("请配置 API Key")
	}

	model := resolveModel(pname, req.Model, plain["model"])
	temp, maxTok := mergeChatParams(plain, req)

	merged := ChatRequest{
		Model:          model,
		Messages:       req.Messages,
		Temperature:    temp,
		MaxTokens:      maxTok,
		ResponseFormat: req.ResponseFormat,
	}

	timeout := g.httpTimeout(ctx, plain)
	callCtx := ctx
	var cancel context.CancelFunc
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		callCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	httpTimeout := timeout
	if httpTimeout < 30*time.Second {
		httpTimeout = 30 * time.Second
	}
	httpClient := &http.Client{Timeout: httpTimeout}

	prov, err := NewProvider(pname, base, apiKey, httpClient)
	if err != nil {
		return nil, err
	}
	return prov.Chat(callCtx, merged)
}

// TestConnection sends a minimal chat request through the gateway.
func (g *Gateway) TestConnection(ctx context.Context) (*ConnectionTestResult, error) {
	if g == nil || g.Settings == nil {
		return nil, fmt.Errorf("ai gateway: not configured")
	}
	plain, err := g.Settings.PlainByGroup(ctx, 0, "ai")
	if err != nil {
		return nil, err
	}
	pname := normalizeProviderName(plain["provider"])
	if pname == "" {
		pname = "openai_compatible"
	}
	model := resolveModel(pname, "", plain["model"])

	start := time.Now()
	_, err = g.Chat(ctx, ChatRequest{
		Messages:    []Message{{Role: "user", Content: "ping"}},
		MaxTokens:   1,
		Temperature: 0,
		Model:       model,
	})
	latency := time.Since(start).Milliseconds()

	res := &ConnectionTestResult{
		Provider:  pname,
		Model:     model,
		LatencyMs: latency,
	}
	if err != nil {
		res.OK = false
		res.Message = err.Error()
		return res, err
	}
	res.OK = true
	res.Message = "连接成功"
	return res, nil
}
