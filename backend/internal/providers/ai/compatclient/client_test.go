package compatclient

import (
	"encoding/json"
	"testing"
)

func TestChatPayload_ThinkingDisabledAtRoot(t *testing.T) {
	req := Request{
		Model:           "deepseek-v4-pro",
		Messages:        []Message{{Role: "user", Content: "hi"}},
		DisableThinking: true,
		ResponseFormat:  "json_object",
	}
	payload := map[string]any{
		"model":    req.Model,
		"messages": req.Messages,
	}
	if req.DisableThinking {
		payload["thinking"] = map[string]string{"type": "disabled"}
	}
	if req.ResponseFormat != "" {
		payload["response_format"] = map[string]string{"type": req.ResponseFormat}
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["thinking"]; !ok {
		t.Fatalf("missing top-level thinking: %s", string(b))
	}
	if _, ok := m["extra_body"]; ok {
		t.Fatalf("should not nest thinking in extra_body: %s", string(b))
	}
}
