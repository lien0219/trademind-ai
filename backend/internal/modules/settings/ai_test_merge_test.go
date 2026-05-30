package settings

import (
	"testing"

	aigate "github.com/trademind-ai/trademind/backend/internal/providers/ai"
)

func TestMergeAIPlainProviderSpecificKeys(t *testing.T) {
	stored := map[string]string{
		"provider":         "deepseek",
		"deepseek_api_key": "stored-deepseek-key",
		"openai_api_key":   "stored-openai-key",
	}
	merged := MergeAIPlain(stored, &TestAIOverrides{
		Provider: "openai",
		APIKey:   "sk-****abcd",
		BaseURL:  "https://api.openai.com/v1",
		Model:    "gpt-4o-mini",
	})
	if merged["openai_api_key"] != "stored-openai-key" {
		t.Fatalf("masked key should not overwrite stored openai key, got %q", merged["openai_api_key"])
	}
	if merged[aigate.ProviderBaseURLKey("openai")] != "https://api.openai.com/v1" {
		t.Fatalf("expected openai base url override")
	}
	if merged[aigate.ProviderModelKey("openai")] != "gpt-4o-mini" {
		t.Fatalf("expected openai model override")
	}

	merged2 := MergeAIPlain(stored, &TestAIOverrides{
		Provider: "openai",
		APIKey:   "new-openai-key",
	})
	if merged2["openai_api_key"] != "new-openai-key" {
		t.Fatalf("expected new openai key, got %q", merged2["openai_api_key"])
	}
	if merged2["deepseek_api_key"] != "stored-deepseek-key" {
		t.Fatalf("deepseek key should remain untouched")
	}
}
