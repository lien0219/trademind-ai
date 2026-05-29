package ai

import "testing"

func TestResolveProviderAPIKey(t *testing.T) {
	plain := map[string]string{
		"provider":         "deepseek",
		"api_key":          "legacy-key",
		"deepseek_api_key": "deepseek-key",
		"openai_api_key":   "openai-key",
	}
	if got := ResolveProviderAPIKey(plain, "deepseek"); got != "deepseek-key" {
		t.Fatalf("deepseek: got %q", got)
	}
	if got := ResolveProviderAPIKey(plain, "openai"); got != "openai-key" {
		t.Fatalf("openai: got %q", got)
	}
	if got := ResolveProviderAPIKey(plain, "qwen"); got != "legacy-key" {
		t.Fatalf("qwen fallback: got %q", got)
	}
}

func TestResolveProviderBaseURL(t *testing.T) {
	plain := map[string]string{
		"deepseek_base_url": "https://api.deepseek.com/v1",
		"base_url":          "https://legacy.example/v1",
	}
	if got := ResolveProviderBaseURL(plain, "deepseek"); got != "https://api.deepseek.com/v1" {
		t.Fatalf("provider-specific: got %q", got)
	}
	if got := ResolveProviderBaseURL(plain, "openai"); got != "https://api.openai.com/v1" {
		t.Fatalf("default openai: got %q", got)
	}
}

func TestResolveProviderModel(t *testing.T) {
	plain := map[string]string{
		"deepseek_model": "deepseek-chat",
		"model":          "legacy-model",
	}
	if got := ResolveProviderModel(plain, "deepseek", ""); got != "deepseek-chat" {
		t.Fatalf("provider-specific: got %q", got)
	}
	if got := ResolveProviderModel(plain, "qwen", ""); got != "qwen-plus" {
		t.Fatalf("default qwen: got %q", got)
	}
	if got := ResolveProviderModel(plain, "deepseek", "override"); got != "override" {
		t.Fatalf("request override: got %q", got)
	}
}
