// Package dashscopeimage calls Alibaba DashScope Wan text-to-image (async task + poll).
package dashscopeimage

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

const defaultBaseURL = "https://dashscope.aliyuncs.com/api/v1"

// Options configures DashScope image synthesis.
type Options struct {
	BaseURL string
	APIKey  string
	Model   string
	Size    string // e.g. 1024*1024
	Timeout time.Duration
}

// Client generates images via text2image/image-synthesis.
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
		return nil, fmt.Errorf("dashscope_image_api_key is not configured")
	}
	model := strings.TrimSpace(opt.Model)
	if model == "" {
		model = "wanx2.1-t2i-turbo"
	}
	size := strings.TrimSpace(opt.Size)
	if size == "" {
		size = "1024*1024"
	}
	// normalize 1024x1024 -> 1024*1024
	size = strings.ReplaceAll(size, "x", "*")
	size = strings.ReplaceAll(size, "X", "*")

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

type submitBody struct {
	Model string `json:"model"`
	Input struct {
		Prompt string `json:"prompt"`
	} `json:"input"`
	Parameters struct {
		Size string `json:"size"`
		N    int    `json:"n"`
	} `json:"parameters"`
}

type submitResp struct {
	Output struct {
		TaskID     string `json:"task_id"`
		TaskStatus string `json:"task_status"`
		Results    []struct {
			URL string `json:"url"`
		} `json:"results"`
	} `json:"output"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type taskResp struct {
	Output struct {
		TaskStatus string `json:"task_status"`
		Results    []struct {
			URL string `json:"url"`
		} `json:"results"`
	} `json:"output"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (c *Client) authReq(ctx context.Context, method, path string, body []byte) (*http.Request, error) {
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
	req.Header.Set("X-DashScope-Async", "enable")
	return req, nil
}

// GenerateScene submits text2image and polls until success or timeout.
func (c *Client) GenerateScene(ctx context.Context, prompt string) ([]byte, string, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return nil, "", fmt.Errorf("dashscope_image: empty prompt")
	}
	var body submitBody
	body.Model = c.opt.Model
	body.Input.Prompt = prompt
	body.Parameters.Size = c.opt.Size
	body.Parameters.N = 1
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, "", err
	}
	req, err := c.authReq(ctx, http.MethodPost, "/services/aigc/text2image/image-synthesis", raw)
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
	taskID := strings.TrimSpace(sub.Output.TaskID)
	if taskID == "" {
		// sync path
		if len(sub.Output.Results) > 0 && strings.TrimSpace(sub.Output.Results[0].URL) != "" {
			return c.download(ctx, sub.Output.Results[0].URL)
		}
		return nil, "", fmt.Errorf("dashscope_image: no task_id in response")
	}
	return c.pollTask(ctx, taskID)
}

func (c *Client) pollTask(ctx context.Context, taskID string) ([]byte, string, error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(c.opt.Timeout)
	}
	interval := 2 * time.Second
	for time.Now().Before(deadline) {
		req, err := c.authReq(ctx, http.MethodGet, "/tasks/"+taskID, nil)
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
			if len(tr.Output.Results) == 0 || strings.TrimSpace(tr.Output.Results[0].URL) == "" {
				return nil, "", fmt.Errorf("dashscope_image: empty result url")
			}
			return c.download(ctx, tr.Output.Results[0].URL)
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
