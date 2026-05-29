package imagetask

import "testing"

func TestCountSourceBlocksStillPresent(t *testing.T) {
	orig := &translateOCRResult{
		Blocks: []translateTextBlock{
			{Text: "金属底座"},
			{Text: "折叠支架"},
			{Text: "牛奶白"},
		},
	}
	post := &translateOCRResult{
		Blocks: []translateTextBlock{
			{Text: "金属", Confidence: 0.9},
		},
	}
	if got := countSourceBlocksStillPresent(post, orig); got != 1 {
		t.Fatalf("expected 1 block still present, got %d", got)
	}
	post2 := &translateOCRResult{
		Blocks: []translateTextBlock{
			{Text: "noise", Confidence: 0.4},
		},
	}
	if got := countSourceBlocksStillPresent(post2, orig); got != 0 {
		t.Fatalf("expected 0 for low confidence noise, got %d", got)
	}
}

func TestSourceEraseRemainThreshold(t *testing.T) {
	ocr := &translateOCRResult{
		Blocks: []translateTextBlock{
			{Text: "A"},
			{Text: "金属底座"},
			{Text: "折叠支架"},
		},
	}
	if sourceEraseRemainThreshold(ocr) != 1 {
		t.Fatalf("expected threshold 1 for 2 valid blocks, got %d", sourceEraseRemainThreshold(ocr))
	}
	ocr4 := &translateOCRResult{
		Blocks: []translateTextBlock{
			{Text: "金属底座"},
			{Text: "折叠支架"},
			{Text: "手机 / 平板"},
			{Text: "牛奶白"},
		},
	}
	if sourceEraseRemainThreshold(ocr4) != 2 {
		t.Fatalf("expected threshold 2 for 4 blocks, got %d", sourceEraseRemainThreshold(ocr4))
	}
}
