package amazon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

var amazonMessagingThrottleMu sync.Mutex
var amazonMessagingLastCall time.Time

const amazonMessagingMinGap = 1100 * time.Millisecond

func throttleAmazonMessagingCall(ctx context.Context) error {
	deadline, ok := ctx.Deadline()
	amazonMessagingThrottleMu.Lock()
	defer amazonMessagingThrottleMu.Unlock()
	if !amazonMessagingLastCall.IsZero() {
		wait := amazonMessagingMinGap - time.Since(amazonMessagingLastCall)
		if wait > 0 {
			if ok && time.Until(deadline) <= wait {
				return fmt.Errorf("amazon messaging: retryable: context deadline before throttle window")
			}
			t := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				t.Stop()
				return fmt.Errorf("amazon messaging: retryable: %w", ctx.Err())
			case <-t.C:
			}
		}
	}
	amazonMessagingLastCall = time.Now()
	return nil
}

func messagingOrderPath(amazonOrderID string) string {
	oid := url.PathEscape(strings.TrimSpace(amazonOrderID))
	return strings.Replace(PathMessagingOrderActions, "{amazonOrderId}", oid, 1)
}

func messagingAttributesPath(amazonOrderID string) string {
	oid := url.PathEscape(strings.TrimSpace(amazonOrderID))
	return strings.Replace(PathMessagingAttributes, "{amazonOrderId}", oid, 1)
}

func messagingSendPath(amazonOrderID, template string) string {
	oid := url.PathEscape(strings.TrimSpace(amazonOrderID))
	tpl := url.PathEscape(strings.TrimSpace(template))
	p := strings.Replace(PathMessagingSendTemplate, "{amazonOrderId}", oid, 1)
	return strings.Replace(p, "{template}", tpl, 1)
}

func marketplaceQuery(mp string) url.Values {
	q := url.Values{}
	q.Add("marketplaceIds", strings.TrimSpace(mp))
	return q
}

func doMessagingGET(ctx context.Context, cfg RuntimeConfig, access, relPath string, q url.Values) (int, []byte, error) {
	if err := throttleAmazonMessagingCall(ctx); err != nil {
		return 0, nil, err
	}
	code, raw, _, err := doSPAPIFull(ctx, cfg, http.MethodGet, relPath, q, nil, access, "application/hal+json")
	return code, raw, err
}

func doMessagingPOSTJSON(ctx context.Context, cfg RuntimeConfig, access, relPath string, q url.Values, body []byte) (int, []byte, http.Header, error) {
	if err := throttleAmazonMessagingCall(ctx); err != nil {
		return 0, nil, nil, err
	}
	return doSPAPIFull(ctx, cfg, http.MethodPost, relPath, q, body, access, "application/hal+json")
}

func extractMessagingActionNames(root map[string]any) []string {
	if root == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string

	appendNames := func(items []any) {
		for _, it := range items {
			m, _ := it.(map[string]any)
			if m == nil {
				continue
			}
			n := strings.TrimSpace(fmt.Sprint(m["name"]))
			if n == "" {
				continue
			}
			if _, ok := seen[n]; ok {
				continue
			}
			seen[n] = struct{}{}
			out = append(out, n)
		}
	}

	if links, ok := root["_links"].(map[string]any); ok {
		if raw, ok := links["actions"]; ok {
			switch v := raw.(type) {
			case []any:
				appendNames(v)
			case map[string]any:
				appendNames([]any{v})
			}
		}
	}

	if emb, ok := root["_embedded"].(map[string]any); ok {
		if raw, ok := emb["actions"]; ok {
			if arr, ok := raw.([]any); ok {
				for _, row := range arr {
					rm, _ := row.(map[string]any)
					if rm == nil {
						continue
					}
					if inner, ok := rm["_links"].(map[string]any); ok {
						if self, ok := inner["self"].(map[string]any); ok {
							n := strings.TrimSpace(fmt.Sprint(self["name"]))
							if n != "" {
								if _, ok := seen[n]; !ok {
									seen[n] = struct{}{}
									out = append(out, n)
								}
							}
						}
					}
				}
			}
		}
	}

	return out
}

func getMessagingActionsForOrder(ctx context.Context, cfg RuntimeConfig, access, mp, amazonOrderID string) ([]string, error) {
	path := messagingOrderPath(amazonOrderID)
	code, raw, err := doMessagingGET(ctx, cfg, access, path, marketplaceQuery(mp))
	if err != nil {
		return nil, err
	}
	return parseMessagingActionsResponse(code, raw)
}

func parseMessagingActionsResponse(code int, raw []byte) ([]string, error) {
	if code == http.StatusTooManyRequests {
		return nil, fmt.Errorf("amazon messaging: retryable rate limit (http 429)")
	}
	if code == http.StatusUnauthorized || code == http.StatusForbidden {
		return nil, classifyAmazonMessagingForbidden(code, raw)
	}
	if code == http.StatusNotFound {
		return []string{}, nil
	}
	if code < 200 || code >= 300 {
		return nil, classifyAmazonMessagingFailure(code, raw)
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("amazon messaging: actions parse: %w", err)
	}
	return extractMessagingActionNames(root), nil
}

func getMessagingAttributes(ctx context.Context, cfg RuntimeConfig, access, mp, amazonOrderID string) map[string]any {
	path := messagingAttributesPath(amazonOrderID)
	code, raw, err := doMessagingGET(ctx, cfg, access, path, marketplaceQuery(mp))
	if err != nil {
		return map[string]any{}
	}
	if code == http.StatusTooManyRequests || code == http.StatusUnauthorized || code == http.StatusForbidden {
		return map[string]any{}
	}
	if code < 200 || code >= 300 {
		return map[string]any{}
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return map[string]any{}
	}
	return root
}

func classifyAmazonMessagingForbidden(code int, raw []byte) error {
	snip := apiErrorSnippet(raw)
	return fmt.Errorf("%w: amazon messaging http %d: %s", platformp.ErrPlatformCustomerMessagePermissionDenied, code, snip)
}

func classifyAmazonMessagingFailure(code int, raw []byte) error {
	snip := apiErrorSnippet(raw)
	low := strings.ToLower(snip)
	switch code {
	case http.StatusUnauthorized, http.StatusForbidden:
		return classifyAmazonMessagingForbidden(code, raw)
	case http.StatusTooManyRequests:
		return fmt.Errorf("amazon messaging: retryable rate limit (http 429)")
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return fmt.Errorf("amazon messaging: retryable upstream error (http %d): %s", code, snip)
	default:
		if strings.Contains(low, "quota") || strings.Contains(low, "throttl") {
			return fmt.Errorf("amazon messaging: retryable: %s", snip)
		}
		return fmt.Errorf("amazon messaging http %d: %s", code, snip)
	}
}

func pickAmazonSendTemplate(actions []string) string {
	set := map[string]struct{}{}
	for _, a := range actions {
		v := strings.TrimSpace(a)
		if v != "" {
			set[v] = struct{}{}
		}
	}
	for _, cand := range amazonSendTemplatesOrdered {
		if _, ok := set[cand]; ok {
			return cand
		}
	}
	return ""
}

func truncateAmazonMessagingText(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || s == "" {
		return s
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

func sendAmazonMessagingTextTemplate(ctx context.Context, cfg RuntimeConfig, access, mp, amazonOrderID, template, text string, idempotencyKey string) (*platformp.SendMessageResult, error) {
	path := messagingSendPath(amazonOrderID, template)
	payload := map[string]any{"text": text}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	code, raw, hdr, err := doMessagingPOSTJSON(ctx, cfg, access, path, marketplaceQuery(mp), body)
	if err != nil {
		return nil, err
	}
	reqID := strings.TrimSpace(hdr.Get("x-amzn-RequestId"))

	if code == http.StatusTooManyRequests {
		return nil, fmt.Errorf("amazon messaging: retryable rate limit (http 429)")
	}
	if code == http.StatusUnauthorized || code == http.StatusForbidden {
		return nil, classifyAmazonMessagingForbidden(code, raw)
	}
	if code != http.StatusCreated && code != http.StatusOK {
		return nil, classifyAmazonMessagingFailure(code, raw)
	}

	extMid := strings.TrimSpace(reqID)
	if extMid == "" {
		extMid = fmt.Sprintf("amazon-send:%s:%d", template, time.Now().UTC().UnixNano())
	} else {
		extMid = fmt.Sprintf("amazon-send:%s:%s", template, extMid)
	}
	rawSum := map[string]any{
		"amazonOrderId": strings.TrimSpace(amazonOrderID),
		"marketplaceId": strings.TrimSpace(mp),
		"template":      strings.TrimSpace(template),
	}
	if reqID != "" {
		rawSum["amazonRequestId"] = reqID
	}
	if strings.TrimSpace(idempotencyKey) != "" {
		rawSum["idempotencyKey"] = strings.TrimSpace(idempotencyKey)
	}
	sentAt := time.Now().UTC()
	return &platformp.SendMessageResult{
		ExternalMessageID: extMid,
		SentAt:            &sentAt,
		RawSummary:        platformp.TrimRawMap(rawSum, 14, 400),
	}, nil
}
