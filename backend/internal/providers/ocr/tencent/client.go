package tencent

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/providers/ocr/ocrerror"
)

const (
	defaultEndpoint = "ocr.tencentcloudapi.com"
	defaultRegion   = "ap-guangzhou"
	defaultAPIName  = "GeneralBasicOCR"
	apiVersion      = "2018-11-19"
	serviceName     = "ocr"
)

type Options struct {
	Endpoint      string
	Region        string
	SecretID      string
	SecretKey     string
	APIName       string
	Timeout       time.Duration
	MinConfidence float64
}

type Client struct {
	opts       Options
	httpClient *http.Client
}

type DetectRequest struct {
	ImageURL          string
	ImageBase64       string
	LocalPath         string
	DetectOrientation bool
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

func New(opts Options) *Client {
	opts.Endpoint = strings.TrimSpace(opts.Endpoint)
	if opts.Endpoint == "" {
		opts.Endpoint = defaultEndpoint
	}
	opts.Region = strings.TrimSpace(opts.Region)
	if opts.Region == "" {
		opts.Region = defaultRegion
	}
	opts.APIName = normalizeAPIName(opts.APIName)
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.MinConfidence <= 0 {
		opts.MinConfidence = 0.75
	}
	return &Client{
		opts: opts,
		httpClient: &http.Client{
			Timeout: opts.Timeout,
		},
	}
}

func normalizeAPIName(apiName string) string {
	switch strings.TrimSpace(apiName) {
	case "GeneralFastOCR":
		return "GeneralFastOCR"
	default:
		return defaultAPIName
	}
}

func (c *Client) DetectText(ctx context.Context, req DetectRequest) (*DetectResult, error) {
	if strings.TrimSpace(c.opts.SecretID) == "" {
		return nil, ocrerror.New(ocrerror.CodeSecretMissing, "腾讯云 OCR SecretId 未配置，请填写后再测试")
	}
	if strings.TrimSpace(c.opts.SecretKey) == "" {
		return nil, ocrerror.New(ocrerror.CodeSecretMissing, "腾讯云 OCR SecretKey 未配置，请填写后再测试")
	}
	host := endpointHost(c.opts.Endpoint)
	if host == "" {
		return nil, fmt.Errorf("腾讯云 OCR Endpoint 不能为空")
	}

	payload, err := buildPayload(req)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("tencent ocr encode request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+host, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("tencent ocr create request: %w", err)
	}
	now := time.Now().UTC()
	httpReq.Header.Set("Content-Type", "application/json; charset=utf-8")
	httpReq.Header.Set("Host", host)
	httpReq.Header.Set("X-TC-Action", c.opts.APIName)
	httpReq.Header.Set("X-TC-Version", apiVersion)
	httpReq.Header.Set("X-TC-Region", c.opts.Region)
	httpReq.Header.Set("X-TC-Timestamp", strconv.FormatInt(now.Unix(), 10))
	httpReq.Header.Set("Authorization", c.authorization(host, string(body), now))

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("腾讯云 OCR 请求失败：%w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("tencent ocr read response: %w", err)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, ocrerror.New(ocrerror.CodeRateLimited, "腾讯云 OCR 调用频率过高，请降低批量并发或稍后重试")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("腾讯云 OCR HTTP %d", resp.StatusCode)
	}

	var apiResp responseEnvelope
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("tencent ocr decode response: %w", err)
	}
	if apiResp.Response.Error != nil {
		return nil, mapTencentError(apiResp.Response.Error.Code, apiResp.Response.Error.Message, apiResp.Response.RequestID)
	}

	out := convertResponse(c.opts.APIName, c.opts.MinConfidence, apiResp.Response)
	return out, nil
}

func endpointHost(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")
	if u, err := url.Parse("https://" + endpoint); err == nil {
		return strings.TrimSpace(u.Host)
	}
	return endpoint
}

func buildPayload(req DetectRequest) (map[string]any, error) {
	out := map[string]any{}
	if u := strings.TrimSpace(req.ImageURL); isPublicHTTPURL(u) {
		out["ImageUrl"] = u
		return out, nil
	}
	if b64 := strings.TrimSpace(req.ImageBase64); b64 != "" {
		out["ImageBase64"] = stripDataURLPrefix(b64)
		return out, nil
	}
	if p := strings.TrimSpace(req.LocalPath); p != "" {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("读取本地图片失败：%w", err)
		}
		out["ImageBase64"] = base64.StdEncoding.EncodeToString(data)
		return out, nil
	}
	return nil, ocrerror.New(ocrerror.CodeImageURLInaccessible, "图片地址无法被腾讯云 OCR 访问，请先上传到当前存储服务后再识别")
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

func (c *Client) authorization(host, payload string, now time.Time) string {
	hashedPayload := sha256Hex(payload)
	headers := map[string]string{
		"content-type": "application/json; charset=utf-8",
		"host":         host,
	}
	var keys []string
	for k := range headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var canonicalHeaders strings.Builder
	for _, k := range keys {
		canonicalHeaders.WriteString(k)
		canonicalHeaders.WriteString(":")
		canonicalHeaders.WriteString(headers[k])
		canonicalHeaders.WriteString("\n")
	}
	signedHeaders := strings.Join(keys, ";")
	canonicalRequest := strings.Join([]string{
		http.MethodPost,
		"/",
		"",
		canonicalHeaders.String(),
		signedHeaders,
		hashedPayload,
	}, "\n")

	date := now.Format("2006-01-02")
	credentialScope := date + "/" + serviceName + "/tc3_request"
	stringToSign := strings.Join([]string{
		"TC3-HMAC-SHA256",
		strconv.FormatInt(now.Unix(), 10),
		credentialScope,
		sha256Hex(canonicalRequest),
	}, "\n")

	secretDate := hmacSHA256([]byte("TC3"+c.opts.SecretKey), date)
	secretService := hmacSHA256(secretDate, serviceName)
	secretSigning := hmacSHA256(secretService, "tc3_request")
	signature := hex.EncodeToString(hmacSHA256(secretSigning, stringToSign))

	return fmt.Sprintf("TC3-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		c.opts.SecretID, credentialScope, signedHeaders, signature)
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key []byte, msg string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(msg))
	return mac.Sum(nil)
}

type responseEnvelope struct {
	Response responseBody `json:"Response"`
}

type responseBody struct {
	Error          *responseError  `json:"Error,omitempty"`
	RequestID      string          `json:"RequestId,omitempty"`
	TextDetections []textDetection `json:"TextDetections,omitempty"`
}

type responseError struct {
	Code    string `json:"Code"`
	Message string `json:"Message"`
}

type textDetection struct {
	DetectedText string      `json:"DetectedText"`
	Confidence   float64     `json:"Confidence"`
	Language     string      `json:"Language"`
	Angel        float64     `json:"Angel"`
	Angle        float64     `json:"Angle"`
	ItemPolygon  itemPolygon `json:"ItemPolygon"`
	Polygon      []point     `json:"Polygon"`
}

type itemPolygon struct {
	X      int `json:"X"`
	Y      int `json:"Y"`
	Width  int `json:"Width"`
	Height int `json:"Height"`
}

type point struct {
	X int `json:"X"`
	Y int `json:"Y"`
}

func convertResponse(apiName string, minConfidence float64, resp responseBody) *DetectResult {
	blocks := make([]DetectBlock, 0, len(resp.TextDetections))
	filtered := 0
	detectedLanguage := ""
	for i, item := range resp.TextDetections {
		text := strings.TrimSpace(item.DetectedText)
		if text == "" {
			continue
		}
		conf := normalizeConfidence(item.Confidence)
		if conf > 0 && minConfidence > 0 && conf < minConfidence {
			filtered++
			continue
		}
		if detectedLanguage == "" && strings.TrimSpace(item.Language) != "" {
			detectedLanguage = strings.TrimSpace(item.Language)
		}
		polygon := make([]DetectPoint, 0, len(item.Polygon))
		for _, p := range item.Polygon {
			polygon = append(polygon, DetectPoint{X: p.X, Y: p.Y})
		}
		angle := item.Angel
		if angle == 0 {
			angle = item.Angle
		}
		blocks = append(blocks, DetectBlock{
			ID:         fmt.Sprintf("tencent_%d", i+1),
			Text:       text,
			Confidence: conf,
			BBox: DetectBBox{
				X:      item.ItemPolygon.X,
				Y:      item.ItemPolygon.Y,
				Width:  item.ItemPolygon.Width,
				Height: item.ItemPolygon.Height,
			},
			Polygon:   polygon,
			Angle:     angle,
			Direction: "horizontal",
		})
	}
	if detectedLanguage == "" {
		detectedLanguage = "zh"
	}
	if len(blocks) == 0 {
		return &DetectResult{
			Provider:            "tencent",
			APIName:             apiName,
			DetectedLanguage:    detectedLanguage,
			Blocks:              blocks,
			FilteredBlocksCount: filtered,
			Raw: map[string]any{
				"requestId": resp.RequestID,
			},
		}
	}
	return &DetectResult{
		Provider:            "tencent",
		APIName:             apiName,
		DetectedLanguage:    detectedLanguage,
		Blocks:              blocks,
		FilteredBlocksCount: filtered,
		Raw: map[string]any{
			"requestId": resp.RequestID,
		},
	}
}

func normalizeConfidence(v float64) float64 {
	if v > 1 {
		return v / 100
	}
	return v
}

func mapTencentError(code, msg, requestID string) error {
	code = strings.TrimSpace(code)
	lower := strings.ToLower(code + " " + msg)
	switch {
	case strings.Contains(lower, "unauthorizedoperation"):
		return ocrerror.NewWithRequestID(ocrerror.CodePermissionDenied, "当前账号没有 OCR 调用权限，请检查 CAM 权限", requestID)
	case strings.Contains(lower, "authfailure"):
		return ocrerror.NewWithRequestID(ocrerror.CodeAuthFailed, "当前 SecretId / SecretKey 无效，请检查密钥配置", requestID)
	case strings.Contains(lower, "unsupportedoperation") || strings.Contains(lower, "not activated") || strings.Contains(lower, "not enabled"):
		return ocrerror.NewWithRequestID(ocrerror.CodeServiceNotOpen, "腾讯云 OCR 服务未开通，请先到腾讯云控制台开通文字识别 OCR 服务", requestID)
	case strings.Contains(lower, "insufficientbalance") || strings.Contains(lower, "resourcesoldout") || strings.Contains(lower, "arrears"):
		return ocrerror.NewWithRequestID(ocrerror.CodePermissionDenied, "当前账号欠费或资源包已用尽，请检查腾讯云费用状态", requestID)
	case strings.Contains(lower, "limitexceeded") || strings.Contains(lower, "requestlimitexceeded") || strings.Contains(lower, "too many"):
		return ocrerror.NewWithRequestID(ocrerror.CodeRateLimited, "腾讯云 OCR 调用频率过高，请降低批量并发或稍后重试", requestID)
	case strings.Contains(lower, "invalidparameter") && strings.Contains(lower, "image"):
		return ocrerror.NewWithRequestID(ocrerror.CodeImageURLInaccessible, "图片地址无法被腾讯云 OCR 访问，请先上传到当前存储服务后再识别", requestID)
	default:
		return ocrerror.NewWithRequestID(ocrerror.CodeUnknown, "腾讯云 OCR 调用失败："+strings.TrimSpace(msg), requestID)
	}
}
