package collect

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

// CollectorClient calls the Node collector HTTP API with strict timeouts.
type CollectorClient struct {
	BaseURL string
	Client  *http.Client
}

// NewCollectorClient builds an HTTP client using baseURL (no trailing slash) and timeout.
func NewCollectorClient(baseURL string, timeout time.Duration) *CollectorClient {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &CollectorClient{
		BaseURL: baseURL,
		Client: &http.Client{
			Timeout: timeout,
		},
	}
}

// CollectorRejectedError is returned when collector responds with ok=false (e.g. HTTP 422).
type CollectorRejectedError struct {
	Code    string
	Message string
}

func (e *CollectorRejectedError) Error() string {
	if e == nil {
		return "collector rejected"
	}
	return fmt.Sprintf("collector rejected: %s (%s)", e.Message, e.Code)
}

type collectEnvelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type collectDataProduct struct {
	Product json.RawMessage `json:"product"`
}

// CollectOutcome holds parsed normalized JSON returned by the collector.
type CollectOutcome struct {
	ProductJSON json.RawMessage
}

// Collect invokes POST /v1/collect and returns normalized product JSON on success.
func (c *CollectorClient) Collect(ctx context.Context, source, rawURL string, options map[string]any) (*CollectOutcome, error) {
	if c == nil || c.Client == nil {
		return nil, fmt.Errorf("collector client unavailable")
	}
	if c.BaseURL == "" {
		return nil, fmt.Errorf("collector base url is empty")
	}

	body := map[string]any{
		"source": strings.TrimSpace(source),
		"url":    strings.TrimSpace(rawURL),
	}
	if len(options) > 0 {
		body["options"] = options
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/collect", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("collector request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("collector read body: %w", err)
	}

	var env collectEnvelope
	if err := json.Unmarshal(respBody, &env); err != nil {
		return nil, fmt.Errorf("collector invalid json (http %d): %w", resp.StatusCode, err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		if !env.OK || env.Error != nil {
			msg := "collector returned error"
			code := "UNKNOWN"
			if env.Error != nil {
				if env.Error.Message != "" {
					msg = env.Error.Message
				}
				if env.Error.Code != "" {
					code = env.Error.Code
				}
			}
			return nil, &CollectorRejectedError{Code: code, Message: msg}
		}
		var wrap collectDataProduct
		if err := json.Unmarshal(env.Data, &wrap); err != nil {
			return nil, fmt.Errorf("collector parse data: %w", err)
		}
		if len(wrap.Product) == 0 {
			return nil, errors.New("collector returned empty product")
		}
		return &CollectOutcome{ProductJSON: wrap.Product}, nil

	case http.StatusUnprocessableEntity:
		if env.Error != nil {
			return nil, &CollectorRejectedError{
				Code:    env.Error.Code,
				Message: env.Error.Message,
			}
		}
		return nil, &CollectorRejectedError{Code: "UNPROCESSABLE", Message: string(respBody)}

	default:
		if env.Error != nil && env.Error.Message != "" {
			return nil, fmt.Errorf("collector http %d: %s", resp.StatusCode, env.Error.Message)
		}
		return nil, fmt.Errorf("collector http %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
}

// FetchProviders calls GET /v1/providers with a short timeout (independent of Collect timeout).
func (c *CollectorClient) FetchProviders(parent context.Context) ([]CollectProviderDTO, error) {
	if c == nil || strings.TrimSpace(c.BaseURL) == "" {
		return nil, fmt.Errorf("collector client unavailable")
	}
	ctx := parent
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/v1/providers", nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var env collectEnvelope
	if err := json.Unmarshal(respBody, &env); err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK || !env.OK {
		return nil, fmt.Errorf("collector providers http %d", resp.StatusCode)
	}
	var list []CollectProviderDTO
	if err := json.Unmarshal(env.Data, &list); err != nil {
		return nil, err
	}
	return list, nil
}

// ProbeHealth GET /health with a short timeout for observability (do not use for server-wide /health).
func (c *CollectorClient) ProbeHealth(parent context.Context) (reachable bool, message string) {
	if c == nil || strings.TrimSpace(c.BaseURL) == "" {
		return false, "collector client unavailable"
	}
	ctx := parent
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/health", nil)
	if err != nil {
		return false, err.Error()
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, err.Error()
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return true, "ok"
	}
	return false, fmt.Sprintf("collector http %d", resp.StatusCode)
}
