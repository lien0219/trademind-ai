package collect

import (
	"encoding/json"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/modules/product"
)

// normalizePinduoduoImport adjusts price/currency and may synthesize a default SKU for pinduoduo beta collects.
func normalizePinduoduoImport(source string, n *normalizedProduct, fullJSON json.RawMessage) (product.ImportDraftParams, json.RawMessage) {
	params := n.importParams(fullJSON)
	if !strings.EqualFold(strings.TrimSpace(source), "pinduoduo") && !strings.EqualFold(strings.TrimSpace(source), "pdd") {
		return params, fullJSON
	}

	var raw rawProductEnvelope
	_ = json.Unmarshal(fullJSON, &raw)

	curr := strings.TrimSpace(params.Currency)
	if curr != "" && looksLikePriceText(curr) {
		if p, c := parsePriceCurrency(curr); p > 0 {
			if raw.Raw.ProductPrice == nil || *raw.Raw.ProductPrice <= 0 {
				raw.Raw.ProductPrice = &p
			}
			if c != "" {
				params.Currency = c
			} else {
				params.Currency = "CNY"
			}
		}
	}

	if params.Currency == "" {
		params.Currency = "CNY"
	}

	if strings.TrimSpace(params.Description) == "" {
		var top struct {
			MainDescription string `json:"mainDescription"`
		}
		_ = json.Unmarshal(fullJSON, &top)
		if s := strings.TrimSpace(top.MainDescription); s != "" {
			params.Description = s
		} else if s := strings.TrimSpace(raw.Raw.MainDescription); s != "" {
			params.Description = s
		}
	}

	// Only synthesize default SKU when collector returned no SKU lines.
	if len(params.SKUs) == 0 && raw.Raw.ProductPrice != nil && *raw.Raw.ProductPrice > 0 {
		price := *raw.Raw.ProductPrice
		params.SKUs = []product.ImportSKUParams{{
			SKUName: "默认规格",
			Price:   &price,
		}}
	}

	fullJSON = mergeRawExtractProvider(
		fullJSON,
		"pinduoduo",
		raw.Raw.QualityWarnings,
		raw.Raw.ProductPrice,
		raw.Raw.PriceText,
		raw.Raw.PriceMin,
		raw.Raw.PriceMax,
		raw.Raw.PriceRange,
	)

	return params, fullJSON
}

func mergeRawExtractProvider(
	full json.RawMessage,
	provider string,
	warnings []string,
	price *float64,
	priceText string,
	priceMin *float64,
	priceMax *float64,
	priceRange string,
) json.RawMessage {
	var m map[string]any
	if err := json.Unmarshal(full, &m); err != nil {
		return full
	}
	rawObj, _ := m["raw"].(map[string]any)
	if rawObj == nil {
		rawObj = map[string]any{}
	}
	rawObj["extractProvider"] = provider
	if len(warnings) > 0 {
		rawObj["qualityWarnings"] = warnings
	}
	if price != nil {
		rawObj["productPrice"] = *price
	}
	if priceMin != nil {
		rawObj["priceMin"] = *priceMin
	}
	if priceMax != nil {
		rawObj["priceMax"] = *priceMax
	}
	if priceRange != "" {
		rawObj["priceRange"] = priceRange
	}
	if priceText != "" {
		rawObj["priceText"] = priceText
	}
	m["raw"] = rawObj
	out, err := json.Marshal(m)
	if err != nil {
		return full
	}
	return out
}
