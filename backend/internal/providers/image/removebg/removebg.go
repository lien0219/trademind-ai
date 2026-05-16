package removebg

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Options configures remove.bg HTTP client (credentials must never be logged).
type Options struct {
	APIKey  string
	BaseURL string
	Timeout time.Duration
}

// Client calls remove.bg remote API (PNG response body).
type Client struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

// NewClient constructs a Client with HTTP timeout enforcement.
func NewClient(opts Options) Client {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	base := strings.TrimSpace(opts.BaseURL)
	if base == "" {
		base = "https://api.remove.bg/v1.0"
	}
	base = strings.TrimRight(base, "/")
	return Client{
		apiKey:  strings.TrimSpace(opts.APIKey),
		baseURL: base,
		http: &http.Client{
			Timeout: timeout,
		},
	}
}

// RemoveBackgroundPNG posts image_url multipart field and returns PNG bytes on success.
func (c Client) RemoveBackgroundPNG(ctx context.Context, imageURL string) ([]byte, error) {
	src := strings.TrimSpace(imageURL)
	if src == "" {
		return nil, errors.New("remove.bg: source url required")
	}
	if err := ensurePublicHTTPSURL(src); err != nil {
		return nil, err
	}
	if strings.TrimSpace(c.apiKey) == "" {
		return nil, errors.New("remove.bg: api key not configured")
	}
	endpoint := c.baseURL + "/removebg"

	var buf bytes.Buffer
	mp := multipart.NewWriter(&buf)
	if err := mp.WriteField("image_url", src); err != nil {
		return nil, fmt.Errorf("remove.bg: build form: %w", err)
	}
	if err := mp.Close(); err != nil {
		return nil, fmt.Errorf("remove.bg: close multipart: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &buf)
	if err != nil {
		return nil, fmt.Errorf("remove.bg: request: %w", err)
	}
	httpReq.Header.Set("X-Api-Key", c.apiKey)
	httpReq.Header.Set("Content-Type", mp.FormDataContentType())

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("remove.bg: http: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("remove.bg: read body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := parseRemoveBGError(body)
		if msg == "" {
			msg = fmt.Sprintf("remove.bg: HTTP %d", resp.StatusCode)
		}
		return nil, errors.New(msg)
	}

	ct := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if ct != "" && !strings.Contains(strings.ToLower(ct), "png") && !strings.Contains(strings.ToLower(ct), "image") && !strings.Contains(strings.ToLower(ct), "octet-stream") {
		if len(body) < 8 || body[0] != 0x89 || string(body[1:4]) != "PNG" {
			msg := parseRemoveBGError(body)
			if msg == "" {
				msg = "remove.bg: unexpected response content-type"
			}
			return nil, errors.New(msg)
		}
	}

	return body, nil
}

func ensurePublicHTTPSURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return errors.New("source image URL is not publicly accessible")
	}
	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return errors.New("source image URL is not publicly accessible")
	}
	hl := strings.ToLower(host)
	if hl == "localhost" || hl == "127.0.0.1" || hl == "::1" || strings.HasSuffix(hl, ".localhost") {
		return errors.New("source image URL is not publicly accessible")
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return errors.New("source image URL is not publicly accessible")
		}
	}
	return nil
}

func parseRemoveBGError(body []byte) string {
	s := strings.TrimSpace(string(body))
	if s == "" {
		return ""
	}
	var outer map[string]json.RawMessage
	if err := json.Unmarshal(body, &outer); err != nil {
		if len(s) > 512 {
			s = s[:512] + "…"
		}
		return s
	}
	if raw, ok := outer["errors"]; ok {
		var errs []struct {
			Title string `json:"title"`
			Code  string `json:"code"`
		}
		if json.Unmarshal(raw, &errs) == nil && len(errs) > 0 {
			parts := make([]string, 0, len(errs))
			for _, e := range errs {
				t := strings.TrimSpace(e.Title)
				if t == "" {
					continue
				}
				if c := strings.TrimSpace(e.Code); c != "" {
					parts = append(parts, fmt.Sprintf("[%s] %s", c, t))
				} else {
					parts = append(parts, t)
				}
			}
			if len(parts) > 0 {
				return strings.Join(parts, "; ")
			}
		}
	}
	if raw, ok := outer["error"]; ok {
		var msg string
		if json.Unmarshal(raw, &msg) == nil && strings.TrimSpace(msg) != "" {
			return strings.TrimSpace(msg)
		}
	}
	if len(s) > 512 {
		s = s[:512] + "…"
	}
	return s
}
