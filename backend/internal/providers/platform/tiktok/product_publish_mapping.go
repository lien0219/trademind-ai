package tiktok

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

type tiktokPublishMerged struct {
	CategoryID         string
	ShippingTemplateID string
	WarehouseID        string
	BrandID            string
	WeightValue        string
	WeightUnit         string
	DimLength          string
	DimWidth           string
	DimHeight          string
	DimUnit            string
	PublishAsDraft     bool
	ProductStatus      string
	SizeChartID        string
}

func mergeTikTokPublish(pub map[string]any, opt map[string]any) tiktokPublishMerged {
	out := tiktokPublishMerged{
		WeightUnit: "KILOGRAM",
		DimUnit:    "CENTIMETER",
	}
	base := loweredScalarMap(pub)
	ov := loweredScalarMap(opt)
	merged := overlayMaps(base, ov)

	out.CategoryID = merged["default_category_id"]
	out.ShippingTemplateID = merged["shipping_template_id"]
	out.WarehouseID = merged["warehouse_id"]
	out.BrandID = merged["default_brand_id"]
	out.WeightValue = strings.TrimSpace(merged["default_weight"])
	out.DimLength = strings.TrimSpace(merged["default_length"])
	out.DimWidth = strings.TrimSpace(merged["default_width"])
	out.DimHeight = strings.TrimSpace(merged["default_height"])
	out.PublishAsDraft = truthy(merged["publish_as_draft"])
	out.ProductStatus = merged["product_status"]
	out.SizeChartID = merged["size_chart_id"]

	return out
}

func loweredScalarMap(in map[string]any) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		kk := strings.ToLower(strings.TrimSpace(k))
		if kk == "" {
			continue
		}
		out[kk] = coerceScalarToString(v)
	}
	return out
}

func overlayMaps(base, over map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range over {
		if strings.TrimSpace(v) != "" {
			out[k] = strings.TrimSpace(v)
		}
	}
	return out
}

func coerceScalarToString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case bool:
		if x {
			return "true"
		}
		return "false"
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case string:
		return strings.TrimSpace(x)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", x))
	}
}

func truthy(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func validateMergedRequired(m tiktokPublishMerged) error {
	var missing []string
	if strings.TrimSpace(m.CategoryID) == "" {
		missing = append(missing, "default_category_id")
	}
	if strings.TrimSpace(m.ShippingTemplateID) == "" {
		missing = append(missing, "shipping_template_id")
	}
	if strings.TrimSpace(m.WarehouseID) == "" {
		missing = append(missing, "warehouse_id")
	}
	if len(missing) > 0 {
		return fmt.Errorf("platform publish config incomplete: missing default_category_id / shipping_template_id / warehouse_id")
	}
	return nil
}

func optionalDecimalString(s string, fallback string) string {
	s = strings.TrimSpace(s)
	if s != "" {
		return s
	}
	return strings.TrimSpace(fallback)
}

func parseSalesAttributes(attrs map[string]any) ([]map[string]interface{}, error) {
	if attrs == nil {
		return nil, nil
	}
	raw, ok := attrs["sales_attributes"]
	if !ok || raw == nil {
		return nil, nil
	}
	body, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("sales_attributes must be JSON-serializable")
	}
	var arr []map[string]interface{}
	if err := json.Unmarshal(body, &arr); err != nil {
		return nil, fmt.Errorf("sales_attributes must be a JSON array of objects")
	}
	out := make([]map[string]interface{}, 0, len(arr))
	for _, row := range arr {
		if row == nil {
			continue
		}
		out = append(out, row)
	}
	return out, nil
}

func parseProductAttributes(attrs map[string]any, opt map[string]any) ([]map[string]interface{}, error) {
	if attrs != nil {
		if raw, ok := attrs["product_attributes"]; ok && raw != nil {
			body, err := json.Marshal(raw)
			if err != nil {
				return nil, fmt.Errorf("product_attributes invalid")
			}
			var arr []map[string]interface{}
			if err := json.Unmarshal(body, &arr); err != nil {
				return nil, fmt.Errorf("product_attributes must be a JSON array")
			}
			return arr, nil
		}
	}
	if len(opt) > 0 {
		if raw, ok := opt["product_attributes"]; ok && raw != nil {
			body, err := json.Marshal(raw)
			if err != nil {
				return nil, fmt.Errorf("product_attributes invalid")
			}
			var arr []map[string]interface{}
			if err := json.Unmarshal(body, &arr); err != nil {
				return nil, fmt.Errorf("options.product_attributes must be a JSON array")
			}
			return arr, nil
		}
	}
	return nil, nil
}

func buildCreateProductBody(draft platformp.PlatformProductDraft, merged tiktokPublishMerged, mainImageURIs []string, deliveryOptionID string, productAttrs []map[string]interface{}) (map[string]interface{}, error) {
	if len(mainImageURIs) == 0 {
		return nil, fmt.Errorf("tiktok product publish: no uploaded main images")
	}
	mainImages := make([]map[string]interface{}, 0, len(mainImageURIs))
	for _, u := range mainImageURIs {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		mainImages = append(mainImages, map[string]interface{}{"uri": u})
	}
	if len(mainImages) == 0 {
		return nil, fmt.Errorf("tiktok product publish: no valid image uri")
	}

	skusBody := make([]interface{}, 0, len(draft.SKUs))
	for _, sku := range draft.SKUs {
		sa, err := parseSalesAttributes(sku.Attrs)
		if err != nil {
			return nil, err
		}
		if len(draft.SKUs) > 1 && len(sa) == 0 {
			return nil, fmt.Errorf("tiktok product publish: multi-SKU listings require sales_attributes on each SKU attrs JSON (Partner Center category rules)")
		}
		priceAmount := fmt.Sprintf("%.2f", sku.Price)
		if sku.Price < 0 {
			return nil, fmt.Errorf("tiktok product publish: invalid sku price")
		}
		extSKU := strings.TrimSpace(sku.SKUCode)
		if extSKU == "" {
			extSKU = sku.LocalSKUID.String()
		}
		row := map[string]interface{}{
			"inventory": []interface{}{
				map[string]interface{}{
					"warehouse_id": strings.TrimSpace(merged.WarehouseID),
					"quantity":     maxInt(0, sku.Stock),
				},
			},
			"price": map[string]interface{}{
				"currency": strings.TrimSpace(strings.ToUpper(draft.Currency)),
				"amount":   priceAmount,
			},
			"seller_sku": extSKU,
		}
		if sku.LocalSKUID != uuid.Nil {
			row["external_sku_id"] = sku.LocalSKUID.String()
		}
		if len(sa) > 0 {
			row["sales_attributes"] = sa
		}
		skusBody = append(skusBody, row)
	}

	pkgW := map[string]interface{}{
		"value": optionalDecimalString(merged.WeightValue, "0.1"),
		"unit":  merged.WeightUnit,
	}
	dim := map[string]interface{}{
		"length": optionalDecimalString(merged.DimLength, "10"),
		"width":  optionalDecimalString(merged.DimWidth, "10"),
		"height": optionalDecimalString(merged.DimHeight, "10"),
		"unit":   merged.DimUnit,
	}

	body := map[string]interface{}{
		"title":               strings.TrimSpace(draft.Title),
		"description":         strings.TrimSpace(draft.Description),
		"category_id":         strings.TrimSpace(merged.CategoryID),
		"main_images":         mainImages,
		"skus":                skusBody,
		"package_weight":      pkgW,
		"package_dimensions":  dim,
		"delivery_option_ids": []interface{}{strings.TrimSpace(deliveryOptionID)},
		"external_product_id": draft.ProductID.String(),
	}
	if bid := strings.TrimSpace(merged.BrandID); bid != "" {
		body["brand_id"] = bid
	}
	if merged.PublishAsDraft {
		body["save_mode"] = "AS_DRAFT"
	} else {
		body["save_mode"] = "LISTING"
	}
	if ps := strings.TrimSpace(merged.ProductStatus); ps != "" {
		body["product_status"] = ps
	}
	if sid := strings.TrimSpace(merged.SizeChartID); sid != "" {
		body["size_chart"] = map[string]interface{}{
			"template": map[string]interface{}{"id": sid},
		}
	}
	if len(productAttrs) > 0 {
		arr := make([]interface{}, 0, len(productAttrs))
		for _, p := range productAttrs {
			if p != nil {
				arr = append(arr, p)
			}
		}
		if len(arr) > 0 {
			body["product_attributes"] = arr
		}
	}
	return body, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
