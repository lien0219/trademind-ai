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
		"nextPage":       next,
		"receivedAt":     time.Now().UTC().Format(time.RFC3339),
	}
	for k, v := range sum {
		raw[k] = v
	}
	return raw
}

// SyncOrdersPaginated pulls up to maxPages from order.searchList, capped at maxOrderSyncRecordsPerTask.
func SyncOrdersPaginated(ctx context.Context, client *Client, cursor string, limit, maxPages int, start, end *time.Time) (*platformp.SyncOrdersResult, error) {
	if client == nil {
		return nil, NewError(CodeDouyinOrderSyncFailed, "douyin order sync client missing", "", "", "")
	}
	if maxPages <= 0 {
		maxPages = defaultOrderSyncMaxPages
	}
	if limit <= 0 {
		limit = 50
	}

	allOrders := make([]platformp.PlatformOrder, 0)
	pageErrors := make([]platformp.PageSyncError, 0)
	totalPages := 0
	successPages := 0
	failedPages := 0
	nextCursor := strings.TrimSpace(cursor)
	hasMore := false
	lastPageSum := map[string]any{}

	for attempt := 0; attempt < maxPages; attempt++ {
		if len(allOrders) >= maxOrderSyncRecordsPerTask {
			hasMore = true
			break
		}

		pageNo := parsePageCursor(nextCursor)
		totalPages++
		orders, next, more, sum, err := SyncOrdersPage(ctx, client, nextCursor, limit, start, end)
		if err != nil {
			failedPages++
			pageErrors = append(pageErrors, platformp.PageSyncError{
				Page:  pageNo,
				Error: SanitizeErrorText(err.Error()),
			})
			if len(allOrders) == 0 && attempt == 0 {
				return nil, err
			}
			nextCursor = formatPageCursor(pageNo + 1)
			if attempt+1 >= maxPages {
				hasMore = more
				break
			}
			continue
		}

		successPages++
		lastPageSum = sum
		remaining := maxOrderSyncRecordsPerTask - len(allOrders)
		if remaining < len(orders) {
			orders = orders[:remaining]
			hasMore = true
		} else {
			hasMore = more
		}
		allOrders = append(allOrders, orders...)
		nextCursor = next
		if !hasMore || len(allOrders) >= maxOrderSyncRecordsPerTask {
			break
		}
	}

	if len(allOrders) == 0 && failedPages > 0 {
		return nil, NewError(CodeDouyinOrderListFailed, pageErrors[0].Error, "", "", "")
	}

	raw := fmtOrderSyncSummary(lastPageSum, allOrders, hasMore, nextCursor)
	raw["totalFetched"] = len(allOrders)
	raw["totalPages"] = totalPages
	raw["successPages"] = successPages
	raw["failedPages"] = failedPages
	raw["maxPages"] = maxPages
	raw["maxRecords"] = maxOrderSyncRecordsPerTask
	if len(pageErrors) > 0 {
		raw["pageErrors"] = pageErrors
	}

	return &platformp.SyncOrdersResult{
		Orders:       allOrders,
		NextCursor:   nextCursor,
		NextPage:     nextCursor,
		HasMore:      hasMore,
		TotalFetched: len(allOrders),
		TotalPages:   totalPages,
		SuccessPages: successPages,
		FailedPages:  failedPages,
		PageErrors:   pageErrors,
		RawSummary:   raw,
	}, nil
}

func assertShopAuthorized(auth platformp.TestConnectionRequest) error {
	if strings.TrimSpace(auth.RefreshToken) == "" && strings.TrimSpace(auth.AccessToken) == "" {
		return NewError(CodeDouyinStoreNotAuthorized, "douyin store not authorized", "", "", "")
	}
	return nil
}
