package douyinshop

import (
	"context"
	"net/http"
	"strings"
	"time"
)

const (
	MethodTokenCreate  = "token.create"
	MethodTokenRefresh = "token.refresh"

	defaultAPIBaseURL  = "https://openapi-fxg.jinritemai.com"
	defaultAuthBaseURL = "https://fuwu.jinritemai.com"

	tokenRefreshSkew = time.Hour
)

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type SafeRequestLog struct {
	Method       string
	RequestID    string
	TraceID      string
	ElapsedMs    int64
	PlatformCode string
	Success      bool
	ErrorCode    string
}

type SafeLogger interface {
	LogDouyinRequest(ctx context.Context, entry SafeRequestLog)
}

type SafeLoggerFunc func(ctx context.Context, entry SafeRequestLog)

func (f SafeLoggerFunc) LogDouyinRequest(ctx context.Context, entry SafeRequestLog) {
	if f != nil {
		f(ctx, entry)
	}
}

type Client struct {
	Config RuntimeConfig
	HTTP   HTTPDoer
	Now    func() time.Time

	AccessToken           string
	RefreshTokenValue     string
	AccessTokenExpiresAt  *time.Time
	RefreshTokenExpiresAt *time.Time

	PersistRefreshedToken func(ctx context.Context, tok *TokenBundle) error
	MarkAuthStatus        func(ctx context.Context, status string) error
	Logger                SafeLogger
	TraceID               func(context.Context) string
}

func (c *Client) now() time.Time {
	if c != nil && c.Now != nil {
		return c.Now().UTC()
	}
	return time.Now().UTC()
}

func (c *Client) httpClient() HTTPDoer {
	if c != nil && c.HTTP != nil {
		return c.HTTP
	}
	timeout := 30 * time.Second
	if c != nil && c.Config.HTTPTimeout > 0 {
		timeout = c.Config.HTTPTimeout
	}
	return &http.Client{Timeout: timeout}
}

func (c *Client) apiBaseURL() string {
	if c != nil && strings.TrimSpace(c.Config.APIBaseURL) != "" {
		return strings.TrimRight(strings.TrimSpace(c.Config.APIBaseURL), "/")
	}
	return defaultAPIBaseURL
}

func (c *Client) traceID(ctx context.Context) string {
	if c != nil && c.TraceID != nil {
		return strings.TrimSpace(c.TraceID(ctx))
	}
	if ctx == nil {
		return ""
	}
	for _, key := range []any{"traceId", "trace_id", "requestId", "request_id"} {
		if v, ok := ctx.Value(key).(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func (c *Client) logRequest(ctx context.Context, entry SafeRequestLog) {
	if c == nil || c.Logger == nil {
		return
	}
	entry.TraceID = c.traceID(ctx)
	c.Logger.LogDouyinRequest(ctx, entry)
}

func (c *Client) markAuthStatus(ctx context.Context, status string) {
	if c != nil && c.MarkAuthStatus != nil && strings.TrimSpace(status) != "" {
		_ = c.MarkAuthStatus(ctx, strings.TrimSpace(status))
	}
}

func (c *Client) accessFresh(now time.Time) bool {
	if c == nil || strings.TrimSpace(c.AccessToken) == "" {
		return false
	}
	if c.AccessTokenExpiresAt == nil {
		return true
	}
	return c.AccessTokenExpiresAt.After(now.Add(tokenRefreshSkew))
}

func (c *Client) refreshUsable(now time.Time) bool {
	if c == nil || strings.TrimSpace(c.RefreshTokenValue) == "" {
		return false
	}
	if c.RefreshTokenExpiresAt == nil {
		return true
	}
	return c.RefreshTokenExpiresAt.After(now)
}

func (c *Client) EnsureFreshAccess(ctx context.Context) (string, error) {
	now := c.now()
	if c.accessFresh(now) {
		return strings.TrimSpace(c.AccessToken), nil
	}
	if !c.refreshUsable(now) {
		c.markAuthStatus(ctx, "expired")
		return "", NewError(CodeDouyinAuthExpired, "douyin authorization expired", "", "", "")
	}
	tok, err := c.RefreshAccessToken(ctx)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(tok.AccessToken), nil
}

func (c *Client) RefreshAccessToken(ctx context.Context) (*TokenBundle, error) {
	if c == nil || strings.TrimSpace(c.RefreshTokenValue) == "" {
		c.markAuthStatus(ctx, "expired")
		return nil, NewError(CodeDouyinAuthExpired, "douyin authorization expired", "", "", "")
	}
	tok, err := c.RefreshToken(ctx, c.RefreshTokenValue)
	if err != nil {
		var de *Error
		if AsError(err, &de) && de.Code == CodeDouyinAuthExpired {
			c.markAuthStatus(ctx, "expired")
			return nil, de
		}
		return nil, NewError(CodeDouyinTokenRefreshFailed, "douyin token refresh failed", platformCodeOf(err), safeMessageOf(err), requestIDOf(err))
	}
	if strings.TrimSpace(tok.RefreshToken) == "" {
		tok.RefreshToken = c.RefreshTokenValue
	}
	if c.PersistRefreshedToken != nil {
		if err := c.PersistRefreshedToken(ctx, tok); err != nil {
			return nil, err
		}
	}
	c.AccessToken = strings.TrimSpace(tok.AccessToken)
	c.RefreshTokenValue = strings.TrimSpace(tok.RefreshToken)
	c.AccessTokenExpiresAt = tok.AccessExpiresAt
	c.RefreshTokenExpiresAt = tok.RefreshExpiresAt
	c.markAuthStatus(ctx, "authorized")
	return tok, nil
}

func (c *Client) Do(ctx context.Context, method string, params map[string]any, out any) error {
	access, err := c.EnsureFreshAccess(ctx)
	if err != nil {
		return err
	}
	return c.do(ctx, method, params, access, out)
}
