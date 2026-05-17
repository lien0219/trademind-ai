package shopee

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

type shopeePublishMerged struct {
	CategoryID       uint64
	LogisticID       uint64
	BrandID          uint64
	BrandName        string
	Weight           float64
	DimLength        int
	DimWidth         int
	DimHeight        int
	Condition        string
	DaysToShip       int
	PublishAsDraft   bool
	VariationTierKey string // optional single-tier label override (options)
}

type tierDimSpec struct {
	Name       string
	SKUAttrKey string
}

func mergeShopeePublish(pub map[string]any, opt map[string]any) shopeePublishMerged {
	out := shopeePublishMerged{}
	base := loweredScalarMap(pub)
	ov := loweredScalarMap(opt)
	merged := overlayStringMaps(base, ov)

	out.CategoryID = parseUint64Merged(merged["default_category_id"])
	out.LogisticID = parseUint64Merged(merged["logistic_channel_id"])
	out.BrandID = parseUint64Merged(merged["default_brand_id"])
	out.BrandName = strings.TrimSpace(merged["default_brand_name"])
	out.Weight = parseFloatMerged(merged["default_weight"])
	out.DimLength = parseIntMerged(merged["default_length"], 1)
	out.DimWidth = parseIntMerged(merged["default_width"], 1)
	out.DimHeight = parseIntMerged(merged["default_height"], 1)
	out.Condition = strings.TrimSpace(merged["condition"])
	out.DaysToShip = parseIntMerged(merged["days_to_ship"], 0)
	out.PublishAsDraft = truthyString(merged["publish_as_draft"])
	out.VariationTierKey = strings.TrimSpace(merged["variation_tier_name"])

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

func truthyString(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func parseUint64Merged(s string) uint64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil || v == 0 {
		return 0
	}
	return v
}

func parseFloatMerged(s string) float64 {
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

func parseIntMerged(s string, def int) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

func validateShopeePublishMerged(m shopeePublishMerged) error {
	if m.CategoryID == 0 || m.LogisticID == 0 || m.Weight <= 0 {
		return fmt.Errorf("platform publish config incomplete: please configure settings.platform_publish_shopee first (missing default_category_id / logistic_channel_id / default_weight)")
	}
	return nil
}

func normalizeCondition(raw string) string {
	s := strings.TrimSpace(strings.ToUpper(raw))
	switch s {
	case "", "_CUSTOM":
		return "NEW"
	case "NEW", "USED":
		return s
	default:
		return s
	}
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

func skuVariationLabel(sku platformp.PlatformProductSKU) string {
	if x := strings.TrimSpace(sku.SKUName); x != "" {
		return x
	}
	if x := strings.TrimSpace(sku.SKUCode); x != "" {
		return x
	}
	if sku.LocalSKUID != uuid.Nil {
		return sku.LocalSKUID.String()
	}
	return "SKU"
}

func attrsJoinedLabel(attrs map[string]any) string {
	if len(attrs) == 0 {
		return ""
	}
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		k = strings.TrimSpace(k)
		if k == "" || strings.EqualFold(k, "sales_attributes") || strings.EqualFold(k, "product_attributes") || strings.HasPrefix(strings.ToLower(k), "shopee_") {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteString(" / ")
		}
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(strings.TrimSpace(fmt.Sprint(attrs[k])))
	}
	return strings.TrimSpace(b.String())
}

func parseTierDimSpecs(opt map[string]any) ([]tierDimSpec, error) {
	raw, ok := opt["shopee_tier_variation"]
	if !ok || raw == nil {
		return nil, nil
	}
	body, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("options.shopee_tier_variation must be JSON-serializable")
	}
	var arr []map[string]any
	if err := json.Unmarshal(body, &arr); err != nil {
		return nil, fmt.Errorf("options.shopee_tier_variation must be a JSON array")
	}
	out := make([]tierDimSpec, 0, len(arr))
	for _, row := range arr {
		if row == nil {
			continue
		}
		name := strings.TrimSpace(fmt.Sprint(row["name"]))
		key := strings.TrimSpace(fmt.Sprint(row["sku_attr_key"]))
		if key == "" {
			key = name
		}
		if name == "" || key == "" {
			return nil, fmt.Errorf("shopee_tier_variation entries require name and sku_attr_key")
		}
		out = append(out, tierDimSpec{Name: name, SKUAttrKey: key})
	}
	return out, nil
}

func parseShopeeAttributeList(attrs map[string]any, opt map[string]any) ([]map[string]any, error) {
	raw := extractShopeeAttrRaw(attrs, opt)
	if raw == nil {
		return nil, nil
	}
	body, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("shopee_attribute_list invalid")
	}
	var arr []map[string]any
	if err := json.Unmarshal(body, &arr); err != nil {
		return nil, fmt.Errorf("shopee_attribute_list must be a JSON array of objects")
	}
	out := make([]map[string]any, 0, len(arr))
	for _, row := range arr {
		if row == nil {
			continue
		}
		out = append(out, row)
	}
	return out, nil
}

func extractShopeeAttrRaw(attrs map[string]any, opt map[string]any) any {
	if len(opt) != 0 {
		if v, ok := opt["shopee_attribute_list"]; ok && v != nil {
			return v
		}
	}
	if len(attrs) != 0 {
		if v, ok := attrs["shopee_attribute_list"]; ok && v != nil {
			return v
		}
	}
	return nil
}

func attributeListToAPI(rows []map[string]any) ([]map[string]any, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		aidAny, ok := row["attribute_id"]
		if !ok {
			return nil, fmt.Errorf("shopee_attribute_list row missing attribute_id")
		}
		aid := parseUint64Merged(coerceScalarToString(aidAny))
		if aid == 0 {
			return nil, fmt.Errorf("shopee_attribute_list invalid attribute_id")
		}
		vals, ok := row["attribute_value_list"]
		if !ok || vals == nil {
			return nil, fmt.Errorf("shopee_attribute_list row missing attribute_value_list")
		}
		vb, err := json.Marshal(vals)
		if err != nil {
			return nil, fmt.Errorf("attribute_value_list must be JSON-serializable")
		}
		var vlist []map[string]any
		if err := json.Unmarshal(vb, &vlist); err != nil {
			return nil, fmt.Errorf("attribute_value_list must be a JSON array")
		}
		out = append(out, map[string]any{
			"attribute_id":         aid,
			"attribute_value_list": vlist,
		})
	}
	return out, nil
}

func buildTierMatrices(skus []platformp.PlatformProductSKU, dims []tierDimSpec) (tierVar []map[string]any, models []map[string]any, err error) {
	if len(dims) == 0 {
		return nil, nil, fmt.Errorf("shopee tier variation spec required")
	}
	optionLists := make([][]string, len(dims))
	for di, d := range dims {
		seen := map[string]bool{}
		var opts []string
		for _, sku := range skus {
			raw := attrValueCI(sku.Attrs, d.SKUAttrKey)
			if strings.TrimSpace(raw) == "" {
				return nil, nil, fmt.Errorf("shopee publish: SKU missing variation attribute %q", d.SKUAttrKey)
			}
			if !seen[raw] {
				seen[raw] = true
				opts = append(opts, raw)
			}
		}
		sort.Strings(opts)
		optionLists[di] = opts
	}

	tierVar = make([]map[string]any, 0, len(dims))
	for di, d := range dims {
		opts := optionLists[di]
		optObjs := make([]map[string]any, 0, len(opts))
		for _, op := range opts {
			optObjs = append(optObjs, map[string]any{"option": op})
		}
		tierVar = append(tierVar, map[string]any{
			"name":        d.Name,
			"option_list": optObjs,
		})
	}

	models = make([]map[string]any, 0, len(skus))
	for _, sku := range skus {
		idx := make([]int, len(dims))
		for di, d := range dims {
			want := attrValueCI(sku.Attrs, d.SKUAttrKey)
			pos := indexOfSorted(optionLists[di], want)
			if pos < 0 {
				return nil, nil, fmt.Errorf("shopee publish: inconsistent SKU attrs for %q", d.SKUAttrKey)
			}
			idx[di] = pos
		}
		models = append(models, map[string]any{
			"tier_index":     idx,
			"normal_stock":   maxInt(0, sku.Stock),
			"original_price": sku.Price,
			"model_sku":      modelSKUCode(sku),
		})
	}
	return tierVar, models, nil
}

func indexOfSorted(sorted []string, want string) int {
	for i, s := range sorted {
		if s == want {
			return i
		}
	}
	return -1
}

func attrValueCI(attrs map[string]any, key string) string {
	if len(attrs) == 0 {
		return ""
	}
	kwant := strings.TrimSpace(strings.ToLower(key))
	for k, v := range attrs {
		if strings.TrimSpace(strings.ToLower(k)) == kwant {
			return strings.TrimSpace(fmt.Sprint(v))
		}
	}
	return ""
}

func modelSKUCode(sku platformp.PlatformProductSKU) string {
	if x := strings.TrimSpace(sku.SKUCode); x != "" {
		return x
	}
	if sku.LocalSKUID != uuid.Nil {
		return sku.LocalSKUID.String()
	}
	return "default-sku"
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func buildSingleTierVariation(skus []platformp.PlatformProductSKU, tierName string, skuMainImageIDs map[int]string) ([]map[string]any, []map[string]any, error) {
	name := strings.TrimSpace(tierName)
	if name == "" {
		name = "Variant"
	}
	seen := map[string]bool{}
	labels := make([]string, len(skus))
	for i, sku := range skus {
		l := skuVariationLabel(sku)
		if strings.TrimSpace(sku.SKUName) == "" && strings.TrimSpace(sku.SKUCode) == "" {
			if alt := strings.TrimSpace(attrsJoinedLabel(sku.Attrs)); alt != "" {
				l = alt
			}
		}
		if seen[l] {
			return nil, nil, fmt.Errorf("shopee publish: duplicate variation label %q; use distinct SKU names/attrs or options.shopee_tier_variation", l)
		}
		seen[l] = true
		labels[i] = l
	}
	optObjs := make([]map[string]any, 0, len(labels))
	for i, l := range labels {
		obj := map[string]any{"option": l}
		if skuMainImageIDs != nil {
			if id := strings.TrimSpace(skuMainImageIDs[i]); id != "" {
				obj["image"] = map[string]any{"image_id": id}
			}
		}
		optObjs = append(optObjs, obj)
	}
	tierVar := []map[string]any{
		{"name": name, "option_list": optObjs},
	}
	models := make([]map[string]any, 0, len(skus))
	for i, sku := range skus {
		models = append(models, map[string]any{
			"tier_index":     []int{i},
			"normal_stock":   maxInt(0, sku.Stock),
			"original_price": sku.Price,
			"model_sku":      modelSKUCode(sku),
		})
	}
	return tierVar, models, nil
}

func buildAddItemPayload(draft platformp.PlatformProductDraft, merged shopeePublishMerged, imageIDs []string, attrRows []map[string]any, multiSKU bool, singleSKU *platformp.PlatformProductSKU) (map[string]any, error) {
	if len(imageIDs) == 0 {
		return nil, fmt.Errorf("shopee publish: no listing images")
	}
	attrAPI, err := attributeListToAPI(attrRows)
	if err != nil {
		return nil, err
	}

	body := map[string]any{
		"category_id": merged.CategoryID,
		"item_name":   strings.TrimSpace(draft.Title),
		"description": strings.TrimSpace(draft.Description),
		"image": map[string]any{
			"image_id_list": imageIDs,
		},
		"weight": merged.Weight,
		"dimension": map[string]any{
			"package_length": merged.DimLength,
			"package_width":  merged.DimWidth,
			"package_height": merged.DimHeight,
		},
		"logistic_info": []map[string]any{
			{
				"logistic_id":            merged.LogisticID,
				"enabled":                true,
				"is_free":                false,
				"shipping_fee":           0,
				"size_id":                0,
				"estimated_shipping_fee": 0,
			},
		},
		"condition": normalizeCondition(merged.Condition),
	}
	if merged.DaysToShip > 0 {
		body["days_to_ship"] = merged.DaysToShip
	}
	if merged.PublishAsDraft {
		body["item_status"] = "UNLIST"
	} else {
		body["item_status"] = "NORMAL"
	}
	if len(attrAPI) > 0 {
		body["attribute_list"] = attrAPI
	}
	if merged.BrandID > 0 {
		bm := map[string]any{"brand_id": merged.BrandID}
		if bn := strings.TrimSpace(merged.BrandName); bn != "" {
			bm["original_brand_name"] = bn
		}
		body["brand"] = bm
	}

	if multiSKU {
		body["has_model"] = true
		minP := draft.SKUs[0].Price
		for _, s := range draft.SKUs {
			if s.Price < minP {
				minP = s.Price
			}
		}
		body["original_price"] = minP
		body["normal_stock"] = 0
		body["item_sku"] = listingParentSKU(draft)
		return body, nil
	}

	if singleSKU == nil {
		return nil, fmt.Errorf("shopee publish: internal single SKU payload error")
	}
	body["has_model"] = false
	body["original_price"] = singleSKU.Price
	body["normal_stock"] = maxInt(0, singleSKU.Stock)
	body["item_sku"] = modelSKUCode(*singleSKU)
	return body, nil
}

func listingParentSKU(draft platformp.PlatformProductDraft) string {
	base := strings.TrimSpace(draft.ProductID.String())
	base = strings.ReplaceAll(base, "-", "")
	if len(base) > 50 {
		base = base[:50]
	}
	if base == "" {
		return "tm-item"
	}
	return "tm-" + base
}
