package compatclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client calls an OpenAI-compatible POST /chat/completions endpoint.
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// Chat performs a chat completion request.
func (c *Client) Chat(ctx context.Context, req Request) (*Result, error) {
	if c == nil {
		return nil, fmt.Errorf("compatclient: nil client")
	}
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if base == "" {
		return nil, fmt.Errorf("compatclient: base_url empty")
	}
	if strings.TrimSpace(c.APIKey) == "" {
		return nil, fmt.Errorf("compatclient: api_key empty")
	}
	if len(req.Messages) == 0 {
		return nil, fmt.Errorf("compatclient: no messages")
	}
	if strings.TrimSpace(req.Model) == "" {
		return nil, fmt.Errorf("compatclient: model empty")
	}

	payload := map[string]any{
		"model":       req.Model,
		"messages":    req.Messages,
		"temperature": req.Temperature,
	}
	if req.MaxTokens > 0 {
		payload["max_tokens"] = req.MaxTokens
	}
	if strings.TrimSpace(req.ResponseFormat) != "" {
		payload["response_format"] = map[string]string{"type": req.ResponseFormat}
	}
	// DeepSeek v4: thinking must be a top-level field (not OpenAI SDK extra_body).
	if req.DisableThinking {
		payload["thinking"] = map[string]string{"type": "disabled"}
	}

	rawBody, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/chat/completions", bytes.NewReader(rawBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.APIKey))

	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 120 * time.Second}
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := readBodyLimit(resp.Body, 4<<20)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &HTTPError{StatusCode: resp.StatusCode, Body: respBytes}
	}

	var envelope struct {
		Choices []json.RawMessage `json:"choices"`
		Usage   struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
		Model string `json:"model"`
	}
	if err := json.Unmarshal(respBytes, &envelope); err != nil {
		return nil, fmt.Errorf("compatclient: decode: %w", err)
	}
	content := ""
	if len(envelope.Choices) > 0 {
		content = strings.TrimSpace(extractChoiceContent(envelope.Choices[0]))
	}
	return &Result{
		Content:      content,
		Model:        strings.TrimSpace(envelope.Model),
		Raw:          sanitizeRawResponse(respBytes),
		InputTokens:  envelope.Usage.PromptTokens,
		OutputTokens: envelope.Usage.CompletionTokens,
	}, nil
}

func readBodyLimit(r io.Reader, max int64) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, max))
}

func sanitizeRawResponse(raw []byte) []byte {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return raw
	}
	delete(m, "system_fingerprint")
	out, err := json.Marshal(m)
	if err != nil {
		return raw
	}
	return out
}

// AsHTTPError returns *HTTPError when err wraps a non-2xx API response.
func AsHTTPError(err error) (*HTTPError, bool) {
	var he *HTTPError
	if errors.As(err, &he) {
		return he, true
	}
	return nil, false
}
