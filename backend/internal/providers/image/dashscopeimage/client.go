// Package dashscopeimage calls Alibaba DashScope Wan image generation (async task + poll).
package dashscopeimage

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

const (
	defaultBaseURL = "https://dashscope.aliyuncs.com/api/v1"
	defaultModel   = "wan2.7-image-pro"
	defaultSize    = "2048*2048"
	// Wan 2.7 sync HTTP: multimodal-generation (no X-DashScope-Async header).
	syncGenerationPath = "/services/aigc/multimodal-generation/generation"
	// Wan 2.7 async HTTP: image-generation + X-DashScope-Async: enable + poll.
	asyncGenerationPath = "/services/aigc/image-generation/generation"
)

// Options configures DashScope image synthesis.
type Options struct {
	BaseURL string
	APIKey  string
	Model   string
	Size    string // e.g. 2K, 1K, 4K
	Timeout time.Duration
}

// Client generates images via Wan 2.7 multimodal-generation API.
type Client struct {
	opt        Options
	httpClient *http.Client
}

func normalizeBase(u string) string {
	s := strings.TrimRight(strings.TrimSpace(u), "/")
	if s == "" {
		return defaultBaseURL
	}
	return s
}

// NewClient builds a Client.
func NewClient(opt Options) (*Client, error) {
	key := strings.TrimSpace(opt.APIKey)
	if key == "" {
		return nil, fmt.Errorf("未配置通义万相 API Key")
	}
	model := strings.TrimSpace(opt.Model)
	if model == "" {
		model = defaultModel
	}
	size := normalizeSize(opt.Size)

	sec := opt.Timeout
	if sec <= 0 {
		sec = 120 * time.Second
	}
	return &Client{
		opt: Options{
			BaseURL: normalizeBase(opt.BaseURL),
			APIKey:  key,
			Model:   model,
			Size:    size,
			Timeout: sec,
		},
		httpClient: &http.Client{Timeout: sec},
	}, nil
}

func (c *Client) ResolvedModel() string { return c.opt.Model }

type messageContent struct {
	Text  string `json:"text,omitempty"`
	Image string `json:"image,omitempty"`
}

type submitBody struct {
	Model string `json:"model"`
	Input struct {
		Messages []struct {
			Role    string           `json:"role"`
			Content []messageContent `json:"content"`
		} `json:"messages"`
	} `json:"input"`
	Parameters struct {
		Size      string `json:"size"`
		N         int    `json:"n"`
		Watermark bool   `json:"watermark,omitempty"`
	} `json:"parameters"`
}

type outputChoice struct {
	Message struct {
		Content []struct {
			Image string `json:"image"`
			URL   string `json:"url"`
		} `json:"content"`
	} `json:"message"`
}

type submitResp struct {
	Output struct {
		TaskID     string         `json:"task_id"`
		TaskStatus string         `json:"task_status"`
		Choices    []outputChoice `json:"choices"`
	} `json:"output"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type taskResp struct {
	Output struct {
		TaskStatus string         `json:"task_status"`
		Choices    []outputChoice `json:"choices"`
	} `json:"output"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// DataURL encodes image bytes for DashScope image field (data:{mime};base64,...).
func DataURL(data []byte, contentType string) string {
	ct := strings.TrimSpace(contentType)
	if ct == "" {
		ct = http.DetectContentType(data)
	}
	if !strings.HasPrefix(strings.ToLower(ct), "image/") {
		ct = "image/jpeg"
	}
	return fmt.Sprintf("data:%s;base64,%s", ct, base64.StdEncoding.EncodeToString(data))
}

func (c *Client) authReq(ctx context.Context, method, path string, body []byte, async bool) (*http.Request, error) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	u := c.opt.BaseURL + path
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.opt.APIKey)
	req.Header.Set("Content-Type", "application/json")
	if async {
		req.Header.Set("X-DashScope-Async", "enable")
	}
	return req, nil
}

func (c *Client) buildSubmitBody(content []messageContent) ([]byte, error) {
	var body submitBody
	body.Model = c.opt.Model
	body.Input.Messages = []struct {
		Role    string           `json:"role"`
		Content []messageContent `json:"content"`
	}{
		{Role: "user", Content: content},
	}
	body.Parameters.Size = c.opt.Size
	body.Parameters.N = 1
	body.Parameters.Watermark = false
	return json.Marshal(body)
}

func (c *Client) submitMultimodal(ctx context.Context, content []messageContent) ([]byte, string, error) {
	raw, err := c.buildSubmitBody(content)
	if err != nil {
		return nil, "", err
	}
	// Prefer sync endpoint (documented for wan2.7 image edit); async header on this path returns HTTP 403.
	if img, ct, err := c.postGeneration(ctx, syncGenerationPath, raw, false); err == nil {
		return img, ct, nil
	} else if !shouldRetryAsync(err) {
		return nil, "", err
	}
	return c.postGeneration(ctx, asyncGenerationPath, raw, true)
}

func shouldRetryAsync(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "asynchronous calls") {
		return false
	}
	return strings.Contains(msg, "synchronous calls") ||
		strings.Contains(msg, "does not support synchronous")
}

func (c *Client) postGeneration(ctx context.Context, path string, raw []byte, async bool) ([]byte, string, error) {
	req, err := c.authReq(ctx, http.MethodPost, path, raw, async)
	if err != nil {
		return nil, "", err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("dashscope_image: request: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("dashscope_image: HTTP %d %s", resp.StatusCode, trimAPIErr(respBody))
	}
	var sub submitResp
	if err := json.Unmarshal(respBody, &sub); err != nil {
		return nil, "", fmt.Errorf("dashscope_image: decode submit: %w", err)
	}
	if sub.Code != "" && sub.Code != "Success" && !strings.EqualFold(sub.Code, "OK") {
		return nil, "", fmt.Errorf("dashscope_image: %s", strings.TrimSpace(sub.Message))
	}
	if imgURL := firstImageURL(sub.Output.Choices); imgURL != "" {
		return c.download(ctx, imgURL)
	}
	taskID := strings.TrimSpace(sub.Output.TaskID)
	if taskID != "" && async {
		return c.pollTask(ctx, taskID)
	}
	return nil, "", fmt.Errorf("dashscope_image: empty result in response")
}

// GenerateScene submits text-to-image and polls until success or timeout.
func (c *Client) GenerateScene(ctx context.Context, prompt string) ([]byte, string, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return nil, "", fmt.Errorf("dashscope_image: empty prompt")
	}
	return c.submitMultimodal(ctx, []messageContent{{Text: prompt}})
}

// EditImage submits image+text editing (inpaint / cleanup / upscale) and polls until done.
func (c *Client) EditImage(ctx context.Context, imageRef, prompt string) ([]byte, string, error) {
	imageRef = strings.TrimSpace(imageRef)
	prompt = strings.TrimSpace(prompt)
	if imageRef == "" {
		return nil, "", fmt.Errorf("dashscope_image: source image required")
	}
	if prompt == "" {
		return nil, "", fmt.Errorf("dashscope_image: empty prompt")
	}
	return c.submitMultimodal(ctx, []messageContent{
		{Image: imageRef},
		{Text: prompt},
	})
}

func (c *Client) pollTask(ctx context.Context, taskID string) ([]byte, string, error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(c.opt.Timeout)
	}
	interval := 2 * time.Second
	for time.Now().Before(deadline) {
		req, err := c.authReq(ctx, http.MethodGet, "/tasks/"+taskID, nil, false)
		if err != nil {
			return nil, "", err
		}
		req.Header.Del("X-DashScope-Async")
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, "", fmt.Errorf("dashscope_image: poll: %w", err)
		}
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, "", fmt.Errorf("dashscope_image: poll HTTP %d %s", resp.StatusCode, trimAPIErr(b))
		}
		var tr taskResp
		if err := json.Unmarshal(b, &tr); err != nil {
			return nil, "", fmt.Errorf("dashscope_image: decode poll: %w", err)
		}
		st := strings.ToUpper(strings.TrimSpace(tr.Output.TaskStatus))
		switch st {
		case "SUCCEEDED", "SUCCESS":
			if imgURL := firstImageURL(tr.Output.Choices); imgURL != "" {
				return c.download(ctx, imgURL)
			}
			return nil, "", fmt.Errorf("dashscope_image: empty result url")
		case "FAILED", "CANCELED", "CANCELLED":
			return nil, "", fmt.Errorf("dashscope_image: task %s: %s", st, strings.TrimSpace(tr.Message))
		}
		select {
		case <-ctx.Done():
			return nil, "", ctx.Err()
		case <-time.After(interval):
		}
	}
	return nil, "", fmt.Errorf("dashscope_image: poll timeout")
}

func (c *Client) download(ctx context.Context, rawURL string) ([]byte, string, error) {
	u := strings.TrimSpace(rawURL)
	if u == "" {
		return nil, "", fmt.Errorf("dashscope_image: empty url")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("dashscope_image: download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slurp, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, "", fmt.Errorf("dashscope_image: download HTTP %d %s", resp.StatusCode, strings.TrimSpace(string(slurp)))
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

func normalizeSize(raw string) string {
	size := strings.TrimSpace(raw)
	if size == "" {
		return defaultSize
	}
	normalized := strings.ReplaceAll(size, "x", "*")
	normalized = strings.ReplaceAll(normalized, "X", "*")
	upper := strings.ToUpper(strings.TrimSpace(normalized))
	switch upper {
	case "1K":
		return "1024*1024"
	case "2K":
		return "2048*2048"
	case "4K":
		return "4096*4096"
	}
	if strings.Contains(normalized, "*") {
		return normalized
	}
	return size
}

func firstImageURL(choices []outputChoice) string {
	for _, choice := range choices {
		for _, item := range choice.Message.Content {
			if u := strings.TrimSpace(item.Image); u != "" {
				return u
			}
			if u := strings.TrimSpace(item.URL); u != "" {
				return u
			}
		}
	}
	return ""
}

func trimAPIErr(b []byte) string {
	var w struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	}
	if json.Unmarshal(b, &w) == nil && strings.TrimSpace(w.Message) != "" {
		return w.Message
	}
	return strings.TrimSpace(string(b))
}
