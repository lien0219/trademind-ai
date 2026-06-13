package douyinshop

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func (c *Client) do(ctx context.Context, method string, params map[string]any, accessToken string, out any) error {
	started := time.Now()
	paramJSON, err := json.Marshal(normalizeParams(params))
	if err != nil {
		return err
	}
	u, err := url.Parse(c.apiBaseURL() + "/" + strings.ReplaceAll(strings.TrimSpace(method), ".", "/"))
	if err != nil {
		return err
	}
	common := map[string]string{
		"app_key":     strings.TrimSpace(c.Config.AppKey),
		"method":      strings.TrimSpace(method),
		"param_json":  string(paramJSON),
		"sign_method": "hmac-sha256",
		"timestamp":   strconv.FormatInt(c.now().Unix(), 10),
		"v":           "2",
	}
	if at := strings.TrimSpace(accessToken); at != "" {
		common["access_token"] = at
	}
	common["sign"] = Sign(common, c.Config.AppSecret)

	q := u.Query()
	for k, v := range common {
		if k == "param_json" {
			continue
		}
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(paramJSON))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	var platformCode, requestID string
	resp, err := c.httpClient().Do(req)
	if err != nil {
		outErr := mapTransportError(err)
		c.logRequest(ctx, SafeRequestLog{
			Method:    method,
			ElapsedMs: time.Since(started).Milliseconds(),
			Success:   false,
			ErrorCode: outErr.Code,
		})
		return outErr
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	requestID = strings.TrimSpace(resp.Header.Get("X-Tt-Logid"))
	if requestID == "" {
		requestID = strings.TrimSpace(resp.Header.Get("X-Request-Id"))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		outErr := MapHTTPError(resp.StatusCode, requestID)
		if HTTPStatusRetryable(resp.StatusCode) {
			outErr.Retryable = true
		}
		retryAfter := ParseRetryAfter(resp.Header.Get("Retry-After"))
		if retryAfter > 0 && outErr.RateLimited {
			_ = retryAfter
		}
		c.logRequest(ctx, SafeRequestLog{
			Method:       method,
			RequestID:    requestID,
			ElapsedMs:    time.Since(started).Milliseconds(),
			PlatformCode: outErr.PlatformCode,
			Success:      false,
			ErrorCode:    outErr.Code,
		})
		return outErr
	}

	env, err := parseEnvelope(body)
	if err != nil {
		var de *Error
		_ = errors.As(err, &de)
		c.logRequest(ctx, SafeRequestLog{
			Method:    method,
			ElapsedMs: time.Since(started).Milliseconds(),
			Success:   false,
			ErrorCode: codeOrUnknown(de),
		})
		return err
	}
	if env.RequestID != "" {
		requestID = env.RequestID
	}
	platformCode = env.Code
	if !env.success() {
		outErr := MapPlatformError(env.Code, env.Message, requestID)
		c.logRequest(ctx, SafeRequestLog{
			Method:       method,
			RequestID:    requestID,
			ElapsedMs:    time.Since(started).Milliseconds(),
			PlatformCode: platformCode,
			Success:      false,
			ErrorCode:    outErr.Code,
		})
		return outErr
	}
	if err := env.decodeData(out); err != nil {
		var de *Error
		_ = errors.As(err, &de)
		c.logRequest(ctx, SafeRequestLog{
			Method:       method,
			RequestID:    requestID,
			ElapsedMs:    time.Since(started).Milliseconds(),
			PlatformCode: platformCode,
			Success:      false,
			ErrorCode:    codeOrUnknown(de),
		})
		return err
	}
	c.logRequest(ctx, SafeRequestLog{
		Method:       method,
		RequestID:    requestID,
		ElapsedMs:    time.Since(started).Milliseconds(),
		PlatformCode: platformCode,
		Success:      true,
	})
	return nil
}

func normalizeParams(params map[string]any) map[string]any {
	if params == nil {
		return map[string]any{}
	}
	return params
}

func mapTransportError(err error) *Error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return NewError(CodeDouyinRequestTimeout, "douyin openapi request timeout", "", err.Error(), "")
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return NewError(CodeDouyinRequestTimeout, "douyin openapi request timeout", "", err.Error(), "")
	}
	return NewError(CodeUnknownDouyinError, "douyin openapi request failed", "", err.Error(), "")
}

func codeOrUnknown(e *Error) string {
	if e == nil || strings.TrimSpace(e.Code) == "" {
		return CodeUnknownDouyinError
	}
	return e.Code
}
