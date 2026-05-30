package imagetask

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/pkg/imagerender"
)

func TestApplyRemoveTextThenRenderHintsDefault(t *testing.T) {
	hints := applyRemoveTextThenRenderHints(map[string]any{})
	if hints["renderMode"] != RenderModeRemoveTextThenRender {
		t.Fatalf("renderMode = %v, want remove_text_then_render", hints["renderMode"])
	}
	if hints["eraseMode"] != imagerender.EraseTextPixelMask {
		t.Fatalf("eraseMode = %v, want text_pixel_mask", hints["eraseMode"])
	}
	if hints["layoutTemplate"] != layoutTemplatePreserveOriginal {
		t.Fatalf("layoutTemplate = %v, want preserve_original", hints["layoutTemplate"])
	}
	if hints["allowTextOverflow"] != true {
		t.Fatalf("allowTextOverflow = %v, want true", hints["allowTextOverflow"])
	}
	if hints["allowTextBoxExpand"] != true {
		t.Fatalf("allowTextBoxExpand = %v, want true", hints["allowTextBoxExpand"])
	}
}

func TestTranslationFixedShortPriority(t *testing.T) {
	b := translateTextBlock{
		Text:                  "折叠伸缩版/通用手机",
		TranslatedText:        "Foldable Telescopic Version / Universal Phone",
		StandardTranslation:   "Foldable Telescopic Version / Universal Phone",
		CompactTranslation:    "Foldable Universal Stand",
		BadgeTranslation:      "Universal Phone Stand",
		FixedShortTranslation: "Universal Phone Stand",
		BlockClass:            blockClassBadge,
	}
	got := selectTranslationVersion(b, blockClassBadge, 320, 1)
	if got != "Universal Phone Stand" {
		t.Fatalf("got %q, want Universal Phone Stand", got)
	}
}

func TestComputeRemoveTextRenderBlocksSnowWhitePhoneStand(t *testing.T) {
	blocks := []translateTextBlock{
		{
			ID: "b1", Text: "雪花白", BlockClass: blockClassTitle,
			TranslatedText: "Snow White", BBox: translateTextBBox{X: 610, Y: 80, Width: 180, Height: 70},
			Style: titleGroupStyle(),
		},
		{
			ID: "b2", Text: "折叠伸缩版/通用手机", BlockClass: blockClassBadge,
			TranslatedText: "Foldable Telescopic Version / Universal Phone",
			BBox:           translateTextBBox{X: 550, Y: 180, Width: 330, Height: 48},
			Style:          badgeGroupStyle(),
		},
	}
	populateTranslationVersions(blocks, "en")
	opts := parseTranslateLayoutOptions(applyRemoveTextThenRenderHints(nil), "en")
	renderBlocks, summary := computeRemoveTextRenderBlocks(blocks, opts, 900, 900)
	if summary.OverflowBlocks > 0 {
		t.Fatalf("unexpected overflow: %+v", summary)
	}
	if len(renderBlocks) != 2 {
		t.Fatalf("render blocks = %d, want 2", len(renderBlocks))
	}
	if renderBlocks[0].Lines[0] != "Snow White" {
		t.Fatalf("title line = %q, want Snow White", renderBlocks[0].Lines[0])
	}
	if renderBlocks[1].Lines[0] != "Universal Phone Stand" && strings.Join(renderBlocks[1].Lines, " ") != "Universal Phone Stand" {
		t.Fatalf("badge lines = %v, want Universal Phone Stand", renderBlocks[1].Lines)
	}
	if renderBlocks[0].EraseBBox.Width != blocks[0].BBox.Width {
		t.Fatalf("title erase bbox should match OCR bbox, got %+v", renderBlocks[0].EraseBBox)
	}
	if renderBlocks[0].ErasePadding > 2 || renderBlocks[1].ErasePadding > 2 {
		t.Fatalf("erase padding too large: %d / %d", renderBlocks[0].ErasePadding, renderBlocks[1].ErasePadding)
	}
}

func TestTextPixelMaskEraseAreaWithinLimits(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 900, 900))
	for y := 0; y < 900; y++ {
		for x := 0; x < 900; x++ {
			img.Set(x, y, color.RGBA{235, 235, 235, 255})
		}
	}
	drawDarkTextPatch(img, 610, 80, 180, 70)
	drawCapsuleBadge(img, 550, 180, 330, 48)

	blocks := []translateTextBlock{
		{ID: "b1", Text: "雪花白", BlockClass: blockClassTitle, TranslatedText: "Snow White", BBox: translateTextBBox{X: 610, Y: 80, Width: 180, Height: 70}, Style: titleGroupStyle()},
		{ID: "b2", Text: "折叠伸缩版/通用手机", BlockClass: blockClassBadge, TranslatedText: "Universal Phone Stand", BBox: translateTextBBox{X: 550, Y: 180, Width: 330, Height: 48}, Style: badgeGroupStyle()},
	}
	populateTranslationVersions(blocks, "en")
	opts := parseTranslateLayoutOptions(applyRemoveTextThenRenderHints(nil), "en")
	renderBlocks, _ := computeRemoveTextRenderBlocks(blocks, opts, 900, 900)
	imageBlocks := buildImageRenderBlocks(renderBlocks)

	_, stats, used, err := imagerender.EraseRegions(img, imageBlocks, imagerender.Options{EraseMode: imagerender.EraseTextPixelMask})
	if err != nil {
		t.Fatalf("erase: %v", err)
	}
	if used != imagerender.EraseTextPixelMask {
		t.Fatalf("erase mode = %q", used)
	}
	imageArea := 900 * 900
	ratio := float64(stats.ErasePixels) / float64(imageArea)
	if ratio > imagerender.MaxEraseMaskRatioTotal {
		t.Fatalf("erase ratio %.4f exceeds 6%%", ratio)
	}
	if stats.LargePatchDetected {
		t.Fatal("large patch detected")
	}
}

func TestTextPixelMaskPreservesCapsuleBackground(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 900, 900))
	for y := 0; y < 900; y++ {
		for x := 0; x < 900; x++ {
			img.Set(x, y, color.RGBA{230, 230, 230, 255})
		}
	}
	drawCapsuleBadge(img, 550, 180, 330, 48)
	block := imagerender.TextBlock{
		ID: "badge", BlockClass: "badge",
		EraseBBox:    imagerender.BBox{X: 550, Y: 180, Width: 330, Height: 48},
		ErasePadding: 1, MaskDilate: 1, TextPolarity: "light",
		Style: imagerender.TextStyle{BackgroundColor: "#111111", Color: "#ffffff"},
	}
	_, _, _, err := imagerender.EraseRegions(img, []imagerender.TextBlock{block}, imagerender.Options{EraseMode: imagerender.EraseTextPixelMask})
	if err != nil {
		t.Fatalf("erase: %v", err)
	}
	corner := img.RGBAAt(560, 185)
	if luminanceAt(corner) > 80 {
		t.Fatal("capsule corner background should remain dark")
	}
}

func drawDarkTextPatch(img *image.RGBA, x, y, w, h int) {
	for dy := h/3 + 2; dy < h*2/3; dy++ {
		for dx := w / 6; dx < w*5/6; dx++ {
			if (dx*5+dy*7)%23 < 1 {
				img.Set(x+dx, y+dy, color.RGBA{25, 25, 25, 255})
			}
		}
	}
}

func drawCapsuleBadge(img *image.RGBA, x, y, w, h int) {
	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			img.Set(x+dx, y+dy, color.RGBA{20, 20, 20, 255})
		}
	}
	// sparse white strokes simulating capsule label text
	strokeRows := []int{h/2 - 2, h/2 + 1}
	for _, row := range strokeRows {
		for dx := w / 5; dx < w*4/5; dx++ {
			if (dx*2+row)%7 < 2 {
				img.Set(x+dx, y+row, color.RGBA{245, 245, 245, 255})
			}
		}
	}
}

func luminanceAt(c color.RGBA) float64 {
	return 0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)
}

func TestRenderModeFromHintsDefaultPureTextReplace(t *testing.T) {
	if got := renderModeFromHints(nil); got != RenderModePureTextReplace {
		t.Fatalf("default render mode = %q, want pure_text_replace", got)
	}
	if got := renderModeFromHints(map[string]any{"renderMode": RenderModeRemoveTextThenRender}); got != RenderModeRemoveTextThenRender {
		t.Fatalf("explicit remove_text_then_render = %q", got)
	}
}

func TestEffectiveEraseModeRemoveTextThenRender(t *testing.T) {
	opts := translateRenderOptions{RenderMode: RenderModeRemoveTextThenRender}
	if got := effectiveEraseMode(opts); got != imagerender.EraseTextPixelMask {
		t.Fatalf("erase mode = %q", got)
	}
}

func TestBuildTranslateTextGroupsNoProductRelayoutDefault(t *testing.T) {
	blocks := []translateTextBlock{
		{ID: "b1", Text: "雪花白", TranslatedText: "Snow White", BBox: translateTextBBox{X: 610, Y: 80, Width: 180, Height: 70}, Style: titleGroupStyle()},
		{ID: "b2", Text: "折叠伸缩版/通用手机", TranslatedText: "Universal Phone Stand", BBox: translateTextBBox{X: 550, Y: 180, Width: 330, Height: 48}, Style: badgeGroupStyle()},
	}
	_, tpl := buildTranslateTextGroups(blocks, applyRemoveTextThenRenderHints(nil), 900, 900)
	if tpl == layoutTemplateProductRelayout {
		t.Fatalf("template should not default to product_relayout, got %q", tpl)
	}
}

func TestTranslationCandidatesNoLongLiteral(t *testing.T) {
	b := translateTextBlock{
		Text:                "折叠伸缩版/通用手机",
		TranslatedText:      "Foldable Telescopic Version / Universal Phone",
		StandardTranslation: "Foldable Telescopic Version / Universal Phone",
		CompactTranslation:  "Universal Phone Stand",
	}
	cands := translationCandidatesInPriority(b, blockClassBadge)
	if len(cands) == 0 {
		t.Fatal("expected candidates")
	}
	if strings.Contains(cands[0], "Telescopic") {
		t.Fatalf("first candidate should be short, got %q", cands[0])
	}
}
