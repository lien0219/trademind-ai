package imagetask

import "testing"

func TestNeedsOCRBBoxRepairAllZeroY(t *testing.T) {
	blocks := []translateTextBlock{
		{BBox: translateTextBBox{Y: 0, Width: 100, Height: 40}, Text: "A"},
		{BBox: translateTextBBox{Y: 0, Width: 100, Height: 40}, Text: "B"},
	}
	if !needsOCRBBoxRepair(blocks) {
		t.Fatal("expected repair needed for zero-y blocks")
	}
}

func TestNeedsOCRBBoxRepairValidBoxes(t *testing.T) {
	blocks := []translateTextBlock{
		{BBox: translateTextBBox{Y: 34, Width: 100, Height: 40}, Text: "A"},
		{BBox: translateTextBBox{Y: 97, Width: 100, Height: 40}, Text: "B"},
	}
	if needsOCRBBoxRepair(blocks) {
		t.Fatal("expected no repair for valid boxes")
	}
}

func TestHeuristicRepairOCRBlockBBoxesStacksVertically(t *testing.T) {
	blocks := []translateTextBlock{
		{BBox: translateTextBBox{Y: 0, Width: 270, Height: 120}, Text: "金属底座"},
		{BBox: translateTextBBox{Y: 0, Width: 270, Height: 240}, Text: "折叠支架"},
	}
	out := heuristicRepairOCRBlockBBoxes(blocks, 800, 800)
	if len(out) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(out))
	}
	if out[0].BBox.Y <= 0 || out[1].BBox.Y <= out[0].BBox.Y {
		t.Fatalf("expected stacked y positions, got %+v and %+v", out[0].BBox, out[1].BBox)
	}
}

func TestInferBlockStylesDefaultsBlack(t *testing.T) {
	blocks := []translateTextBlock{{Text: "test"}}
	inferBlockStyles(nil, blocks)
	if blocks[0].Style.Color != defaultTranslateTextColor {
		t.Fatalf("expected black text, got %q", blocks[0].Style.Color)
	}
}

func TestClampOCRBlockBBoxes(t *testing.T) {
	blocks := []translateTextBlock{{
		Text: "暗夜黑",
		BBox: translateTextBBox{X: 25, Y: 799, Width: 275, Height: 1},
	}}
	out := clampOCRBlockBBoxes(blocks, 800, 800)
	if out[0].BBox.Height < 24 {
		t.Fatalf("bbox height too small: %+v", out[0].BBox)
	}
	if out[0].BBox.Y+out[0].BBox.Height > 800 {
		t.Fatalf("bbox out of image: %+v", out[0].BBox)
	}
}
