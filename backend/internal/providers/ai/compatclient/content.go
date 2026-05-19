package compatclient

import (
	"encoding/json"
	"strings"
)

type flexibleMessage struct {
	Content          json.RawMessage `json:"content"`
	ReasoningContent string          `json:"reasoning_content"`
}

func extractChoiceContent(choiceJSON []byte) string {
	if len(choiceJSON) == 0 {
		return ""
	}
	var choice struct {
		Message flexibleMessage `json:"message"`
		Text    string          `json:"text"`
	}
	if err := json.Unmarshal(choiceJSON, &choice); err != nil {
		return ""
	}
	if c := messageText(choice.Message); c != "" {
		return c
	}
	return strings.TrimSpace(choice.Text)
}

func messageText(m flexibleMessage) string {
	if c := parseContentField(m.Content); c != "" {
		return c
	}
	// Reasoning is chain-of-thought prose; only treat as output when it is JSON.
	r := strings.TrimSpace(m.ReasoningContent)
	if r != "" && looksLikeJSONPayload(r) {
		return r
	}
	return ""
}

func looksLikeJSONPayload(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[")
}

func parseContentField(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s)
	}
	var parts []struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err == nil {
		var b strings.Builder
		for _, p := range parts {
			if t := strings.TrimSpace(p.Text); t != "" {
				b.WriteString(t)
			}
		}
		return strings.TrimSpace(b.String())
	}
	return strings.TrimSpace(string(raw))
}
