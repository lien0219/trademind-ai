package imagetask

import (
	"strings"
	"testing"
)

func TestFlexibleBadgeLayoutNoFailOnNarrowBox(t *testing.T) {
	blocks := []translateTextBlock{
		{
			ID: "badge", Text: "折叠伸缩版/通用手机", BlockClass: blockClassBadge,
			TranslatedText:        "Foldable Telescopic Version / Universal Phone",
			FixedShortTranslation: "Universal Phone Stand",
			BBox:                  translateTextBBox{X: 620, Y: 180, Width: 120, Height: 36},
			Style:                 badgeGroupStyle(),
		},
	}
	populateTranslationVersions(blocks, "en")
	opts := parseTranslateLayoutOptions(applyRemoveTextThenRenderHints(nil), "en")
	renderBlocks, summary := computeRemoveTextRenderBlocks(blocks, opts, 900, 900)
	if summary.OverflowBlocks > 0 {
		t.Fatalf("overflow should not fail task: %+v", summary)
	}
	if len(renderBlocks) != 1 {
		t.Fatalf("blocks = %d", len(renderBlocks))
	}
	rb := renderBlocks[0]
	joined := strings.Join(rb.Lines, " ")
	if !strings.Contains(joined, "Universal") || !strings.Contains(joined, "Stand") {
		t.Fatalf("lines = %v", rb.Lines)
	}
	if len(rb.Lines) < 1 {
		t.Fatal("expected at least one line")
	}
}
