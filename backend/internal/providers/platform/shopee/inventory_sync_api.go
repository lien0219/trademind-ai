package shopee

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

func classifyShopeeInventoryHTTP(status int) error {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return platformp.ErrPlatformInventorySyncPermissionDenied
	case http.StatusTooManyRequests:
		return fmt.Errorf("shopee inventory sync: retryable: rate limited (http 429)")
	default:
		if status >= 500 {
			return fmt.Errorf("shopee inventory sync: retryable: upstream error (http %d)", status)
		}
	}
	return nil
}

func isPermissionLikeInventoryMsg(msg string) bool {
	s := strings.ToLower(strings.TrimSpace(msg))
	return strings.Contains(s, "permission") ||
		strings.Contains(s, "forbidden") ||
		strings.Contains(s, "access denied") ||
		strings.Contains(s, "unauthorized") ||
		strings.Contains(s, "not authorized") ||
		strings.Contains(s, "invalid_access_token") ||
		strings.Contains(s, "invalid access_token") ||
		strings.Contains(s, "no permission") ||
		strings.Contains(s, "no_permission") ||
		strings.Contains(s, "error_auth")
}

func mapShopeeInventorySyncErr(httpStatus int, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, platformp.ErrPlatformInventorySyncPermissionDenied) {
		return err
	}
	if isPermissionLikeInventoryMsg(err.Error()) {
		return platformp.ErrPlatformInventorySyncPermissionDenied
	}
	if er := classifyShopeeInventoryHTTP(httpStatus); er != nil {
		return er
	}
	return err
}

func maybeRetryableInventoryTransportErr(err error) error {
	if err == nil {
		return nil
	}
	s := strings.ToLower(err.Error())
	if strings.Contains(s, "timeout") || strings.Contains(s, "timed out") ||
		strings.Contains(s, "connection reset") || strings.Contains(s, "eof") {
		return fmt.Errorf("shopee inventory sync: retryable: %w", err)
	}
	return err
}
