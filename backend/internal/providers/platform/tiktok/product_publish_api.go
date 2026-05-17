package tiktok

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

func decodeProductAPIResponse(raw []byte, httpStatus int) (map[string]interface{}, error) {
	if err := classifyProductHTTPError(httpStatus); err != nil {
		return nil, err
	}
	root, err := firstJSONMap(raw)
	if err != nil {
		return nil, fmt.Errorf("tiktok product publish: invalid json response")
	}
	if err := classifyProductBusinessError(root); err != nil {
		return root, err
	}
	return root, nil
}

func classifyProductHTTPError(status int) error {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return platformp.ErrPlatformProductPublishPermissionDenied
	case http.StatusTooManyRequests:
		return fmt.Errorf("tiktok product publish: retryable rate limit (http 429)")
	default:
		if status >= 500 {
			return fmt.Errorf("tiktok product publish: retryable upstream error (http %d)", status)
		}
	}
	return nil
}

func classifyProductBusinessError(root map[string]interface{}) error {
	if root == nil {
		return nil
	}
	codeRaw, hasCode := root["code"]
	if !hasCode {
		return nil
	}
	ok := false
	switch c := codeRaw.(type) {
	case float64:
		ok = int(c) == 0
	case string:
		ok = strings.TrimSpace(c) == "0"
	default:
		s := strings.TrimSpace(fmt.Sprint(c))
		ok = s == "0" || s == ""
	}
	if ok {
		return nil
	}
	msg := strings.ToLower(strings.TrimSpace(fmt.Sprint(root["message"])))
	if msg == "" {
		msg = "tiktok api error"
	}
	full := fmt.Sprintf("tiktok product publish: %s", msg)
	if isPermissionLikePublishError(msg) {
		return platformp.ErrPlatformProductPublishPermissionDenied
	}
	if isRetryablePublishBusinessHint(msg) {
		return fmt.Errorf("tiktok product publish: retryable: %s", msg)
	}
	return fmt.Errorf("%s", full)
}

func isPermissionLikePublishError(msg string) bool {
	return strings.Contains(msg, "permission") ||
		strings.Contains(msg, "forbidden") ||
		strings.Contains(msg, "access denied") ||
		strings.Contains(msg, "unauthorized") ||
		strings.Contains(msg, "not authorized") ||
		strings.Contains(msg, "scope") ||
		strings.Contains(msg, "no permission")
}

func isRetryablePublishBusinessHint(msg string) bool {
	return strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "too many requests") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "system error") ||
		strings.Contains(msg, "internal error") ||
		strings.Contains(msg, "service unavailable")
}

func extractProductAPIData(root map[string]interface{}) map[string]interface{} {
	if root == nil {
		return map[string]interface{}{}
	}
	if d, ok := root["data"].(map[string]interface{}); ok && d != nil {
		return d
	}
	return map[string]interface{}{}
}

func bizCodeSummary(root map[string]interface{}) string {
	if root == nil {
		return ""
	}
	switch c := root["code"].(type) {
	case float64:
		return strconv.FormatInt(int64(c), 10)
	case string:
		return strings.TrimSpace(c)
	default:
		return strings.TrimSpace(fmt.Sprint(c))
	}
}
