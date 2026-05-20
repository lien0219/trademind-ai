package collectruleai

import (
	"encoding/json"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/modules/collectrule"
)

const qualityEnableThreshold = 60

var broadSelectorPatterns = []string{
	"^h1$",
	"^img$",
	"^div$",
	"^span$",
	"^a$",
	`\[class\*="title"\]`,
	`\[class\*="price"\]`,
}

var coreEnableRuleKeys = []string{"title", "mainImages"}

type QualityGateDTO struct {
	Score            int            `json:"score"`
	AllowSaveEnabled bool           `json:"allowSaveEnabled"`
	AllowSaveDraft   bool           `json:"allowSaveDraft"`
	BlockReasons     []string       `json:"blockReasons,omitempty"`
	Suggestions      []string       `json:"suggestions,omitempty"`
	FieldHits        []FieldHitDTO  `json:"fieldHits,omitempty"`
	ScoreBreakdown   map[string]int `json:"scoreBreakdown,omitempty"`
}

type FieldHitDTO struct {
	Field     string `json:"field"`
	Label     string `json:"label"`
	InRule    bool   `json:"inRule"`
	Extracted bool   `json:"extracted"`
	Points    int    `json:"points"`
	MaxPoints int    `json:"maxPoints"`
}

func ruleTopKeys(rule json.RawMessage) map[string]struct{} {
	out := map[string]struct{}{}
	if len(rule) == 0 {
		return out
	}
	var root map[string]json.RawMessage
	if json.Unmarshal(rule, &root) != nil {
		return out
	}
	for k := range root {
		out[strings.TrimSpace(k)] = struct{}{}
	}
	return out
}

func missingGeneratedFields(rule json.RawMessage, targetFields []string) []string {
	keys := ruleTopKeys(rule)
	out := make([]string, 0)
	for _, f := range targetFields {
		f = strings.TrimSpace(f)
		if f == "" || f == "skus" {
			continue
		}
		if _, ok := keys[f]; !ok {
			out = append(out, f)
		}
	}
	return out
}

func isTitleOnlyRule(rule json.RawMessage, targetFields []string) bool {
	keys := ruleTopKeys(rule)
	dataKeys := 0
	for k := range keys {
		if k == "fallbacks" {
			continue
		}
		dataKeys++
	}
	if dataKeys <= 1 {
		if _, ok := keys["title"]; ok {
			return true
		}
	}
	missing := missingGeneratedFields(rule, targetFields)
	if len(missing) == 0 {
		return false
	}
	// User selected multiple fields but rule only has title (+ optional fallbacks).
	if _, ok := keys["title"]; ok && dataKeys <= 2 && len(missing) >= 2 {
		return true
	}
	return false
}

func titleUsesBroadSelector(rule json.RawMessage) (bool, string) {
	if len(rule) == 0 {
		return false, ""
	}
	var root map[string]json.RawMessage
	if json.Unmarshal(rule, &root) != nil {
		return false, ""
	}
	rawTitle, ok := root["title"]
	if !ok {
		return false, ""
	}
	var spec struct {
		Selectors []string `json:"selectors"`
	}
	if json.Unmarshal(rawTitle, &spec) != nil || len(spec.Selectors) == 0 {
		return false, ""
	}
	first := strings.TrimSpace(strings.ToLower(spec.Selectors[0]))
	for _, p := range broadSelectorPatterns {
		if matched, _ := matchSimplePattern(first, p); matched {
			return true, spec.Selectors[0]
		}
	}
	if first == "h1" {
		return true, spec.Selectors[0]
	}
	return false, ""
}

func matchSimplePattern(s, pattern string) (bool, error) {
	// lightweight check without importing regexp in hot path for known patterns
	switch pattern {
	case "^h1$":
		return s == "h1", nil
	case "^img$":
		return s == "img", nil
	case "^div$":
		return s == "div", nil
	case "^span$":
		return s == "span", nil
	case "^a$":
		return s == "a", nil
	case `[class*="title"]`:
		return strings.Contains(s, `[class*="title"]`) || strings.Contains(s, `[class*='title']`), nil
	case `[class*="price"]`:
		return strings.Contains(s, `[class*="price"]`) || strings.Contains(s, `[class*='price']`), nil
	}
	return false, nil
}

func mergeFieldCoverageWarnings(warnings []string, rule json.RawMessage, targetFields []string, aiMissing []string) []string {
	missing := dedupeStrings(append(append([]string(nil), aiMissing...), missingGeneratedFields(rule, targetFields)...))

	if isTitleOnlyRule(rule, targetFields) {
		warnings = append(warnings, "AI 只识别到了商品标题，未能找到价格、图片和参数的位置。建议重新生成，或先确认该页面是否需要登录 / 是否为动态页面。")
	}
	for _, m := range missing {
		switch m {
		case "price":
			warnings = append(warnings, "未能生成价格规则，价格可能由网站动态加载。")
		case "mainImages":
			warnings = append(warnings, "未能生成主图规则，请手动补充或重新生成。")
		case "descriptionImages":
			warnings = append(warnings, "未能生成详情图规则，详情区可能需要滚动加载。")
		case "attributes":
			warnings = append(warnings, "未能生成商品参数规则，可尝试 pairs 模式或手动补充。")
		}
	}
	if broad, sel := titleUsesBroadSelector(rule); broad {
		warnings = append(warnings, "当前标题位置过于宽泛（"+sel+"），可能会抓到非商品标题，建议重新生成或手动调整。")
	}
	return dedupeStrings(warnings)
}

func dedupeStrings(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func computeQualityGate(
	rule json.RawMessage,
	targetFields []string,
	testResult *collectrule.RuleTestResultDTO,
) QualityGateDTO {
	gate := QualityGateDTO{
		Score:            0,
		AllowSaveEnabled: false,
		AllowSaveDraft:   true,
	}

	if testResult != nil && testResult.QualityScore != nil {
		if sc, ok := testResult.QualityScore["score"].(float64); ok {
			gate.Score = int(sc)
		}
	}

	keys := ruleTopKeys(rule)
	block := make([]string, 0)
	suggest := make([]string, 0)

	if isTitleOnlyRule(rule, targetFields) {
		if gate.Score > 30 {
			gate.Score = 30
		}
		block = append(block, "规则仅包含标题，无法用于正式采集")
		suggest = append(suggest, "重新生成", "手动补充价格、主图与参数规则")
	}

	if broad, sel := titleUsesBroadSelector(rule); broad {
		gate.Score -= 15
		if gate.Score < 0 {
			gate.Score = 0
		}
		suggest = append(suggest, "调整标题 selector（当前："+sel+"）")
	}

	for _, k := range coreEnableRuleKeys {
		if _, ok := keys[k]; !ok {
			block = append(block, "规则缺少必要字段："+k)
		}
	}

	if testResult == nil {
		block = append(block, "规则自动测试未通过")
	} else {
		ef := testResult.ExtractedFields
		if ef != nil {
			if title, ok := ef["title"].(bool); !ok || !title {
				block = append(block, "测试未识别商品标题")
			}
			if suspect, ok := ef["titleSuspectWrong"].(bool); ok && suspect {
				block = append(block, "测试标题疑似非商品标题")
				suggest = append(suggest, "重新生成或手动调整标题 selector")
			}
			if main, ok := ef["mainImage"].(bool); !ok || !main {
				block = append(block, "测试未识别商品主图")
			}
		}
	}

	if gate.Score < qualityEnableThreshold {
		block = append(block, "识别效果评分低于 60，不建议直接启用")
		suggest = append(suggest, "重新生成", "手动调整规则后再次测试")
	}

	gate.BlockReasons = dedupeStrings(block)
	gate.Suggestions = dedupeStrings(suggest)

	gate.AllowSaveEnabled = len(gate.BlockReasons) == 0 && gate.Score >= qualityEnableThreshold

	missing := missingGeneratedFields(rule, targetFields)
	hasCoreMissing := false
	for _, m := range missing {
		if m == "title" || m == "mainImages" {
			hasCoreMissing = true
			break
		}
	}
	if hasCoreMissing {
		gate.AllowSaveEnabled = false
	}

	if !gate.AllowSaveEnabled && len(gate.Suggestions) == 0 {
		gate.Suggestions = []string{"重新生成", "保存为草稿后手动调整", "再次测试确认效果"}
	}

	gate.FieldHits, gate.ScoreBreakdown = buildFieldHitsAndBreakdown(rule, targetFields, testResult, gate.Score)

	return gate
}

var fieldHitLabels = map[string]string{
	"title": "商品标题", "price": "商品价格", "mainImages": "商品主图",
	"descriptionImages": "详情图片", "attributes": "商品参数",
}

func buildFieldHitsAndBreakdown(
	rule json.RawMessage,
	targetFields []string,
	testResult *collectrule.RuleTestResultDTO,
	totalScore int,
) ([]FieldHitDTO, map[string]int) {
	keys := ruleTopKeys(rule)
	targetSet := map[string]struct{}{}
	for _, f := range targetFields {
		targetSet[strings.TrimSpace(f)] = struct{}{}
	}
	ef := map[string]interface{}{}
	if testResult != nil && testResult.ExtractedFields != nil {
		ef = testResult.ExtractedFields
	}
	qs := map[string]interface{}{}
	if testResult != nil && testResult.QualityScore != nil {
		qs = testResult.QualityScore
	}

	checks := []struct {
		field, qsKey string
		efKey        string
		maxPts       int
	}{
		{"title", "titleOk", "title", 40},
		{"price", "priceOk", "price", 15},
		{"mainImages", "mainImagesOk", "mainImage", 15},
		{"descriptionImages", "descriptionImagesOk", "detailImagesCount", 10},
		{"attributes", "attributesOk", "attributesCount", 10},
	}
	breakdown := map[string]int{"total": totalScore}
	hits := make([]FieldHitDTO, 0, len(checks))
	for _, c := range checks {
		if len(targetSet) > 0 {
			if _, want := targetSet[c.field]; !want {
				continue
			}
		}
		_, inRule := keys[c.field]
		extracted := false
		switch c.efKey {
		case "title", "price", "mainImage":
			if v, ok := ef[c.efKey].(bool); ok {
				extracted = v
			}
		default:
			if v, ok := ef[c.efKey].(float64); ok {
				extracted = v > 0
			}
		}
		points := 0
		if v, ok := qs[c.qsKey].(bool); ok && v {
			points = c.maxPts
		} else if inRule && !extracted {
			points = c.maxPts / 4
		}
		breakdown[c.field] = points
		hits = append(hits, FieldHitDTO{
			Field: c.field, Label: fieldHitLabels[c.field],
			InRule: inRule, Extracted: extracted,
			Points: points, MaxPoints: c.maxPts,
		})
	}
	return hits, breakdown
}
