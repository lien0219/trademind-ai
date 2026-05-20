package collect

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	Code         string
	Message      string
	AccessReport json.RawMessage `json:"-"`
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
// CustomRuleTestResult is the structured preview from POST /v1/collect/custom-rule-test.
type CustomRuleTestResult struct {
	AccessStatus    string          `json:"accessStatus"`
	FinalURL        string          `json:"finalUrl"`
	HTTPStatus      int             `json:"httpStatus,omitempty"`
	ExtractedFields json.RawMessage `json:"extractedFields"`
	MissingFields   []string        `json:"missingFields"`
	Warnings        []string        `json:"warnings"`
	ErrorCode       string          `json:"errorCode,omitempty"`
	Suggestion      string          `json:"suggestion"`
	Product         json.RawMessage `json:"product,omitempty"`
}

// AnalyzePageDigest is a safe page structure summary from POST /v1/custom/analyze-page.
type AnalyzePageDigest struct {
	URL          string          `json:"url"`
	FinalURL     string          `json:"finalUrl"`
	AccessStatus string          `json:"accessStatus"`
	Title        string          `json:"title"`
	Meta         json.RawMessage `json:"meta"`
	Candidates   json.RawMessage `json:"candidates"`
	DomHints     []string        `json:"domHints"`
}

// AnalyzePage invokes POST /v1/custom/analyze-page (no full HTML).
func (c *CollectorClient) AnalyzePage(ctx context.Context, rawURL string, options map[string]any) (*AnalyzePageDigest, error) {
	if c == nil || c.Client == nil {
		return nil, fmt.Errorf("collector client unavailable")
	}
	if c.BaseURL == "" {
		return nil, fmt.Errorf("collector base url is empty")
	}
	body := map[string]any{"url": strings.TrimSpace(rawURL)}
	for k, v := range options {
		body[k] = v
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/custom/analyze-page", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("collector request: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	var env collectEnvelope
	if err := json.Unmarshal(respBody, &env); err != nil {
		return nil, fmt.Errorf("collector invalid json: %w", err)
	}
	if resp.StatusCode != http.StatusOK || !env.OK {
		msg := "analyze page failed"
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
	var out AnalyzePageDigest
	if err := json.Unmarshal(env.Data, &out); err != nil {
		return nil, fmt.Errorf("parse analyze page: %w", err)
	}
	return &out, nil
}

// CustomRuleTest invokes POST /v1/collect/custom-rule-test (no collect_tasks / products).
func (c *CollectorClient) CustomRuleTest(ctx context.Context, rawURL string, options map[string]any) (*CustomRuleTestResult, error) {
	if c == nil || c.Client == nil {
		return nil, fmt.Errorf("collector client unavailable")
	}
	if c.BaseURL == "" {
		return nil, fmt.Errorf("collector base url is empty")
	}
	body := map[string]any{"url": strings.TrimSpace(rawURL), "options": options}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/collect/custom-rule-test", bytes.NewReader(payload))
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
		return nil, err
	}
	var env collectEnvelope
	if err := json.Unmarshal(respBody, &env); err != nil {
		return nil, fmt.Errorf("collector invalid json: %w", err)
	}
	if resp.StatusCode != http.StatusOK || !env.OK {
		msg := "custom rule test failed"
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
	var out CustomRuleTestResult
	if err := json.Unmarshal(env.Data, &out); err != nil {
		return nil, fmt.Errorf("parse custom rule test: %w", err)
	}
	return &out, nil
}

// ProfileAccessDTO mirrors collector POST /v1/browser-profiles/:key/check.
type ProfileAccessDTO struct {
	AccessStatus string `json:"accessStatus"`
	FinalURL     string `json:"finalUrl"`
	HTTPStatus   int    `json:"httpStatus,omitempty"`
	ErrorCode    string `json:"errorCode,omitempty"`
	Message      string `json:"message"`
}

type profileOpenLoginData struct {
	Message     string `json:"message"`
	ProfilePath string `json:"profilePath"`
	AlreadyOpen bool   `json:"alreadyOpen"`
}

// OpenBrowserProfileLogin opens headed persistent browser for manual login (custom profiles).
func (c *CollectorClient) OpenBrowserProfileLogin(ctx context.Context, profileKey, rawURL string) (string, error) {
	if c == nil || c.Client == nil {
		return "", fmt.Errorf("collector client unavailable")
	}
	profileKey = strings.TrimSpace(profileKey)
	rawURL = strings.TrimSpace(rawURL)
	if profileKey == "" || rawURL == "" {
		return "", fmt.Errorf("profileKey and url required")
	}
	body, _ := json.Marshal(map[string]string{"url": rawURL})
	path := fmt.Sprintf("%s/v1/browser-profiles/%s/open-login", c.BaseURL, url.PathEscape(profileKey))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, path, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("collector request: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}
	var env collectEnvelope
	if err := json.Unmarshal(respBody, &env); err != nil {
		return "", fmt.Errorf("collector invalid json: %w", err)
	}
	if resp.StatusCode != http.StatusOK || !env.OK {
		code, msg := "UNKNOWN", "open login failed"
		if env.Error != nil {
			if env.Error.Code != "" {
				code = env.Error.Code
			}
			if env.Error.Message != "" {
				msg = env.Error.Message
			}
		}
		return "", &CollectorRejectedError{Code: code, Message: msg}
	}
	var data profileOpenLoginData
	_ = json.Unmarshal(env.Data, &data)
	if data.Message != "" {
		return data.Message, nil
	}
	return "采集浏览器已打开", nil
}

// CheckBrowserProfileAccess uses profile persistent context to probe URL access.
func (c *CollectorClient) CheckBrowserProfileAccess(ctx context.Context, profileKey, rawURL string) (*ProfileAccessDTO, error) {
	if c == nil || c.Client == nil {
		return nil, fmt.Errorf("collector client unavailable")
	}
	profileKey = strings.TrimSpace(profileKey)
	rawURL = strings.TrimSpace(rawURL)
	body, _ := json.Marshal(map[string]string{"url": rawURL})
	path := fmt.Sprintf("%s/v1/browser-profiles/%s/check", c.BaseURL, url.PathEscape(profileKey))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("collector request: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	var env collectEnvelope
	if err := json.Unmarshal(respBody, &env); err != nil {
		return nil, fmt.Errorf("collector invalid json: %w", err)
	}
	if resp.StatusCode != http.StatusOK || !env.OK {
		code, msg := "UNKNOWN", "profile check failed"
		if env.Error != nil {
			if env.Error.Code != "" {
				code = env.Error.Code
			}
			if env.Error.Message != "" {
				msg = env.Error.Message
			}
		}
		return nil, &CollectorRejectedError{Code: code, Message: msg}
	}
	var out ProfileAccessDTO
	if err := json.Unmarshal(env.Data, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

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
		rej := &CollectorRejectedError{Code: "UNPROCESSABLE", Message: string(respBody)}
		if env.Error != nil {
			rej.Code = env.Error.Code
			rej.Message = env.Error.Message
		}
		if len(env.Data) > 0 {
			rej.AccessReport = env.Data
		}
		return nil, rej

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

func (c *CollectorClient) decodeDataEnvelope(parent context.Context, method, path string, timeout time.Duration) (json.RawMessage, error) {
	if c == nil || strings.TrimSpace(c.BaseURL) == "" {
		return nil, fmt.Errorf("collector client unavailable")
	}
	ctx := parent
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("collector request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("collector read body: %w", err)
	}
	var env collectEnvelope
	if err := json.Unmarshal(respBody, &env); err != nil {
		return nil, fmt.Errorf("collector invalid json (http %d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK || !env.OK {
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
	return env.Data, nil
}

// Get1688AuthStatus calls GET /v1/providers/1688/auth-status.
func (c *CollectorClient) Get1688AuthStatus(parent context.Context) (*Provider1688AuthStatusDTO, error) {
	raw, err := c.decodeDataEnvelope(parent, http.MethodGet, "/v1/providers/1688/auth-status", 90*time.Second)
	if err != nil {
		return nil, err
	}
	var out Provider1688AuthStatusDTO
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("collector parse auth status: %w", err)
	}
	return &out, nil
}

// Open1688LoginBrowser calls POST /v1/providers/1688/open-login-browser.
func (c *CollectorClient) Open1688LoginBrowser(parent context.Context) (*Provider1688OpenLoginResultDTO, error) {
	raw, err := c.decodeDataEnvelope(parent, http.MethodPost, "/v1/providers/1688/open-login-browser", 60*time.Second)
	if err != nil {
		return nil, err
	}
	var out Provider1688OpenLoginResultDTO
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("collector parse open login: %w", err)
	}
	return &out, nil
}
