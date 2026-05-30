package imagetask

import (
	"strings"
	"testing"
)

func TestApplyOCRCoordinateMappingSameRatioDifferentSize(t *testing.T) {
	ocr := &translateOCRResult{
		Blocks: []translateTextBlock{
			{ID: "b1", Text: "A", BBox: translateTextBBox{X: 100, Y: 100, Width: 200, Height: 50}},
		},
	}
	hints := map[string]any{
		"ocrImageWidth":  800,
		"ocrImageHeight": 600,
	}
	meta, err := applyOCRCoordinateMapping(ocr, 1600, 1200, 1600, 1200, hints)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.MappingTier != mappingTierSmall && meta.MappingTier != mappingTierExact {
		t.Fatalf("tier = %q, want small or exact", meta.MappingTier)
	}
	got := ocr.Blocks[0].BBox
	if got.X < 195 || got.X > 205 {
		t.Fatalf("mapped x = %d, want ~200", got.X)
	}
	if got.Width < 395 || got.Width > 405 {
		t.Fatalf("mapped width = %d, want ~400", got.Width)
	}
}

func TestApplyOCRCoordinateMappingWithCropOffset(t *testing.T) {
	ocr := &translateOCRResult{
		Blocks: []translateTextBlock{
			{ID: "b1", Text: "A", BBox: translateTextBBox{X: 20, Y: 30, Width: 100, Height: 40}},
		},
	}
	hints := map[string]any{
		"ocrImageWidth":    400,
		"ocrImageHeight":   300,
		"cropOffsetX":      100,
		"cropOffsetY":      50,
		"cropRenderWidth":  800,
		"cropRenderHeight": 600,
	}
	meta, err := applyOCRCoordinateMapping(ocr, 1000, 1000, 1000, 1000, hints)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.MappingMode != mappingModeCrop {
		t.Fatalf("mapping mode = %q, want crop", meta.MappingMode)
	}
	got := ocr.Blocks[0].BBox
	if got.X < 140 || got.X > 145 {
		t.Fatalf("mapped x = %d, want ~142", got.X)
	}
}

func TestApplyOCRCoordinateMappingMediumAspectDiffDegrades(t *testing.T) {
	ocr := &translateOCRResult{
		Blocks: []translateTextBlock{
			{ID: "b1", Text: "A", BBox: translateTextBBox{X: 10, Y: 10, Width: 180, Height: 40}},
		},
	}
	hints := map[string]any{
		"ocrImageWidth":  950,
		"ocrImageHeight": 700,
	}
	meta, err := applyOCRCoordinateMapping(ocr, 1000, 770, 1000, 770, hints)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.MappingTier != mappingTierMedium {
		t.Fatalf("tier = %q, want medium", meta.MappingTier)
	}
	if !meta.GroupRelayoutFallback || meta.Fallback != "group_relayout" {
		t.Fatalf("expected group relayout fallback, meta=%+v", meta)
	}
}

func TestApplyOCRCoordinateMappingLargeAspectDiffFails(t *testing.T) {
	ocr := &translateOCRResult{
		Blocks: []translateTextBlock{
			{ID: "b1", Text: "A", BBox: translateTextBBox{X: 1, Y: 1, Width: 8, Height: 8}},
		},
	}
	hints := map[string]any{
		"ocrImageWidth":  8,
		"ocrImageHeight": 64,
	}
	_, err := applyOCRCoordinateMapping(ocr, 4000, 4000, 4000, 4000, hints)
	if err == nil {
		t.Fatal("expected error for extreme aspect ratio")
	}
	if !strings.Contains(err.Error(), "TRANSLATE_RENDER_FAILED") {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestApplyOCRCoordinateMappingNoScaleWhenMatched(t *testing.T) {
	ocr := &translateOCRResult{
		Blocks: []translateTextBlock{
			{ID: "b1", Text: "A", BBox: translateTextBBox{X: 10, Y: 20, Width: 100, Height: 40}},
		},
	}
	hints := map[string]any{
		"ocrImageWidth":  110,
		"ocrImageHeight": 60,
	}
	meta, err := applyOCRCoordinateMapping(ocr, 110, 60, 110, 60, hints)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.CoordScaleApplied {
		t.Fatalf("expected no scale, meta=%+v", meta)
	}
	if meta.MappingMode != mappingModeDirect {
		t.Fatalf("mapping mode = %q, want direct", meta.MappingMode)
	}
}

func TestClassifyCoordMappingTierSmall(t *testing.T) {
	if got := classifyCoordMappingTier(1.8, 1.8, 0.01); got != mappingTierSmall {
		t.Fatalf("got %q want small", got)
	}
}

func TestClassifyCoordMappingTierLarge(t *testing.T) {
	if got := classifyCoordMappingTier(4.0, 4.0, 0.02); got != mappingTierLarge {
		t.Fatalf("got %q want large", got)
	}
}
