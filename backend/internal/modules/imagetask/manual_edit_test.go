package imagetask

import "testing"

func TestManualBlocksFromOutputUsesEditableTranslationsAndCleansWhiteBadgeText(t *testing.T) {
	out := map[string]any{
		"ocr": map[string]any{
			"blocks": []any{
				map[string]any{
					"id":             "aliyun_1",
					"text":           "炫酷黑",
					"translatedText": "Cool Black",
					"bbox":           map[string]any{"x": float64(685), "y": float64(70), "width": float64(266), "height": float64(92)},
					"style":          map[string]any{"color": "#ffffff", "fontWeight": "bold", "align": "center"},
				},
			},
		},
		"blockClassifications": []any{
			map[string]any{
				"id":                    "aliyun_1",
				"text":                  "炫酷黑",
				"blockClass":            blockClassPill,
				"standardTranslation":   "Cool Black",
				"fixedShortTranslation": "Cool Black",
				"erase_bbox":            map[string]any{"x": float64(685), "y": float64(70), "width": float64(266), "height": float64(92)},
				"layout_bbox":           map[string]any{"x": float64(690), "y": float64(78), "width": float64(245), "height": float64(64)},
			},
		},
	}

	blocks := manualBlocksFromOutput(out)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	b := blocks[0]
	if b.Text != "Cool Black" {
		t.Fatalf("unexpected text: %q", b.Text)
	}
	if b.Color != "#111111" {
		t.Fatalf("manual edit should redraw cleaned badge text in dark color, got %q", b.Color)
	}
	if !b.RemoveSourceBackground {
		t.Fatalf("expected source cleanup enabled by default")
	}
	if b.BBox.X != 690 || b.EraseBBox.X != 685 {
		t.Fatalf("expected layout bbox and erase bbox to stay separate, got layout=%+v erase=%+v", b.BBox, b.EraseBBox)
	}
}

func TestBuildManualImageBlocksCanDisableSourceCleanupPerBlock(t *testing.T) {
	blocks := buildManualImageBlocks([]TranslateManualEditBlock{
		{
			ID:                     "b1",
			Text:                   "Cool Black",
			BlockClass:             blockClassTitle,
			BBox:                   translateTextBBox{X: 10, Y: 20, Width: 180, Height: 60},
			EraseBBox:              translateTextBBox{X: 8, Y: 18, Width: 190, Height: 70},
			FontSize:               28,
			Color:                  "#111111",
			RemoveSourceBackground: false,
		},
	}, 500, 500)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].EraseBBox.Width != 0 || blocks[0].EraseBBox.Height != 0 {
		t.Fatalf("expected erase bbox disabled, got %+v", blocks[0].EraseBBox)
	}
}
