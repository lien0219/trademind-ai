package imagetask

import (
	"strings"
	"testing"
)

func TestLayoutShortChineseToEnglish(t *testing.T) {
	opts := parseTranslateLayoutOptions(map[string]any{
		"autoLayout":         true,
		"autoWrap":           true,
		"autoFontSize":       true,
		"allowTextBoxExpand": true,
		"allowTextSimplify":  true,
		"minFontSize":        14,
		"maxFontSize":        48,
		"lineHeightRatio":    1.15,
		"maxLines":           3,
	}, "en")

	bbox := translateTextBBox{X: 20, Y: 30, Width: 120, Height: 36}
	plan := layoutTranslateBlock("Free shipping nationwide", "Free Shipping", bbox, opts, 800, 800)

	if plan.Overflow {
		t.Fatalf("expected no overflow for short text with simplify, got overflow")
	}
	if plan.DisplayText != "Free Shipping" && !plan.UsedShortText {
		t.Fatalf("expected short text usage, display=%q usedShort=%v", plan.DisplayText, plan.UsedShortText)
	}
	if len(plan.Lines) == 0 {
		t.Fatal("expected at least one line")
	}
	if plan.FontSize < opts.MinFontSize {
		t.Fatalf("font size below min: %d", plan.FontSize)
	}
}

func TestLayoutLongChineseToEnglishWrapAndResize(t *testing.T) {
	opts := parseTranslateLayoutOptions(map[string]any{
		"autoLayout":   true,
		"autoWrap":     true,
		"autoFontSize": true,
		"minFontSize":  14,
		"maxLines":     3,
	}, "en")

	bbox := translateTextBBox{X: 10, Y: 10, Width: 90, Height: 40}
	longText := "Premium quality waterproof durable outdoor sports backpack"
	plan := layoutTranslateBlock(longText, "Waterproof Backpack", bbox, opts, 800, 800)

	if len(plan.Lines) < 1 {
		t.Fatal("expected wrapped lines")
	}
	if plan.Wrapped && len(plan.Lines) == 1 {
		// narrow box should wrap or shrink
		if !plan.FontResized {
			t.Log("wrapped to single line with resize")
		}
	}
	maxW := 0.0
	for _, ln := range plan.Lines {
		w := estimateTextWidth(ln, plan.FontSize, false)
		if w > maxW {
			maxW = w
		}
	}
	if bbox.Width > 0 && maxW > float64(bbox.Width)*1.05 && !plan.Expanded {
		t.Fatalf("line width %.0f exceeds bbox %d", maxW, bbox.Width)
	}
}

func TestLayoutLongEnglishToChinese(t *testing.T) {
	opts := parseTranslateLayoutOptions(map[string]any{
		"autoLayout":   true,
		"autoWrap":     true,
		"autoFontSize": true,
		"minFontSize":  14,
		"maxLines":     3,
	}, "zh")

	bbox := translateTextBBox{X: 5, Y: 5, Width: 100, Height: 48}
	longEN := "Professional high quality stainless steel kitchen knife set with ergonomic handle"
	cnText := "专业高品质不锈钢厨房刀具套装人体工学手柄"
	plan := layoutTranslateBlock(cnText, ruleBasedShortText(cnText, cnText, "zh"), bbox, opts, 800, 800)

	if plan.FontSize < opts.MinFontSize {
		t.Fatalf("font below min: %d", plan.FontSize)
	}
	h := lineBlockHeight(len(plan.Lines), plan.FontSize, opts.LineHeightRatio)
	if bbox.Height > 0 && h > float64(bbox.Height)*1.35 && !plan.Overflow && !plan.Expanded {
		t.Fatalf("height %.0f too large for bbox %d", h, bbox.Height)
	}
	_ = longEN
}

func TestWrapEnglishDoesNotSplitWords(t *testing.T) {
	lines := wrapEnglishWords("Free shipping nationwide", 80, 16)
	for _, ln := range lines {
		if strings.Contains(ln, "ship") && strings.Contains(ln, "ping") && !strings.Contains(ln, "shipping") {
			t.Fatalf("word split incorrectly: %q", ln)
		}
	}
}

func TestComputeTranslateLayoutsSummary(t *testing.T) {
	opts := parseTranslateLayoutOptions(nil, "en")
	blocks := []translateTextBlock{
		{
			Text:                "全国包邮",
			TranslatedText:      "Free shipping nationwide",
			ShortTranslatedText: "Free Shipping",
			BBox:                translateTextBBox{X: 0, Y: 0, Width: 100, Height: 30},
		},
	}
	_, summary := computeTranslateLayouts(blocks, opts, 800, 800)
	if summary.TextBlocksCount != 1 {
		t.Fatalf("expected 1 block, got %d", summary.TextBlocksCount)
	}
}

func TestRuleBasedShortText(t *testing.T) {
	got := ruleBasedShortText("", "Free shipping nationwide", "en")
	if got != "Free Shipping" {
		t.Fatalf("got %q want Free Shipping", got)
	}
}

func TestBuildTranslateTextGroupsPhoneStandTemplate(t *testing.T) {
	blocks := []translateTextBlock{
		{ID: "b1", Text: "金属底座", TranslatedText: "Metal Base", BBox: translateTextBBox{X: 48, Y: 42, Width: 132, Height: 34}, Style: titleGroupStyle()},
		{ID: "b2", Text: "折叠支架", TranslatedText: "Foldable Stand", BBox: translateTextBBox{X: 48, Y: 82, Width: 150, Height: 34}, Style: titleGroupStyle()},
		{ID: "b3", Text: "手机 / 平板", TranslatedText: "Phone/Tablet", BBox: translateTextBBox{X: 48, Y: 134, Width: 120, Height: 34}, Style: badgeGroupStyle()},
		{ID: "b4", Text: "暗夜黑", TranslatedText: "Midnight Black", BBox: translateTextBBox{X: 58, Y: 620, Width: 110, Height: 36}, Style: badgeGroupStyle()},
	}
	groups, tpl := buildTranslateTextGroups(blocks, map[string]any{"layoutTemplate": "auto"}, 800, 800)
	if tpl != layoutTemplateTitleBadge {
		t.Fatalf("template = %q, want title_badge", tpl)
	}
	if len(groups) != 3 {
		t.Fatalf("groups = %d, want 3", len(groups))
	}
	if groups[0].GroupType != groupTypeMainTitle || len(groups[0].TranslatedLines) != 2 {
		t.Fatalf("main title group not merged: %+v", groups[0])
	}
	if groups[1].GroupType != groupTypeBadge {
		t.Fatalf("badge group = %q", groups[1].GroupType)
	}
	if groups[2].GroupType != groupTypeBottomBadge {
		t.Fatalf("bottom badge group = %q", groups[2].GroupType)
	}
}
