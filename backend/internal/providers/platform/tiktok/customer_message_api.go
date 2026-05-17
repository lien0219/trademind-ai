// TikTok Shop Open API — Customer Service (buyer–seller messaging).
//
// Paths are built from settings.platform_tiktok.api_version (e.g. 202309 / 202407).
// Field names in JSON responses vary by API version — adjust parsers against Partner Center docs when upgrading.
package tiktok

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

// Open-API path templates (no host); version segment = cfg.APIVersion.
const (
	// tiktokCustomerConvSearchPath lists CS conversations for the authorized shop.
	// Doc: TikTok Shop Partner Center → Customer Service → Search conversations.
	tiktokCustomerConvSearchPath = "/customer_service/%s/conversations/search"
	// tiktokCustomerMessagesPathTemplate loads messages for one conversation ({0} = api version, {1} = conversation id).
	tiktokCustomerMessagesPathTemplate = "/customer_service/%s/conversations/%s/messages/search"
	// tiktokCustomerSendPathTemplate sends a seller/agent reply.
	tiktokCustomerSendPathTemplate = "/customer_service/%s/conversations/%s/messages/send"
)

func customerConvSearchPath(ver string) string {
	return fmt.Sprintf(tiktokCustomerConvSearchPath, strings.TrimSpace(ver))
}

func customerMessagesPath(ver, conversationID string) string {
	escaped := url.PathEscape(strings.TrimSpace(conversationID))
	return fmt.Sprintf(tiktokCustomerMessagesPathTemplate, strings.TrimSpace(ver), escaped)
}

func customerSendPath(ver, conversationID string) string {
	escaped := url.PathEscape(strings.TrimSpace(conversationID))
	return fmt.Sprintf(tiktokCustomerSendPathTemplate, strings.TrimSpace(ver), escaped)
}

func postCustomerServiceJSON(ctx context.Context, client http.Client, cfg RuntimeConfig, path, access string, body map[string]interface{}) (map[string]interface{}, int, error) {
	raw, httpStatus, err := signedPOSTJSONStatus(ctx, client, cfg, path, access, body)
	if err != nil {
		return nil, httpStatus, err
	}
	root, err := decodeCustomerServiceResponse(raw, httpStatus)
	if err != nil {
		return root, httpStatus, err
	}
	return root, httpStatus, nil
}

func decodeCustomerServiceResponse(raw []byte, httpStatus int) (map[string]interface{}, error) {
	if err := classifyCustomerHTTPError(httpStatus); err != nil {
		return nil, err
	}
	root, err := firstJSONMap(raw)
	if err != nil {
		return nil, fmt.Errorf("tiktok customer message: invalid json response")
	}
	if err := classifyCustomerBusinessError(root); err != nil {
		return root, err
	}
	return root, nil
}

func classifyCustomerHTTPError(status int) error {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return platformp.ErrPlatformCustomerMessagePermissionDenied
	case http.StatusTooManyRequests:
		return fmt.Errorf("tiktok customer message: retryable rate limit (http 429)")
	default:
		if status >= 500 {
			return fmt.Errorf("tiktok customer message: retryable upstream error (http %d)", status)
		}
	}
	return nil
}

func classifyCustomerBusinessError(root map[string]interface{}) error {
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
	full := fmt.Sprintf("tiktok customer message: %s", msg)
	if isPermissionLikeBusinessError(codeRaw, msg) {
		return platformp.ErrPlatformCustomerMessagePermissionDenied
	}
	if isRetryableBusinessHint(msg) {
		return fmt.Errorf("tiktok customer message: retryable: %s", msg)
	}
	return fmt.Errorf("%s", full)
}

func isPermissionLikeBusinessError(_ interface{}, msg string) bool {
	return strings.Contains(msg, "permission") || strings.Contains(msg, "forbidden") || strings.Contains(msg, "access denied") || strings.Contains(msg, "unauthorized") || strings.Contains(msg, "not authorized") || strings.Contains(msg, "scope")
}

func isRetryableBusinessHint(msg string) bool {
	return strings.Contains(msg, "rate limit") || strings.Contains(msg, "too many requests") || strings.Contains(msg, "timeout") || strings.Contains(msg, "system error") || strings.Contains(msg, "internal error")
}

// parseBizCodeStr returns TikTok Open API business code as string for summaries (never includes secrets).
func parseBizCodeStr(root map[string]interface{}) string {
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
