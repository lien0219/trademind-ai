package imagetask

import (
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/pkg/imagerender"
)

type textAdaptResult struct {
	Lines       []string
	FontSize    int
	Overflow    bool
	Wrapped     bool
	FontResized bool
	UsedShort   bool
	DisplayText string
}

func adaptTextToBBox(text, shortText string, bb translateTextBBox, opts translateLayoutOptions, template string, blockClass string) textAdaptResult {
	text = strings.TrimSpace(text)
	if text == "" {
		return textAdaptResult{Overflow: true}
	}
	cjk := isCJKLang(opts.TargetLang) || mostlyCJK(text)
	maxLines := opts.MaxLines
	if maxLines <= 0 {
		maxLines = 3
	}
	switch blockClass {
	case blockClassTitle:
		maxLines = minInt(maxLines, 2)
	case blockClassBadge, blockClassColorBadge:
		maxLines = 1
	}
	fontSize := initialFontSizeForClass(bb, text, opts, blockClass)
	minFS := opts.MinFontSize
	if minFS <= 0 {
		minFS = 14
	}
	display := text
	if short := strings.TrimSpace(shortText); short != "" && short != text && opts.AllowTextSimplify {
		display = short
	}
	lines := []string{display}
	if opts.AutoWrap && bb.Width > 0 {
		lines = wrapTextToWidth(display, float64(bb.Width), fontSize, cjk, maxLines)
	}
	res := textAdaptResult{
		Lines:       lines,
		FontSize:    fontSize,
		DisplayText: display,
	}
	if len(lines) > 1 {
		res.Wrapped = true
	}
	if opts.AutoFontSize {
		for !textFitsBBox(lines, fontSize, bb, opts) && fontSize > minFS {
			fontSize--
			res.FontResized = true
			if opts.AutoWrap {
				lines = wrapTextToWidth(display, float64(bb.Width), fontSize, cjk, maxLines)
			}
		}
	}
	res.FontSize = fontSize
	res.Lines = lines
	if !textFitsBBox(lines, fontSize, bb, opts) {
		if opts.AllowTextBoxExpand {
			return res
		}
		res.Overflow = true
	}
	if res.Overflow && strings.TrimSpace(shortText) != "" && shortText != display {
		res.UsedShort = true
		res.DisplayText = shortText
		res.Lines = []string{shortText}
		if opts.AutoWrap {
			res.Lines = wrapTextToWidth(shortText, float64(bb.Width), fontSize, cjk, maxLines)
		}
		res.Overflow = !textFitsBBox(res.Lines, fontSize, bb, opts)
	}
	return res
}

func initialFontSizeForClass(bb translateTextBBox, text string, opts translateLayoutOptions, blockClass string) int {
	switch blockClass {
	case blockClassTitle:
		fs := estimateInitialFontSize(bb, text, opts)
		if fs < 20 {
			fs = 20
		}
		return fs
	case blockClassBadge, blockClassColorBadge:
		return clampInt(estimateInitialFontSize(bb, text, opts), 18, badgeFontSize(opts, bb.Width*3))
	default:
		return estimateInitialFontSize(bb, text, opts)
	}
}

func textFitsBBox(lines []string, fontSize int, bb translateTextBBox, opts translateLayoutOptions) bool {
	if bb.Width <= 0 || bb.Height <= 0 {
		return true
	}
	for _, ln := range lines {
		w := measureTextWidth(ln, fontSize, mostlyCJK(ln))
		if w > float64(bb.Width)*1.02 {
			return false
		}
	}
	h := lineBlockHeight(len(lines), fontSize, opts.LineHeightRatio)
	return h <= float64(bb.Height)*1.06
}

func measureTextWidth(text string, fontSize int, cjk bool) float64 {
	if w, err := imagerender.MeasureTextWidth(text, fontSize, cjk); err == nil && w > 0 {
		return w
	}
	return estimateTextWidth(text, fontSize, cjk)
}

func adaptGroupLines(g translateTextGroup, bb translateTextBBox, opts translateLayoutOptions, template string) ([]string, int, bool, bool) {
	blockClass := groupBlockClass(g)
	switch g.GroupType {
	case groupTypeMainTitle, groupTypeTopRightBadge:
		if len(g.Blocks) >= 2 && g.GroupType == groupTypeTopRightBadge {
			var lines []string
			fs := initialFontSizeForClass(bb, "", opts, blockClassTitle)
			overflow := false
			for _, b := range g.Blocks {
				text := selectTranslationVersion(b, classifyBlockForGroup(b, g), bb.Width, 1)
				adapted := adaptTextToBBox(text, b.CompactTranslation, translateTextBBox{X: 0, Y: 0, Width: bb.Width, Height: bb.Height / maxInt(1, len(g.Blocks))}, opts, template, classifyBlockForGroup(b, g))
				lines = append(lines, adapted.Lines...)
				if adapted.FontSize > 0 && (fs == 0 || adapted.FontSize < fs) {
					fs = adapted.FontSize
				}
				overflow = overflow || adapted.Overflow
			}
			return clampLines(lines, 2), fs, overflow, false
		}
		fallthrough
	case groupTypeBadge, groupTypeBottomBadge:
		text := compactBadgeLine(g, "")
		if text == "" && len(g.Blocks) > 0 {
			text = selectTranslationVersion(g.Blocks[0], blockClassBadge, bb.Width, 1)
		}
		adapted := adaptTextToBBox(text, text, bb, opts, template, blockClassBadge)
		return adapted.Lines, adapted.FontSize, adapted.Overflow, adapted.UsedShort
	default:
		text := strings.Join(normalizeGroupLines(g), " ")
		if text == "" && len(g.Blocks) > 0 {
			text = selectTranslationVersion(g.Blocks[0], blockClass, bb.Width, opts.MaxLines)
		}
		adapted := adaptTextToBBox(text, g.Blocks[0].CompactTranslation, bb, opts, template, blockClass)
		return adapted.Lines, adapted.FontSize, adapted.Overflow, adapted.UsedShort
	}
}

func classifyBlockForGroup(b translateTextBlock, g translateTextGroup) string {
	if strings.TrimSpace(b.BlockClass) != "" {
		return b.BlockClass
	}
	if g.GroupType == groupTypeTopRightBadge {
		if isColorBadgeText(b.Text) || isDarkLabelStyle(b.Style) {
			return blockClassBadge
		}
		return blockClassTitle
	}
	return blockClassSmallCaption
}
