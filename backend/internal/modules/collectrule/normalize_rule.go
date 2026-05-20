package collectrule

import (
	"encoding/json"
	"fmt"
	"strings"
)

var allowedRuleTopKeys = map[string]struct{}{
	"title": {}, "price": {}, "currency": {}, "mainImage": {}, "mainImages": {},
	"detailImages": {}, "descriptionImages": {}, "description": {},
	"attributes": {}, "skus": {}, "fallbacks": {},
}

// NormalizeRuleJSON converts v1 { selector, type } fields to internal { selectors, attr } shape for validation/storage.
func NormalizeRuleJSON(raw []byte) ([]byte, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("%w: empty rule", ErrRuleSchema)
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, ErrRuleInvalidJSON
	}
	out := make(map[string]json.RawMessage, len(root))
	for k, v := range root {
		key := strings.TrimSpace(k)
		if key == "mainImage" {
			key = "mainImages"
		}
		if key == "detailImages" {
			key = "descriptionImages"
		}
		if _, ok := allowedRuleTopKeys[k]; !ok && key != k {
			// renamed key still allowed
		}
		switch key {
		case "title", "price", "currency", "mainImages", "descriptionImages", "description":
			norm, err := normalizeFieldSpecJSON(v)
			if err != nil {
				return nil, err
			}
			out[key] = norm
		case "attributes", "skus", "fallbacks":
			out[key] = v
		default:
			if _, ok := allowedRuleTopKeys[key]; ok {
				out[key] = v
			}
		}
	}
	if _, ok := out["mainImages"]; !ok {
		if v, ok := root["mainImage"]; ok {
			norm, err := normalizeFieldSpecJSON(v)
			if err != nil {
				return nil, err
			}
			out["mainImages"] = norm
		}
	}
	if _, ok := out["descriptionImages"]; !ok {
		if v, ok := root["detailImages"]; ok {
			norm, err := normalizeFieldSpecJSON(v)
			if err != nil {
				return nil, err
			}
			out["descriptionImages"] = norm
		}
	}
	if _, ok := out["descriptionImages"]; !ok {
		if v, ok := root["description"]; ok {
			norm, err := normalizeFieldSpecJSON(v)
			if err != nil {
				return nil, err
			}
			out["descriptionImages"] = norm
		}
	}
	return json.Marshal(out)
}

func normalizeFieldSpecJSON(raw json.RawMessage) (json.RawMessage, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return raw, nil
	}
	if _, hasSel := m["selectors"]; hasSel {
		return raw, nil
	}
	selRaw, ok := m["selector"]
	if !ok {
		return raw, nil
	}
	var selector string
	if err := json.Unmarshal(selRaw, &selector); err != nil || strings.TrimSpace(selector) == "" {
		return raw, fmt.Errorf("%w: selector required", ErrRuleSchema)
	}
	typeStr := "text"
	if t, ok := m["type"]; ok {
		_ = json.Unmarshal(t, &typeStr)
	}
	typeStr = strings.TrimSpace(strings.ToLower(typeStr))
	attr := "text"
	multiple := false
	switch typeStr {
	case "text_all":
		attr, multiple = "text", true
	case "html":
		attr = "html"
	case "html_all":
		attr, multiple = "html", true
	case "attr", "attr_all":
		attr = "src"
		if a, ok := m["attr"]; ok {
			var s string
			_ = json.Unmarshal(a, &s)
			if strings.TrimSpace(s) != "" {
				attr = strings.TrimSpace(s)
			}
		}
		if typeStr == "attr_all" {
			multiple = true
		}
	}
	out := map[string]interface{}{
		"selectors": []string{strings.TrimSpace(selector)},
		"attr":      attr,
	}
	if multiple {
		out["multiple"] = true
	}
	if lim, ok := m["limit"]; ok {
		var n int
		if json.Unmarshal(lim, &n) == nil && n > 0 {
			out["limit"] = n
		}
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return b, nil
}
