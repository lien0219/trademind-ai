package productpublish

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
	"gorm.io/gorm"
)

const (
	PublishModeSaveAsPlatformDraft = "save_as_platform_draft"
	PublishModePublishOnline       = "publish_online"
)

// DouyinProductPayload is the normalized product.addV2 request body (safe for task detail display).
type DouyinProductPayload struct {
	OuterProductID string         `json:"outerProductId,omitempty"`
	Name           string         `json:"name"`
	CategoryLeafID string         `json:"categoryLeafId"`
	Pic            string         `json:"pic"`
	Description    string         `json:"description"`
	ProductFormat  map[string]any `json:"productFormat,omitempty"`
	SpecInfo       map[string]any `json:"specInfo,omitempty"`
	SpecPricesV2   []any          `json:"specPricesV2,omitempty"`
	Commit         bool           `json:"commit"`
	StartSaleType  int            `json:"startSaleType"`
	FreightID      int64          `json:"freightId"`
	Mobile         string         `json:"mobile,omitempty"`
}

// DouyinPayloadBuildResult holds payload builder output.
type DouyinPayloadBuildResult struct {
	Payload  *DouyinProductPayload
	APIReq   platformdouyin.CreateProductDraftRequest
	Warnings []product.DouyinMappingIssue
	Errors   []product.DouyinMappingIssue
}

// BuildDouyinProductPayload assembles product.addV2 payload from saved mapping config.
func BuildDouyinProductPayload(ctx context.Context, db *gorm.DB, productID uuid.UUID, configID string) (*DouyinPayloadBuildResult, error) {
	_ = configID
	if db == nil {
		return nil, fmt.Errorf("productpublish: no db")
	}
	var cfg product.ProductPlatformPublishConfig
	if err := db.Where("product_id = ? AND platform = ?", productID, "douyin_shop").First(&cfg).Error; err != nil {
		return nil, fmt.Errorf("douyin mapping config not found")
	}
	mapping := product.DouyinDraftMappingFromConfig(cfg)
	return buildDouyinPayloadFromMapping(mapping)
}

func buildDouyinPayloadFromMapping(m *product.DouyinDraftMapping) (*DouyinPayloadBuildResult, error) {
	out := &DouyinPayloadBuildResult{
		Warnings: append([]product.DouyinMappingIssue{}, m.Warnings...),
		Errors:   append([]product.DouyinMappingIssue{}, m.Errors...),
	}
	if m == nil {
		out.Errors = append(out.Errors, product.DouyinMappingIssue{
			Code: "DOUYIN_MAPPING_MISSING", Level: "error", Message: "Douyin mapping does not exist.",
		})
		return out, nil
	}
	if strings.TrimSpace(m.Title) == "" {
		out.Errors = append(out.Errors, product.DouyinMappingIssue{
			Code: product.DouyinTitleMissing, Level: "error", Field: "title", Message: "Douyin title is required.",
		})
	}
	if strings.TrimSpace(m.CategoryID) == "" {
		out.Errors = append(out.Errors, product.DouyinMappingIssue{
			Code: shop.DouyinCategoryNotSelected, Level: "error", Field: "categoryId", Message: "Douyin category is not selected.",
		})
	}
	mainPics := uploadedImageURLs(m.MainImages)
	if len(mainPics) == 0 {
		out.Errors = append(out.Errors, product.DouyinMappingIssue{
			Code: product.DouyinMainImageNotUploaded, Level: "error", Field: "mainImages", Message: "Main images must be uploaded to Douyin first.",
		})
	}
	detailPics := uploadedImageURLs(m.DetailImages)
	specInfo, specPrices, skuErrs := buildDouyinSpecPayload(m)
	out.Errors = append(out.Errors, skuErrs...)
	productFormat := buildDouyinProductFormat(m.Attributes)
	for _, a := range m.Attributes {
		if a.Required && !douyinValuePresent(a.Value) {
			out.Errors = append(out.Errors, product.DouyinMappingIssue{
				Code: shop.DouyinRequiredAttrMissing, Level: "error", Field: "attributes." + a.AttrID,
				Message: "Required Douyin attribute missing: " + firstNonEmptyStr(a.Name, a.AttrID),
			})
		}
	}
	if len(out.Errors) > 0 {
		return out, nil
	}
	desc := strings.Join(detailPics, "|")
	if desc == "" && strings.TrimSpace(m.Description) != "" {
		desc = strings.TrimSpace(m.Description)
	}
	payload := &DouyinProductPayload{
		OuterProductID: m.ProductID,
		Name:           strings.TrimSpace(m.Title),
		CategoryLeafID: strings.TrimSpace(m.CategoryID),
		Pic:            strings.Join(mainPics, "|"),
		Description:    desc,
		ProductFormat:  productFormat,
		SpecInfo:       specInfo,
		SpecPricesV2:   specPrices,
		Commit:         false,
		StartSaleType:  1,
		FreightID:      0,
	}
	out.Payload = payload
	out.APIReq = platformdouyin.CreateProductDraftRequest{
		OuterProductID: payload.OuterProductID,
		Name:           payload.Name,
		CategoryLeafID: payload.CategoryLeafID,
		Pic:            payload.Pic,
		Description:    payload.Description,
		ProductFormat:  productFormat,
		SpecInfo:       specInfo,
		SpecPricesV2:   toSpecPriceMaps(specPrices),
		FreightID:      payload.FreightID,
	}
	return out, nil
}

func uploadedImageURLs(images []product.DouyinDraftImage) []string {
	out := make([]string, 0, len(images))
	for _, img := range images {
		if !strings.EqualFold(img.UploadStatus, "uploaded") {
			continue
		}
		u := firstNonEmptyStr(img.PlatformImageURL, img.URL)
		if u != "" {
			out = append(out, u)
		}
	}
	return out
}

func buildDouyinProductFormat(attrs []product.DouyinDraftAttr) map[string]any {
	out := map[string]any{}
	for _, a := range attrs {
		if !douyinValuePresent(a.Value) {
			continue
		}
		entry := buildFormatEntry(a)
		if entry == nil {
			continue
		}
		out[a.AttrID] = []map[string]any{entry}
	}
	return out
}

func buildFormatEntry(a product.DouyinDraftAttr) map[string]any {
	name := strings.TrimSpace(a.Name)
	val := a.Value
	diyType := 0
	var valueNum float64
	switch x := val.(type) {
	case float64:
		valueNum = x
	case int:
		valueNum = float64(x)
	case json.Number:
		if n, err := x.Float64(); err == nil {
			valueNum = n
		}
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return nil
		}
		if n, err := strconv.ParseFloat(s, 64); err == nil {
			valueNum = n
		} else {
			name = firstNonEmptyStr(name, s)
			diyType = 1
		}
	default:
		if val == nil {
			return nil
		}
		name = fmt.Sprint(val)
		diyType = 1
	}
	entry := map[string]any{"diy_type": diyType}
	if diyType == 1 {
		entry["name"] = name
		entry["value"] = 0
	} else {
		entry["value"] = valueNum
		if name != "" {
			entry["name"] = name
		}
	}
	return entry
}

func buildDouyinSpecPayload(m *product.DouyinDraftMapping) (map[string]any, []any, []product.DouyinMappingIssue) {
	var errs []product.DouyinMappingIssue
	if len(m.SKUs) == 0 {
		return nil, nil, []product.DouyinMappingIssue{{
			Code: product.DouyinSKUMissing, Level: "error", Field: "skus", Message: "No SKU available.",
		}}
	}
	specProps := collectSpecProperties(m.SKUs)
	specInfo := map[string]any{"spec_values": specProps}
	specPrices := make([]any, 0, len(m.SKUs))
	for _, sku := range m.SKUs {
		if sku.Price <= 0 {
			errs = append(errs, product.DouyinMappingIssue{
				Code: product.DouyinSKUPriceInvalid, Level: "error", Field: "skus.price",
				Message: "SKU price must be greater than 0.", RelatedResourceID: sku.LocalSkuID,
			})
			continue
		}
		stock := 0
		if sku.Stock != nil {
			if *sku.Stock < 0 {
				errs = append(errs, product.DouyinMappingIssue{
					Code: product.DouyinStockInvalid, Level: "error", Field: "skus.stock",
					Message: "SKU stock cannot be negative.", RelatedResourceID: sku.LocalSkuID,
				})
				continue
			}
			stock = *sku.Stock
		}
		sellProps := buildSellProperties(sku, specProps)
		row := map[string]any{
			"sell_properties": sellProps,
			"stock_num":       stock,
			"price":           yuanToFen(sku.Price),
		}
		if id := strings.TrimSpace(sku.LocalSkuID); id != "" {
			row["outer_sku_id"] = id
		}
		specPrices = append(specPrices, row)
	}
	if len(errs) > 0 {
		return specInfo, specPrices, errs
	}
	return specInfo, specPrices, nil
}

func collectSpecProperties(skus []product.DouyinDraftSKU) []map[string]any {
	names := make([]string, 0)
	seen := map[string]struct{}{}
	for _, sku := range skus {
		for k := range sku.Attrs {
			k = strings.TrimSpace(k)
			if k == "" {
				continue
			}
			if _, ok := seen[k]; !ok {
				seen[k] = struct{}{}
				names = append(names, k)
			}
		}
	}
	if len(names) == 0 {
		names = []string{"规格"}
	}
	out := make([]map[string]any, 0, len(names))
	for _, name := range names {
		vals := make([]map[string]any, 0)
		valSeen := map[string]struct{}{}
		for _, sku := range skus {
			raw, ok := sku.Attrs[name]
			if !ok {
				continue
			}
			vn := fmt.Sprint(raw)
			if vn == "" {
				continue
			}
			if _, ok := valSeen[vn]; ok {
				continue
			}
			valSeen[vn] = struct{}{}
			vals = append(vals, map[string]any{"value_name": vn, "value_id": 0})
		}
		if len(vals) == 0 && name == "规格" {
			vals = []map[string]any{{"value_name": "默认", "value_id": 0}}
		}
		out = append(out, map[string]any{
			"property_name": name,
			"property_id":   0,
			"values":        vals,
		})
	}
	return out
}

func buildSellProperties(sku product.DouyinDraftSKU, specProps []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(specProps))
	for _, sp := range specProps {
		pn, _ := sp["property_name"].(string)
		pn = strings.TrimSpace(pn)
		if pn == "" {
			continue
		}
		vn := "默认"
		if v, ok := sku.Attrs[pn]; ok {
			vn = strings.TrimSpace(fmt.Sprint(v))
		}
		if vn == "" {
			vn = "默认"
		}
		out = append(out, map[string]any{"property_name": pn, "value_name": vn})
	}
	if len(out) == 0 {
		out = []map[string]any{{"property_name": "规格", "value_name": "默认"}}
	}
	return out
}

func yuanToFen(y float64) int64 {
	return int64(math.Round(y * 100))
}

func toSpecPriceMaps(rows []any) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		if m, ok := r.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func douyinValuePresent(v any) bool {
	if v == nil {
		return false
	}
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x) != ""
	case float64:
		return true
	case int:
		return true
	case bool:
		return true
	default:
		return fmt.Sprint(v) != ""
	}
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

func sanitizeDouyinPayloadForDisplay(p *DouyinProductPayload) map[string]any {
	if p == nil {
		return nil
	}
	b, _ := json.Marshal(p)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}
