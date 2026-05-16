package removebg

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/pkg/httppublic"
)

const maxSourceBytes = 32 << 20

// Options configures remove.bg HTTP client (credentials must never be logged).
type Options struct {
	APIKey  string
	BaseURL string
	Timeout time.Duration
}

// RemoveBackgroundInput selects image_url vs multipart image_file mode.
type RemoveBackgroundInput struct {
	ImageURL         string
	Image            io.Reader
	ImageFilename    string
	ImageContentType string
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

// RemoveBackground posts image_file or image_url and returns PNG bytes on success.
// Prefer Image when readable; otherwise requires a publicly reachable ImageURL.
func (c Client) RemoveBackground(ctx context.Context, in RemoveBackgroundInput) ([]byte, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return nil, errors.New("remove.bg: api key not configured")
	}
	endpoint := c.baseURL + "/removebg"

	hasFile := in.Image != nil
	urlStr := strings.TrimSpace(in.ImageURL)
	if !hasFile && urlStr == "" {
		return nil, errors.New("source image is not readable and not publicly accessible")
	}

	var buf bytes.Buffer
	mp := multipart.NewWriter(&buf)
	if hasFile {
		data, err := readLimitedImage(in.Image, maxSourceBytes)
		if err != nil {
			return nil, err
		}
		fn := strings.TrimSpace(in.ImageFilename)
		if fn == "" {
			fn = "source.png"
		}
		part, err := mp.CreateFormFile("image_file", fn)
		if err != nil {
			return nil, fmt.Errorf("remove.bg: build form: %w", err)
		}
		if _, err := part.Write(data); err != nil {
			return nil, fmt.Errorf("remove.bg: write image_file: %w", err)
		}
	} else {
		if !httppublic.IsPublicHTTPURL(urlStr) {
			return nil, errors.New("source image is not readable and not publicly accessible")
		}
		if err := mp.WriteField("image_url", urlStr); err != nil {
			return nil, fmt.Errorf("remove.bg: build form: %w", err)
		}
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

func readLimitedImage(r io.Reader, max int) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, int64(max)+1))
	if err != nil {
		return nil, fmt.Errorf("remove.bg: read source: %w", err)
	}
	if len(b) > max {
		return nil, errors.New("source image exceeds maximum size for remove.bg upload")
	}
	if len(b) == 0 {
		return nil, errors.New("source image is not readable and not publicly accessible")
	}
	return b, nil
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
