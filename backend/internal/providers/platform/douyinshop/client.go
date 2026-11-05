package douyinshop

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	douyinmetrics "github.com/trademind-ai/trademind/backend/internal/metrics/douyin"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
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
	ShopID string
	Config RuntimeConfig
	HTTP   HTTPDoer
	Now    func() time.Time

	tokenMu               sync.RWMutex
	AccessToken           string
	RefreshTokenValue     string
	AccessTokenExpiresAt  *time.Time
	RefreshTokenExpiresAt *time.Time
	TokenVersion          int64
	TokenLocker           TokenRefreshLocker

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
	// Reuse the unified HTTP client and circuit breaker; tests inject HTTPDoer.
	return SharedHTTPDoer()
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

func (c *Client) markAuthStatus(ctx context.Context, status string) error {
	if c == nil || c.MarkAuthStatus == nil || strings.TrimSpace(status) == "" {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	c.tokenMu.RLock()
	version := c.TokenVersion
	c.tokenMu.RUnlock()
	ctx = platformp.WithOAuthExpectedTokenVersion(ctx, version)
	return c.MarkAuthStatus(ctx, strings.TrimSpace(status))
}

func (c *Client) accessFresh(now time.Time) bool {
	_, ok := c.freshAccessToken(now)
	return ok
}

func (c *Client) refreshUsable(now time.Time) bool {
	if c == nil {
		return false
	}
	c.tokenMu.RLock()
	defer c.tokenMu.RUnlock()
	if strings.TrimSpace(c.RefreshTokenValue) == "" {
		return false
	}
	if c.RefreshTokenExpiresAt == nil {
		return true
	}
	return c.RefreshTokenExpiresAt.After(now)
}

func (c *Client) freshAccessToken(now time.Time) (string, bool) {
	if c == nil {
		return "", false
	}
	c.tokenMu.RLock()
	defer c.tokenMu.RUnlock()
	token := strings.TrimSpace(c.AccessToken)
	if token == "" {
		return "", false
	}
	if c.AccessTokenExpiresAt == nil {
		return token, true
	}
	if c.AccessTokenExpiresAt.After(now.Add(tokenRefreshSkew)) {
		return token, true
	}
	return "", false
}

func (c *Client) ensureFreshAccessDirect(ctx context.Context) (string, error) {
	now := c.now()
	if token, ok := c.freshAccessToken(now); ok {
		return token, nil
	}
	if !c.refreshUsable(now) {
		authErr := NewError(CodeDouyinAuthExpired, "douyin authorization expired", "", "", "")
		if statusErr := c.markAuthStatus(ctx, "expired"); statusErr != nil {
			return "", errors.Join(authErr, fmt.Errorf("douyin_shop: persist expired auth status: %w", statusErr))
		}
		return "", authErr
	}
	tok, err := c.RefreshAccessToken(ctx)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(tok.AccessToken), nil
}

func (c *Client) EnsureFreshAccess(ctx context.Context) (string, error) {
	return c.EnsureFreshAccessSingleflight(ctx)
}

func (c *Client) RefreshAccessToken(ctx context.Context) (*TokenBundle, error) {
	if c == nil {
		return nil, NewError(CodeDouyinAuthExpired, "douyin authorization expired", "", "", "")
	}
	c.tokenMu.RLock()
	refreshToken := strings.TrimSpace(c.RefreshTokenValue)
	expectedVersion := c.TokenVersion
	c.tokenMu.RUnlock()
	if refreshToken == "" {
		authErr := NewError(CodeDouyinAuthExpired, "douyin authorization expired", "", "", "")
		if statusErr := c.markAuthStatus(ctx, "expired"); statusErr != nil {
			return nil, errors.Join(authErr, fmt.Errorf("douyin_shop: persist expired auth status: %w", statusErr))
		}
		return nil, authErr
	}
	if ctx == nil {
		ctx = context.Background()
	}
	refreshCtx := platformp.WithOAuthExpectedTokenVersion(ctx, expectedVersion)
	tok, err := c.RefreshToken(refreshCtx, refreshToken)
	if err != nil {
		douyinmetrics.RecordTokenRefresh(c.configEnvironment(), err)
		var de *Error
		if AsError(err, &de) && de.Code == CodeDouyinAuthExpired {
			if statusErr := c.markAuthStatus(refreshCtx, "expired"); statusErr != nil {
				return nil, errors.Join(de, fmt.Errorf("douyin_shop: persist expired auth status: %w", statusErr))
			}
			return nil, de
		}
		return nil, NewError(CodeDouyinTokenRefreshFailed, "douyin token refresh failed", platformCodeOf(err), safeMessageOf(err), requestIDOf(err))
	}
	douyinmetrics.RecordTokenRefresh(c.configEnvironment(), nil)
	if strings.TrimSpace(tok.RefreshToken) == "" {
		tok.RefreshToken = refreshToken
	}
	if c.PersistRefreshedToken != nil {
		if err := c.PersistRefreshedToken(refreshCtx, tok); err != nil {
			return nil, err
		}
	}
	c.tokenMu.Lock()
	if c.TokenVersion != expectedVersion {
		actualVersion := c.TokenVersion
		c.tokenMu.Unlock()
		return nil, TokenVersionConflictError(c.ShopID, expectedVersion, actualVersion)
	}
	c.AccessToken = strings.TrimSpace(tok.AccessToken)
	c.RefreshTokenValue = strings.TrimSpace(tok.RefreshToken)
	c.AccessTokenExpiresAt = tok.AccessExpiresAt
	c.RefreshTokenExpiresAt = tok.RefreshExpiresAt
	c.TokenVersion++
	c.tokenMu.Unlock()
	if err := c.markAuthStatus(refreshCtx, "authorized"); err != nil {
		return nil, fmt.Errorf("douyin_shop: persist authorized auth status: %w", err)
	}
	return tok, nil
}

func (c *Client) Do(ctx context.Context, method string, params map[string]any, out any) error {
	ctx = WithShopID(ctx, c.ShopID)
	access, err := c.EnsureFreshAccessSingleflight(ctx)
	if err != nil {
		return err
	}
	policy := DefaultRetryPolicy()
	attempts, err := ExecuteWithRetry(ctx, policy, func(ctx context.Context, attempt int) error {
		if attempt > 1 {
			douyinmetrics.RecordAPIRetry(method, c.configEnvironment())
		}
		doErr := c.do(ctx, method, params, access, out)
		if doErr == nil {
			return nil
		}
		var de *Error
		if AsError(doErr, &de) && de != nil && de.AuthExpired {
			newToken, refreshErr := c.RefreshOnceAfterAuthError(ctx)
			if refreshErr != nil {
				return refreshErr
			}
			access = newToken
			return c.do(ctx, method, params, access, out)
		}
		return doErr
	})
	_ = attempts
	return err
}
