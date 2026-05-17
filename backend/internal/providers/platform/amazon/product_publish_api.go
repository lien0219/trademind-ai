package amazon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

type amazonListingIssue struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
}

type amazonListingPutResponse struct {
	SKU          string               `json:"sku"`
	Status       string               `json:"status"`
	SubmissionID string               `json:"submissionId"`
	ASIN         string               `json:"asin"`
	Issues       []amazonListingIssue `json:"issues"`
	Raw          map[string]any       `json:"-"`
}

func putListingsItem(ctx context.Context, cfg RuntimeConfig, access, sellerID, sellerSKU, marketplaceID, issueLocale string, body map[string]any) (*amazonListingPutResponse, int, error) {
	if strings.TrimSpace(sellerID) == "" {
		return nil, 0, fmt.Errorf("shop is not authorized")
	}
	if strings.TrimSpace(sellerSKU) == "" {
		return nil, 0, fmt.Errorf("amazon product publish: seller sku required")
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, 0, fmt.Errorf("amazon product publish: encode listing payload: %w", err)
	}
	q := url.Values{}
	q.Set("marketplaceIds", strings.TrimSpace(marketplaceID))
	if loc := strings.TrimSpace(issueLocale); loc != "" {
		q.Set("issueLocale", loc)
	}
	code, raw, err := doSPAPI(ctx, cfg, http.MethodPut, listingsItemPath(sellerID, sellerSKU), q, payload, access)
	if err != nil {
		return nil, code, maybeRetryableAmazonPublishTransportErr(err)
	}
	if err := classifyAmazonPublishHTTPError(code, raw); err != nil {
		return nil, code, err
	}
	var root map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &root); err != nil {
			return nil, code, fmt.Errorf("amazon product publish: invalid json response")
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

func classifyAmazonPublishHTTPError(code int, raw []byte) error {
	switch code {
	case http.StatusOK, http.StatusAccepted:
		return nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("%w: amazon listings http %d: %s", platformp.ErrPlatformProductPublishPermissionDenied, code, apiErrorSnippet(raw))
	case http.StatusTooManyRequests:
		return fmt.Errorf("amazon product publish: retryable rate limit (http 429)")
	default:
		if code >= 500 {
			return fmt.Errorf("amazon product publish: retryable upstream error (http %d): %s", code, apiErrorSnippet(raw))
		}
		snip := apiErrorSnippet(raw)
		if code == http.StatusBadRequest && strings.Contains(strings.ToLower(snip), "attribute") {
			return fmt.Errorf("platform publish config incomplete: missing amazon required attributes: %s", snip)
		}
		if code < 200 || code >= 300 {
			return fmt.Errorf("amazon product publish http %d: %s", code, snip)
		}
		return nil
	}
}

func maybeRetryableAmazonPublishTransportErr(err error) error {
	if err == nil {
		return nil
	}
	s := strings.ToLower(err.Error())
	if strings.Contains(s, "timeout") || strings.Contains(s, "timed out") ||
		strings.Contains(s, "connection reset") || strings.Contains(s, "eof") ||
		strings.Contains(s, "context deadline") {
		return fmt.Errorf("amazon product publish: retryable: %w", err)
	}
	return err
}

func summarizeAmazonListingIssues(issues []amazonListingIssue) []map[string]any {
	out := make([]map[string]any, 0, len(issues))
	for _, it := range issues {
		row := map[string]any{}
		if strings.TrimSpace(it.Code) != "" {
			row["code"] = strings.TrimSpace(it.Code)
		}
		if strings.TrimSpace(it.Severity) != "" {
			row["severity"] = strings.TrimSpace(it.Severity)
		}
		if strings.TrimSpace(it.Message) != "" {
			row["message"] = strings.TrimSpace(it.Message)
		}
		if len(row) > 0 {
			out = append(out, row)
		}
	}
	return out
}

func hasAmazonListingErrorIssue(issues []amazonListingIssue) bool {
	for _, it := range issues {
		sev := strings.ToLower(strings.TrimSpace(it.Severity))
		if sev == "error" || sev == "errors" {
			return true
		}
	}
	return false
}
