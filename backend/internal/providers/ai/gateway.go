package ai

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	openaicompat "github.com/trademind-ai/trademind/backend/internal/providers/ai/openai_compatible"
)

func toOpenAIReq(req ChatRequest) openaicompat.Request {
	msgs := make([]openaicompat.Message, 0, len(req.Messages))
	for _, m := range req.Messages {
		msgs = append(msgs, openaicompat.Message{Role: m.Role, Content: m.Content})
	}
	rf := ""
	if req.ResponseFormat != nil {
		rf = strings.TrimSpace(req.ResponseFormat.Type)
	}
	return openaicompat.Request{
		Model:          req.Model,
		Messages:       msgs,
		Temperature:    req.Temperature,
		MaxTokens:      req.MaxTokens,
		ResponseFormat: rf,
	}
}

// Gateway resolves settings.ai and dispatches to a concrete Provider.
// Business code must call the gateway only, never a Provider directly.
type Gateway struct {
	Settings *settings.Service
}

func normalizeProviderName(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	v = strings.ReplaceAll(v, "-", "_")
	return v
}

func (g *Gateway) httpTimeout(ctx context.Context, plain map[string]string) time.Duration {
	timeout := 120 * time.Second
	if plain == nil {
		return timeout
	}
	if sec := strings.TrimSpace(plain["timeout_sec"]); sec != "" {
		if n, err := strconv.Atoi(sec); err == nil && n > 0 && n <= 600 {
			timeout = time.Duration(n) * time.Second
		}
	}
	return timeout
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

	base := strings.TrimRight(strings.TrimSpace(plain["base_url"]), "/")
	apiKey := strings.TrimSpace(plain["api_key"])
	if base == "" {
		return nil, fmt.Errorf("ai: base_url not configured")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("ai: api_key not configured")
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = strings.TrimSpace(plain["model"])
	}
	if model == "" {
		model = "gpt-4o-mini"
	}

	temp := req.Temperature
	if temp == 0 && plain["temperature"] != "" {
		if f, err := strconv.ParseFloat(strings.TrimSpace(plain["temperature"]), 64); err == nil {
			temp = f
		}
	}
	if temp == 0 {
		temp = 0.7
	}

	maxTok := req.MaxTokens
	if maxTok == 0 && plain["max_tokens"] != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(plain["max_tokens"])); err == nil && n > 0 {
			maxTok = n
		}
	}
	if maxTok == 0 {
		maxTok = 512
	}

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

	cli := &openaicompat.Client{
		BaseURL:    base,
		APIKey:     apiKey,
		HTTPClient: httpClient,
	}
	switch pname {
	case "openai_compatible", "openai":
	default:
		return nil, fmt.Errorf("ai: unsupported provider %q (use openai_compatible)", strings.TrimSpace(plain["provider"]))
	}

	oreq := toOpenAIReq(merged)
	res, err := cli.Chat(callCtx, oreq)
	if err != nil {
		return nil, err
	}
	if res.Model == "" {
		res.Model = merged.Model
	}
	return &ChatResponse{
		Content:      res.Content,
		Model:        res.Model,
		Raw:          res.Raw,
		InputTokens:  res.InputTokens,
		OutputTokens: res.OutputTokens,
	}, nil
}
