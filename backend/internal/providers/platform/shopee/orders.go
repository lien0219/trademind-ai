package shopee

import (
	"context"
	"fmt"
	"strings"
	"time"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

// SyncOrders pulls one page / cursor window of orders mapped to PlatformOrder.
func SyncOrders(ctx context.Context, auth platformp.TestConnectionRequest, shopID int64, access string, cursor string, limit int, start, end *time.Time) ([]platformp.PlatformOrder, string, bool, map[string]any, error) {
	cfg, err := ResolveRuntime(auth)
	if err != nil {
		return nil, "", false, nil, err
	}
	if strings.TrimSpace(access) == "" {
		return nil, "", false, nil, fmt.Errorf("missing access_token")
	}
	if shopID <= 0 {
		return nil, "", false, nil, fmt.Errorf("missing shop_id")
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	now := time.Now().UTC()
	st := start
	et := end
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

	listBody := map[string]any{
		"time_range_field": "create_time",
		"time_from":        st.Unix(),
		"time_to":          et.Unix(),
		"page_size":        limit,
	}
	if strings.TrimSpace(cursor) != "" {
		listBody["cursor"] = strings.TrimSpace(cursor)
	}

	rawList, err := postShop(ctx, cfg, PathGetOrderList, shopID, access, listBody)
	if err != nil {
		return nil, "", false, nil, err
	}

	sum := compactSummary(rawList)

	var ordersRaw []any
	if v, ok := rawList["order_list"].([]any); ok {
		ordersRaw = v
	}

	orderSNs := make([]string, 0, len(ordersRaw))
	light := make([]map[string]any, 0, len(ordersRaw))
	for _, o := range ordersRaw {
		m, ok := o.(map[string]any)
		if !ok {
			continue
		}
		sn := pickStr(m, "order_sn", "ordersn")
		if sn != "" {
			orderSNs = append(orderSNs, sn)
		}
		light = append(light, m)
	}

	detailBySN := map[string]map[string]any{}
	for i := 0; i < len(orderSNs); i += 50 {
		j := i + 50
		if j > len(orderSNs) {
			j = len(orderSNs)
		}
		batch := orderSNs[i:j]
		if len(batch) == 0 {
			break
		}
		detBody := map[string]any{"order_sn_list": batch}
		dres, derr := postShop(ctx, cfg, PathGetOrderDetail, shopID, access, detBody)
		if derr != nil {
			return nil, "", false, sum, derr
		}
		var detList []any
		if v, ok := dres["order_list"].([]any); ok {
			detList = v
		}
		for _, ent := range detList {
			dm, ok := ent.(map[string]any)
			if !ok {
				continue
			}
			sn := pickStr(dm, "order_sn", "ordersn")
			if sn != "" {
				detailBySN[sn] = dm
			}
		}
	}

	out := make([]platformp.PlatformOrder, 0, len(light))
	for _, lm := range light {
		sn := pickStr(lm, "order_sn", "ordersn")
		dm := detailBySN[sn]
		if dm == nil {
			dm = lm
		} else {
			// merge light list fields not present in detail
			for k, v := range lm {
				if _, ok := dm[k]; !ok {
					dm[k] = v
				}
			}
		}
		out = append(out, mapOrder(dm))
	}

	next := pickStr(rawList, "next_cursor", "cursor")
	more := false
	if b, ok := rawList["more"].(bool); ok {
		more = b
	} else if next != "" {
		more = true
	}

	return out, next, more, sum, nil
}

func compactSummary(m map[string]any) map[string]any {
	out := map[string]any{}
	if m == nil {
		return out
	}
	if v, ok := m["order_list"]; ok {
		if ar, ok2 := v.([]any); ok2 {
			out["returnedOrders"] = len(ar)
		}
	}
	out["receivedAt"] = time.Now().UTC().Format(time.RFC3339)
	return out
}
