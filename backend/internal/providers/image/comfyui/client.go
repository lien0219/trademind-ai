package comfyui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	_ "golang.org/x/image/webp"
)

// Options configures the ComfyUI HTTP integration.
type Options struct {
	BaseURL      string
	APIKey       string
	WorkflowJSON string
	PromptNodeID string
	ImageNodeID  string
	OutputNodeID string
	HTTPTimeout  time.Duration
	PollInterval time.Duration
	MaxPoll      time.Duration
}

// Client calls ComfyUI REST endpoints.
type Client struct {
	opts   Options
	httpCl *http.Client
}

// NewClient builds a ComfyUI client with sane defaults.
func NewClient(opts Options) (*Client, error) {
	base := strings.TrimSpace(opts.BaseURL)
	if base == "" {
		return nil, fmt.Errorf("comfyui_base_url is not configured")
	}
	base = strings.TrimRight(base, "/")
	opts.BaseURL = base
	if opts.HTTPTimeout <= 0 {
		opts.HTTPTimeout = 180 * time.Second
	}
	if opts.PollInterval <= 0 {
		opts.PollInterval = 2 * time.Second
	}
	if opts.MaxPoll <= 0 {
		opts.MaxPoll = 180 * time.Second
	}
	return &Client{
		opts: opts,
		httpCl: &http.Client{
			Timeout: opts.HTTPTimeout,
		},
	}, nil
}

func (c *Client) authHeader(req *http.Request) {
	k := strings.TrimSpace(c.opts.APIKey)
	if k == "" {
		return
	}
	req.Header.Set("Authorization", "Bearer "+k)
}

func (c *Client) readErrorBody(resp *http.Response) string {
	if resp == nil || resp.Body == nil {
		return ""
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	s := strings.TrimSpace(string(b))
	if len(s) > 2000 {
		s = s[:2000] + "…"
	}
	return s
}

func workflowConfigured(raw string) bool {
	s := strings.TrimSpace(raw)
	if s == "" || s == "{}" {
		return false
	}
	var v any
	if json.Unmarshal([]byte(s), &v) != nil {
		return false
	}
	switch x := v.(type) {
	case map[string]any:
		return len(x) > 0
	default:
		return false
	}
}

func expandWorkflowTemplate(tpl string, vars map[string]string) (string, error) {
	if !workflowConfigured(tpl) {
		return "", fmt.Errorf("comfyui_workflow_json is empty or invalid")
	}
	out := tpl
	for k, v := range vars {
		placeholder := "{{" + k + "}}"
		out = strings.ReplaceAll(out, placeholder, v)
	}
	if strings.Contains(out, "{{") && strings.Contains(out, "}}") {
		return "", fmt.Errorf("workflow_json still contains unreplaced placeholders after variable substitution")
	}
	return out, nil
}

func parseWorkflowObject(workflowJSON string) (map[string]any, error) {
	var m map[string]any
	if err := json.Unmarshal([]byte(workflowJSON), &m); err != nil {
		return nil, fmt.Errorf("workflow_json is not valid JSON after substitution: %w", err)
	}
	if len(m) == 0 {
		return nil, fmt.Errorf("workflow_json parsed to an empty object")
	}
	return m, nil
}

func stringFromVars(input map[string]any, keys ...string) string {
	if input == nil {
		return ""
	}
	for _, k := range keys {
		if v, ok := input[k]; ok && v != nil {
			switch x := v.(type) {
			case string:
				if t := strings.TrimSpace(x); t != "" {
					return t
				}
			default:
				if t := strings.TrimSpace(fmt.Sprint(x)); t != "" {
					return t
				}
			}
		}
	}
	return ""
}

func buildVarMap(input map[string]any, sourceURL string) map[string]string {
	return map[string]string{
		"prompt":         stringFromVars(input, "assembled_prompt", "prompt"),
		"negativePrompt": stringFromVars(input, "negativePrompt", "negative_prompt"),
		"sourceImageUrl": sourceURL,
		"productTitle":   stringFromVars(input, "productTitle", "product_title"),
		"scene":          stringFromVars(input, "scene"),
		"style":          stringFromVars(input, "style"),
		"background":     stringFromVars(input, "background"),
		"platform":       stringFromVars(input, "platform"),
	}
}

func setNodeTextPrompt(workflow map[string]any, nodeID, text string) error {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return nil
	}
	n, ok := workflow[nodeID]
	if !ok || n == nil {
		return fmt.Errorf("comfyui_prompt_node_id %q not found in workflow", nodeID)
	}
	nm, ok := n.(map[string]any)
	if !ok {
		return fmt.Errorf("comfyui prompt node %q is not an object", nodeID)
	}
	in, ok := nm["inputs"].(map[string]any)
	if !ok {
		in = map[string]any{}
		nm["inputs"] = in
	}
	in["text"] = text
	return nil
}

func setNodeLoadImage(workflow map[string]any, nodeID, imageName string) error {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return nil
	}
	n, ok := workflow[nodeID]
	if !ok || n == nil {
		return fmt.Errorf("comfyui_image_node_id %q not found in workflow", nodeID)
	}
	nm, ok := n.(map[string]any)
	if !ok {
		return fmt.Errorf("comfyui image node %q is not an object", nodeID)
	}
	in, ok := nm["inputs"].(map[string]any)
	if !ok {
		in = map[string]any{}
		nm["inputs"] = in
	}
	in["image"] = imageName
	return nil
}

type promptResp struct {
	PromptID   string          `json:"prompt_id"`
	NodeErrors json.RawMessage `json:"node_errors"`
	Error      json.RawMessage `json:"error"`
}

func (c *Client) postPrompt(ctx context.Context, workflow map[string]any) (string, error) {
	u := c.opts.BaseURL + "/prompt"
	body := map[string]any{
		"prompt":    workflow,
		"client_id": "trademind-image",
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	c.authHeader(req)
	resp, err := c.httpCl.Do(req)
	if err != nil {
		return "", fmt.Errorf("comfyui POST /prompt: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("comfyui POST /prompt: status %d: %s", resp.StatusCode, c.readErrorBody(resp))
	}
	var pr promptResp
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&pr); err != nil {
		return "", fmt.Errorf("comfyui POST /prompt: decode: %w", err)
	}
	if len(pr.NodeErrors) > 0 && string(pr.NodeErrors) != "null" && string(pr.NodeErrors) != "{}" {
		return "", fmt.Errorf("comfyui workflow validation failed: node_errors=%s", strings.TrimSpace(string(pr.NodeErrors)))
	}
	pid := strings.TrimSpace(pr.PromptID)
	if pid == "" {
		return "", fmt.Errorf("comfyui POST /prompt: missing prompt_id")
	}
	return pid, nil
}

func (c *Client) getHistoryEntry(ctx context.Context, promptID string) (map[string]any, bool, error) {
	u := c.opts.BaseURL + "/history/" + url.PathEscape(promptID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, false, err
	}
	c.authHeader(req)
	resp, err := c.httpCl.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("comfyui GET /history: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, false, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, false, fmt.Errorf("comfyui GET /history: status %d: %s", resp.StatusCode, c.readErrorBody(resp))
	}
	var top any
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&top); err != nil {
		return nil, false, fmt.Errorf("comfyui history decode: %w", err)
	}
	return normalizeHistoryEntry(top, promptID)
}

func normalizeHistoryEntry(top any, promptID string) (map[string]any, bool, error) {
	switch v := top.(type) {
	case []any:
		if len(v) == 0 {
			return nil, false, nil
		}
		if m, ok := v[0].(map[string]any); ok {
			if _, ok := m["outputs"]; ok {
				return m, true, nil
			}
		}
		return nil, false, nil
	case map[string]any:
		raw := v
		if e, ok := raw[promptID]; ok {
			if m, ok := e.(map[string]any); ok {
				return m, true, nil
			}
			if a, ok := e.([]any); ok && len(a) > 0 {
				if m, ok := a[0].(map[string]any); ok {
					return m, true, nil
				}
			}
		}
		if _, ok := raw["outputs"]; ok {
			return raw, true, nil
		}
	}
	return nil, false, nil
}

func firstOutputImage(entry map[string]any, outputNodeID string) (filename, subfolder, typ string, err error) {
	outputNodeID = strings.TrimSpace(outputNodeID)
	if outputNodeID == "" {
		return "", "", "", fmt.Errorf("comfyui_output_node_id is not configured")
	}
	outRoot, ok := entry["outputs"].(map[string]any)
	if !ok || outRoot == nil {
		return "", "", "", fmt.Errorf("comfyui history: no outputs yet")
	}
	n, ok := outRoot[outputNodeID]
	if !ok || n == nil {
		return "", "", "", fmt.Errorf("comfyui history: output node %q not found", outputNodeID)
	}
	nm, ok := n.(map[string]any)
	if !ok {
		return "", "", "", fmt.Errorf("comfyui history: output node %q malformed", outputNodeID)
	}
	arr, ok := nm["images"].([]any)
	if !ok || len(arr) == 0 {
		return "", "", "", fmt.Errorf("comfyui history: node %q has no images", outputNodeID)
	}
	img0, ok := arr[0].(map[string]any)
	if !ok {
		return "", "", "", fmt.Errorf("comfyui history: image entry malformed")
	}
	filename, _ = img0["filename"].(string)
	subfolder, _ = img0["subfolder"].(string)
	typ, _ = img0["type"].(string)
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return "", "", "", fmt.Errorf("comfyui history: empty filename")
	}
	if typ == "" {
		typ = "output"
	}
	return filename, subfolder, typ, nil
}

func (c *Client) downloadView(ctx context.Context, filename, subfolder, typ string) ([]byte, string, error) {
	q := url.Values{}
	q.Set("filename", filename)
	q.Set("subfolder", subfolder)
	q.Set("type", typ)
	u := c.opts.BaseURL + "/view?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, "", err
	}
	c.authHeader(req)
	resp, err := c.httpCl.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("comfyui GET /view: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("comfyui GET /view: status %d: %s", resp.StatusCode, c.readErrorBody(resp))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 40<<20))
	if err != nil {
		return nil, "", err
	}
	ct := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if ct == "" {
		ct = "image/png"
	}
	return data, ct, nil
}

func encodeAsPNG(b []byte) ([]byte, error) {
	img, format, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("decode result image (%s): %w", format, err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}
	return buf.Bytes(), nil
}

func (c *Client) uploadImage(ctx context.Context, fileName string, data []byte) (string, error) {
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		fileName = "source.png"
	}
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("image", fileName)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(data); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	u := c.opts.BaseURL + "/upload/image"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	c.authHeader(req)
	resp, err := c.httpCl.Do(req)
	if err != nil {
		return "", fmt.Errorf("comfyui POST /upload/image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("comfyui POST /upload/image: status %d: %s", resp.StatusCode, c.readErrorBody(resp))
	}
	var meta struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&meta); err != nil {
		return "", fmt.Errorf("comfyui upload decode: %w", err)
	}
	name := strings.TrimSpace(meta.Name)
	if name == "" {
		return "", fmt.Errorf("comfyui upload: missing image name")
	}
	return name, nil
}

func (c *Client) downloadSource(ctx context.Context, imageURL string, maxBytes int64) ([]byte, error) {
	u := strings.TrimSpace(imageURL)
	if u == "" {
		return nil, fmt.Errorf("empty source image url")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpCl.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download source image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download source image: status %d: %s", resp.StatusCode, c.readErrorBody(resp))
	}
	if maxBytes <= 0 {
		maxBytes = 15 << 20
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxBytes))
}
