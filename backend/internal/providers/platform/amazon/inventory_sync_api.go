package amazon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

func classifyAmazonInventoryHTTPError(code int, raw []byte) error {
	switch code {
	case http.StatusOK, http.StatusAccepted:
		return nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return platformp.ErrPlatformInventorySyncPermissionDenied
	case http.StatusTooManyRequests:
		return fmt.Errorf("amazon inventory sync: retryable rate limit (http 429)")
	default:
		if code >= 500 {
			return fmt.Errorf("amazon inventory sync: retryable upstream error (http %d): %s", code, apiErrorSnippet(raw))
		}
		snip := apiErrorSnippet(raw)
		if code < 200 || code >= 300 {
			low := strings.ToLower(snip)
			if strings.Contains(low, "access to requested resource denied") ||
				strings.Contains(low, "permission") ||
				strings.Contains(low, "not authorized") ||
				strings.Contains(low, "Unauthorized") {
				return platformp.ErrPlatformInventorySyncPermissionDenied
			}
			return fmt.Errorf("amazon inventory sync http %d: %s", code, snip)
		}
		return nil
	}
}

func maybeRetryableAmazonInventoryTransportErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, platformp.ErrPlatformInventorySyncPermissionDenied) {
		return err
	}
	s := strings.ToLower(err.Error())
	if strings.Contains(s, "timeout") || strings.Contains(s, "timed out") ||
		strings.Contains(s, "connection reset") || strings.Contains(s, "eof") ||
		strings.Contains(s, "context deadline") {
		return fmt.Errorf("amazon inventory sync: retryable: %w", err)
	}
	return err
}

func patchListingsItemInventory(ctx context.Context, cfg RuntimeConfig, access, sellerID, sellerSKU, marketplaceID, issueLocale, productType, fulfillmentChannel string, qty int) (*amazonListingPutResponse, int, error) {
	if strings.TrimSpace(sellerID) == "" {
		return nil, 0, fmt.Errorf("shop is not authorized")
	}
	sku := strings.TrimSpace(sellerSKU)
	if sku == "" {
		return nil, 0, fmt.Errorf("product publication sku mapping incomplete")
	}
	pt := strings.TrimSpace(productType)
	if pt == "" {
		return nil, 0, fmt.Errorf("platform inventory config incomplete: missing product_type")
	}
	fc := strings.TrimSpace(fulfillmentChannel)
	if fc == "" {
		return nil, 0, fmt.Errorf("platform inventory config incomplete: missing fulfillment_channel")
	}
	mp := strings.TrimSpace(marketplaceID)
	if mp == "" {
		return nil, 0, fmt.Errorf("platform inventory config incomplete: missing marketplace_id")
	}

	bodyObj := map[string]any{
		"productType": pt,
		"patches": []any{
			map[string]any{
				"op":    "replace",
				"path":  "/attributes/fulfillment_availability",
				"value": []any{map[string]any{"fulfillment_channel_code": fc, "quantity": maxAmazonInt(0, qty)}},
			},
		},
	}
	payload, err := json.Marshal(bodyObj)
	if err != nil {
		return nil, 0, fmt.Errorf("amazon inventory sync: encode patch: %w", err)
	}
	q := url.Values{}
	q.Set("marketplaceIds", mp)
	if loc := strings.TrimSpace(issueLocale); loc != "" {
		q.Set("issueLocale", loc)
	}
	code, raw, err := doSPAPI(ctx, cfg, http.MethodPatch, listingsItemPath(sellerID, sku), q, payload, access)
	if err != nil {
		return nil, code, maybeRetryableAmazonInventoryTransportErr(err)
	}
	if err := classifyAmazonInventoryHTTPError(code, raw); err != nil {
		return nil, code, err
	}
	var root map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &root); err != nil {
			return nil, code, fmt.Errorf("amazon inventory sync: invalid json response")
		}
	}
	res := &amazonListingPutResponse{Raw: root}
	if root != nil {
		res.SKU = strFromAny(root["sku"])
		res.Status = strFromAny(root["status"])
		res.SubmissionID = strFromAny(root["submissionId"])
		res.ASIN = strFromAny(root["asin"])
		if rawIssues, ok := root["issues"].([]any); ok {
			for _, it := range rawIssues {
				m, _ := it.(map[string]any)
				if m == nil {
					continue
				}
				res.Issues = append(res.Issues, amazonListingIssue{
					Code:     strFromAny(m["code"]),
					Message:  strFromAny(m["message"]),
					Severity: strFromAny(m["severity"]),
				})
			}
		}
	}
	return res, code, nil
}
