package lazada

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

const lazadaShortDescMaxRunes = 500

type lazadaPublishMerged struct {
	PrimaryCategory string
	Brand           string
	PackageWeight   float64
	PackageLength   float64
	PackageWidth    float64
	PackageHeight   float64
	WarrantyType    string
	WarrantyPeriod  string
	DeliveryOption  string
	PublishAsDraft  bool
}

func mergeLazadaPublish(pub map[string]any, opt map[string]any) lazadaPublishMerged {
	base := loweredScalarMap(pub)
	ov := loweredScalarMap(opt)
	merged := overlayStringMaps(base, ov)

	return lazadaPublishMerged{
		PrimaryCategory: strings.TrimSpace(merged["default_category_id"]),
		Brand:           strings.TrimSpace(merged["default_brand_id"]),
		PackageWeight:   parsePositiveFloat(merged["package_weight"]),
		PackageLength:   parsePositiveFloat(merged["package_length"]),
		PackageWidth:    parsePositiveFloat(merged["package_width"]),
		PackageHeight:   parsePositiveFloat(merged["package_height"]),
		WarrantyType:    strings.TrimSpace(merged["warranty_type"]),
		WarrantyPeriod:  strings.TrimSpace(merged["warranty_period"]),
		DeliveryOption:  strings.TrimSpace(merged["delivery_option"]),
		PublishAsDraft:  truthyStringLazada(merged["publish_as_draft"]),
	}
}

func validateLazadaPublishMerged(m lazadaPublishMerged) error {
	if strings.TrimSpace(m.PrimaryCategory) == "" ||
		m.PackageWeight <= 0 || m.PackageLength <= 0 || m.PackageWidth <= 0 || m.PackageHeight <= 0 {
		return fmt.Errorf("platform publish config incomplete: missing default_category_id / package_weight / package_size")
	}
	if strings.TrimSpace(m.DeliveryOption) == "" {
		return fmt.Errorf("platform publish config incomplete: please configure settings.platform_publish_lazada first")
	}
	return nil
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
		out[kk] = coerceScalarToStringLazada(v)
	}
	return out
}

func overlayStringMaps(base, over map[string]string) map[string]string {
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

func coerceScalarToStringLazada(v any) string {
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

func truthyStringLazada(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func parsePositiveFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || v <= 0 {
		return 0
	}
	return v
}

func orderedListingImages(images []platformp.PlatformProductImage) []platformp.PlatformProductImage {
	type tagged struct {
		im   platformp.PlatformProductImage
		rank int
		ord  int
	}
	var rows []tagged
	for _, im := range images {
		t := strings.ToLower(strings.TrimSpace(im.Type))
		rank := 3
		switch t {
		case "main":
			rank = 0
		case "detail", "description":
			rank = 1
		case "sku":
			rank = 2
		}
		rows = append(rows, tagged{im: im, rank: rank, ord: im.SortOrder})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].rank != rows[j].rank {
			return rows[i].rank < rows[j].rank
		}
		return rows[i].ord < rows[j].ord
	})
	out := make([]platformp.PlatformProductImage, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.im)
	}
	return out
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max])
}

func shortDescriptionFromDraft(title, description string) string {
	d := strings.TrimSpace(description)
	if d == "" {
		return truncateRunes(title, lazadaShortDescMaxRunes)
	}
	plain := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(d, "\r", " "), "\n", " "))
	if utf8.RuneCountInString(plain) <= lazadaShortDescMaxRunes {
		return plain
	}
	return truncateRunes(plain, lazadaShortDescMaxRunes)
}

// mergeLazadaAttributeMaps builds string attributes for Lazada CreateProduct (values only; keys from platform / user).
func mergeLazadaAttributeMaps(merged lazadaPublishMerged, draft platformp.PlatformProductDraft, options map[string]any) (map[string]string, error) {
	out := map[string]string{}

	if merged.WarrantyType != "" {
		out["warranty_type"] = merged.WarrantyType
	}
	if merged.WarrantyPeriod != "" {
		out["warranty"] = merged.WarrantyPeriod
	}
	if merged.DeliveryOption != "" {
		out["delivery_option"] = merged.DeliveryOption
	}

	for k, v := range draft.Attributes {
		kk := strings.TrimSpace(k)
		if kk == "" {
			continue
		}
		out[kk] = coerceScalarToStringLazada(v)
	}

	overlay := extractLazadaOptionsAttributes(options)
	for k, v := range overlay {
		if strings.TrimSpace(v) != "" {
			out[k] = v
		}
	}

	title := strings.TrimSpace(draft.Title)
	if title == "" {
		return nil, fmt.Errorf("product title is required")
	}
	out["name"] = title

	desc := strings.TrimSpace(draft.Description)
	if desc == "" {
		return nil, fmt.Errorf("product description is required")
	}
	out["description"] = desc

	out["short_description"] = shortDescriptionFromDraft(title, desc)

	brand := strings.TrimSpace(merged.Brand)
	if b := strings.TrimSpace(out["brand"]); b != "" {
		brand = b
	}
	if brand == "" {
		return nil, fmt.Errorf("platform publish config incomplete: please configure settings.platform_publish_lazada first (brand is required: default_brand_id or lazada_attributes.brand)")
	}
	out["brand"] = brand

	return out, nil
}

func extractLazadaOptionsAttributes(options map[string]any) map[string]string {
	out := map[string]string{}
	if len(options) == 0 {
		return out
	}
	raw, ok := options["lazada_attributes"]
	if !ok || raw == nil {
		return out
	}
	switch t := raw.(type) {
	case map[string]any:
		for k, v := range t {
			kk := strings.TrimSpace(k)
			if kk == "" {
				continue
			}
			out[kk] = coerceScalarToStringLazada(v)
		}
	case map[string]string:
		for k, v := range t {
			kk := strings.TrimSpace(k)
			if kk == "" {
				continue
			}
			if strings.TrimSpace(v) != "" {
				out[kk] = strings.TrimSpace(v)
			}
		}
	default:
		// Accept JSON object string
		s := strings.TrimSpace(fmt.Sprint(t))
		if s == "" || s == "{}" {
			return out
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(s), &m); err == nil {
			for k, v := range m {
				kk := strings.TrimSpace(k)
				if kk == "" {
					continue
				}
				out[kk] = coerceScalarToStringLazada(v)
			}
		}
	}
	return out
}

func sellerSkuCode(sku platformp.PlatformProductSKU) string {
	if s := strings.TrimSpace(sku.SKUCode); s != "" {
		return s
	}
	x := strings.ReplaceAll(sku.LocalSKUID.String(), "-", "")
	if len(x) > 32 {
		return x[:32]
	}
	return x
}

func buildLazadaSKUEntries(draft platformp.PlatformProductDraft, merged lazadaPublishMerged, imageURLs []string, skuImagePrepend []string) ([]map[string]any, error) {
	if len(draft.SKUs) == 0 {
		return nil, fmt.Errorf("product SKU is required for publish")
	}
	if len(skuImagePrepend) != len(draft.SKUs) {
		skuImagePrepend = make([]string, len(draft.SKUs))
	}
	max := 8
	if len(imageURLs) > max {
		imageURLs = imageURLs[:max]
	}

	weightStr := formatLazadaDecimal(merged.PackageWeight, 3)
	lenStr := formatLazadaDecimal(merged.PackageLength, 2)
	wStr := formatLazadaDecimal(merged.PackageWidth, 2)
	hStr := formatLazadaDecimal(merged.PackageHeight, 2)

	out := make([]map[string]any, 0, len(draft.SKUs))
	for i, sku := range draft.SKUs {
		skuMap := map[string]any{
			"SellerSku":       sellerSkuCode(sku),
			"price":           formatLazadaDecimal(sku.Price, 2),
			"quantity":        strconv.Itoa(intMax(sku.Stock, 0)),
			"package_weight":  weightStr,
			"package_length":  lenStr,
			"package_width":   wStr,
			"package_height":  hStr,
			"package_content": "",
		}
		for ak, av := range sku.Attrs {
			k := strings.TrimSpace(ak)
			if k == "" {
				continue
			}
			skuMap[k] = coerceScalarToStringLazada(av)
		}
		urls := make([]string, 0, len(imageURLs)+1)
		if u := strings.TrimSpace(skuImagePrepend[i]); u != "" {
			urls = append(urls, u)
		}
		urls = append(urls, imageURLs...)
		if len(urls) > max {
			urls = urls[:max]
		}
		if len(urls) > 0 {
			imgObjs := make([]any, 0, len(urls))
			for _, u := range urls {
				imgObjs = append(imgObjs, u)
			}
			skuMap["Images"] = map[string]any{"Image": imgObjs}
		}
		out = append(out, skuMap)
	}
	return out, nil
}

func intMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func formatLazadaDecimal(f float64, prec int) string {
	s := strconv.FormatFloat(f, 'f', prec, 64)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	if s == "" || s == "-" {
		return "0"
	}
	return s
}

func buildCreateProductPayloadStr(merged lazadaPublishMerged, prodAttrs map[string]string, skuEntries []map[string]any) (string, error) {
	attrAny := map[string]any{}
	for k, v := range prodAttrs {
		attrAny[k] = v
	}
	product := map[string]any{
		"PrimaryCategory": merged.PrimaryCategory,
		"Attributes":      attrAny,
		"Skus": map[string]any{
			"Sku": skuEntries,
		},
	}

	wrap := map[string]any{
		"Request": map[string]any{
			"Product": product,
		},
	}
	b, err := json.Marshal(wrap)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
