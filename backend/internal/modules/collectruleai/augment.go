package collectruleai

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type digestCandidate struct {
	Selector   string  `json:"selector"`
	Sample     string  `json:"sample,omitempty"`
	Attr       string  `json:"attr,omitempty"`
	Count      int     `json:"count"`
	Confidence float64 `json:"confidence"`
}

type digestCandidateGroups struct {
	Title             []digestCandidate `json:"title"`
	Price             []digestCandidate `json:"price"`
	MainImages        []digestCandidate `json:"mainImages"`
	DescriptionImages []digestCandidate `json:"descriptionImages"`
	Attributes        []digestCandidate `json:"attributes"`
}

var broadTitleSelectors = map[string]struct{}{
	"h1": {}, "h2": {}, "title": {},
}

func parseDigestCandidates(raw json.RawMessage) digestCandidateGroups {
	var out digestCandidateGroups
	if len(raw) == 0 {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	return out
}

func pickSelectors(cands []digestCandidate, limit int, skipBroad bool) []string {
	if len(cands) == 0 {
		return nil
	}
	sorted := append([]digestCandidate(nil), cands...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Confidence > sorted[j].Confidence
	})
	out := make([]string, 0, limit)
	for _, c := range sorted {
		sel := strings.TrimSpace(c.Selector)
		if sel == "" {
			continue
		}
		low := strings.ToLower(sel)
		if skipBroad {
			if _, broad := broadTitleSelectors[low]; broad {
				continue
			}
		}
		dup := false
		for _, existing := range out {
			if existing == sel {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		out = append(out, sel)
		if len(out) >= limit {
			break
		}
	}
	if len(out) == 0 && skipBroad && len(sorted) > 0 {
		out = append(out, strings.TrimSpace(sorted[0].Selector))
	}
	return out
}

func defaultImageFilters() map[string]any {
	return map[string]any{
		"minWidth":         300,
		"minHeight":        300,
		"excludeKeywords":  []string{"icon", "logo", "sprite", "play", "arrow", "kefu", "service", "loading"},
		"dedupeByImageKey": true,
	}
}

func defaultFallbacks() map[string]bool {
	return map[string]bool{"meta": true, "jsonLd": true, "openGraph": true}
}

func augmentRuleFromDigest(
	rule json.RawMessage,
	digest *PageStructureDigest,
	targetFields []string,
) (json.RawMessage, []string, error) {
	if digest == nil || len(rule) == 0 {
		return rule, nil, nil
	}
	var root map[string]any
	if err := json.Unmarshal(rule, &root); err != nil {
		return rule, nil, err
	}
	groups := parseDigestCandidates(digest.Candidates)
	warnings := make([]string, 0)
	targetSet := map[string]struct{}{}
	for _, f := range targetFields {
		targetSet[strings.TrimSpace(f)] = struct{}{}
	}

	if _, ok := root["fallbacks"]; !ok {
		root["fallbacks"] = defaultFallbacks()
	}

	if _, hasTitle := root["title"]; !hasTitle {
		if _, want := targetSet["title"]; want || len(targetSet) == 0 {
			sels := pickSelectors(groups.Title, 4, true)
			if len(sels) == 0 && strings.TrimSpace(digest.Meta.OgTitle) != "" {
				sels = []string{`[property='og:title']`}
			}
			if len(sels) > 0 {
				root["title"] = map[string]any{"attr": "text", "selectors": sels}
				warnings = append(warnings, "AI 未生成标题规则，已从页面摘要补全："+strings.Join(sels, ", "))
			}
		}
	}

	if _, hasPrice := root["price"]; !hasPrice {
		if _, want := targetSet["price"]; want {
			sels := pickSelectors(groups.Price, 4, false)
			if len(sels) > 0 {
				root["price"] = map[string]any{"attr": "text", "selectors": sels}
				warnings = append(warnings, "AI 未生成价格规则，已从页面摘要补全。")
			}
		}
	}

	if _, hasMain := root["mainImages"]; !hasMain {
		if _, want := targetSet["mainImages"]; want {
			sels := pickSelectors(groups.MainImages, 5, false)
			if len(sels) == 0 && strings.TrimSpace(digest.Meta.OgImage) != "" {
				sels = []string{`[property='og:image']`}
			}
			if len(sels) > 0 {
				root["mainImages"] = map[string]any{
					"attr": "src", "multiple": true, "limit": 8,
					"selectors": sels,
					"attrs":     []string{"src", "data-src", "data-lazy-img", "data-origin", "data-original"},
					"filters":   defaultImageFilters(),
				}
				warnings = append(warnings, "AI 未生成主图规则，已从页面摘要补全。")
			}
		}
	}

	if _, hasDesc := root["descriptionImages"]; !hasDesc {
		if _, want := targetSet["descriptionImages"]; want {
			sels := pickSelectors(groups.DescriptionImages, 4, false)
			if len(sels) > 0 {
				root["descriptionImages"] = map[string]any{
					"attr": "src", "multiple": true, "limit": 30,
					"selectors": sels,
					"attrs":     []string{"src", "data-src", "data-lazy-img", "data-original"},
					"filters":   defaultImageFilters(),
				}
				warnings = append(warnings, "AI 未生成详情图规则，已从页面摘要补全。")
			}
		}
	}

	if _, hasAttr := root["attributes"]; !hasAttr {
		if _, want := targetSet["attributes"]; want && len(groups.Attributes) > 0 {
			best := groups.Attributes[0]
			rowSel := strings.TrimSpace(best.Selector)
			keySel := "dt, .name"
			valSel := "dd, .value"
			if i := strings.Index(rowSel, "("); i > 0 && strings.Contains(rowSel, "/") {
				rowSel = strings.TrimSpace(rowSel[:i])
			}
			root["attributes"] = map[string]any{
				"mode": "pairs", "rowSelector": rowSel,
				"keySelector": keySel, "valueSelector": valSel,
			}
			warnings = append(warnings, fmt.Sprintf("AI 未生成参数规则，已从页面摘要补全（%s）。", rowSel))
		}
	}

	out, err := json.Marshal(root)
	if err != nil {
		return rule, warnings, err
	}
	return out, warnings, nil
}
