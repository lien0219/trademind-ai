package lazada

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

func lazadaHTTPAndBodySummary(httpStatus int, root map[string]any, err error) string {
	var b strings.Builder
	if httpStatus > 0 {
		fmt.Fprintf(&b, "http=%d ", httpStatus)
	}
	if root != nil {
		if msg := strings.TrimSpace(pickStr(root, "message", "detail", "msg")); msg != "" {
			b.WriteString(strings.ToLower(msg))
			b.WriteByte(' ')
		}
		b.WriteString(strings.ToLower(fmt.Sprint(root["code"])))
	}
	if err != nil {
		b.WriteByte(' ')
		b.WriteString(strings.ToLower(err.Error()))
	}
	return strings.TrimSpace(b.String())
}

func isPermissionLikeLazadaMessage(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return false
	}
	return strings.Contains(s, "permission") ||
		strings.Contains(s, "forbidden") ||
		strings.Contains(s, "illegalaccesstoken") ||
		strings.Contains(s, "illegal_access_token") ||
		strings.Contains(s, "invalid_access_token") ||
		strings.Contains(s, "not authorized") ||
		strings.Contains(s, "access denied") ||
		strings.Contains(s, "unauthorized") ||
		strings.Contains(s, "scope") ||
		strings.Contains(s, "no privilege") ||
		strings.Contains(s, "reject") && strings.Contains(s, "api") ||
		strings.Contains(s, "seller api permission") ||
		strings.Contains(s, "does not apply") ||
		strings.Contains(s, "need authorize") ||
		strings.Contains(s, "authorization") && strings.Contains(s, "fail")
}

// classifyLazadaCustomerMessageError maps HTTP / LazOpen JSON failures for buyer–seller IM APIs.
func classifyLazadaCustomerMessageError(httpStatus int, root map[string]any, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, platformp.ErrPlatformCustomerMessagePermissionDenied) {
		return err
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return err
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return fmt.Errorf("lazada customer message: retryable: %w", err)
	}
	switch httpStatus {
	case http.StatusUnauthorized, http.StatusForbidden:
		return platformp.ErrPlatformCustomerMessagePermissionDenied
	case http.StatusTooManyRequests:
		return fmt.Errorf("lazada customer message: retryable: %w", err)
	default:
		if httpStatus >= 500 {
			return fmt.Errorf("lazada customer message: retryable: %w", err)
		}
	}
	combined := lazadaHTTPAndBodySummary(httpStatus, root, err)
	if isPermissionLikeLazadaMessage(combined) {
		return platformp.ErrPlatformCustomerMessagePermissionDenied
	}
	if strings.Contains(strings.ToLower(err.Error()), "retryable") {
		return err
	}
	return err
}
