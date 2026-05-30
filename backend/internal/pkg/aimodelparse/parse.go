package aimodelparse

import (
	"encoding/json"
	"fmt"
	"strings"
)

// NormalizeJSONContent strips common model wrappers and returns the outermost JSON object substring.
func NormalizeJSONContent(s string) string {
	s = strings.TrimSpace(s)
	s = stripThinkBlocks(s)
	s = stripCodeFences(s)
	if extracted := extractJSONObject(s); extracted != "" {
		return extracted
	}
	return s
}

func stripThinkBlocks(s string) string {
	pairs := [][2]string{
		{"<" + "think" + ">", "</" + "think" + ">"},
		{"<thinking>", "</thinking>"},
		{"<reasoning>", "</reasoning>"},
		{"<think>", "</think>"},
	}
	for _, p := range pairs {
		s = stripTaggedBlock(s, p[0], p[1])
	}
	return strings.TrimSpace(s)
}

func stripTaggedBlock(s, open, close string) string {
	for {
		start := strings.Index(s, open)
		if start < 0 {
			break
		}
		rest := s[start+len(open):]
		endRel := strings.Index(rest, close)
		if endRel < 0 {
			s = s[:start] + rest
			continue
		}
		s = s[:start] + rest[endRel+len(close):]
	}
	return s
}

func stripCodeFences(s string) string {
	s = strings.TrimSpace(s)
	idx := strings.Index(s, "```")
	if idx < 0 {
		return s
	}
	rest := s[idx+3:]
	rest = strings.TrimLeft(rest, "\n")
	if strings.HasPrefix(strings.ToLower(rest), "json") {
		rest = rest[4:]
		rest = strings.TrimLeft(rest, "\n")
	}
	if end := strings.LastIndex(rest, "```"); end >= 0 {
		return strings.TrimSpace(rest[:end])
	}
	return strings.TrimSpace(rest)
}

func extractJSONObject(s string) string {
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return strings.TrimSpace(s)
	}
	depth := 0
	inStr := false
	esc := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inStr {
			if esc {
				esc = false
				continue
			}
			if c == '\\' {
				esc = true
				continue
			}
			if c == '"' {
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return strings.TrimSpace(s[start:])
}

func stringFromAny(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case json.Number:
		return t.String()
	case float64:
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%g", t)
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(b))
	}
}

func stringsFromAny(v any) []string {
	switch t := v.(type) {
	case nil:
		return nil
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return nil
		}
		for _, sep := range []string{",", "，", ";", "；", "|", "\n"} {
			if strings.Contains(s, sep) {
				parts := strings.Split(s, sep)
				out := make([]string, 0, len(parts))
				for _, p := range parts {
					p = strings.TrimSpace(p)
					if p != "" {
						out = append(out, p)
					}
				}
				if len(out) > 0 {
					return out
				}
			}
		}
		return []string{s}
	case []string:
		out := make([]string, 0, len(t))
		for _, s := range t {
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			s := stringFromAny(item)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		s := stringFromAny(v)
		if s == "" {
			return nil
		}
		return []string{s}
	}
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s := stringFromAny(v); s != "" {
				return s
			}
		}
	}
	return ""
}

func firstStrings(m map[string]any, keys ...string) []string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if ss := stringsFromAny(v); len(ss) > 0 {
				return ss
			}
		}
	}
	return nil
}

// TitleOptimize holds normalized product_title_optimize model output.
type TitleOptimize struct {
	OptimizedTitle string
	Keywords       []string
	Reason         string
}

// ParseTitleOptimize parses model JSON for product_title_optimize (camelCase, snake_case, or common aliases).
func ParseTitleOptimize(content string) (TitleOptimize, error) {
	content = NormalizeJSONContent(content)
	if content == "" {
		return TitleOptimize{}, fmt.Errorf("empty content")
	}

	var direct struct {
		OptimizedTitle string   `json:"optimizedTitle"`
		Keywords       []string `json:"keywords"`
		Reason         string   `json:"reason"`
	}
	if err := json.Unmarshal([]byte(content), &direct); err == nil {
		out := TitleOptimize{
			OptimizedTitle: strings.TrimSpace(direct.OptimizedTitle),
			Keywords:       direct.Keywords,
			Reason:         strings.TrimSpace(direct.Reason),
		}
		for i := range out.Keywords {
			out.Keywords[i] = strings.TrimSpace(out.Keywords[i])
		}
		if out.OptimizedTitle != "" {
			return out, nil
		}
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(content), &m); err != nil {
		return TitleOptimize{}, err
	}
	if nested, ok := m["data"].(map[string]any); ok {
		m = nested
	} else if nested, ok := m["result"].(map[string]any); ok {
		m = nested
	}

	out := TitleOptimize{
		OptimizedTitle: firstString(m,
			"optimizedTitle", "optimized_title", "title", "newTitle", "new_title",
			"optimized", "productTitle", "product_title", "优化标题", "标题"),
		Keywords: firstStrings(m, "keywords", "keyword", "tags", "key_words", "关键词"),
		Reason:   firstString(m, "reason", "explanation", "summary", "note", "notes", "原因", "说明"),
	}
	if out.OptimizedTitle == "" {
		return TitleOptimize{}, fmt.Errorf("missing optimizedTitle")
	}
	return out, nil
}
