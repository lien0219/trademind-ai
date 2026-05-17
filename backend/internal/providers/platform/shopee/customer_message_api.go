// Shopee Open API v2 — seller chat (conversation / message / send).
// Request paths are centralized in paths.go; response field names may evolve — align parsers with Partner Center docs.
package shopee

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

func shopeeCombinedErrString(root map[string]any) string {
	if root == nil {
		return ""
	}
	var b strings.Builder
	if v, ok := root["error"]; ok && v != nil {
		b.WriteString(strings.ToLower(strings.TrimSpace(fmt.Sprint(v))))
	}
	if msg, ok := root["message"].(string); ok && strings.TrimSpace(msg) != "" {
		if b.Len() > 0 {
			b.WriteString(" ")
		}
		b.WriteString(strings.ToLower(strings.TrimSpace(msg)))
	}
	return b.String()
}

func isPermissionLikeShopeeMessage(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	return strings.Contains(s, "permission") ||
		strings.Contains(s, "forbidden") ||
		strings.Contains(s, "unauthorized") ||
		strings.Contains(s, "not authorized") ||
		strings.Contains(s, "access denied") ||
		strings.Contains(s, "invalid access") ||
		strings.Contains(s, "scope")
}

// classifyShopeeCustomerMessageError maps HTTP / body errors to permission, retryable, or passthrough.
func classifyShopeeCustomerMessageError(httpStatus int, root map[string]any, err error) error {
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
		return fmt.Errorf("shopee customer message: retryable: %w", err)
	}
	switch httpStatus {
	case http.StatusUnauthorized, http.StatusForbidden:
		return platformp.ErrPlatformCustomerMessagePermissionDenied
	case http.StatusTooManyRequests:
		return fmt.Errorf("shopee customer message: retryable: %w", err)
	default:
		if httpStatus >= 500 {
			return fmt.Errorf("shopee customer message: retryable: %w", err)
		}
	}
	combined := shopeeCombinedErrString(root)
	if isPermissionLikeShopeeMessage(combined) {
		return platformp.ErrPlatformCustomerMessagePermissionDenied
	}
	low := strings.ToLower(err.Error())
	if isPermissionLikeShopeeMessage(low) {
		return platformp.ErrPlatformCustomerMessagePermissionDenied
	}
	if strings.Contains(low, "error_auth") || strings.Contains(combined, "error_auth") {
		return platformp.ErrPlatformCustomerMessagePermissionDenied
	}
	return err
}
