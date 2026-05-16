package amazon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func FetchOrdersPage(ctx context.Context, cfg RuntimeConfig, lwa string, mpID string, cursor string, limit int, start, end *time.Time) ([]map[string]any, string, error) {
	q := url.Values{}
	q.Set("MarketplaceIds", strings.TrimSpace(mpID))
	if strings.TrimSpace(mpID) == "" {
		return nil, "", fmt.Errorf("amazon: MarketplaceIds required")
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
	q.Set("LastUpdatedAfter", st.UTC().Format(time.RFC3339))
	q.Set("LastUpdatedBefore", et.UTC().Format(time.RFC3339))
	if strings.TrimSpace(cursor) != "" {
		q.Set("NextToken", strings.TrimSpace(cursor))
	}
	q.Set("MaxResultsPerPage", strconv.Itoa(limit))

	code, raw, err := doSPAPI(ctx, cfg, http.MethodGet, "/orders/v0/orders", q, nil, lwa)
	if err != nil {
		return nil, "", err
	}
	if code == http.StatusTooManyRequests {
		return nil, "", fmt.Errorf("amazon: SP-API rate limited (429)")
	}
	if code < 200 || code >= 300 {
		return nil, "", fmt.Errorf("amazon: orders list http %d: %s", code, apiErrorSnippet(raw))
	}
	var body struct {
		Payload struct {
			Orders    []map[string]any `json:"Orders"`
			NextToken string           `json:"NextToken"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, "", fmt.Errorf("amazon: orders parse: %w", err)
	}
	return body.Payload.Orders, strings.TrimSpace(body.Payload.NextToken), nil
}

func FetchOrderItems(ctx context.Context, cfg RuntimeConfig, lwa, amazonOrderID string) ([]map[string]any, error) {
	oid := strings.TrimSpace(amazonOrderID)
	if oid == "" {
		return nil, fmt.Errorf("amazon: order id required")
	}
	path := "/orders/v0/orders/" + url.PathEscape(oid) + "/orderItems"
	code, raw, err := doSPAPI(ctx, cfg, http.MethodGet, path, nil, nil, lwa)
	if err != nil {
		return nil, err
	}
	if code == http.StatusTooManyRequests {
		return nil, fmt.Errorf("amazon: SP-API rate limited (429)")
	}
	if code < 200 || code >= 300 {
		return nil, fmt.Errorf("amazon: order items http %d: %s", code, apiErrorSnippet(raw))
	}
	var body struct {
		Payload struct {
			OrderItems []map[string]any `json:"OrderItems"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, fmt.Errorf("amazon: order items parse: %w", err)
	}
	return body.Payload.OrderItems, nil
}

func moneyFromMap(m map[string]any) (amount float64, currency string) {
	if m == nil {
		return 0, ""
	}
	currency = strFromAny(m["CurrencyCode"])
	s := strFromAny(m["Amount"])
	if s == "" {
		return 0, currency
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, currency
	}
	return f, currency
}

func strFromAny(v any) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}
