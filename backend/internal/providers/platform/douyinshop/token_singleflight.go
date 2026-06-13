package douyinshop

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/sync/singleflight"
)

var (
	tokenRefreshFlights singleflight.Group
	refreshStateMu      sync.Mutex
	refreshState        = map[string]struct{}{}
)

// ShopID identifies the shop for token refresh singleflight (set on Client).
type shopIDKey struct{}

// WithShopID attaches shop ID to context for token refresh deduplication.
func WithShopID(ctx context.Context, shopID string) context.Context {
	return context.WithValue(ctx, shopIDKey{}, strings.TrimSpace(shopID))
}

func shopIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(shopIDKey{}).(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// ShopIDFromContext returns the shop ID attached to context for Douyin calls.
func ShopIDFromContext(ctx context.Context) string {
	return shopIDFromContext(ctx)
}

func refreshFlightKey(c *Client, ctx context.Context) string {
	if c != nil && strings.TrimSpace(c.ShopID) != "" {
		return strings.TrimSpace(c.ShopID)
	}
	return shopIDFromContext(ctx)
}

// EnsureFreshAccessSingleflight ensures at most one refresh HTTP call per shop at a time.
func (c *Client) EnsureFreshAccessSingleflight(ctx context.Context) (string, error) {
	now := c.now()
	if token, ok := c.freshAccessToken(now); ok {
		return token, nil
	}

	key := refreshFlightKey(c, ctx)
	if key == "" {
		return c.ensureFreshAccessDirect(ctx)
	}

	v, err, _ := tokenRefreshFlights.Do(key, func() (any, error) {
		return c.ensureFreshAccessDirect(ctx)
	})
	if err != nil {
		return "", err
	}
	token, _ := v.(string)
	if strings.TrimSpace(token) == "" {
		return "", NewError(CodeDouyinTokenRefreshFailed, "douyin token refresh failed", "", "empty token", "")
	}
	return strings.TrimSpace(token), nil
}

// ClearTokenRefreshState removes in-flight refresh tracking for a shop (e.g. on revoke).
func ClearTokenRefreshState(shopID string) {
	shopID = strings.TrimSpace(shopID)
	if shopID == "" {
		return
	}
	refreshStateMu.Lock()
	delete(refreshState, shopID)
	refreshStateMu.Unlock()
	tokenRefreshFlights.Forget(shopID)
}

// RefreshOnceAfterAuthError refreshes token once and is used when platform returns auth expired mid-request.
func (c *Client) RefreshOnceAfterAuthError(ctx context.Context) (string, error) {
	key := refreshFlightKey(c, ctx)
	if key != "" {
		refreshStateMu.Lock()
		if _, seen := refreshState[key]; seen {
			refreshStateMu.Unlock()
			return "", NewError(CodeDouyinAuthExpired, "douyin authorization expired", "", "refresh already attempted", "")
		}
		refreshState[key] = struct{}{}
		refreshStateMu.Unlock()
	}
	return c.EnsureFreshAccessSingleflight(ctx)
}

func resetRefreshAttempt(ctx context.Context, c *Client) {
	key := refreshFlightKey(c, ctx)
	if key == "" {
		return
	}
	refreshStateMu.Lock()
	delete(refreshState, key)
	refreshStateMu.Unlock()
}

func markRefreshAttemptDone(ctx context.Context, c *Client) {
	// Keep the "already refreshed" flag for the duration of the outer Do() call.
	_ = fmt.Sprintf("%v", ctx)
	_ = c
}
