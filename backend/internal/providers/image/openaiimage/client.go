// Package openaiimage is an HTTP client for OpenAI Images (generations endpoint).
package openaiimage

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Options configure generations requests.
type Options struct {
	BaseURL string
	APIKey  string
	Model   string

	Size string

	// Quality: dall-e-3 uses standard|hd; gpt-image-1 prefers low|medium|high (normalized here).
	Quality string

	// Background: transparent|opaque|auto for gpt-style models when configured.
	Background string

	Timeout time.Duration
}

// Client wraps an OpenAI-compatible images HTTP client (no coupling to TradeMind Provider types).
type Client struct {
	opt        resolvedOptions
	httpClient *http.Client
}

type resolvedOptions struct {
	baseURL    string
	apiKey     string
	model      string
	size       string
	quality    string
	background string
}

func normalizeBaseURL(u string) string {
	s := strings.TrimSpace(strings.TrimRight(u, "/"))
	if s == "" {
		return "https://api.openai.com/v1"
	}
	return s
}

// NewClient builds a Client. API key must be non-empty plaintext (already decrypted upstream).
func NewClient(opt Options) (Client, error) {
	apiKey := strings.TrimSpace(opt.APIKey)
	if apiKey == "" {
		return Client{}, fmt.Errorf("openai_image_api_key is not configured")
	}
	sec := opt.Timeout
	if sec <= 0 {
		sec = 60 * time.Second
	}
	model := strings.TrimSpace(opt.Model)
	if model == "" {
		model = "gpt-image-1"
	}
	size := strings.TrimSpace(opt.Size)
	if size == "" {
		size = "1024x1024"
	}
	quality := strings.TrimSpace(opt.Quality)
	if quality == "" {
		quality = "standard"
	}

	return Client{
		opt: resolvedOptions{
			baseURL:    normalizeBaseURL(opt.BaseURL),
			apiKey:     apiKey,
			model:      model,
			size:       size,
			quality:    normalizeQuality(strings.ToLower(quality), model),
			background: strings.TrimSpace(opt.Background),
		},
		httpClient: &http.Client{Timeout: sec},
	}, nil
}

// ResolvedModel exposes the normalized model slug used for outbound requests / meta.
func (c Client) ResolvedModel() string { return c.opt.model }

func normalizeQuality(q, model string) string {
	if q == "" {
		q = "standard"
	}
	lm := strings.ToLower(strings.TrimSpace(model))
	if strings.Contains(lm, "dall-e-3") {
		switch q {
		case "hd", "high":
			return "hd"
		default:
			return "standard"
		}
	}
	switch q {
	case "standard", "medium":
		return "medium"
	case "hd", "high":
		return "high"
	case "draft", "low":
		return "low"
	default:
		switch q {
		case "low", "medium", "high":
			return q
		}
		return "medium"
	}
}

func generationsBody(model, prompt, size, quality, background string) map[string]any {
	lm := strings.ToLower(strings.TrimSpace(model))

	body := map[string]any{
		"model":  model,
		"prompt": prompt,
		"n":      1,
		"size":   size,
	}

	if strings.Contains(lm, "dall-e-3") {
		body["quality"] = quality
		body["response_format"] = "b64_json"
		return body
	}

	body["quality"] = quality
	body["output_format"] = "png"

	if bg := strings.TrimSpace(background); bg != "" &&
		(bg == "transparent" || bg == "opaque" || bg == "auto") {
		body["background"] = bg
	}
	return body
}

type genEnvelope struct {
	Data []json.RawMessage `json:"data"`
	Err  *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type imageItem struct {
	URL       string `json:"url"`
	B64JSON   string `json:"b64_json"`
	Base64PNG string `json:"b64_png,omitempty"`
}

type parsedGeneration struct {
	b64JSON string
	url     string
}

func parseGenerationsEnvelope(body []byte) (*parsedGeneration, error) {
	var env genEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("openai_image: decode response json: %w", err)
	}
	if env.Err != nil && strings.TrimSpace(env.Err.Message) != "" {
		return nil, fmt.Errorf("openai_image: %s", strings.TrimSpace(env.Err.Message))
	}
	if len(env.Data) == 0 {
		return nil, fmt.Errorf("openai_image: no image data returned")
	}
	var item imageItem
	if err := json.Unmarshal(env.Data[0], &item); err != nil {
		return nil, fmt.Errorf("openai_image: decode data item: %w", err)
	}
	src := strings.TrimSpace(item.URL)
	b64 := strings.TrimSpace(item.B64JSON)
	if b64 == "" {
		b64 = strings.TrimSpace(item.Base64PNG)
	}
	if src == "" && b64 == "" {
		return nil, fmt.Errorf("openai_image: neither url nor b64_json returned")
	}
	return &parsedGeneration{b64JSON: b64, url: src}, nil
}

func decodeOrDownload(ctx context.Context, cli *http.Client, p *parsedGeneration) ([]byte, string, error) {
	if strings.TrimSpace(p.b64JSON) != "" {
		decoded, err := base64.StdEncoding.DecodeString(p.b64JSON)
		if err != nil {
			decoded, err = base64.RawStdEncoding.DecodeString(p.b64JSON)
			if err != nil {
				return nil, "", fmt.Errorf("openai_image: invalid base64 image: %w", err)
			}
		}
		return decoded, "image/png", nil
	}
	return downloadImage(ctx, cli, p.url)
}

func downloadImage(ctx context.Context, cli *http.Client, rawURL string) ([]byte, string, error) {
	u := strings.TrimSpace(rawURL)
	if u == "" {
		return nil, "", fmt.Errorf("openai_image: empty image url")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := cli.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("openai_image: download result: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slurp, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, "", fmt.Errorf("openai_image: download result: HTTP %d %s", resp.StatusCode, strings.TrimSpace(string(slurp)))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 30<<20))
	if err != nil {
		return nil, "", err
	}
	ct := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if ct == "" {
		ct = http.DetectContentType(data)
	}
	if !strings.HasPrefix(strings.ToLower(ct), "image/") {
		ct = "image/png"
	}
	return data, ct, nil
}

func extractOpenAIErrorMessage(b []byte) string {
	var w struct {
		Err *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(b, &w) == nil && w.Err != nil {
		return strings.TrimSpace(w.Err.Message)
	}
	return strings.TrimSpace(string(b))
}

func (c Client) postJSON(ctx context.Context, path string, payload map[string]any) ([]byte, int, error) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	u := c.opt.baseURL + path
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.opt.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("openai_image: request: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	return respBody, resp.StatusCode, nil
}

// GenerateScene calls POST /images/generations with ecommerce-safe assembled prompt wiring.
func (c Client) GenerateScene(ctx context.Context, assembledPrompt string) ([]byte, string, error) {
	prompt := strings.TrimSpace(assembledPrompt)
	if prompt == "" {
		return nil, "", fmt.Errorf("openai_image: empty prompt")
	}
	payload := generationsBody(c.opt.model, prompt, c.opt.size, c.opt.quality, c.opt.background)
	body, status, err := c.postJSON(ctx, "/images/generations", payload)
	if err != nil {
		return nil, "", err
	}
	apiMsg := extractOpenAIErrorMessage(body)
	if status < 200 || status >= 300 {
		msg := strings.TrimSpace(apiMsg)
		if msg == "" {
			msg = fmt.Sprintf("openai_image: HTTP %d", status)
		}
		return nil, "", fmt.Errorf("%s", msg)
	}

	parsed, err := parseGenerationsEnvelope(body)
	if err != nil {
		return nil, "", err
	}
	imgBytes, ct, err := decodeOrDownload(ctx, c.httpClient, parsed)
	if err != nil {
		return nil, "", err
	}
	if len(imgBytes) == 0 {
		return nil, "", fmt.Errorf("openai_image: empty image data")
	}
	return imgBytes, ct, nil
}
