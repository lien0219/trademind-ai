package imagetask

import (
	"testing"
)

func TestParseOCRJSONWithCodeFence(t *testing.T) {
	raw := "```json\n{\"detectedLanguage\":\"zh\",\"textBlocksCount\":1,\"blocks\":[{\"text\":\"全国包邮\",\"translatedText\":\"Free Shipping\",\"confidence\":0.9,\"bbox\":{\"x\":1,\"y\":2,\"width\":100,\"height\":30}}]}\n```"
	ocr, err := parseOCRJSON(raw)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(ocr.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(ocr.Blocks))
	}
	if ocr.Blocks[0].Text != "全国包邮" {
		t.Fatalf("unexpected text: %q", ocr.Blocks[0].Text)
	}
}

func TestParseOCRFlexibleSnakeCase(t *testing.T) {
	raw := `{
	  "detected_language": "zh",
	  "text_blocks": [
	    {
	      "original_text": "金属底座 折叠支架",
	      "translated_text": "Metal Folding Stand",
	      "confidence": "0.88",
	      "bounding_box": {"x": 10, "y": 20, "width": 200, "height": 40}
	    }
	  ]
	}`
	ocr, err := parseOCRJSON(raw)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(ocr.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(ocr.Blocks))
	}
	if ocr.Blocks[0].TranslatedText != "Metal Folding Stand" {
		t.Fatalf("unexpected translation: %q", ocr.Blocks[0].TranslatedText)
	}
}
