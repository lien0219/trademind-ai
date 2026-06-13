package storagepublic

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/pkg/httppublic"
)

const (
	maxProbeBodyBytes = 8 << 20
	minImageBytes     = 32
	maxImageBytes     = 20 << 20
)

// ProbeResult is a sanitized diagnostic result (no secrets).
type ProbeResult struct {
	OK               bool           `json:"ok"`
	PublicURL        string         `json:"publicUrl,omitempty"`
	StatusCode       int            `json:"statusCode,omitempty"`
	ContentType      string         `json:"contentType,omitempty"`
	ContentLength    int64          `json:"contentLength,omitempty"`
	ImageWidth       int            `json:"imageWidth,omitempty"`
	ImageHeight      int            `json:"imageHeight,omitempty"`
	ErrorCode        string         `json:"errorCode,omitempty"`
	Message          string         `json:"message,omitempty"`
	TechnicalDetails map[string]any `json:"technicalDetails,omitempty"`
}

// VerifyPublicURL probes rawURL with an anonymous HTTP client (no cookies, no auth headers).
func VerifyPublicURL(ctx context.Context, rawURL string) ProbeResult {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return fail(CodePublicBaseMissing, "未配置图片公开访问地址", map[string]any{"field": "public_base"})
	}
	if !strings.Contains(rawURL, "://") {
		return fail(CodePublicURLInvalid, "图片地址不是完整的公网 URL，请配置 HTTPS 域名", map[string]any{
			"publicUrl": redactURL(rawURL),
			"hint":      "相对路径（如 /static）仅适用于开发代理，抖店等外部平台无法访问",
		})
	}
	if !httppublic.IsPublicHTTPURL(rawURL) {
		return fail(CodePublicURLPrivate, "图片地址指向本机或私网，外部平台无法访问", map[string]any{
			"publicUrl": redactURL(rawURL),
		})
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fail(CodePublicURLInvalid, "图片地址格式无效", map[string]any{"publicUrl": redactURL(rawURL)})
	}
	if strings.ToLower(u.Scheme) != "https" {
		return fail(CodePublicURLInvalid, "图片地址须使用 HTTPS，外部平台才能稳定访问", map[string]any{
			"publicUrl": redactURL(rawURL),
			"scheme":    u.Scheme,
		})
	}
	if err := assertHostNotPrivate(ctx, u.Hostname()); err != nil {
		return fail(CodePublicURLPrivate, "图片域名解析到私网地址，外部平台无法访问", map[string]any{
			"publicUrl": redactURL(rawURL),
			"reason":    err.Error(),
		})
	}

	cli := anonymousHTTPClient()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fail(CodePublicAccessFailed, "无法发起图片访问请求", map[string]any{"error": err.Error()})
	}
	req.Header.Set("User-Agent", "TradeMind-Storage-Public-Probe/1.0")
	req.Header.Set("Accept", "image/*,*/*")

	resp, err := cli.Do(req)
	if err != nil {
		code := CodePublicAccessFailed
		msg := "无法访问图片地址，请检查域名、证书和存储权限"
		details := map[string]any{"error": err.Error()}
		var tlsErr *tls.CertificateVerificationError
		if errors.As(err, &tlsErr) || strings.Contains(strings.ToLower(err.Error()), "certificate") {
			code = CodePublicCertificateInvalid
			msg = "HTTPS 证书无效或不受信任，外部平台可能无法访问图片"
		}
		return fail(code, msg, details)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		loc := strings.TrimSpace(resp.Header.Get("Location"))
		return fail(CodePublicRedirected, "图片地址发生跳转，外部平台可能无法稳定访问", map[string]any{
			"statusCode": resp.StatusCode,
			"location":   redactURL(loc),
		})
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fail(CodePublicAccessFailed, "图片地址返回异常状态，外部平台无法正常读取", map[string]any{
			"statusCode": resp.StatusCode,
			"publicUrl":  redactURL(rawURL),
		})
	}

	ct := strings.TrimSpace(resp.Header.Get("Content-Type"))
	ctLower := strings.ToLower(strings.Split(ct, ";")[0])
	if ctLower != "" && !strings.HasPrefix(ctLower, "image/") {
		if looksLikeLoginHTML(resp) {
			return fail(CodePublicRedirected, "图片地址跳转到登录页，外部平台无法匿名访问", map[string]any{
				"contentType": ct,
			})
		}
		return fail(CodePublicContentTypeInvalid, "返回内容不是图片格式，外部平台可能无法识别", map[string]any{
			"contentType": ct,
		})
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxProbeBodyBytes+1))
	if err != nil {
		return fail(CodePublicAccessFailed, "读取图片内容失败", map[string]any{"error": err.Error()})
	}
	if len(data) > maxProbeBodyBytes {
		return fail(CodePublicAccessFailed, "图片体积过大", map[string]any{"bytesRead": len(data)})
	}
	if int64(len(data)) < minImageBytes {
		return fail(CodePublicAccessFailed, "图片内容过短，可能不是有效图片", map[string]any{"bytesRead": len(data)})
	}

	cfg, format, decErr := image.DecodeConfig(bytes.NewReader(data))
	if decErr != nil {
		return fail(CodePublicImageDecodeFailed, "图片无法解码，外部平台可能无法使用", map[string]any{
			"error": decErr.Error(),
		})
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return fail(CodePublicImageDecodeFailed, "图片尺寸无效", nil)
	}

	return ProbeResult{
		OK:            true,
		PublicURL:     redactURL(rawURL),
		StatusCode:    resp.StatusCode,
		ContentType:   ct,
		ContentLength: int64(len(data)),
		ImageWidth:    cfg.Width,
		ImageHeight:   cfg.Height,
		Message:       "图片存储可以被外部平台正常访问",
		TechnicalDetails: map[string]any{
			"format":      format,
			"bytesRead":   len(data),
			"contentType": ct,
		},
	}
}

func anonymousHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout: 10 * time.Second,
			MaxIdleConns:        4,
		},
	}
}

func assertHostNotPrivate(ctx context.Context, host string) error {
	host = strings.TrimSpace(host)
	if host == "" {
		return fmt.Errorf("empty host")
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
			return fmt.Errorf("literal private IP")
		}
		return nil
	}
	resolver := net.Resolver{}
	ips, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		// DNS failure is not private-network proof; allow probe to continue.
		return nil
	}
	for _, ia := range ips {
		if ia.IP.IsLoopback() || ia.IP.IsPrivate() || ia.IP.IsLinkLocalUnicast() {
			return fmt.Errorf("resolved private IP")
		}
	}
	return nil
}

func looksLikeLoginHTML(resp *http.Response) bool {
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if !strings.Contains(ct, "text/html") {
		return false
	}
	peek, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	body := strings.ToLower(string(peek))
	keywords := []string{"login", "signin", "sign-in", "登录", "auth", "unauthorized"}
	for _, kw := range keywords {
		if strings.Contains(body, kw) {
			return true
		}
	}
	return false
}

func fail(code, msg string, details map[string]any) ProbeResult {
	return ProbeResult{
		OK:               false,
		ErrorCode:        code,
		Message:          msg,
		TechnicalDetails: details,
	}
}

func redactURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if u.User != nil {
		u.User = url.UserPassword("****", "****")
	}
	q := u.Query()
	for k := range q {
		kl := strings.ToLower(k)
		if strings.Contains(kl, "secret") || strings.Contains(kl, "token") || strings.Contains(kl, "signature") || strings.Contains(kl, "accesskey") {
			q.Set(k, "****")
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}
