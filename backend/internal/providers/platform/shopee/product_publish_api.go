package shopee

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

func classifyShopeePublishHTTP(status int) error {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return platformp.ErrPlatformProductPublishPermissionDenied
	case http.StatusTooManyRequests:
		return fmt.Errorf("shopee product publish: retryable: rate limited (http 429)")
	default:
		if status >= 500 {
			return fmt.Errorf("shopee product publish: retryable: upstream error (http %d)", status)
		}
	}
	return nil
}

func isPermissionLikePublishMsg(msg string) bool {
	s := strings.ToLower(strings.TrimSpace(msg))
	return strings.Contains(s, "permission") ||
		strings.Contains(s, "forbidden") ||
		strings.Contains(s, "access denied") ||
		strings.Contains(s, "unauthorized") ||
		strings.Contains(s, "not authorized") ||
		strings.Contains(s, "invalid_access_token") ||
		strings.Contains(s, "no permission") ||
		strings.Contains(s, "no_permission")
}

func mapShopeePublishErr(httpStatus int, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, platformp.ErrPlatformProductPublishPermissionDenied) {
		return err
	}
	if isPermissionLikePublishMsg(err.Error()) {
		return platformp.ErrPlatformProductPublishPermissionDenied
	}
	if er := classifyShopeePublishHTTP(httpStatus); er != nil {
		return er
	}
	return err
}

func maybeRetryableTransportErr(err error) error {
	if err == nil {
		return nil
	}
	s := strings.ToLower(err.Error())
	if strings.Contains(s, "timeout") || strings.Contains(s, "timed out") ||
		strings.Contains(s, "connection reset") || strings.Contains(s, "eof") {
		return fmt.Errorf("shopee product publish: retryable: %w", err)
	}
	return err
}

func extractMediaImageID(m map[string]any) string {
	if m == nil {
		return ""
	}
	if ii, ok := m["image_info"].(map[string]any); ok && ii != nil {
		return strings.TrimSpace(strField(ii, "image_id"))
	}
	return strings.TrimSpace(strField(m, "image_id"))
}

func extractItemIDUint(m map[string]any) uint64 {
	if m == nil {
		return 0
	}
	return parseUint64Merged(strField(m, "item_id"))
}

func extractModelRows(resp map[string]any) []map[string]any {
	if resp == nil {
		return nil
	}
	raw, ok := resp["model"]
	if !ok || raw == nil {
		return nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(arr))
	for _, x := range arr {
		row, ok := x.(map[string]any)
		if ok && row != nil {
			out = append(out, row)
		}
	}
	return out
}

func parseModelID(row map[string]any) uint64 {
	if row == nil {
		return 0
	}
	return parseUint64Merged(strField(row, "model_id"))
}
