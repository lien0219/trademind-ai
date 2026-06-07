package douyinshop

import (
	"context"
	"strings"
	"time"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

type orderSearchListData struct {
	Page          int              `json:"page"`
	Size          int              `json:"size"`
	Total         int              `json:"total"`
	ShopOrderList []map[string]any `json:"shop_order_list"`
}

// SyncOrdersPage pulls one page from order.searchList and maps to PlatformOrder.
// Official Douyin OpenAPI (Phase 8): order.searchList — page from 0, size max 100,
// create_time_start/create_time_end as unix seconds.
func SyncOrdersPage(ctx context.Context, client *Client, cursor string, limit int, start, end *time.Time) ([]platformp.PlatformOrder, string, bool, map[string]any, error) {
	if client == nil {
		return nil, "", false, nil, NewError(CodeDouyinOrderSyncFailed, "douyin order sync client missing", "", "", "")
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	st, et := defaultOrderWindow(start, end)
	page := parsePageCursor(cursor)

	params := map[string]any{
		"page":              page,
		"size":              limit,
		"create_time_start": st.Unix(),
		"create_time_end":   et.Unix(),
		"order_by":          "create_time",
		"order_asc":         false,
	}
	var data orderSearchListData
	if err := client.Do(ctx, MethodOrderSearchList, params, &data); err != nil {
		return nil, "", false, nil, mapOrderListError(err)
	}

	sum := map[string]any{
		"page":           data.Page,
		"size":           data.Size,
		"total":          data.Total,
		"returnedOrders": len(data.ShopOrderList),
		"receivedAt":     time.Now().UTC().Format(time.RFC3339),
	}

	out := make([]platformp.PlatformOrder, 0, len(data.ShopOrderList))
	for _, raw := range data.ShopOrderList {
		if raw == nil {
			continue
		}
		po := mapDouyinOrder(raw)
		if strings.TrimSpace(po.ExternalOrderID) == "" {
			return nil, "", false, sum, NewError(CodeDouyinOrderParseFailed, "douyin order missing order_id", "", "", "")
		}
		out = append(out, po)
	}

	hasMore := false
	next := ""
	size := data.Size
	if size <= 0 {
		size = limit
	}
	if data.Total > 0 {
		nextPage := page + 1
		if nextPage*size < data.Total {
			hasMore = true
			next = formatPageCursor(nextPage)
		}
	} else if len(out) >= limit {
		hasMore = true
		next = formatPageCursor(page + 1)
	}
	return out, next, hasMore, sum, nil
}

func mapOrderListError(err error) error {
	if err == nil {
		return nil
	}
	var de *Error
	if AsError(err, &de) {
		switch de.Code {
		case CodeDouyinAuthExpired, CodeDouyinStoreNotAuthorized:
			return de
		case CodeDouyinPermissionDenied:
			return NewError(CodeDouyinOrderPermissionDenied, "douyin order permission denied", de.PlatformCode, de.PlatformMessage, de.RequestID)
		case CodeDouyinRateLimited:
			return NewError(CodeDouyinOrderRateLimited, "douyin order sync rate limited", de.PlatformCode, de.PlatformMessage, de.RequestID)
		case CodeDouyinResponseParseFailed:
			return NewError(CodeDouyinOrderParseFailed, "douyin order response parse failed", de.PlatformCode, de.PlatformMessage, de.RequestID)
		default:
			return NewError(CodeDouyinOrderListFailed, "douyin order list failed", de.PlatformCode, de.PlatformMessage, de.RequestID)
		}
	}
	msg := SanitizeErrorText(err.Error())
	low := strings.ToLower(msg)
	if strings.Contains(low, "parse") || strings.Contains(low, "unmarshal") {
		return NewError(CodeDouyinOrderParseFailed, "douyin order parse failed", "", msg, "")
	}
	return NewError(CodeDouyinOrderListFailed, "douyin order list failed", "", msg, "")
}

func validateOrderSyncConfig(cfg RuntimeConfig) error {
	if !orderSyncEnabled(cfg) {
		return NewError(CodeDouyinOrderSyncFailed, "douyin order sync is disabled in platform settings (order_sync_enabled=false)", "", "", "")
	}
	return nil
}

func fmtOrderSyncSummary(sum map[string]any, orders []platformp.PlatformOrder, hasMore bool, next string) map[string]any {
	raw := map[string]any{
		"provider":       "douyin_shop",
		"returnedOrders": len(orders),
		"hasMore":        hasMore,
		"nextCursor":     next,
		"receivedAt":     time.Now().UTC().Format(time.RFC3339),
	}
	for k, v := range sum {
		raw[k] = v
	}
	return raw
}

func assertShopAuthorized(auth platformp.TestConnectionRequest) error {
	if strings.TrimSpace(auth.RefreshToken) == "" && strings.TrimSpace(auth.AccessToken) == "" {
		return NewError(CodeDouyinStoreNotAuthorized, "douyin store not authorized", "", "", "")
	}
	return nil
}
