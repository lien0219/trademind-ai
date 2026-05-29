package imagetask

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/pkg/aimodelparse"
)

func parseOCRJSON(content string) (*translateOCRResult, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil, fmt.Errorf("empty ocr content")
	}
	if arr := extractOCRJSONArray(trimmed); arr != "" {
		var blocks []translateTextBlock
		if err := json.Unmarshal([]byte(arr), &blocks); err == nil {
			return finalizeOCRResult(&translateOCRResult{Blocks: blocks}), nil
		}
	}

	normalized := aimodelparse.NormalizeJSONContent(trimmed)
	if normalized == "" {
		return nil, fmt.Errorf("empty ocr content")
	}

	var ocr translateOCRResult
	if err := json.Unmarshal([]byte(normalized), &ocr); err == nil && len(ocr.Blocks) > 0 {
		return finalizeOCRResult(&ocr), nil
	}

	flex, err := parseOCRFlexible(normalized)
	if err == nil {
		return finalizeOCRResult(flex), nil
	}

	if err := json.Unmarshal([]byte(normalized), &ocr); err == nil {
		return finalizeOCRResult(&ocr), nil
	}
	return nil, err
}

func extractOCRJSONArray(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, "```"); idx >= 0 {
		s = aimodelparse.NormalizeJSONContent(s)
	}
	if !strings.HasPrefix(s, "[") {
		return ""
	}
	depth := 0
	inStr := false
	esc := false
	for i := 0; i < len(s); i++ {
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
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return s[:i+1]
			}
		}
	}
	return ""
}

func finalizeOCRResult(ocr *translateOCRResult) *translateOCRResult {
	if ocr == nil {
		return nil
	}
	if ocr.TextBlocksCount <= 0 {
		ocr.TextBlocksCount = len(ocr.Blocks)
	}
	ocr.Blocks = normalizeOCRBlocks(ocr.Blocks)
	ocr.TextBlocksCount = len(ocr.Blocks)
	return ocr
}

func parseOCRFlexible(content string) (*translateOCRResult, error) {
	var root map[string]any
	if err := json.Unmarshal([]byte(content), &root); err != nil {
		return nil, fmt.Errorf("ocr json decode: %w", err)
	}
	if nested, ok := root["data"].(map[string]any); ok {
		root = nested
	} else if nested, ok := root["result"].(map[string]any); ok {
		root = nested
	}

	out := &translateOCRResult{
		DetectedLanguage: firstOCRString(root, "detectedLanguage", "detected_language", "language", "sourceLanguage"),
	}
	rawBlocks := firstOCRAny(root, "blocks", "textBlocks", "text_blocks", "items", "regions")
	switch list := rawBlocks.(type) {
	case []any:
		for _, item := range list {
			if b := parseOCRBlockItem(item); b != nil {
				out.Blocks = append(out.Blocks, *b)
			}
		}
	case map[string]any:
		if b := parseOCRBlockItem(list); b != nil {
			out.Blocks = append(out.Blocks, *b)
		}
	}
	out.TextBlocksCount = len(out.Blocks)
	return out, nil
}

func parseOCRBlockItem(v any) *translateTextBlock {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	text := firstOCRString(m, "text", "original", "originalText", "original_text", "sourceText", "source_text", "content", "source")
	if text == "" {
		return nil
	}
	tr := firstOCRString(m, "translatedText", "translated_text", "translation", "targetText", "target_text", "target")
	conf := firstOCRFloat(m, "confidence", "score")
	bbox := parseOCRBBox(firstOCRAny(m, "bbox", "boundingBox", "bounding_box", "box", "rect", "region"))
	return &translateTextBlock{
		Text:           text,
		TranslatedText: tr,
		Confidence:     conf,
		BBox:           bbox,
	}
}

func parseOCRBBox(v any) translateTextBBox {
	m, ok := v.(map[string]any)
	if !ok {
		return translateTextBBox{}
	}
	if w := ocrInt(m, "width", "w"); w > 0 {
		return translateTextBBox{
			X:      ocrInt(m, "x", "left"),
			Y:      ocrInt(m, "y", "top"),
			Width:  w,
			Height: ocrInt(m, "height", "h"),
		}
	}
	x1 := ocrFloat(m, "x1", "left")
	y1 := ocrFloat(m, "y1", "top")
	x2 := ocrFloat(m, "x2", "right")
	y2 := ocrFloat(m, "y2", "bottom")
	if x2 > x1 && y2 > y1 {
		return translateTextBBox{
			X:      int(x1),
			Y:      int(y1),
			Width:  int(x2 - x1),
			Height: int(y2 - y1),
		}
	}
	return translateTextBBox{
		X:      ocrInt(m, "x", "left"),
		Y:      ocrInt(m, "y", "top"),
		Width:  ocrInt(m, "width", "w"),
		Height: ocrInt(m, "height", "h"),
	}
}

func firstOCRString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			s := strings.TrimSpace(fmt.Sprint(v))
			if s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

func firstOCRAny(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			return v
		}
	}
	return nil
}

func firstOCRFloat(m map[string]any, keys ...string) float64 {
	for _, k := range keys {
		v, ok := m[k]
		if !ok || v == nil {
			continue
		}
		switch x := v.(type) {
		case float64:
			return x
		case float32:
			return float64(x)
		case int:
			return float64(x)
		case int64:
			return float64(x)
		case json.Number:
			f, _ := x.Float64()
			return f
		case string:
			var f float64
			if _, err := fmt.Sscan(strings.TrimSpace(x), &f); err == nil {
				return f
			}
		}
	}
	return 0
}

func ocrInt(m map[string]any, keys ...string) int {
	return int(firstOCRFloat(m, keys...))
}

func ocrFloat(m map[string]any, keys ...string) float64 {
	return firstOCRFloat(m, keys...)
}
