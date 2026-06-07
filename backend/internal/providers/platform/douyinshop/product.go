package douyinshop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const MethodProductAddV2 = "product.addV2"

// CreateProductDraftRequest is the normalized Douyin product.addV2 payload.
// Official Douyin OpenAPI documentation checked for Phase 7:
// product.addV2 creates product + specs + SKUs in one call.
// commit=false saves to platform draft box only; start_sale_type=1 keeps off shelf.
type CreateProductDraftRequest struct {
	OuterProductID  string
	Name            string
	CategoryLeafID  string
	Pic             string
	Description     string
	ProductFormat   map[string]any
	SpecInfo        map[string]any
	SpecPricesV2    []map[string]any
	FreightID       int64
	Mobile          string
	StandardBrandID int64
	PublishConfig   map[string]string
}

// PlatformProductResult is returned after a successful product.addV2 call.
type PlatformProductResult struct {
	PlatformProductID string         `json:"platformProductId"`
	PlatformStatus    string         `json:"platformStatus"`
	RequestID         string         `json:"requestId,omitempty"`
	SKUMappings       []SKUMapping   `json:"skuMappings,omitempty"`
	Raw               map[string]any `json:"raw,omitempty"`
}

// SKUMapping links local SKU to platform SKU when returned by the API.
type SKUMapping struct {
	LocalSKUID    string  `json:"localSkuId,omitempty"`
	OuterSKUID    string  `json:"outerSkuId,omitempty"`
	PlatformSKUID string  `json:"platformSkuId,omitempty"`
	Price         float64 `json:"price,omitempty"`
	Stock         int     `json:"stock,omitempty"`
}

// CreateProductDraft calls product.addV2 with commit=false (platform draft only).
func (c *Client) CreateProductDraft(ctx context.Context, shopID string, req CreateProductDraftRequest) (*PlatformProductResult, error) {
	_ = strings.TrimSpace(shopID)
	if strings.TrimSpace(req.Name) == "" {
		return nil, NewError(CodeDouyinProductPayloadInvalid, "douyin product name required", "", "", "")
	}
	if strings.TrimSpace(req.CategoryLeafID) == "" {
		return nil, NewError(CodeDouyinCategoryMissing, "douyin category_leaf_id required", "", "", "")
	}
	if strings.TrimSpace(req.Pic) == "" {
		return nil, NewError(CodeDouyinMainImageNotUploaded, "douyin main image required", "", "", "")
	}
	params := map[string]any{
		"product_type":       0,
		"category_leaf_id":   strings.TrimSpace(req.CategoryLeafID),
		"name":               strings.TrimSpace(req.Name),
		"pic":                strings.TrimSpace(req.Pic),
		"description":        strings.TrimSpace(req.Description),
		"pay_type":           1,
		"reduce_type":        1,
		"freight_id":         req.FreightID,
		"delivery_delay_day": pickInt64(req.PublishConfig, "delivery_delay_day", 2),
		"presell_type":       0,
		"mobile":             firstNonEmpty(strings.TrimSpace(req.Mobile), configString(req.PublishConfig, "default_mobile", "40012345")),
		"commit":             false,
		"start_sale_type":    1,
		"standard_brand_id":  pickBrandID(req),
		"after_sale_service": configString(req.PublishConfig, "after_sale_service", `{"supply_day_return_selector":"7-1"}`),
	}
	if v := strings.TrimSpace(req.OuterProductID); v != "" {
		params["outer_product_id"] = v
	}
	if len(req.ProductFormat) > 0 {
		b, err := json.Marshal(req.ProductFormat)
		if err != nil {
			return nil, err
		}
		params["product_format_new"] = string(b)
	}
	if len(req.SpecInfo) > 0 {
		b, err := json.Marshal(req.SpecInfo)
		if err != nil {
			return nil, err
		}
		params["spec_info"] = string(b)
	}
	if len(req.SpecPricesV2) > 0 {
		b, err := json.Marshal(req.SpecPricesV2)
		if err != nil {
			return nil, err
		}
		params["spec_prices_v2"] = string(b)
	}

	var data map[string]any
	if err := c.Do(ctx, MethodProductAddV2, params, &data); err != nil {
		return nil, mapCreateProductError(err)
	}
	res := parseCreateProductResult(data)
	if res.PlatformProductID == "" {
		return nil, NewError(CodeDouyinCreateProductFailed, "douyin product create response missing product id", "", "", res.RequestID)
	}
	res.Raw = sanitizeRawMap(data)
	return res, nil
}

func pickBrandID(req CreateProductDraftRequest) int64 {
	if req.StandardBrandID > 0 {
		return req.StandardBrandID
	}
	if v := configString(req.PublishConfig, "standard_brand_id", ""); v != "" {
		if n, err := parseInt64(v); err == nil && n > 0 {
			return n
		}
	}
	return 596120136
}

func pickInt64(cfg map[string]string, key string, def int64) int64 {
	if cfg == nil {
		return def
	}
	if v := strings.TrimSpace(cfg[key]); v != "" {
		if n, err := parseInt64(v); err == nil {
			return n
		}
	}
	return def
}

func configString(cfg map[string]string, key, def string) string {
	if cfg == nil {
		return def
	}
	if v := strings.TrimSpace(cfg[key]); v != "" {
		return v
	}
	return def
}

func mapString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if m == nil {
			return ""
		}
		if v, ok := m[k]; ok && v != nil {
			switch x := v.(type) {
			case string:
				if s := strings.TrimSpace(x); s != "" {
					return s
				}
			case float64:
				return fmt.Sprintf("%.0f", x)
			case json.Number:
				return x.String()
			case int64:
				return fmt.Sprintf("%d", x)
			case int:
				return fmt.Sprintf("%d", x)
			}
		}
	}
	return ""
}

func parseInt64(s string) (int64, error) {
	var n int64
	_, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &n)
	return n, err
}

func parseCreateProductResult(data map[string]any) *PlatformProductResult {
	if data == nil {
		return &PlatformProductResult{PlatformStatus: "draft_created"}
	}
	pid := firstNonEmpty(
		mapString(data, "product_id"),
		mapString(data, "property_id"),
		mapString(data, "id"),
	)
	status := firstNonEmpty(mapString(data, "status"), "draft_created")
	out := &PlatformProductResult{
		PlatformProductID: pid,
		PlatformStatus:    status,
		RequestID:         mapString(data, "request_id"),
	}
	out.SKUMappings = parseSKUMappings(data)
	return out
}

func parseSKUMappings(data map[string]any) []SKUMapping {
	raw, ok := data["sku"].([]any)
	if !ok {
		if m, ok := data["skus"].([]any); ok {
			raw = m
		}
	}
	if len(raw) == 0 {
		return nil
	}
	out := make([]SKUMapping, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, SKUMapping{
			OuterSKUID:    firstNonEmpty(mapString(m, "outer_sku_id"), mapString(m, "out_sku_id")),
			PlatformSKUID: firstNonEmpty(mapString(m, "sku_id"), mapString(m, "id")),
		})
	}
	return out
}

func mapCreateProductError(err error) error {
	if err == nil {
		return nil
	}
	var de *Error
	if AsError(err, &de) {
		switch de.Code {
		case CodeDouyinAuthExpired:
			return NewError(CodeDouyinAuthExpired, de.Message, de.PlatformCode, de.PlatformMessage, de.RequestID)
		case CodeDouyinPermissionDenied:
			return NewError(CodeDouyinPermissionDenied, de.Message, de.PlatformCode, de.PlatformMessage, de.RequestID)
		case CodeDouyinRateLimited:
			return NewError(CodeDouyinRateLimited, de.Message, de.PlatformCode, de.PlatformMessage, de.RequestID)
		case CodeDouyinRequestTimeout:
			return NewError(CodeDouyinRequestTimeout, de.Message, de.PlatformCode, de.PlatformMessage, de.RequestID)
		default:
			return NewError(CodeDouyinCreateProductFailed, "douyin product create failed", de.PlatformCode, de.PlatformMessage, de.RequestID)
		}
	}
	return NewError(CodeDouyinCreateProductFailed, "douyin product create failed", "", SanitizeErrorText(err.Error()), "")
}
