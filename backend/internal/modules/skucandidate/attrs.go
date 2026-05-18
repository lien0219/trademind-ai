package skucandidate

import (
	"encoding/json"
	"strconv"
	"strings"
	"unicode"

	"gorm.io/datatypes"
)

type attrSignals struct {
	color string
	size  string
	model string
	extra map[string]string
}

func slugKey(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
		} else if r == ' ' || r == '_' || r == '-' {
			b.WriteRune('_')
		}
	}
	return strings.Trim(b.String(), "_")
}

func parseAttrsJSON(j datatypes.JSON) map[string]string {
	if len(j) == 0 {
		return nil
	}
	var raw map[string]any
	if err := json.Unmarshal(j, &raw); err != nil {
		return nil
	}
	out := map[string]string{}
	for k, v := range raw {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		switch t := v.(type) {
		case string:
			s := strings.TrimSpace(t)
			if s != "" {
				out[k] = s
			}
		case float64:
			out[k] = strings.TrimSpace(strconv.FormatFloat(t, 'f', -1, 64))
		case json.Number:
			out[k] = strings.TrimSpace(t.String())
		case bool:
			out[k] = strconv.FormatBool(t)
		default:
			if t == nil {
				continue
			}
			bs, err := json.Marshal(t)
			if err == nil && len(bs) < 220 {
				out[k] = strings.TrimSpace(string(bs))
			}
		}
	}
	return out
}

func classifyAttrKeys(m map[string]string) attrSignals {
	as := attrSignals{extra: map[string]string{}}
	if m == nil {
		return as
	}
	for k0, val := range m {
		key := slugKey(strings.ReplaceAll(strings.ReplaceAll(k0, "__", "_"), "/", "_"))
		v := slugKey(val)
		if v == "" {
			continue
		}
		lk := strings.ToLower(key)
		switch {
		case strings.Contains(lk, "color") || strings.Contains(lk, "颜色") || lk == "colour" || lk == "shade":
			if as.color == "" {
				as.color = v
			}
		case strings.Contains(lk, "size") || strings.Contains(lk, "尺码") || strings.Contains(lk, "尺寸"):
			if as.size == "" {
				as.size = v
			}
		case strings.Contains(lk, "model") || strings.Contains(lk, "型号") || strings.Contains(lk, "style") || strings.Contains(lk, "规格"):
			if as.model == "" {
				as.model = v
			}
		default:
			if _, ok := as.extra[key]; !ok && len(as.extra) < 12 {
				as.extra[key] = v
			}
		}
	}
	return as
}

func extractSignalsFromSKUName(name string) attrSignals {
	return extractSignalsBlob(name)
}

func extractSignalsBlob(blob string) attrSignals {
	as := attrSignals{extra: map[string]string{}}
	blob = strings.TrimSpace(blob)
	if blob == "" {
		return as
	}

	extractKV := func(full, lower string, prefixes ...string) string {
		for _, pfx := range prefixes {
			px := strings.ToLower(pfx)
			if !strings.Contains(lower, px) {
				continue
			}
			idx := strings.Index(lower, px)
			if idx < 0 {
				continue
			}
			rest := strings.TrimSpace(full[idx+len(pfx):])
			rest = strings.Trim(rest, `:： `)
			rest = strings.TrimSpace(strings.Split(rest, ",")[0])
			rest = strings.TrimSpace(strings.Split(rest, "|")[0])
			rest = strings.TrimSpace(strings.Split(rest, ";")[0])
			rest = strings.TrimSpace(strings.Split(rest, "；")[0])
			if rest != "" {
				return slugKey(rest)
			}
		}
		return ""
	}

	line := blob
	seps := []string{" / ", "/", "|"}
	for _, sp := range seps {
		line = strings.ReplaceAll(line, sp, ";")
	}
	lowBlob := strings.ToLower(line)
	as.color = firstNonEmpty(as.color,
		firstNonEmpty(
			extractKV(line, lowBlob, "颜色", "色号", "color", "colour"),
			"",
		),
	)
	as.size = firstNonEmpty(as.size, extractKV(line, lowBlob, "尺码", "尺寸", "size"))
	as.model = firstNonEmpty(as.model, extractKV(line, lowBlob, "型号", "model", "规格"))

	legacy := classifyAttrKeys(parseAttrsLikeFromText(blob))
	as.color = firstNonEmpty(as.color, legacy.color)
	as.size = firstNonEmpty(as.size, legacy.size)
	as.model = firstNonEmpty(as.model, legacy.model)
	for k, v := range legacy.extra {
		if _, ok := as.extra[k]; !ok {
			as.extra[k] = v
		}
	}
	return as
}

func parseAttrsLikeFromText(s string) map[string]string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "{") {
		var m map[string]any
		if err := json.Unmarshal([]byte(s), &m); err == nil {
			out := map[string]string{}
			for k, v := range m {
				if vv, ok := v.(string); ok {
					out[k] = vv
				}
			}
			return out
		}
	}
	return map[string]string{}
}

func mergeAttrMaps(a, b attrSignals) attrSignals {
	out := a
	if out.extra == nil {
		out.extra = map[string]string{}
	}
	if b.color != "" && out.color == "" {
		out.color = b.color
	}
	if b.size != "" && out.size == "" {
		out.size = b.size
	}
	if b.model != "" && out.model == "" {
		out.model = b.model
	}
	for k, v := range b.extra {
		if _, ok := out.extra[k]; !ok {
			out.extra[k] = v
		}
	}
	return out
}

func mergeAttrSignalsForOrderLine(skuName string, attrs datatypes.JSON, rawHint string) attrSignals {
	m := classifyAttrKeys(parseAttrsJSON(attrs))
	a := classifyAttrKeys(parseAttrsLikeFromText(rawHint))
	ns := extractSignalsFromSKUName(skuName)
	return mergeAttrMaps(mergeAttrMaps(m, a), ns)
}

func attrsSimilaritySignals(orderSignals, skuSignals attrSignals) (score int, sigs []string) {
	base := 0
	if orderSignals.color != "" && skuSignals.color != "" {
		if orderSignals.color == skuSignals.color || strings.Contains(skuSignals.color, orderSignals.color) || strings.Contains(orderSignals.color, skuSignals.color) {
			base += 28
			sigs = append(sigs, "attrs_color_equal")
		}
	}
	if orderSignals.size != "" && skuSignals.size != "" {
		if orderSignals.size == skuSignals.size {
			base += 28
			sigs = append(sigs, "attrs_size_equal")
		}
	}
	if orderSignals.model != "" && skuSignals.model != "" {
		if orderSignals.model == skuSignals.model || strings.Contains(skuSignals.model, orderSignals.model) {
			base += 24
			sigs = append(sigs, "attrs_model_equal")
		}
	}
	base += jsonKeyOverlapBonus(orderSignals, skuSignals)
	if base == 0 {
		return 0, nil
	}
	if base < 40 {
		base = 40
	}
	if base > 70 {
		base = 70
	}
	return base, sigs
}

func jsonKeyOverlapBonus(orderSignals, skuSignals attrSignals) int {
	if len(orderSignals.extra) == 0 || len(skuSignals.extra) == 0 {
		return 0
	}
	score := 0
	for k, ov := range orderSignals.extra {
		for sk, sv := range skuSignals.extra {
			if slugKey(k) != slugKey(sk) {
				continue
			}
			if ov != "" && ov == sv {
				score += 6
			}
			if score >= 24 {
				return 24
			}
		}
	}
	return score
}

func firstNonEmpty(a, b string) string {
	a = strings.TrimSpace(a)
	if a != "" {
		return a
	}
	return strings.TrimSpace(b)
}
