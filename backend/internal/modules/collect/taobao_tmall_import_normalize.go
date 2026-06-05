package collect

import (
	"encoding/json"

	"github.com/trademind-ai/trademind/backend/internal/modules/product"
)

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
