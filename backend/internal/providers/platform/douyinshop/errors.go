package douyinshop

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

const (
	CodeDouyinAPIError            = "DOUYIN_API_ERROR"
	CodeDouyinAuthExpired         = "DOUYIN_AUTH_EXPIRED"
	CodeDouyinTokenRefreshFailed  = "DOUYIN_TOKEN_REFRESH_FAILED"
	CodeDouyinPermissionDenied    = "DOUYIN_PERMISSION_DENIED"
	CodeDouyinRateLimited         = "DOUYIN_RATE_LIMITED"
	CodeDouyinRequestTimeout      = "DOUYIN_REQUEST_TIMEOUT"
	CodeDouyinResponseParseFailed = "DOUYIN_RESPONSE_PARSE_FAILED"
	CodeDouyinShopInfoFailed      = "DOUYIN_SHOP_INFO_FAILED"
	CodeUnknownDouyinError        = "UNKNOWN_DOUYIN_ERROR"
)

type Error struct {
	Code             string
	Message          string
	PlatformCode     string
	PlatformMessage  string
	RequestID        string
	Retryable        bool
	RateLimited      bool
	PermissionDenied bool
	AuthExpired      bool
}

func NewError(code, msg, platformCode, platformMsg, requestID string) *Error {
	e := &Error{
		Code:            strings.TrimSpace(code),
		Message:         strings.TrimSpace(msg),
		PlatformCode:    strings.TrimSpace(platformCode),
		PlatformMessage: SanitizeErrorText(platformMsg),
		RequestID:       strings.TrimSpace(requestID),
	}
	if e.Code == "" {
		e.Code = CodeUnknownDouyinError
	}
	if e.Message == "" {
		e.Message = e.Code
	}
	switch e.Code {
	case CodeDouyinAuthExpired:
		e.AuthExpired = true
	case CodeDouyinPermissionDenied:
		e.PermissionDenied = true
	case CodeDouyinRateLimited:
		e.RateLimited = true
		e.Retryable = true
	case CodeDouyinRequestTimeout:
		e.Retryable = true
	}
	return e
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	parts := []string{e.Code}
	if e.PlatformCode != "" {
		parts = append(parts, "platformCode="+e.PlatformCode)
	}
	if e.RequestID != "" {
		parts = append(parts, "requestId="+e.RequestID)
	}
	if e.Message != "" {
		parts = append(parts, e.Message)
	}
	return strings.Join(parts, " ")
}

func AsError(err error, target **Error) bool {
	return errors.As(err, target)
}

func SanitizeErrorText(raw string) string {
	msg := strings.TrimSpace(raw)
	if msg == "" {
		return ""
	}
	low := strings.ToLower(msg)
	for _, marker := range []string{"app_secret", "appsecret", "access_token", "accesstoken", "refresh_token", "refreshtoken", "secret", "token"} {
		if strings.Contains(low, marker) {
			return "douyin openapi returned a sensitive error"
		}
	}
	if len(msg) > 500 {
		msg = msg[:500] + "..."
	}
	return msg
}

func MapHTTPError(status int, requestID string) *Error {
	switch status {
	case http.StatusUnauthorized:
		return NewError(CodeDouyinAuthExpired, "douyin authorization expired", fmt.Sprint(status), "unauthorized", requestID)
	case http.StatusForbidden:
		return NewError(CodeDouyinPermissionDenied, "douyin permission denied", fmt.Sprint(status), "forbidden", requestID)
	case http.StatusTooManyRequests:
		return NewError(CodeDouyinRateLimited, "douyin openapi rate limited", fmt.Sprint(status), "rate limited", requestID)
	case http.StatusGatewayTimeout, http.StatusRequestTimeout:
		return NewError(CodeDouyinRequestTimeout, "douyin openapi request timeout", fmt.Sprint(status), "timeout", requestID)
	default:
		return NewError(CodeDouyinAPIError, "douyin openapi http error", fmt.Sprint(status), "", requestID)
	}
}

func MapPlatformError(platformCode, platformMsg, requestID string) *Error {
	pc := strings.TrimSpace(platformCode)
	pm := SanitizeErrorText(platformMsg)
	low := strings.ToLower(pc + " " + strings.TrimSpace(platformMsg))
	switch {
	case strings.Contains(low, "rate") || strings.Contains(low, "limit") || strings.Contains(low, "frequency"):
		return NewError(CodeDouyinRateLimited, "douyin openapi rate limited", pc, pm, requestID)
	case strings.Contains(low, "permission") || strings.Contains(low, "forbid") || strings.Contains(low, "unauthoriz"):
		return NewError(CodeDouyinPermissionDenied, "douyin permission denied", pc, pm, requestID)
	case strings.Contains(low, "refresh") && (strings.Contains(low, "expire") || strings.Contains(low, "invalid") || strings.Contains(low, "fail")):
		return NewError(CodeDouyinAuthExpired, "douyin authorization expired", pc, pm, requestID)
	case strings.Contains(low, "access_token") || strings.Contains(low, "token expired") || strings.Contains(low, "invalid token"):
		return NewError(CodeDouyinAuthExpired, "douyin authorization expired", pc, pm, requestID)
	default:
		return NewError(CodeDouyinAPIError, "douyin openapi error", pc, pm, requestID)
	}
}

func platformCodeOf(err error) string {
	var de *Error
	if errors.As(err, &de) {
		return de.PlatformCode
	}
	return ""
}

func requestIDOf(err error) string {
	var de *Error
	if errors.As(err, &de) {
		return de.RequestID
	}
	return ""
}

func safeMessageOf(err error) string {
	var de *Error
	if errors.As(err, &de) {
		if de.PlatformMessage != "" {
			return de.PlatformMessage
		}
		return de.Message
	}
	if err == nil {
		return ""
	}
	return SanitizeErrorText(err.Error())
}
