package imagetask

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/pkg/imagerender"
)

func TestApplyPureTextReplaceHintsDefault(t *testing.T) {
	hints := applyPureTextReplaceHints(map[string]any{})
	if hints["renderMode"] != RenderModePureTextReplace {
		t.Fatalf("renderMode = %v, want pure_text_replace", hints["renderMode"])
	}
	if hints["eraseMode"] != imagerender.EraseTextPixelMask {
		t.Fatalf("eraseMode = %v, want text_pixel_mask", hints["eraseMode"])
	}
	if hints["allowTextBoxExpand"] != false {
		t.Fatalf("allowTextBoxExpand = %v, want false", hints["allowTextBoxExpand"])
	}
	if hints["allowTextOverflow"] != false {
		t.Fatalf("allowTextOverflow = %v, want false", hints["allowTextOverflow"])
	}
	if hints["pureTextReplace"] != true {
		t.Fatalf("pureTextReplace = %v, want true", hints["pureTextReplace"])
	}
}

func TestComputePureTextRenderBlocksNoBackground(t *testing.T) {
	blocks := []translateTextBlock{
		{
			ID: "b1", Text: "雪花白", BlockClass: blockClassTitle,
			TranslatedText: "Snow White", FixedShortTranslation: "Snow White",
			BBox:  translateTextBBox{X: 610, Y: 80, Width: 180, Height: 70},
			Style: translateTextStyle{Color: "#ffffff", FontWeight: "bold"},
		},
		{
			ID: "b2", Text: "折叠伸缩版/通用手机", BlockClass: blockClassBadge,
			TranslatedText:        "Foldable Telescopic Version / Universal Phone",
			FixedShortTranslation: "Universal Phone Stand",
			BadgeTranslation:      "Universal Stand",
			BBox:                  translateTextBBox{X: 550, Y: 180, Width: 120, Height: 48},
			Style:                 badgeGroupStyle(),
		},
	}
	populateTranslationVersions(blocks, "en")
	opts := parseTranslateLayoutOptions(applyPureTextReplaceHints(nil), "en")
	renderBlocks, summary := computePureTextRenderBlocks(blocks, opts, 900, 900)
	if summary.OverflowBlocks > 0 {
		t.Fatalf("unexpected overflow: %+v", summary)
	}
	if len(renderBlocks) != 2 {
		t.Fatalf("render blocks = %d, want 2", len(renderBlocks))
	}
	if renderBlocks[0].Lines[0] != "Snow White" {
		t.Fatalf("title = %q, want Snow White", renderBlocks[0].Lines[0])
	}
	if renderBlocks[1].Lines[0] != "For Phones" && renderBlocks[1].Lines[0] != "Universal Stand" {
		t.Fatalf("badge = %q, want For Phones or Universal Stand (shortest that fits)", renderBlocks[1].Lines[0])
	}
	if renderBlocks[0].Style.Color != "#ffffff" {
		t.Fatalf("white title should draw white text, got %q", renderBlocks[0].Style.Color)
	}
	for _, rb := range renderBlocks {
		if strings.TrimSpace(rb.Style.BackgroundColor) != "" {
			t.Fatalf("block %s must not draw background, got %+v", rb.ID, rb.Style)
		}
		if rb.Style.BorderRadius > 0 {
			t.Fatalf("block %s must not use border radius", rb.ID)
		}
	}
	if !validatePureTextRenderStyles(renderBlocks) {
		t.Fatal("validatePureTextRenderStyles failed")
	}
}

func TestPureTextTranslationRejectsLongLiteral(t *testing.T) {
	b := translateTextBlock{
		Text:                  "折叠伸缩版/通用手机",
		TranslatedText:        "Foldable Telescopic Version / Universal Phone",
		FixedShortTranslation: "Universal Phone Stand",
		BadgeTranslation:      "Universal Stand",
	}
	cands := pureTextTranslationCandidates(b, blockClassBadge, 120)
	for _, c := range cands {
		if strings.Contains(c, "Telescopic") {
			t.Fatalf("candidate must not include Telescopic: %q", c)
		}
	}
}

func TestDrawTextSkipsBackgroundInPureMode(t *testing.T) {
	img := imagerender.ToRGBA(testSolidRGBA(40, 40, 200, 200, 240, 240, 240))
	block := imagerender.TextBlock{
		Lines:    []string{"Test"},
		FontSize: 18,
		BBox:     imagerender.BBox{X: 10, Y: 10, Width: 80, Height: 30},
		Style: imagerender.TextStyle{
			Color:           "#ffffff",
			BackgroundColor: "#111111",
			BorderRadius:    12,
			Align:           "center",
		},
	}
	before := img.RGBAAt(20, 20)
	if err := imagerender.DrawText(img, block, imagerender.Options{PureTextReplace: true}); err != nil {
		t.Fatal(err)
	}
	after := img.RGBAAt(20, 20)
	if before != after {
		t.Fatal("pure text replace must not paint background patch at sample pixel")
	}
}

func testSolidRGBA(x0, y0, w, h int, r, g, b uint8) *image.RGBA {
	img := image.NewRGBA(image.Rect(x0, y0, x0+w, y0+h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x0+x, y0+y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	return img
}
