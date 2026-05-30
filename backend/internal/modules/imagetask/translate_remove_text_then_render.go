package imagetask

import (
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/pkg/imagerender"
)

const (
	blockClassPill = "pill"
)

func applyRemoveTextThenRenderHints(hints map[string]any) map[string]any {
	if hints == nil {
		hints = map[string]any{}
	}
	if strings.TrimSpace(stringFromMap(hints, "renderMode")) == "" {
		hints["renderMode"] = RenderModeRemoveTextThenRender
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
	hints["allowTextOverflow"] = true
	if hints["allowTextBoxExpand"] == nil {
		hints["allowTextBoxExpand"] = true
	}
	if hints["allowTextSimplify"] == nil {
		hints["allowTextSimplify"] = true
	}
	return hints
}

func isRemoveTextThenRenderMode(mode string) bool {
	return strings.EqualFold(strings.TrimSpace(mode), RenderModeRemoveTextThenRender)
}

func computeRemoveTextRenderBlocks(
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
		baseStyle := drawStyleForBlock(b, blockClass)
		maxLines := maxLinesForBlockClass(blockClass, opts)
		wrapWidth := wrapWidthForBlock(anchorBBox, blockClass, imageW)
		text, fontSize, lines, overflow, renderStyle := selectTranslationWithFlexibleLayout(
			b, blockClass, anchorBBox, wrapWidth, opts, maxLines, baseStyle, imageW, imageH,
		)
		drawBBox := expandDrawBBoxToFitLines(lines, fontSize, anchorBBox, opts, imageW, imageH, blockClass)
		if overflow && opts.AllowTextBoxExpand {
			overflow = false
		}
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
		_ = text
	}
	if summary.MinFontSizeUsed <= 0 || summary.MinFontSizeUsed > opts.MaxFontSize {
		summary.MinFontSizeUsed = opts.MinFontSize
	}
	return out, summary
}

func toGroupPlansFromRenderBlocks(blocks []translateRenderBlock) []translateGroupLayoutPlan {
	plans := make([]translateGroupLayoutPlan, 0, len(blocks))
	for _, b := range blocks {
		plans = append(plans, translateGroupLayoutPlan{
			ID:           b.ID,
			GroupType:    b.GroupType,
			BlockClass:   b.BlockClass,
			Lines:        append([]string(nil), b.Lines...),
			FontSize:     b.FontSize,
			BBox:         b.BBox,
			EraseBBox:    b.EraseBBox,
			OriginalBBox: b.OriginalBBox,
			Style:        b.Style,
			ErasePadding: b.ErasePadding,
			Overflow:     false,
		})
	}
	return plans
}

func isCapsuleBlockClassForRender(class string) bool {
	switch class {
	case blockClassSubtitle, blockClassBadge, blockClassColorBadge, blockClassPill:
		return true
	default:
		return false
	}
}

func groupTypeForBlockClass(class string) string {
	switch class {
	case blockClassTitle:
		return groupTypeMainTitle
	case blockClassSubtitle:
		return groupTypeSubtitle
	case blockClassBadge, blockClassColorBadge, blockClassPill:
		return groupTypeBadge
	default:
		return groupTypeNormalText
	}
}

func drawStyleForBlock(b translateTextBlock, blockClass string) translateTextStyle {
	if isLightCapsuleBlock(b, blockClass) {
		style := b.Style
		if strings.TrimSpace(style.BackgroundColor) == "" {
			style.BackgroundColor = "#ffffff"
		}
		if isWhiteTextStyle(style) || strings.TrimSpace(style.Color) == "" {
			style.Color = "#111111"
		}
		style.Align = "center"
		if strings.TrimSpace(style.FontWeight) == "" {
			style.FontWeight = "bold"
		}
		return style
	}
	if isCapsuleBlockClassForRender(blockClass) && isDarkLabelStyle(b.Style) {
		style := badgeGroupStyle()
		if strings.TrimSpace(b.Style.BackgroundColor) != "" {
			style.BackgroundColor = b.Style.BackgroundColor
		}
		style.Align = "center"
		style.Color = "#ffffff"
		return style
	}
	if strings.TrimSpace(b.Style.Color) != "" || strings.TrimSpace(b.Style.Align) != "" {
		style := b.Style
		if strings.TrimSpace(style.Align) == "" {
			style.Align = "left"
		}
		if strings.TrimSpace(style.Color) == "" {
			style.Color = defaultTranslateTextColor
		}
		return style
	}
	return titleGroupStyle()
}

func isLightCapsuleBlock(b translateTextBlock, blockClass string) bool {
	if blockClass == blockClassColorBadge || isColorBadgeText(b.Text) {
		return true
	}
	if !isCapsuleBlockClassForRender(blockClass) {
		return false
	}
	if isDarkLabelStyle(b.Style) {
		return false
	}
	return isDarkTextStyle(b.Style) || (!isWhiteTextStyle(b.Style) && strings.TrimSpace(b.Style.BackgroundColor) == "")
}

func textPolarityForBlock(b translateTextBlock) string {
	if isWhiteTextStyle(b.Style) && isDarkLabelStyle(b.Style) {
		return "light"
	}
	if isDarkTextStyle(b.Style) {
		return "dark"
	}
	if isWhiteTextStyle(b.Style) {
		return "light"
	}
	if isDarkLabelStyle(b.Style) {
		return "light"
	}
	return "dark"
}

func maxLinesForBlockClass(blockClass string, opts translateLayoutOptions) int {
	maxLines := opts.MaxLines
	if maxLines <= 0 {
		maxLines = 3
	}
	if blockClass == blockClassTitle {
		return minInt(maxLines, 3)
	}
	if isCapsuleBlockClassForRender(blockClass) {
		return minInt(maxInt(maxLines, 3), 4)
	}
	return maxLines
}

func wrapWidthForBlock(bbox translateTextBBox, blockClass string, imageW int) float64 {
	w := float64(bbox.Width)
	if w <= 0 {
		return float64(imageW) * 0.4
	}
	if isCapsuleBlockClassForRender(blockClass) {
		// Badge may wrap within a wider band; anchor stays near original x.
		expand := w * 1.6
		maxW := float64(imageW-bbox.X) - 8
		if maxW > expand {
			return expand
		}
		if maxW > w {
			return maxW
		}
		return w
	}
	return w
}

func expandDrawBBoxToFitLines(
	lines []string,
	fontSize int,
	anchor translateTextBBox,
	opts translateLayoutOptions,
	imageW, imageH int,
	blockClass string,
) translateTextBBox {
	if len(lines) == 0 || fontSize <= 0 {
		return anchor
	}
	padX, padY := 6, 4
	if isCapsuleBlockClassForRender(blockClass) {
		padX, padY = 4, 3
	}
	maxLineW := 0.0
	for _, ln := range lines {
		w := measureTextWidth(ln, fontSize, mostlyCJK(ln))
		if w > maxLineW {
			maxLineW = w
		}
	}
	needW := int(maxLineW) + padX*2
	needH := int(lineBlockHeight(len(lines), fontSize, opts.LineHeightRatio)) + padY*2
	if needW < anchor.Width {
		needW = anchor.Width
	}
	if needH < anchor.Height {
		needH = anchor.Height
	}
	out := translateTextBBox{
		X:      anchor.X,
		Y:      anchor.Y,
		Width:  needW,
		Height: needH,
	}
	// Prefer growing down/right from OCR anchor; clamp to canvas.
	if out.X+out.Width > imageW {
		shift := out.X + out.Width - imageW
		out.X -= shift
		if out.X < 0 {
			out.X = 0
			out.Width = minInt(out.Width, imageW)
		}
	}
	if out.Y+out.Height > imageH {
		out.Height = minInt(out.Height, imageH-out.Y)
	}
	return clampGroupBBox(out, imageW, imageH)
}

func plainTextStyleForFlexibleBadge(b translateTextBlock) translateTextStyle {
	style := titleGroupStyle()
	style.Align = "left"
	if strings.TrimSpace(b.Style.Color) != "" && !isWhiteTextStyle(b.Style) {
		style.Color = b.Style.Color
	}
	style.BackgroundColor = ""
	style.BorderRadius = 0
	return style
}

func selectTranslationWithFlexibleLayout(
	b translateTextBlock,
	blockClass string,
	anchor translateTextBBox,
	wrapWidth float64,
	opts translateLayoutOptions,
	maxLines int,
	style translateTextStyle,
	imageW, imageH int,
) (text string, fontSize int, lines []string, overflow bool, outStyle translateTextStyle) {
	outStyle = style
	candidates := translationCandidatesInPriority(b, blockClass)
	minFS := opts.MinFontSize
	if minFS <= 0 {
		minFS = 14
	}
	readableMin := maxInt(10, minFS-4)
	measureBBox := anchor
	if opts.AllowTextBoxExpand {
		measureBBox.Width = int(wrapWidth)
		if measureBBox.Width < anchor.Width {
			measureBBox.Width = anchor.Width
		}
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		fs := initialFontSizeForClass(anchor, candidate, opts, blockClass)
		display := candidate
		wrapped := []string{display}
		if opts.AutoWrap && wrapWidth > 0 {
			wrapped = wrapTextToWidth(display, wrapWidth, fs, false, maxLines)
		}
		for fit := textFitsBBox(wrapped, fs, measureBBox, opts); !fit && fs > readableMin; fs-- {
			if opts.AutoWrap && wrapWidth > 0 {
				wrapped = wrapTextToWidth(display, wrapWidth, fs, false, maxLines)
			}
			fit = textFitsBBox(wrapped, fs, measureBBox, opts)
		}
		if len(wrapped) > 0 {
			expanded := expandDrawBBoxToFitLines(wrapped, fs, anchor, opts, imageW, imageH, blockClass)
			if textFitsBBox(wrapped, fs, expanded, opts) || opts.AllowTextBoxExpand {
				return display, fs, wrapped, false, outStyle
			}
		}
	}
	// Fallback: plain text without pill background, wider wrap.
	if isCapsuleBlockClassForRender(blockClass) && strings.TrimSpace(outStyle.BackgroundColor) != "" {
		outStyle = plainTextStyleForFlexibleBadge(b)
		wrapWidth = float64(imageW-anchor.X) - 8
		if wrapWidth < float64(anchor.Width) {
			wrapWidth = float64(anchor.Width) * 1.5
		}
		measureBBox = anchor
		measureBBox.Width = int(wrapWidth)
		for _, candidate := range candidates {
			if candidate == "" {
				continue
			}
			fs := initialFontSizeForClass(anchor, candidate, opts, blockClassTitle)
			if fs > 22 {
				fs = 22
			}
			wrapped := wrapTextToWidth(candidate, wrapWidth, fs, false, maxLines)
			for fit := textFitsBBox(wrapped, fs, measureBBox, opts); !fit && fs > readableMin; fs-- {
				wrapped = wrapTextToWidth(candidate, wrapWidth, fs, false, maxLines)
			}
			if len(wrapped) > 0 {
				return candidate, fs, wrapped, false, outStyle
			}
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
	wrapped := wrapTextToWidth(shortest, wrapWidth, fs, false, maxLines)
	if opts.AllowTextBoxExpand {
		return shortest, fs, wrapped, false, outStyle
	}
	return shortest, fs, wrapped, !textFitsBBox(wrapped, fs, measureBBox, opts), outStyle
}

func selectTranslationWithMeasure(
	b translateTextBlock,
	blockClass string,
	bbox translateTextBBox,
	opts translateLayoutOptions,
	maxLines int,
) (text string, fontSize int, lines []string, overflow bool) {
	candidates := translationCandidatesInPriority(b, blockClass)
	minFS := opts.MinFontSize
	if minFS <= 0 {
		minFS = 14
	}
	readableMin := maxInt(10, minFS-4)
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		fs := initialFontSizeForClass(bbox, candidate, opts, blockClass)
		display := candidate
		wrapped := []string{display}
		if opts.AutoWrap && bbox.Width > 0 && maxLines > 1 {
			wrapped = wrapTextToWidth(display, float64(bbox.Width), fs, false, maxLines)
		}
		for fit := textFitsBBox(wrapped, fs, bbox, opts); !fit && fs > readableMin; fs-- {
			if opts.AutoWrap && bbox.Width > 0 {
				wrapped = wrapTextToWidth(display, float64(bbox.Width), fs, false, maxLines)
			}
			fit = textFitsBBox(wrapped, fs, bbox, opts)
		}
		if textFitsBBox(wrapped, fs, bbox, opts) {
			return display, fs, wrapped, false
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
		return "", 0, nil, true
	}
	fs := readableMin
	wrapped := []string{shortest}
	if opts.AutoWrap && bbox.Width > 0 {
		wrapped = wrapTextToWidth(shortest, float64(bbox.Width), fs, false, maxLines)
	}
	if textFitsBBox(wrapped, fs, bbox, opts) {
		return shortest, fs, wrapped, false
	}
	return shortest, fs, wrapped, true
}

func translationCandidatesInPriority(b translateTextBlock, blockClass string) []string {
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
	}
	if !isBadge {
		order[1], order[2] = order[2], order[1]
	}
	seen := map[string]bool{}
	var out []string
	for _, s := range order {
		if s == "" || seen[s] {
			continue
		}
		if strings.Contains(s, "Telescopic") || s == "Universal Phone" {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
