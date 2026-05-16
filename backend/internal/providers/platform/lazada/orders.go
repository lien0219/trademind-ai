package lazada

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

// SyncOrders fetches a single page of orders and maps them to PlatformOrder.
func SyncOrders(ctx context.Context, auth platformp.TestConnectionRequest, access string, cursor string, limit int, start, end *time.Time) ([]platformp.PlatformOrder, string, bool, map[string]any, error) {
	cfg, err := ResolveRuntime(auth)
	if err != nil {
		return nil, "", false, nil, err
	}
	if strings.TrimSpace(access) == "" {
		return nil, "", false, nil, fmt.Errorf("missing access_token")
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	offset := 0
	if s := strings.TrimSpace(cursor); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v >= 0 {
			offset = v
		}
	}

	now := time.Now().UTC()
	st, et := start, end
	if st == nil && et == nil {
		et = &now
		x := now.Add(-720 * time.Hour)
		st = &x
	}
	if st == nil {
		x := et.Add(-720 * time.Hour)
		st = &x
	}
	if et == nil {
		x := st.Add(720 * time.Hour)
		if x.After(now) {
			x = now
		}
		et = &x
	}

	extra := map[string]string{
		"offset": strconv.Itoa(offset),
		"limit":  strconv.Itoa(limit),
	}
	if st != nil {
		extra["create_after"] = strconv.FormatInt(st.UnixMilli(), 10)
	}
	if et != nil {
		extra["create_before"] = strconv.FormatInt(et.UnixMilli(), 10)
	}

	root, err := getSigned(ctx, cfg, cfg.APIRESTBase, PathOrdersGet, access, extra)
	if err != nil {
		return nil, "", false, nil, err
	}
	sum := compactSummary(root)

	data, _ := root["data"].(map[string]any)
	var ordersRaw []any
	if data != nil {
		if arr, ok := data["orders"].([]any); ok {
			ordersRaw = arr
		}
	}

	out := make([]platformp.PlatformOrder, 0, len(ordersRaw))
	for _, raw := range ordersRaw {
		lm, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		oid := orderIDString(lm)
		if oid == "" {
			out = append(out, mapOrder(lm))
			continue
		}
		detail, derr := fetchOrderDetail(ctx, cfg, access, oid)
		if derr != nil {
			return nil, "", false, sum, derr
		}
		merged := mergeOrderMaps(lm, detail)
		if needItems(merged) {
			items, ierr := fetchOrderItems(ctx, cfg, access, oid)
			if ierr != nil {
				return nil, "", false, sum, ierr
			}
			if len(items) > 0 {
				merged["__items_override"] = items
			}
		}
		out = append(out, mapOrder(merged))
	}

	nextOffset := offset + len(ordersRaw)
	hasMore := len(ordersRaw) >= limit
	nextCursor := ""
	if hasMore {
		nextCursor = strconv.Itoa(nextOffset)
	}

	return out, nextCursor, hasMore, sum, nil
}

func needItems(m map[string]any) bool {
	if m == nil {
		return true
	}
	for _, k := range []string{"items", "order_items", "orderItems", "line_items"} {
		if v, ok := m[k]; ok {
			if arr, ok2 := v.([]any); ok2 && len(arr) > 0 {
				return false
			}
		}
	}
	return true
}

func mergeOrderMaps(light, detail map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range light {
		out[k] = v
	}
	for k, v := range detail {
		if _, exists := out[k]; !exists {
			out[k] = v
		}
	}
	return out
}

func fetchOrderDetail(ctx context.Context, cfg RuntimeConfig, access, orderID string) (map[string]any, error) {
	root, err := getSigned(ctx, cfg, cfg.APIRESTBase, PathOrderGet, access, map[string]string{
		"order_id": orderID,
	})
	if err != nil {
		return nil, err
	}
	if d, ok := root["data"].(map[string]any); ok {
		return d, nil
	}
	return map[string]any{}, nil
}

func fetchOrderItems(ctx context.Context, cfg RuntimeConfig, access, orderID string) ([]any, error) {
	root, err := getSigned(ctx, cfg, cfg.APIRESTBase, PathOrderItemsGet, access, map[string]string{
		"order_id": orderID,
	})
	if err != nil {
		return nil, err
	}
	data, _ := root["data"].([]any)
	if len(data) > 0 {
		return data, nil
	}
	if m, ok := root["data"].(map[string]any); ok {
		for _, k := range []string{"items", "order_items", "order_items_list"} {
			if arr, ok2 := m[k].([]any); ok2 {
				return arr, nil
			}
		}
	}
	return nil, nil
}

func orderIDString(m map[string]any) string {
	return firstNonEmpty(
		pickStr(m, "order_id"),
		fmt.Sprint(m["order_id"]),
	)
}

func compactSummary(root map[string]any) map[string]any {
	out := map[string]any{"provider": "lazada"}
	if root == nil {
		return out
	}
	if rid, ok := root["request_id"].(string); ok && rid != "" {
		out["requestId"] = rid
	}
	return out
}
