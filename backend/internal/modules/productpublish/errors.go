package productpublish

import (
	"errors"
	"strings"
)

// ErrRedisQueueUnavailable indicates LIST operations failed while queue mode is enabled.
var ErrRedisQueueUnavailable = errors.New("Redis queue unavailable")

const (
	ErrorPublishCheckFailed   = "PUBLISH_CHECK_FAILED"
	ErrorPriceInvalid         = "PRICE_INVALID"
	ErrorImageMissing         = "IMAGE_MISSING"
	ErrorSKUInvalid           = "SKU_INVALID"
	ErrorStoreNotConfigured   = "STORE_NOT_CONFIGURED"
	ErrorPlatformAuthRequired = "PLATFORM_AUTH_REQUIRED"
	ErrorPlatformAPI          = "PLATFORM_API_ERROR"
	ErrorUnknownPublish       = "UNKNOWN_PUBLISH_ERROR"
)

func inferPublishErrorCode(msg string) string {
	m := strings.ToLower(strings.TrimSpace(msg))
	switch {
	case strings.Contains(m, "readiness") || strings.Contains(m, "check failed"):
		return ErrorPublishCheckFailed
	case strings.Contains(m, "price") || strings.Contains(m, "cost") || strings.Contains(m, "profit"):
		return ErrorPriceInvalid
	case strings.Contains(m, "image") || strings.Contains(m, "main image"):
		return ErrorImageMissing
	case strings.Contains(m, "sku"):
		return ErrorSKUInvalid
	case strings.Contains(m, "shop not") || strings.Contains(m, "store") || strings.Contains(m, "publish config") || strings.Contains(m, "warehouse") || strings.Contains(m, "marketplace"):
		return ErrorStoreNotConfigured
	case strings.Contains(m, "token") || strings.Contains(m, "auth") || strings.Contains(m, "unauthorized") || strings.Contains(m, "permission") || strings.Contains(m, "403"):
		return ErrorPlatformAuthRequired
	case strings.Contains(m, "platform") || strings.Contains(m, "api") || strings.Contains(m, "500") || strings.Contains(m, "502") || strings.Contains(m, "503") || strings.Contains(m, "504"):
		return ErrorPlatformAPI
	default:
		return ErrorUnknownPublish
	}
}
