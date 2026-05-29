package ai

import "testing"

func TestResolveVisionModel(t *testing.T) {
	if got := resolveVisionModel("qwen", "", "", "qwen3-vl-plus", ""); got != "qwen3-vl-plus" {
		t.Fatalf("qwen configured vl model: got %q", got)
	}
	if got := resolveVisionModel("qwen", "", "", "qwen-plus", ""); got != "qwen-vl-plus" {
		t.Fatalf("qwen text model fallback: got %q", got)
	}
	if got := resolveVisionModel("qwen", "", "", "", "qwen-vl-max"); got != "qwen-vl-max" {
		t.Fatalf("explicit vision_model: got %q", got)
	}
	if got := resolveVisionModel("openai", "", "", "gpt-4o", ""); got != "gpt-4o" {
		t.Fatalf("openai configured vision: got %q", got)
	}
	if got := resolveVisionModel("openai", "", "", "gpt-3.5-turbo", ""); got != "gpt-4o-mini" {
		t.Fatalf("openai default vision: got %q", got)
	}
}
