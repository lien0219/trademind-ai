package imagetask

import "testing"

func TestMergeImagePlain_UsesStoredSecretWhenMasked(t *testing.T) {
	stored := map[string]string{
		"provider":                "dashscope_image",
		"dashscope_image_api_key": "real-secret-key",
		"dashscope_image_model":   "wan2.7-image-pro",
	}
	merged := MergeImagePlain(stored, map[string]string{
		"dashscope_image_api_key": "sk-****abcd",
		"dashscope_image_model":   "qwen-image-v2.0",
	})
	if merged["dashscope_image_api_key"] != "real-secret-key" {
		t.Fatalf("expected stored api key, got %q", merged["dashscope_image_api_key"])
	}
	if merged["dashscope_image_model"] != "qwen-image-v2.0" {
		t.Fatalf("expected overridden model, got %q", merged["dashscope_image_model"])
	}
}

func TestMergeImagePlain_UsesNewSecretWhenProvided(t *testing.T) {
	stored := map[string]string{
		"dashscope_image_api_key": "old-secret",
	}
	merged := MergeImagePlain(stored, map[string]string{
		"dashscope_image_api_key": "new-secret-key",
	})
	if merged["dashscope_image_api_key"] != "new-secret-key" {
		t.Fatalf("expected new api key, got %q", merged["dashscope_image_api_key"])
	}
}
