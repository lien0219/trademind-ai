package compatclient

import "testing"

func TestExtractChoiceContent_StringContent(t *testing.T) {
	raw := []byte(`{"message":{"content":"{\"optimizedTitle\":\"Hi\"}"}}`)
	got := extractChoiceContent(raw)
	if got == "" {
		t.Fatal("expected content")
	}
}

func TestExtractChoiceContent_ReasoningJSONFallback(t *testing.T) {
	raw := []byte(`{"message":{"content":"","reasoning_content":"{\"optimizedTitle\":\"FromReasoning\"}"}}`)
	got := extractChoiceContent(raw)
	if got != `{"optimizedTitle":"FromReasoning"}` {
		t.Fatalf("got %q", got)
	}
}

func TestExtractChoiceContent_ReasoningProseIgnored(t *testing.T) {
	raw := []byte(`{"message":{"content":"","reasoning_content":"We are asked to optimize a title"}}`)
	got := extractChoiceContent(raw)
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestExtractChoiceContent_PartsArray(t *testing.T) {
	raw := []byte(`{"message":{"content":[{"type":"text","text":"hello"}]}}`)
	got := extractChoiceContent(raw)
	if got != "hello" {
		t.Fatalf("got %q", got)
	}
}
