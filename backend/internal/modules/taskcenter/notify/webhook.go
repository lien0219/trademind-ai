package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SendWebhook posts a JSON payload to the configured webhook.
func SendWebhook(ctx context.Context, d WebhookDeps, payload AlertNotificationPayload) AlertNotificationResult {
	res := AlertNotificationResult{Channel: "webhook"}
	rawURL := strings.TrimSpace(d.URL)
	if rawURL == "" {
		res.Status = "skipped"
		res.ErrorMessage = "webhook url empty"
		return res
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		res.Status = "failed"
		res.ErrorMessage = "invalid webhook url"
		return res
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
	case "http":
		if !d.AllowHTTP {
			res.Status = "skipped"
			res.ErrorMessage = "http not allowed"
			res.Target = maskWebhookTarget(u)
			return res
		}
	default:
		res.Status = "failed"
		res.ErrorMessage = "only http/https"
		return res
	}

	res.Target = maskWebhookTarget(u)

	bodyMap := map[string]any{
		"source":          "trademind",
		"event":           "task_alert.generated",
		"alertId":         payload.AlertID,
		"severity":        payload.Severity,
		"failureCategory": payload.FailureCategory,
		"taskType":        payload.TaskType,
		"title":           truncateStr(payload.Title, 240),
		"message":         truncateStr(payload.Message, 500),
		"suggestedAction": truncateStr(payload.SuggestedAction, 400),
		"detailUrl":       payload.DetailURL,
		"occurredAt":      payload.OccurredAtRFC3339,
	}
	bodyBytes, err := json.Marshal(bodyMap)
	if err != nil {
		res.Status = "failed"
		res.ErrorMessage = "encode json"
		return res
	}

	method := strings.TrimSpace(strings.ToUpper(d.Method))
	if method == "" {
		method = http.MethodPost
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, bytes.NewReader(bodyBytes))
	if err != nil {
		res.Status = "failed"
		res.ErrorMessage = truncateStr(err.Error(), 400)
		return res
	}
	req.Header.Set("Content-Type", "application/json")
	if sec := strings.TrimSpace(d.Secret); sec != "" {
		mac := hmac.New(sha256.New, []byte(sec))
		_, _ = mac.Write(bodyBytes)
		sig := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-TradeMind-Signature", sig)
	}

	timeout := d.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= 2 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		res.Status = "failed"
		res.ErrorMessage = truncateStr(err.Error(), 500)
		return res
	}
	defer resp.Body.Close()
	lim := io.LimitReader(resp.Body, 4096)
	respBody, _ := io.ReadAll(lim)

	raw := map[string]any{
		"httpStatus": resp.StatusCode,
		"bodySample": truncateStr(string(respBody), 400),
	}
	res.RawSummary = raw

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		res.Status = "failed"
		res.ErrorMessage = fmt.Sprintf("http %d", resp.StatusCode)
		return res
	}
	res.Status = "success"
	return res
}

func maskWebhookTarget(u *url.URL) string {
	if u == nil {
		return ""
	}
	h := sha256.Sum256([]byte(u.Host + u.Path))
	return fmt.Sprintf("%s#%s", u.Host, hex.EncodeToString(h[:6]))
}
