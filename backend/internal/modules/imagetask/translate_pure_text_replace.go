package imagetask

import (
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/pkg/imagerender"
)

const (
	warningPureTextSourceNotErased = "pure_text_source_not_erased"
	warningPureTextExtraBackground = "pure_text_extra_background"
	warningPureTextOverlap         = "pure_text_overlap"
)

func applyPureTextReplaceHints(hints map[string]any) map[string]any {
	if hints == nil {
		hints = map[string]any{}
	}
	if strings.TrimSpace(stringFromMap(hints, "renderMode")) == "" {
		hints["renderMode"] = RenderModePureTextReplace
	}
	if strings.TrimSpace(stringFromMap(hints, "eraseMode")) == "" {
		hints["eraseMode"] = imagerender.EraseTextPixelMask
	}
	tpl := strings.TrimSpace(strings.ToLower(stringFromMap(hints, "layoutTemplate")))
	if tpl == "" || tpl == layoutTemplateAuto {
		hints["layoutTemplate"] = layoutTemplatePreserveOriginal
	}
	hints["removeOriginalText"] = true
	hints["compactTranslation"] = true
	hints["pureTextReplace"] = true
	hints["allowTextOverflow"] = false
	hints["allowTextBoxExpand"] = false
	hints["allowTextSimplify"] = true
	return hints
}

func applyTranslateRenderHints(hints map[string]any) map[string]any {
	mode := renderModeFromHints(hints)
	if isPureTextReplaceMode(mode) {
		return applyPureTextReplaceHints(hints)
	}
	return applyRemoveTextThenRenderHints(hints)
}

func isPureTextReplaceMode(mode string) bool {
	return strings.EqualFold(strings.TrimSpace(mode), RenderModePureTextReplace)
}

func computePureTextRenderBlocks(
	blocks []translateTextBlock,
	opts translateLayoutOptions,
	imageW, imageH int,
) ([]translateRenderBlock, translateLayoutSummary) {
	summary := translateLayoutSummary{
		AutoLayout:      opts.AutoLayout,
		LayoutTemplate:  layoutTemplatePreserveOriginal,
		TextBlocksCount: len(blocks),
		MinFontSizeUsed: opts.MaxFontSize,
		Warnings:        []string{},
	}
	if summary.MinFontSizeUsed <= 0 {
		summary.MinFontSizeUsed = opts.MinFontSize
	}
	out := make([]translateRenderBlock, 0, len(blocks))
	for _, b := range blocks {
		if strings.TrimSpace(b.TranslatedText) == "" {
			continue
		}
		blockClass := strings.TrimSpace(b.BlockClass)
		if blockClass == "" {
			blockClass = blockClassSmallCaption
		}
		anchorBBox := clampGroupBBox(b.BBox, imageW, imageH)
		detectBBox := anchorBBox
		baseStyle := drawStyleForPureTextBlock(b, blockClass)
		maxLines := maxLinesForPureTextBlock(blockClass, opts)
		text, fontSize, lines, overflow, renderStyle := selectPureTextTranslation(
			b, blockClass, anchorBBox, opts, maxLines, baseStyle,
		)
		drawBBox := anchorBBox
		if overflow {
			summary.OverflowBlocks++
			summary.Warnings = appendUniqueCodeWarning(summary.Warnings, layoutWarningOverflow)
		}
		if fontSize > 0 && (summary.MinFontSizeUsed == 0 || fontSize < summary.MinFontSizeUsed) {
			summary.MinFontSizeUsed = fontSize
		}
		if len(lines) == 0 && text != "" {
			lines = []string{text}
		}
		out = append(out, translateRenderBlock{
			ID:           b.ID,
			GroupType:    groupTypeForBlockClass(blockClass),
			BlockClass:   blockClass,
			Lines:        lines,
			FontSize:     fontSize,
			BBox:         drawBBox,
			EraseBBox:    detectBBox,
			OriginalBBox: clampGroupBBox(b.BBox, imageW, imageH),
			Style:        renderStyle,
			ErasePadding: erasePaddingForBlockClass(blockClass),
			MaskDilate:   1,
			TextPolarity: textPolarityForBlock(b),
		})
	}
	if summary.MinFontSizeUsed <= 0 || summary.MinFontSizeUsed > opts.MaxFontSize {
		summary.MinFontSizeUsed = opts.MinFontSize
	}
	return out, summary
}

func drawStyleForPureTextBlock(b translateTextBlock, blockClass string) translateTextStyle {
	if isCapsuleBlockClassForRender(blockClass) && isDarkLabelStyle(b.Style) {
		style := translateTextStyle{
			Color:      "#ffffff",
			FontWeight: "bold",
			Align:      "center",
		}
		return style
	}
	if isCapsuleBlockClassForRender(blockClass) {
		style := translateTextStyle{
			Color:      "#111111",
			FontWeight: "bold",
			Align:      "center",
		}
		if strings.TrimSpace(b.Style.Color) != "" {
			style.Color = b.Style.Color
		}
		return style
	}
	if blockClass == blockClassTitle || blockClass == blockClassColorBadge || isColorBadgeText(b.Text) {
		style := titleGroupStyle()
		if isWhiteTextStyle(b.Style) {
			style.Color = "#ffffff"
		} else if c := strings.TrimSpace(b.Style.Color); c != "" {
			style.Color = c
		}
		if strings.TrimSpace(b.Style.FontWeight) != "" {
			style.FontWeight = b.Style.FontWeight
		}
		if strings.TrimSpace(b.Style.Align) != "" {
			style.Align = b.Style.Align
		}
		return style
	}
	style := b.Style
	if strings.TrimSpace(style.Color) == "" {
		style.Color = defaultTranslateTextColor
	}
	if strings.TrimSpace(style.Align) == "" {
		style.Align = "left"
	}
	style.BackgroundColor = ""
	style.BorderRadius = 0
	return style
}

func maxLinesForPureTextBlock(blockClass string, opts translateLayoutOptions) int {
	if isCapsuleBlockClassForRender(blockClass) {
		return 1
	}
	maxLines := opts.MaxLines
	if maxLines <= 0 {
		maxLines = 2
	}
	if blockClass == blockClassTitle {
		return minInt(maxLines, 2)
	}
	return maxLines
}

func pureTextTranslationCandidates(b translateTextBlock, blockClass string, boxWidth int) []string {
	isBadge := isCapsuleBlockClassForRender(blockClass) ||
		blockClass == blockClassBadge || blockClass == blockClassColorBadge
	order := []string{
		strings.TrimSpace(b.FixedShortTranslation),
		strings.TrimSpace(b.BadgeTranslation),
		strings.TrimSpace(b.CompactTranslation),
		strings.TrimSpace(b.ShortTranslatedText),
		strings.TrimSpace(b.StandardTranslation),
		strings.TrimSpace(b.TranslatedText),
	}
	if isBadge {
		order = append(order, "Universal Stand", "For Phones", "Phone Stand")
	} else {
		order[1], order[2] = order[2], order[1]
	}
	seen := map[string]bool{}
	var out []string
	for _, s := range order {
		if s == "" || seen[s] {
			continue
		}
		if pureTextRejectLongLiteral(s) {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	if isBadge && boxWidth > 0 {
		var fitted []string
		for _, s := range out {
			if measureTextWidth(s, initialFontSizeForClass(translateTextBBox{Width: boxWidth}, s, translateLayoutOptions{MaxFontSize: 52, MinFontSize: 14}, blockClass), false) <= float64(boxWidth)*1.02 {
				fitted = append(fitted, s)
			}
		}
		if len(fitted) > 0 {
			return fitted
		}
	}
	return out
}

func pureTextRejectLongLiteral(s string) bool {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "Telescopic") {
		return true
	}
	if s == "Universal Phone" {
		return true
	}
	if strings.Contains(s, "Foldable Telescopic") {
		return true
	}
	return false
}

func selectPureTextTranslation(
	b translateTextBlock,
	blockClass string,
	anchor translateTextBBox,
	opts translateLayoutOptions,
	maxLines int,
	style translateTextStyle,
) (text string, fontSize int, lines []string, overflow bool, outStyle translateTextStyle) {
	outStyle = style
	candidates := pureTextTranslationCandidates(b, blockClass, anchor.Width)
	minFS := opts.MinFontSize
	if minFS <= 0 {
		minFS = 14
	}
	readableMin := maxInt(10, minFS-4)
	measureBBox := anchor
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		fs := initialFontSizeForClass(anchor, candidate, opts, blockClass)
		display := candidate
		wrapped := []string{display}
		if opts.AutoWrap && anchor.Width > 0 && maxLines > 1 {
			wrapped = wrapTextToWidth(display, float64(anchor.Width), fs, false, maxLines)
		}
		for fit := textFitsBBox(wrapped, fs, measureBBox, opts); !fit && fs > readableMin; fs-- {
			if opts.AutoWrap && anchor.Width > 0 {
				wrapped = wrapTextToWidth(display, float64(anchor.Width), fs, false, maxLines)
			}
			fit = textFitsBBox(wrapped, fs, measureBBox, opts)
		}
		if len(wrapped) > 0 && textFitsBBox(wrapped, fs, measureBBox, opts) {
			return display, fs, wrapped, false, outStyle
		}
	}
	shortest := ""
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if shortest == "" || len([]rune(c)) < len([]rune(shortest)) {
			shortest = c
		}
	}
	if shortest == "" {
		return "", 0, nil, true, outStyle
	}
	fs := readableMin
	wrapped := []string{shortest}
	if opts.AutoWrap && anchor.Width > 0 && maxLines > 1 {
		wrapped = wrapTextToWidth(shortest, float64(anchor.Width), fs, false, maxLines)
	}
	if textFitsBBox(wrapped, fs, measureBBox, opts) {
		return shortest, fs, wrapped, false, outStyle
	}
	return shortest, fs, wrapped, true, outStyle
}
