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

func TestNeedsOCRBBoxRepairLeftCluster(t *testing.T) {
	blocks := []translateTextBlock{
		{BBox: translateTextBBox{X: 0, Y: 0, Width: 100, Height: 40}, Text: "雪花白"},
		{BBox: translateTextBBox{X: 5, Y: 2, Width: 200, Height: 36}, Text: "折叠伸缩版/通用手机"},
	}
	if !needsOCRBBoxRepair(blocks) {
		t.Fatal("expected repair for left-cluster blocks")
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

func TestHeuristicRepairOCRBlockBBoxesTopRightProductLabels(t *testing.T) {
	blocks := []translateTextBlock{
		{BBox: translateTextBBox{Y: 0, Width: 120, Height: 50}, Text: "雪花白"},
		{BBox: translateTextBBox{Y: 0, Width: 280, Height: 40}, Text: "折叠伸缩版/通用手机"},
	}
	out := heuristicRepairOCRBlockBBoxes(blocks, 900, 900)
	if len(out) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(out))
	}
	if out[0].BBox.X < 500 {
		t.Fatalf("title should be placed top-right, got x=%d", out[0].BBox.X)
	}
	if out[1].BBox.X < 500 {
		t.Fatalf("pill should be placed top-right, got x=%d", out[1].BBox.X)
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

func TestPreferPolygonBBoxesForRotatedAliyunOCR(t *testing.T) {
	blocks := []translateTextBlock{
		{
			Text:  "炫酷黑",
			Angle: -89,
			BBox:  translateTextBBox{X: 774, Y: -15, Width: 87, Height: 264},
			Polygon: []translateTextPoint{
				{X: 686, Y: 70},
				{X: 951, Y: 74},
				{X: 949, Y: 162},
				{X: 685, Y: 158},
			},
		},
		{
			Text:  "折叠伸缩版/通用手机",
			Angle: -89,
			BBox:  translateTextBBox{X: 760, Y: 29, Width: 40, Height: 356},
			Polygon: []translateTextPoint{
				{X: 602, Y: 185},
				{X: 958, Y: 189},
				{X: 958, Y: 229},
				{X: 601, Y: 225},
			},
		},
	}
	out := preferPolygonBBoxesForRotatedOCR(blocks, 987, 987)
	if out[0].BBox.Width < 250 || out[0].BBox.Height < 80 {
		t.Fatalf("title should use polygon bbox, got %+v", out[0].BBox)
	}
	if out[1].BBox.Width < 330 || out[1].BBox.Height < 35 {
		t.Fatalf("pill should use polygon bbox, got %+v", out[1].BBox)
	}
	if out[1].BBox.X > 620 || out[1].BBox.Y < 180 {
		t.Fatalf("pill bbox should cover original top-right capsule, got %+v", out[1].BBox)
	}
}
