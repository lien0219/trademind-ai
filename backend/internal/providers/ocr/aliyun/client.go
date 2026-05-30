package aliyun

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	ocrapi "github.com/alibabacloud-go/ocr-api-20210707/v3/client"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/trademind-ai/trademind/backend/internal/providers/ocr/ocrerror"
)

const (
	defaultEndpoint = "ocr-api.cn-hangzhou.aliyuncs.com"
	defaultRegion   = "cn-hangzhou"
	defaultAPIName  = "RecognizeGeneral"
)

type Options struct {
	Endpoint        string
	Region          string
	AccessKeyID     string
	AccessKeySecret string
	APIName         string
	Timeout         time.Duration
	MinConfidence   float64
}

type Client struct {
	opts   Options
	client *ocrapi.Client
}

func New(opts Options) (*Client, error) {
	opts.Endpoint = firstNonEmpty(opts.Endpoint, defaultEndpoint)
	opts.Region = firstNonEmpty(opts.Region, defaultRegion)
	opts.APIName = firstNonEmpty(opts.APIName, defaultAPIName)
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.MinConfidence <= 0 {
		opts.MinConfidence = 0.75
	}
	cfg := &openapi.Config{
		AccessKeyId:     tea.String(opts.AccessKeyID),
		AccessKeySecret: tea.String(opts.AccessKeySecret),
		Endpoint:        tea.String(opts.Endpoint),
		RegionId:        tea.String(opts.Region),
	}
	cli, err := ocrapi.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{opts: opts, client: cli}, nil
}

type DetectRequest struct {
	ImageURL    string
	ImageBase64 string
	LocalPath   string
}

type DetectBBox struct {
	X      int
	Y      int
	Width  int
	Height int
}

type DetectPoint struct {
	X int
	Y int
}

type DetectBlock struct {
	ID         string
	Text       string
	Confidence float64
	BBox       DetectBBox
	Polygon    []DetectPoint
	Angle      float64
	Direction  string
}

type DetectResult struct {
	Provider            string
	APIName             string
	DetectedLanguage    string
	Blocks              []DetectBlock
	FilteredBlocksCount int
	Raw                 map[string]any
}

func (c *Client) DetectText(ctx context.Context, req DetectRequest) (*DetectResult, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("阿里云 OCR 客户端不可用")
	}
	apiReq, err := buildRequest(req)
	if err != nil {
		return nil, err
	}
	timeoutMs := int(30_000)
	if c.opts.Timeout > 0 {
		timeoutMs = int(c.opts.Timeout / time.Millisecond)
	}
	resp, err := c.client.RecognizeGeneralWithOptions(apiReq, &util.RuntimeOptions{
		ReadTimeout:    tea.Int(timeoutMs),
		ConnectTimeout: tea.Int(timeoutMs),
	})
	if err != nil {
		return nil, mapAliyunError(err.Error(), "")
	}
	if resp == nil || resp.Body == nil {
		return nil, fmt.Errorf("阿里云 OCR 未返回有效响应")
	}
	requestID := tea.StringValue(resp.Body.RequestId)
	code := strings.TrimSpace(tea.StringValue(resp.Body.Code))
	if code != "" && code != "200" && !strings.EqualFold(code, "OK") {
		return nil, mapAliyunError(tea.StringValue(resp.Body.Message), requestID)
	}
	data := strings.TrimSpace(tea.StringValue(resp.Body.Data))
	if data == "" {
		return nil, ocrerror.NewWithRequestID(ocrerror.CodeEmptyBlocks, "阿里云 OCR 未识别到文字，请更换图片或降低最低置信度", requestID)
	}
	return convertData(c.opts.APIName, c.opts.MinConfidence, requestID, data)
}

func buildRequest(req DetectRequest) (*ocrapi.RecognizeGeneralRequest, error) {
	out := &ocrapi.RecognizeGeneralRequest{}
	if u := strings.TrimSpace(req.ImageURL); isPublicHTTPURL(u) {
		out.Url = tea.String(u)
		return out, nil
	}
	if b64 := strings.TrimSpace(req.ImageBase64); b64 != "" {
		data, err := base64.StdEncoding.DecodeString(stripDataURLPrefix(b64))
		if err != nil {
			return nil, fmt.Errorf("阿里云 OCR 图片 base64 无效")
		}
		out.Body = bytes.NewReader(data)
		return out, nil
	}
	if p := strings.TrimSpace(req.LocalPath); p != "" {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("读取本地图片失败：%w", err)
		}
		out.Body = bytes.NewReader(data)
		return out, nil
	}
	return nil, ocrerror.New(ocrerror.CodeImageURLInaccessible, "图片地址无法被阿里云 OCR 访问，请先上传到当前存储服务后再识别")
}

type recognizeData struct {
	Content        string          `json:"content"`
	PrismWordsInfo []prismWordInfo `json:"prism_wordsInfo"`
}

type prismWordInfo struct {
	Word      string        `json:"word"`
	Prob      float64       `json:"prob"`
	X         int           `json:"x"`
	Y         int           `json:"y"`
	Width     int           `json:"width"`
	Height    int           `json:"height"`
	Angle     float64       `json:"angle"`
	Direction int           `json:"direction"`
	Pos       []aliyunPoint `json:"pos"`
}

type aliyunPoint struct {
	X int `json:"x"`
	Y int `json:"y"`
}

func convertData(apiName string, minConfidence float64, requestID string, data string) (*DetectResult, error) {
	var parsed recognizeData
	if err := json.Unmarshal([]byte(data), &parsed); err != nil {
		return nil, fmt.Errorf("阿里云 OCR 结果解析失败：%w", err)
	}
	blocks := make([]DetectBlock, 0, len(parsed.PrismWordsInfo))
	filtered := 0
	for i, item := range parsed.PrismWordsInfo {
		text := strings.TrimSpace(item.Word)
		if text == "" {
			continue
		}
		conf := normalizeConfidence(item.Prob)
		if conf > 0 && minConfidence > 0 && conf < minConfidence {
			filtered++
			continue
		}
		polygon := make([]DetectPoint, 0, len(item.Pos))
		for _, p := range item.Pos {
			polygon = append(polygon, DetectPoint{X: p.X, Y: p.Y})
		}
		bbox := DetectBBox{X: item.X, Y: item.Y, Width: item.Width, Height: item.Height}
		if (bbox.Width <= 0 || bbox.Height <= 0) && len(polygon) >= 4 {
			bbox = bboxFromPolygon(polygon)
		}
		blocks = append(blocks, DetectBlock{
			ID:         fmt.Sprintf("aliyun_%d", i+1),
			Text:       text,
			Confidence: conf,
			BBox:       bbox,
			Polygon:    polygon,
			Angle:      item.Angle,
			Direction:  directionLabel(item.Direction),
		})
	}
	return &DetectResult{
		Provider:            "aliyun",
		APIName:             apiName,
		DetectedLanguage:    "zh",
		Blocks:              blocks,
		FilteredBlocksCount: filtered,
		Raw: map[string]any{
			"requestId": requestID,
		},
	}, nil
}

func bboxFromPolygon(points []DetectPoint) DetectBBox {
	minX, minY := points[0].X, points[0].Y
	maxX, maxY := minX, minY
	for _, p := range points[1:] {
		if p.X < minX {
			minX = p.X
		}
		if p.Y < minY {
			minY = p.Y
		}
		if p.X > maxX {
			maxX = p.X
		}
		if p.Y > maxY {
			maxY = p.Y
		}
	}
	return DetectBBox{X: minX, Y: minY, Width: maxX - minX, Height: maxY - minY}
}

func directionLabel(v int) string {
	if v == 1 {
		return "vertical"
	}
	return "horizontal"
}

func normalizeConfidence(v float64) float64 {
	if v > 1 {
		return v / 100
	}
	return v
}

func mapAliyunError(msg, requestID string) error {
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "timeout") || strings.Contains(lower, "deadline exceeded") || strings.Contains(lower, "i/o timeout"):
		return ocrerror.NewWithRequestID(ocrerror.CodeTimeout, "阿里云 OCR 调用超时，请检查网络或调大 OCR 超时", requestID)
	case strings.Contains(lower, "notopen") || strings.Contains(lower, "not open") || strings.Contains(lower, "not activate") || strings.Contains(lower, "service not"):
		return ocrerror.NewWithRequestID(ocrerror.CodeServiceNotOpen, "阿里云 OCR 服务未开通，请先到阿里云控制台开通文字识别服务", requestID)
	case strings.Contains(lower, "unauthorized") || strings.Contains(lower, "forbidden") || strings.Contains(lower, "no permission"):
		return ocrerror.NewWithRequestID(ocrerror.CodePermissionDenied, "当前 AccessKey 没有 OCR 调用权限，请检查 RAM 权限", requestID)
	case strings.Contains(lower, "invalidaccesskey") || strings.Contains(lower, "signature"):
		return ocrerror.NewWithRequestID(ocrerror.CodeAuthFailed, "AccessKeyId 或 AccessKeySecret 不正确，请检查密钥配置", requestID)
	case strings.Contains(lower, "insufficient") || strings.Contains(lower, "balance") || strings.Contains(lower, "quota"):
		return ocrerror.NewWithRequestID(ocrerror.CodePermissionDenied, "当前账号欠费或资源包已用尽，请检查阿里云费用状态", requestID)
	case strings.Contains(lower, "throttl") || strings.Contains(lower, "rate") || strings.Contains(lower, "too many"):
		return ocrerror.NewWithRequestID(ocrerror.CodeRateLimited, "阿里云 OCR 调用频率过高，请降低批量并发或稍后重试", requestID)
	default:
		return ocrerror.NewWithRequestID(ocrerror.CodeUnknown, "阿里云 OCR 调用失败："+strings.TrimSpace(msg), requestID)
	}
}

func isPublicHTTPURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".local") {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return false
		}
	}
	return true
}

func stripDataURLPrefix(s string) string {
	if idx := strings.Index(s, ","); strings.HasPrefix(strings.ToLower(s), "data:") && idx >= 0 {
		return strings.TrimSpace(s[idx+1:])
	}
	return s
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
