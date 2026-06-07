package collect

import (
	"encoding/json"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/modules/product"
)

func normalizeTaobaoTmallImageURL(raw string) string {
	u := strings.TrimSpace(raw)
	if u == "" {
		return ""
	}
	lower := strings.ToLower(u)
	if strings.Contains(lower, "/s.gif") || strings.Contains(lower, "spaceball.gif") {
		return ""
	}
	for _, suffix := range []string{"_.jpg", "_.jpeg", "_.png", "_.webp"} {
		if strings.HasSuffix(lower, suffix) {
			u = u[:len(u)-len(suffix)]
			break
		}
	}
	if i := strings.Index(u, "?"); i >= 0 {
		u = u[:i]
	}
	return u
}

func normalizeTaobaoTmallImport(source string, n *normalizedProduct, fullJSON json.RawMessage) (product.ImportDraftParams, json.RawMessage) {
	params := n.importParams(fullJSON)
	if !isTaobaoTmallCollectSource(source) {
		return params, fullJSON
	}

	var raw rawProductEnvelope
	_ = json.Unmarshal(fullJSON, &raw)

	if params.Currency == "" {
		params.Currency = "CNY"
	}

	params.MainImages = normalizeTaobaoTmallImageList(params.MainImages)
	params.DescriptionImages = normalizeTaobaoTmallImageList(params.DescriptionImages)
	for i := range params.SKUs {
		params.SKUs[i].ImageURL = normalizeTaobaoTmallImageURL(params.SKUs[i].ImageURL)
	}

	if len(params.SKUs) == 0 && raw.Raw.ProductPrice != nil && *raw.Raw.ProductPrice > 0 {
		price := *raw.Raw.ProductPrice
		params.SKUs = []product.ImportSKUParams{{
			SKUName: "默认规格",
			Price:   &price,
		}}
	}

	fullJSON = mergeRawExtractProvider(
		fullJSON,
		"taobao_tmall",
		raw.Raw.QualityWarnings,
		raw.Raw.ProductPrice,
		raw.Raw.PriceText,
		raw.Raw.PriceMin,
		raw.Raw.PriceMax,
		raw.Raw.PriceRange,
	)

	return params, fullJSON
}

func normalizeTaobaoTmallImageList(urls []string) []string {
	if len(urls) == 0 {
		return urls
	}
	out := make([]string, 0, len(urls))
	seen := make(map[string]struct{}, len(urls))
	for _, raw := range urls {
		u := normalizeTaobaoTmallImageURL(raw)
		if u == "" {
			continue
		}
		key := strings.ToLower(u)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, u)
	}
	return out
}
