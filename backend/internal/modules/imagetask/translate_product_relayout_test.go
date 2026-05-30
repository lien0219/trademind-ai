package imagetask

import (
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/pkg/imagerender"
)

func TestProductRelayoutDefaultTemplate(t *testing.T) {
	hints := applyProductRelayoutHints(map[string]any{})
	if hints["renderMode"] != RenderModePureTextReplace {
		t.Fatalf("renderMode = %v, want pure_text_replace", hints["renderMode"])
	}
	if hints["eraseMode"] != imagerender.EraseTextPixelMask {
		t.Fatalf("eraseMode = %v, want text_pixel_mask", hints["eraseMode"])
	}
	if hints["removeOriginalText"] != true {
		t.Fatal("removeOriginalText should be true")
	}
}

func TestBuildTranslateTextGroupsCoolBlackPerBlock(t *testing.T) {
	blocks := []translateTextBlock{
		{ID: "b1", Text: "炫酷黑", TranslatedText: "Cool Black", CompactTranslation: "Cool Black", BadgeTranslation: "Cool Black", BBox: translateTextBBox{X: 610, Y: 80, Width: 180, Height: 70}, Style: titleGroupStyle()},
		{ID: "b2", Text: "折叠伸缩版/通用手机", TranslatedText: "Foldable Universal Phone Stand", CompactTranslation: "Universal Phone Stand", BadgeTranslation: "Universal Phone Stand", BBox: translateTextBBox{X: 550, Y: 180, Width: 330, Height: 48}, Style: badgeGroupStyle()},
	}
	populateTranslationVersions(blocks, "en")
	groups, tpl := buildTranslateTextGroups(blocks, map[string]any{"layoutTemplate": "auto"}, 900, 900)
	if tpl == layoutTemplateProductRelayout {
		t.Fatalf("template = %q, should not use product_relayout", tpl)
	}
	if len(groups) < 2 {
		t.Fatalf("groups = %d, want separate blocks not merged erase group", len(groups))
	}
	opts := parseTranslateLayoutOptions(applyRemoveTextThenRenderHints(nil), "en")
	renderBlocks, summary := computeRemoveTextRenderBlocks(blocks, opts, 900, 900)
	if summary.OverflowBlocks > 0 {
		t.Fatalf("unexpected overflow: %+v", summary)
	}
	if len(renderBlocks) != 2 {
		t.Fatalf("render blocks = %d, want 2", len(renderBlocks))
	}
	if renderBlocks[0].Lines[0] != "Cool Black" {
		t.Fatalf("title = %q", renderBlocks[0].Lines[0])
	}
	if renderBlocks[1].Lines[0] != "Universal Phone Stand" {
		t.Fatalf("badge = %q", renderBlocks[1].Lines[0])
	}
}

func TestTranslationVersionPriority(t *testing.T) {
	b := translateTextBlock{
		Text:                  "折叠伸缩版/通用手机",
		TranslatedText:        "Foldable Universal Phone Stand",
		StandardTranslation:   "Foldable Universal Phone Stand",
		CompactTranslation:    "Universal Phone Stand",
		BadgeTranslation:      "Universal Phone Stand",
		FixedShortTranslation: "Universal Phone Stand",
		BlockClass:            blockClassBadge,
	}
	got := selectTranslationVersion(b, blockClassBadge, 160, 1)
	if got != "Universal Phone Stand" {
		t.Fatalf("badge priority selection = %q", got)
	}
	title := translateTextBlock{
		Text:                  "雪花白",
		TranslatedText:        "Snow White",
		FixedShortTranslation: "Snow White",
		CompactTranslation:    "Snow White",
		BlockClass:            blockClassTitle,
	}
	if got := selectTranslationVersion(title, blockClassTitle, 200, 1); got != "Snow White" {
		t.Fatalf("title = %q, want Snow White", got)
	}
}

func TestRenderQualityThresholdsRemoveTextMode(t *testing.T) {
	rq := buildTranslateRenderQuality(
		translateQualitySummary{},
		translateLayoutSummary{},
		translateVerificationMeta{TargetTextDetected: true},
		translateRenderOptions{RenderMode: RenderModeRemoveTextThenRender},
		nil,
		nil,
	)
	if rq.SourceTextRemovedScore < 85 || rq.ReadabilityScore < 80 || rq.CommercialUsabilityScore < 80 {
		t.Fatalf("baseline scores too low: %+v", rq)
	}
	if !rq.Passed {
		t.Fatalf("expected passed baseline quality, got warnings=%v", rq.Warnings)
	}
}

func TestApplyOCRCoordinateMappingWithCropMeta(t *testing.T) {
	ocr := &translateOCRResult{
		Blocks: []translateTextBlock{
			{ID: "b1", Text: "A", BBox: translateTextBBox{X: 10, Y: 20, Width: 480, Height: 460}},
		},
	}
	hints := map[string]any{"ocrImageWidth": 490, "ocrImageHeight": 480}
	meta, err := applyOCRCoordinateMapping(ocr, 1000, 1000, 1000, 1000, hints)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.OriginalImageWidth != 1000 || meta.RenderImageWidth != 1000 {
		t.Fatalf("meta sizes = %+v", meta)
	}
	if !meta.CoordScaleApplied {
		t.Fatalf("expected coord scale applied, meta=%+v", meta)
	}
	if ocr.Blocks[0].BBox.Width < 900 {
		t.Fatalf("expected scaled up bbox, got=%+v", ocr.Blocks[0].BBox)
	}
}
