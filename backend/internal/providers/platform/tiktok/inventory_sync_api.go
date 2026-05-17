package tiktok

import (
	"fmt"
	"net/http"
	"strings"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

func classifyInventoryHTTPError(status int) error {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return platformp.ErrPlatformInventorySyncPermissionDenied
	case http.StatusTooManyRequests:
		return fmt.Errorf("tiktok inventory sync: retryable rate limit (http 429)")
	default:
		if status >= 500 {
			return fmt.Errorf("tiktok inventory sync: retryable upstream error (http %d)", status)
		}
	}
	return nil
}

func classifyInventoryBusinessError(root map[string]interface{}) error {
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
	full := fmt.Sprintf("tiktok inventory sync: %s", strings.TrimSpace(fmt.Sprint(root["message"])))
	if isPermissionLikeInventoryError(msg) {
		return platformp.ErrPlatformInventorySyncPermissionDenied
	}
	if isRetryableInventoryBusinessHint(msg) {
		return fmt.Errorf("tiktok inventory sync: retryable: %s", msg)
	}
	return fmt.Errorf("%s", full)
}

func decodeInventoryAPIResponse(raw []byte, httpStatus int) (map[string]interface{}, error) {
	if err := classifyInventoryHTTPError(httpStatus); err != nil {
		return nil, err
	}
	root, err := firstJSONMap(raw)
	if err != nil {
		return nil, fmt.Errorf("tiktok inventory sync: invalid json response")
	}
	if err := classifyInventoryBusinessError(root); err != nil {
		return root, err
	}
	return root, nil
}

func isPermissionLikeInventoryError(msg string) bool {
	return strings.Contains(msg, "permission") ||
		strings.Contains(msg, "forbidden") ||
		strings.Contains(msg, "access denied") ||
		strings.Contains(msg, "unauthorized") ||
		strings.Contains(msg, "not authorized") ||
		strings.Contains(msg, "scope") ||
		strings.Contains(msg, "no permission")
}

func isRetryableInventoryBusinessHint(msg string) bool {
	return strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "too many requests") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "system error") ||
		strings.Contains(msg, "internal error") ||
		strings.Contains(msg, "service unavailable")
}

func maybeRetryableInventoryTransportErr(err error) error {
	if err == nil {
		return nil
	}
	s := strings.ToLower(err.Error())
	if strings.Contains(s, "timeout") || strings.Contains(s, "timed out") ||
		strings.Contains(s, "connection reset") || strings.Contains(s, "eof") {
		return fmt.Errorf("tiktok inventory sync: retryable: %w", err)
	}
	return err
}
