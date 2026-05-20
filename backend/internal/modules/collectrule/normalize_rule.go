package collectrule

import (
	"encoding/json"
	"fmt"
	"strings"
)

func normalizeFieldSpecJSON(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, fmt.Errorf("%w: empty field spec", ErrRuleSchema)
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		s := strings.TrimSpace(asString)
		if s == "" {
			return nil, fmt.Errorf("%w: empty field spec", ErrRuleSchema)
		}
		return json.Marshal(map[string]any{
			"selectors": []string{s},
			"attr":      "text",
		})
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("%w: invalid field spec", ErrRuleSchema)
	}

	selectors, err := parseSelectorList(m["selectors"])
	if err != nil {
		return nil, err
	}
	if len(selectors) == 0 {
		if selRaw, ok := m["selector"]; ok {
			var selector string
			if json.Unmarshal(selRaw, &selector) == nil {
				selectors = trimNonEmptySelectors([]string{selector})
			}
		}
	}
	if len(selectors) == 0 {
		return nil, fmt.Errorf("%w: selectors required", ErrRuleSchema)
	}

	attr := "text"
	if a, ok := m["attr"]; ok {
		var s string
		if json.Unmarshal(a, &s) == nil && strings.TrimSpace(s) != "" {
			attr = strings.TrimSpace(s)
		}
	} else if t, ok := m["type"]; ok {
		attr, _ = attrFromLegacyType(t, m["attr"])
	}

	out := map[string]any{
		"selectors": selectors,
		"attr":      attr,
	}

	if v, ok := m["multiple"]; ok {
		var b bool
		if json.Unmarshal(v, &b) == nil && b {
			out["multiple"] = true
		}
	} else if t, ok := m["type"]; ok {
		_, multiple := attrFromLegacyType(t, m["attr"])
		if multiple {
			out["multiple"] = true
		}
	}

	for _, key := range []string{"limit", "fallback", "filters", "attrs", "scrollIntoView"} {
		if v, ok := m[key]; ok && len(v) > 0 && string(v) != "null" {
			var hold any
			if json.Unmarshal(v, &hold) == nil {
				out[key] = hold
			}
		}
	}

	return json.Marshal(out)
}

func parseSelectorList(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return trimNonEmptySelectors(arr), nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return trimNonEmptySelectors([]string{s}), nil
	}
	return nil, fmt.Errorf("%w: selectors must be string or array", ErrRuleSchema)
}

func trimNonEmptySelectors(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func attrFromLegacyType(typeRaw, attrRaw json.RawMessage) (string, bool) {
	typeStr := "text"
	if typeRaw != nil {
		_ = json.Unmarshal(typeRaw, &typeStr)
	}
	typeStr = strings.TrimSpace(strings.ToLower(typeStr))
	switch typeStr {
	case "text_all":
		return "text", true
	case "html":
		return "html", false
	case "html_all":
		return "html", true
	case "attr", "attr_all":
		attr := "src"
		if attrRaw != nil {
			var s string
			if json.Unmarshal(attrRaw, &s) == nil && strings.TrimSpace(s) != "" {
				attr = strings.TrimSpace(s)
			}
		}
		return attr, typeStr == "attr_all"
	default:
		return "text", false
	}
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

var allowedRuleTopKeys = map[string]struct{}{
	"title": {}, "price": {}, "currency": {}, "mainImage": {}, "mainImages": {},
	"detailImages": {}, "descriptionImages": {}, "description": {},
	"attributes": {}, "skus": {}, "fallbacks": {},
}
