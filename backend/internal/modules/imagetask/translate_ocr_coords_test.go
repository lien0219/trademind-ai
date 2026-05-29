package imagetask

import "testing"

func TestApplyOCRCoordinateMappingScalesBlocks(t *testing.T) {
	ocr := &translateOCRResult{
		Blocks: []translateTextBlock{
			{Text: "A", BBox: translateTextBBox{X: 10, Y: 20, Width: 480, Height: 460}},
		},
	}
	meta, err := applyOCRCoordinateMapping(ocr, 1000, 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !meta.CoordScaleApplied {
		t.Fatalf("expected coord scale applied, meta=%+v", meta)
	}
	if ocr.Blocks[0].BBox.Width < 900 {
		t.Fatalf("expected scaled up bbox, got=%+v", ocr.Blocks[0].BBox)
	}
}

func TestApplyOCRCoordinateMappingNoScaleWhenMatched(t *testing.T) {
	ocr := &translateOCRResult{
		Blocks: []translateTextBlock{
			{Text: "A", BBox: translateTextBBox{X: 10, Y: 20, Width: 100, Height: 40}},
		},
	}
	meta, err := applyOCRCoordinateMapping(ocr, 110, 60)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.CoordScaleApplied {
		t.Fatalf("expected no scale, meta=%+v", meta)
	}
}

func TestApplyOCRCoordinateMappingRejectsExtremeRatio(t *testing.T) {
	ocr := &translateOCRResult{
		Blocks: []translateTextBlock{
			{Text: "A", BBox: translateTextBBox{X: 1, Y: 1, Width: 8, Height: 8}},
		},
	}
	_, err := applyOCRCoordinateMapping(ocr, 4000, 4000)
	if err == nil {
		t.Fatal("expected error for extreme scale")
	}
}
