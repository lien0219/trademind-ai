package openaicompat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const providerName = "openai_compatible"

// Message is one chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Request is a chat/completions payload.
type Request struct {
	Model          string
	Messages       []Message
	Temperature    float64
	MaxTokens      int
	ResponseFormat string // e.g. json_object
}

// Result is a normalized completion outcome.
type Result struct {
	Content      string
	Raw          []byte
	Model        string
	InputTokens  int
	OutputTokens int
}

// Client calls an OpenAI-compatible POST /chat/completions endpoint.
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// Name identifies this adapter.
func (c *Client) Name() string { return providerName }

// Chat calls the remote API.
func (c *Client) Chat(ctx context.Context, req Request) (*Result, error) {
	if c == nil {
		return nil, fmt.Errorf("openai_compatible: nil client")
	}
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if base == "" {
		return nil, fmt.Errorf("openai_compatible: base_url empty")
	}
	if strings.TrimSpace(c.APIKey) == "" {
		return nil, fmt.Errorf("openai_compatible: api_key empty")
	}
	if len(req.Messages) == 0 {
		return nil, fmt.Errorf("openai_compatible: no messages")
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
		return nil, fmt.Errorf("openai_compatible: request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := readBodyLimit(resp.Body, 4<<20)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("openai_compatible: HTTP %d", resp.StatusCode)
	}

	var envelope struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
		Model string `json:"model"`
	}
	if err := json.Unmarshal(respBytes, &envelope); err != nil {
		return nil, fmt.Errorf("openai_compatible: decode: %w", err)
	}
	content := ""
	if len(envelope.Choices) > 0 {
		content = strings.TrimSpace(envelope.Choices[0].Message.Content)
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
