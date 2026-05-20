package collect

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/modules/product"
)

var (
	cnyPricePattern = regexp.MustCompile(`(?i)[¥￥]|元`)
	usdPricePattern = regexp.MustCompile(`\$`)
)

type rawProductEnvelope struct {
	Raw struct {
		ProductPrice     *float64      `json:"productPrice"`
		PriceMin         *float64      `json:"priceMin"`
		PriceMax         *float64      `json:"priceMax"`
		PriceText        string        `json:"priceText"`
		PriceRange       string        `json:"priceRange"`
		MainDescription  string        `json:"mainDescription"`
		QualityWarnings  []string      `json:"qualityWarnings"`
		Warnings         []string      `json:"warnings"`
		TitleDiagnostics *titleDiagRaw `json:"titleDiagnostics"`
	} `json:"raw"`
}

type titleDiagRaw struct {
	SuspectWrongTitle bool `json:"suspectWrongTitle"`
}

// normalizeCustomImport adjusts currency/price and may synthesize a default SKU for custom collects.
func normalizeCustomImport(source string, n *normalizedProduct, fullJSON json.RawMessage) (product.ImportDraftParams, json.RawMessage) {
	params := n.importParams(fullJSON)
	if !strings.EqualFold(strings.TrimSpace(source), "custom") {
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

	if len(params.SKUs) == 0 && raw.Raw.ProductPrice != nil && *raw.Raw.ProductPrice > 0 {
		price := *raw.Raw.ProductPrice
		params.SKUs = []product.ImportSKUParams{{
			SKUName: "默认规格",
			Price:   &price,
		}}
	}

	if raw.Raw.TitleDiagnostics != nil && raw.Raw.TitleDiagnostics.SuspectWrongTitle {
		warn := "当前标题可能不是商品标题，请人工核对后再发布。"
		raw.Raw.QualityWarnings = appendUniqueString(raw.Raw.QualityWarnings, warn)
	}

	if len(raw.Raw.QualityWarnings) > 0 || raw.Raw.ProductPrice != nil {
		fullJSON = mergeRawQualityWarnings(fullJSON, raw.Raw.QualityWarnings, raw.Raw.ProductPrice, raw.Raw.PriceText)
	}

	return params, fullJSON
}

func looksLikePriceText(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if cnyPricePattern.MatchString(s) || usdPricePattern.MatchString(s) {
		return true
	}
	if _, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", ""), 64); err == nil {
		return true
	}
	return false
}

func parsePriceCurrency(text string) (float64, string) {
	curr := ""
	if cnyPricePattern.MatchString(text) {
		curr = "CNY"
	} else if usdPricePattern.MatchString(text) {
		curr = "USD"
	}
	cleaned := regexp.MustCompile(`[^\d.]`).ReplaceAllString(strings.ReplaceAll(text, ",", ""), " ")
	fields := strings.Fields(cleaned)
	for _, f := range fields {
		if p, err := strconv.ParseFloat(f, 64); err == nil && p > 0 {
			return p, curr
		}
	}
	return 0, curr
}

func appendUniqueString(list []string, s string) []string {
	for _, x := range list {
		if x == s {
			return list
		}
	}
	return append(list, s)
}

func mergeRawQualityWarnings(full json.RawMessage, warnings []string, price *float64, priceText string) json.RawMessage {
	var m map[string]any
	if err := json.Unmarshal(full, &m); err != nil {
		return full
	}
	rawObj, _ := m["raw"].(map[string]any)
	if rawObj == nil {
		rawObj = map[string]any{}
	}
	if len(warnings) > 0 {
		rawObj["qualityWarnings"] = warnings
	}
	if price != nil {
		rawObj["productPrice"] = *price
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
