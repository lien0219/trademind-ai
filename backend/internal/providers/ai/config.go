package ai

import (
	"strconv"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/providers/ai/compatclient"
)

const (
	defaultOpenAIModel     = "gpt-4o-mini"
	defaultDeepSeekModel   = "deepseek-chat"
	defaultQwenModel       = "qwen-plus"
	defaultOpenAIBaseURL   = "https://api.openai.com/v1"
	defaultDeepSeekBaseURL = "https://api.deepseek.com/v1"
	defaultQwenBaseURL     = "https://dashscope.aliyuncs.com/compatible-mode/v1"
)

func normalizeProviderName(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	v = strings.ReplaceAll(v, "-", "_")
	return v
}

func defaultBaseURL(provider string) string {
	switch provider {
	case "openai":
		return defaultOpenAIBaseURL
	case "deepseek":
		return defaultDeepSeekBaseURL
	case "qwen":
		return defaultQwenBaseURL
	default:
		return ""
	}
}

func defaultModel(provider string) string {
	switch provider {
	case "deepseek":
		return defaultDeepSeekModel
	case "qwen":
		return defaultQwenModel
	case "openai", "openai_compatible":
		return defaultOpenAIModel
	default:
		return defaultOpenAIModel
	}
}

func resolveBaseURL(provider, configured string) string {
	base := strings.TrimRight(strings.TrimSpace(configured), "/")
	if base != "" {
		return base
	}
	return strings.TrimRight(defaultBaseURL(provider), "/")
}

func resolveModel(provider, reqModel, configured string) string {
	if m := strings.TrimSpace(reqModel); m != "" {
		return m
	}
	if m := strings.TrimSpace(configured); m != "" {
		return m
	}
	return defaultModel(provider)
}

func mergeChatParams(plain map[string]string, req ChatRequest) (temp float64, maxTok int) {
	temp = req.Temperature
	if temp == 0 && plain != nil && plain["temperature"] != "" {
		if f, err := strconv.ParseFloat(strings.TrimSpace(plain["temperature"]), 64); err == nil {
			temp = f
		}
	}
	if temp == 0 {
		temp = 0.7
	}

	maxTok = req.MaxTokens
	if maxTok == 0 && plain != nil && plain["max_tokens"] != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(plain["max_tokens"])); err == nil && n > 0 {
			maxTok = n
		}
	}
	if maxTok == 0 {
		maxTok = 512
	}
	return temp, maxTok
}

func resolveVisionModel(pname, reqVision, reqModel, configured, plainVision string) string {
	if m := strings.TrimSpace(reqVision); m != "" {
		return m
	}
	if m := strings.TrimSpace(plainVision); m != "" {
		return m
	}
	switch pname {
	case "qwen":
		return "qwen-vl-plus"
	case "openai", "openai_compatible":
		if m := strings.TrimSpace(configured); m != "" && strings.Contains(strings.ToLower(m), "gpt-4") {
			return m
		}
		return "gpt-4o-mini"
	default:
		if m := strings.TrimSpace(reqModel); m != "" {
			return m
		}
		return resolveModel(pname, "", configured)
	}
}

func messagesHaveVision(msgs []Message) bool {
	for _, m := range msgs {
		if len(m.ImageURLs) > 0 {
			return true
		}
	}
	return false
}

func toCompatMessages(msgs []Message) []compatclient.Message {
	out := make([]compatclient.Message, 0, len(msgs))
	for _, m := range msgs {
		content := any(strings.TrimSpace(m.Content))
		if len(m.ImageURLs) > 0 {
			parts := []compatclient.ContentPart{{Type: "text", Text: strings.TrimSpace(m.Content)}}
			for _, u := range m.ImageURLs {
				u = strings.TrimSpace(u)
				if u == "" {
					continue
				}
				parts = append(parts, compatclient.ContentPart{
					Type:     "image_url",
					ImageURL: &compatclient.ImageURLDetail{URL: u},
				})
			}
			content = parts
		}
		out = append(out, compatclient.Message{Role: m.Role, Content: content})
	}
	return out
}

func responseFormatType(req ChatRequest) string {
	if req.ResponseFormat == nil {
		return ""
	}
	return strings.TrimSpace(req.ResponseFormat.Type)
}
