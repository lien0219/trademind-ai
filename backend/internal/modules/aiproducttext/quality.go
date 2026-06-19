package aiproducttext

import (
	"strings"
	"unicode/utf8"
)

type QualityWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func hasRepeatWords(title string) bool {
	words := strings.Fields(strings.ToLower(title))
	if len(words) < 3 {
		return false
	}
	for i := 0; i+2 < len(words); i++ {
		if words[i] == words[i+1] && words[i] == words[i+2] && len(words[i]) >= 2 {
			return true
		}
	}
	return false
}

func checkTitleQuality(title string, opts TextGenerationOptions, forbidden []string) []QualityWarning {
	var out []QualityWarning
	t := strings.TrimSpace(title)
	if t == "" {
		out = append(out, QualityWarning{Code: "title_empty", Message: "标题为空，建议重新生成后再应用。"})
		return out
	}
	n := utf8.RuneCountInString(t)
	if n < 8 {
		out = append(out, QualityWarning{Code: "title_too_short", Message: "标题可能过短，建议补充核心卖点。"})
	}
	maxLen := opts.MaxLength
	if maxLen <= 0 {
		maxLen = 120
	}
	if n > maxLen {
		out = append(out, QualityWarning{Code: "title_too_long", Message: "标题可能过长，建议缩短后再应用。"})
	}
	if hasRepeatWords(t) {
		out = append(out, QualityWarning{Code: "title_repeat_words", Message: "标题重复词过多，建议精简表达。"})
	}
	for _, w := range forbidden {
		w = strings.TrimSpace(w)
		if w != "" && strings.Contains(strings.ToLower(t), strings.ToLower(w)) {
			out = append(out, QualityWarning{Code: "title_forbidden_word", Message: "标题含禁用词「" + w + "」，请修改后再应用。"})
		}
	}
	if strings.Contains(t, "1688") || strings.Contains(t, "淘宝") || strings.Contains(t, "厂家直销") {
		out = append(out, QualityWarning{Code: "title_collect_noise", Message: "标题可能包含采集噪声，建议人工润色。"})
	}
	return out
}

func checkDescriptionQuality(desc, productTitle string, forbidden []string) []QualityWarning {
	var out []QualityWarning
	d := strings.TrimSpace(desc)
	if d == "" {
		out = append(out, QualityWarning{Code: "desc_empty", Message: "描述为空，建议重新生成后再应用。"})
		return out
	}
	n := utf8.RuneCountInString(d)
	if n < 40 {
		out = append(out, QualityWarning{Code: "desc_too_short", Message: "描述可能过短，建议补充使用场景与卖点。"})
	}
	if !strings.Contains(d, "•") && !strings.Contains(d, "-") && !strings.Contains(d, "：") && n < 120 {
		out = append(out, QualityWarning{Code: "desc_no_bullets", Message: "描述缺少卖点列表，建议补充核心卖点。"})
	}
	if n >= 80 && !strings.Contains(d, "\n") && !strings.Contains(d, "<") && !strings.Contains(d, "•") {
		out = append(out, QualityWarning{Code: "desc_unclear_structure", Message: "描述结构不够清晰，建议分段或使用列表呈现卖点。"})
	}
	if !strings.Contains(strings.ToLower(d), "规格") && !strings.Contains(strings.ToLower(d), "参数") && n < 200 {
		out = append(out, QualityWarning{Code: "desc_no_specs", Message: "描述缺少规格信息，建议保留原参数。"})
	}
	for _, w := range forbidden {
		w = strings.TrimSpace(w)
		if w != "" && strings.Contains(strings.ToLower(d), strings.ToLower(w)) {
			out = append(out, QualityWarning{Code: "desc_forbidden_word", Message: "描述含禁用词「" + w + "」，请修改后再应用。"})
		}
	}
	pt := strings.TrimSpace(productTitle)
	if pt != "" && !strings.Contains(d, pt) && utf8.RuneCountInString(pt) >= 4 {
		// loose match: at least partial overlap
		words := strings.Fields(pt)
		matched := 0
		for _, w := range words {
			if len([]rune(w)) >= 2 && strings.Contains(d, w) {
				matched++
			}
		}
		if matched == 0 {
			out = append(out, QualityWarning{Code: "desc_title_mismatch", Message: "描述与商品标题关联较弱，建议核对后再应用。"})
		}
	}
	return out
}
