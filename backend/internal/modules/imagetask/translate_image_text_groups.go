package imagetask

import (
	"math"
	"sort"
	"strings"
)

const (
	groupTypeMainTitle   = "main_title"
	groupTypeSubtitle    = "subtitle"
	groupTypeBadge       = "badge"
	groupTypeBottomBadge = "bottom_badge"
	groupTypeCornerLabel = "corner_label"
	groupTypeNormalText  = "normal_text"

	layoutTemplatePreserveOriginal = "preserve_original"
	layoutTemplateEcommerceClean   = "ecommerce_clean"
	layoutTemplateTitleBadge       = "title_badge"
	layoutTemplateAuto             = "auto"

	layoutWarningPatchVisible = "background_patch_visible"
	layoutWarningNotNatural   = "layout_not_natural"
)

type translateTextGroup struct {
	ID              string               `json:"id,omitempty"`
	GroupType       string               `json:"groupType"`
	Blocks          []translateTextBlock `json:"blocks"`
	BBox            translateTextBBox    `json:"bbox"`
	Style           translateTextStyle   `json:"style,omitempty"`
	TranslatedLines []string             `json:"translatedLines"`
}

type translateGroupLayoutPlan struct {
	ID           string
	GroupType    string
	Lines        []string
	FontSize     int
	BBox         translateTextBBox
	EraseBBox    translateTextBBox
	Style        translateTextStyle
	ErasePadding int
	Overflow     bool
}

func layoutTemplateFromHints(hints map[string]any) string {
	tpl := strings.TrimSpace(strings.ToLower(stringFromMap(hints, "layoutTemplate")))
	switch tpl {
	case layoutTemplatePreserveOriginal, layoutTemplateEcommerceClean, layoutTemplateTitleBadge:
		return tpl
	case "", layoutTemplateAuto:
		return layoutTemplateAuto
	default:
		return layoutTemplateAuto
	}
}

func buildTranslateTextGroups(blocks []translateTextBlock, hints map[string]any, imageW, imageH int) ([]translateTextGroup, string) {
	if len(blocks) == 0 {
		return nil, layoutTemplateAuto
	}
	tpl := layoutTemplateFromHints(hints)
	sorted := append([]translateTextBlock(nil), blocks...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if absInt(sorted[i].BBox.Y-sorted[j].BBox.Y) > maxInt(8, minPositive(sorted[i].BBox.Height, sorted[j].BBox.Height)/2) {
			return sorted[i].BBox.Y < sorted[j].BBox.Y
		}
		return sorted[i].BBox.X < sorted[j].BBox.X
	})

	used := make([]bool, len(sorted))
	var groups []translateTextGroup

	mainIdx := findMainTitleIndexes(sorted, imageW, imageH)
	if len(mainIdx) > 0 {
		var bs []translateTextBlock
		for _, idx := range mainIdx {
			used[idx] = true
			bs = append(bs, sorted[idx])
		}
		groups = append(groups, makeTranslateGroup("group_main_title", groupTypeMainTitle, bs, titleGroupStyle()))
	}

	for i, b := range sorted {
		if used[i] {
			continue
		}
		gt := inferGroupType(b, imageW, imageH)
		groups = append(groups, makeTranslateGroup(formatGroupID(len(groups)+1), gt, []translateTextBlock{b}, groupStyleForType(gt, b.Style)))
		used[i] = true
	}

	if tpl == layoutTemplateAuto && looksLikeTitleBadge(groups) {
		tpl = layoutTemplateTitleBadge
	}
	if tpl == layoutTemplateAuto {
		tpl = layoutTemplatePreserveOriginal
	}
	sort.SliceStable(groups, func(i, j int) bool {
		return groupOrder(groups[i].GroupType) < groupOrder(groups[j].GroupType)
	})
	for i := range groups {
		if strings.TrimSpace(groups[i].ID) == "" {
			groups[i].ID = formatGroupID(i + 1)
		}
	}
	return groups, tpl
}

func findMainTitleIndexes(blocks []translateTextBlock, imageW, imageH int) []int {
	var out []int
	for i, b := range blocks {
		text := strings.TrimSpace(b.Text)
		top := imageH <= 0 || b.BBox.Y < int(float64(imageH)*0.42)
		left := imageW <= 0 || b.BBox.X < int(float64(imageW)*0.55)
		if !top || !left || isDarkLabelStyle(b.Style) || looksLikeBadgeText(text) {
			continue
		}
		if text == "金属底座" || text == "折叠支架" {
			out = append(out, i)
			continue
		}
		if len(out) == 0 {
			out = append(out, i)
			continue
		}
		prev := blocks[out[len(out)-1]]
		gap := b.BBox.Y - (prev.BBox.Y + prev.BBox.Height)
		xAligned := absInt(b.BBox.X-prev.BBox.X) <= maxInt(24, prev.BBox.Width/3)
		closeY := gap >= -maxInt(prev.BBox.Height, b.BBox.Height)/2 && gap <= maxInt(36, maxInt(prev.BBox.Height, b.BBox.Height)*2)
		if xAligned && closeY {
			out = append(out, i)
		}
		if len(out) >= 2 {
			break
		}
	}
	if len(out) >= 2 {
		return out
	}
	return nil
}

func inferGroupType(b translateTextBlock, imageW, imageH int) string {
	text := strings.TrimSpace(b.Text)
	if text == "手机 / 平板" || text == "手机/平板" {
		return groupTypeBadge
	}
	if text == "暗夜黑" {
		return groupTypeBottomBadge
	}
	if isDarkLabelStyle(b.Style) || looksLikeBadgeText(text) {
		if imageH > 0 && b.BBox.Y > int(float64(imageH)*0.58) {
			return groupTypeBottomBadge
		}
		return groupTypeBadge
	}
	if imageH > 0 && b.BBox.Y < int(float64(imageH)*0.18) {
		return groupTypeCornerLabel
	}
	return groupTypeNormalText
}

func makeTranslateGroup(id, gt string, blocks []translateTextBlock, style translateTextStyle) translateTextGroup {
	bb := unionTextBBox(blocks)
	lines := make([]string, 0, len(blocks))
	for _, b := range blocks {
		tr := strings.TrimSpace(b.DrawText)
		if tr == "" {
			tr = strings.TrimSpace(b.ShortTranslatedText)
		}
		if tr == "" {
			tr = strings.TrimSpace(b.TranslatedText)
		}
		if tr != "" {
			lines = append(lines, tr)
		}
	}
	return translateTextGroup{ID: id, GroupType: gt, Blocks: blocks, BBox: bb, Style: style, TranslatedLines: lines}
}

func computeTranslateGroupLayouts(groups []translateTextGroup, opts translateLayoutOptions, imageW, imageH int, template string) ([]translateGroupLayoutPlan, translateLayoutSummary) {
	summary := translateLayoutSummary{
		AutoLayout:      opts.AutoLayout,
		TextBlocksCount: len(groups),
		MinFontSizeUsed: opts.MaxFontSize,
		Warnings:        []string{},
		LayoutTemplate:  template,
	}
	plans := make([]translateGroupLayoutPlan, 0, len(groups))
	for _, g := range groups {
		plan := layoutTranslateGroup(g, opts, imageW, imageH, template)
		if plan.Overflow {
			summary.OverflowBlocks++
			summary.Warnings = appendUniqueCodeWarning(summary.Warnings, layoutWarningOverflow)
		}
		if plan.FontSize > 0 && (summary.MinFontSizeUsed == 0 || plan.FontSize < summary.MinFontSizeUsed) {
			summary.MinFontSizeUsed = plan.FontSize
		}
		plans = append(plans, plan)
	}
	if summary.MinFontSizeUsed <= 0 || summary.MinFontSizeUsed > opts.MaxFontSize {
		summary.MinFontSizeUsed = opts.MinFontSize
	}
	if template == layoutTemplateTitleBadge {
		summary.Warnings = appendUniqueCodeWarning(summary.Warnings, "style_inherited")
	}
	return plans, summary
}

func layoutTranslateGroup(g translateTextGroup, opts translateLayoutOptions, imageW, imageH int, template string) translateGroupLayoutPlan {
	style := groupStyleForType(g.GroupType, g.Style)
	lines := normalizeGroupLines(g)
	bb := g.BBox
	eraseBBox := tightEraseBBoxForGroup(g, imageW, imageH)
	erasePad := 6
	switch g.GroupType {
	case groupTypeMainTitle:
		lines = clampLines(lines, 2)
		bb = layoutMainTitleBBox(g, imageW, imageH, template)
		opts.LineHeightRatio = clampFloat(opts.LineHeightRatio, 1.05, 1.15)
		if opts.MaxFontSize > 0 {
			opts.MaxFontSize = minInt(opts.MaxFontSize, maxInt(28, imageW/18))
		}
		style = titleGroupStyle()
		erasePad = 2
	case groupTypeBadge, groupTypeBottomBadge:
		lines = []string{strings.Join(lines, " / ")}
		bb = layoutBadgeBBox(g, lines[0], opts, imageW, imageH, template)
		style = badgeGroupStyle()
		erasePad = 2
	default:
		erasePad = 2
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	fontSize := fontSizeForGroup(g.GroupType, lines, bb, opts, imageW)
	overflow := false
	for _, line := range lines {
		if estimateTextWidth(line, fontSize, false) > float64(bb.Width) {
			overflow = true
		}
	}
	return translateGroupLayoutPlan{
		ID:           g.ID,
		GroupType:    g.GroupType,
		Lines:        lines,
		FontSize:     fontSize,
		BBox:         bb,
		EraseBBox:    eraseBBox,
		Style:        style,
		ErasePadding: erasePad,
		Overflow:     overflow,
	}
}

func tightEraseBBoxForGroup(g translateTextGroup, imageW, imageH int) translateTextBBox {
	bb := unionTextBBox(g.Blocks)
	if bb.Width <= 0 || bb.Height <= 0 {
		bb = g.BBox
	}
	if g.GroupType == groupTypeMainTitle {
		bb.Width = int(math.Round(float64(bb.Width) * 1.03))
		bb.Height = int(math.Round(float64(bb.Height) * 1.05))
	}
	return clampGroupBBox(bb, imageW, imageH)
}

func normalizeGroupLines(g translateTextGroup) []string {
	if len(g.TranslatedLines) > 0 {
		return append([]string(nil), g.TranslatedLines...)
	}
	var out []string
	for _, b := range g.Blocks {
		t := strings.TrimSpace(b.DrawText)
		if t == "" {
			t = strings.TrimSpace(b.ShortTranslatedText)
		}
		if t == "" {
			t = strings.TrimSpace(b.TranslatedText)
		}
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func layoutMainTitleBBox(g translateTextGroup, imageW, imageH int, template string) translateTextBBox {
	bb := g.BBox
	if template == layoutTemplateTitleBadge && imageW > 0 {
		minW := int(float64(imageW) * 0.26)
		maxW := int(float64(imageW) * 0.46)
		bb.Width = clampInt(maxInt(bb.Width, minW), bb.Width, maxW)
	}
	if len(g.Blocks) >= 2 {
		bb.Height = unionTextBBox(g.Blocks).Height
	}
	if bb.Height > 0 {
		bb.Height = int(math.Round(float64(bb.Height) * 1.18))
	}
	return clampGroupBBox(bb, imageW, imageH)
}

func layoutBadgeBBox(g translateTextGroup, text string, opts translateLayoutOptions, imageW, imageH int, template string) translateTextBBox {
	bb := g.BBox
	fs := badgeFontSize(opts, imageW)
	paddingX := clampInt(imageW/70, 10, 16)
	paddingY := clampInt(imageW/180, 4, 8)
	width := int(math.Ceil(estimateTextWidth(text, fs, false))) + paddingX*2
	height := int(math.Ceil(float64(fs)*1.15)) + paddingY*2
	bb.Width = maxInt(bb.Width, width)
	bb.Height = maxInt(bb.Height, height)
	if template == layoutTemplateTitleBadge && g.GroupType == groupTypeBadge {
		main := g.BBox
		bb.X = main.X
		bb.Y = main.Y
	}
	return clampGroupBBox(bb, imageW, imageH)
}

func fontSizeForGroup(gt string, lines []string, bb translateTextBBox, opts translateLayoutOptions, imageW int) int {
	switch gt {
	case groupTypeMainTitle:
		fs := int(math.Floor(float64(bb.Height) / (float64(maxInt(1, len(lines))) * 1.10)))
		if imageW > 0 {
			fs = minInt(fs, imageW/18)
		}
		return clampInt(fs, maxInt(opts.MinFontSize, 20), minPositive(opts.MaxFontSize, maxInt(28, imageW/18)))
	case groupTypeBadge, groupTypeBottomBadge:
		return badgeFontSize(opts, imageW)
	default:
		return clampInt(estimateInitialFontSize(bb, strings.Join(lines, " "), opts), opts.MinFontSize, opts.MaxFontSize)
	}
}

func badgeFontSize(opts translateLayoutOptions, imageW int) int {
	size := 22
	if imageW > 0 {
		size = clampInt(imageW/36, 18, 28)
	}
	if opts.MaxFontSize > 0 {
		size = minInt(size, opts.MaxFontSize)
	}
	if opts.MinFontSize > 0 {
		size = maxInt(size, minInt(opts.MinFontSize, 18))
	}
	return size
}

func applyGroupPlansToOCR(groups []translateTextGroup, plans []translateGroupLayoutPlan, ocr *translateOCRResult) {
	if ocr == nil {
		return
	}
	for gi := range groups {
		if gi >= len(plans) {
			break
		}
		groups[gi].BBox = plans[gi].BBox
		groups[gi].Style = plans[gi].Style
		groups[gi].TranslatedLines = append([]string(nil), plans[gi].Lines...)
		for _, gb := range groups[gi].Blocks {
			for bi := range ocr.Blocks {
				if ocr.Blocks[bi].ID == gb.ID {
					ocr.Blocks[bi].DrawText = strings.Join(plans[gi].Lines, "\n")
					break
				}
			}
		}
	}
	ocr.Groups = groups
}

func buildRenderBlocksFromGroups(groups []translateTextGroup, plans []translateGroupLayoutPlan) []translateRenderBlock {
	out := make([]translateRenderBlock, 0, len(plans))
	for i, p := range plans {
		if len(p.Lines) == 0 {
			continue
		}
		id := p.ID
		if id == "" && i < len(groups) {
			id = groups[i].ID
		}
		out = append(out, translateRenderBlock{
			ID:           id,
			GroupType:    p.GroupType,
			Lines:        append([]string(nil), p.Lines...),
			FontSize:     p.FontSize,
			BBox:         p.BBox,
			EraseBBox:    p.EraseBBox,
			Style:        p.Style,
			ErasePadding: p.ErasePadding,
		})
	}
	return out
}

type translateRenderBlock struct {
	ID           string
	GroupType    string
	Lines        []string
	FontSize     int
	BBox         translateTextBBox
	EraseBBox    translateTextBBox
	Style        translateTextStyle
	ErasePadding int
}

func renderBlocksHaveCommercialTemplate(blocks []translateRenderBlock) bool {
	var title, badge, bottom bool
	for _, b := range blocks {
		switch b.GroupType {
		case groupTypeMainTitle:
			title = true
		case groupTypeBadge:
			badge = true
		case groupTypeBottomBadge:
			bottom = true
		}
	}
	return title && badge && bottom
}

func unionTextBBox(blocks []translateTextBlock) translateTextBBox {
	if len(blocks) == 0 {
		return translateTextBBox{}
	}
	minX, minY := blocks[0].BBox.X, blocks[0].BBox.Y
	maxX, maxY := blocks[0].BBox.X+blocks[0].BBox.Width, blocks[0].BBox.Y+blocks[0].BBox.Height
	for _, b := range blocks[1:] {
		minX = minInt(minX, b.BBox.X)
		minY = minInt(minY, b.BBox.Y)
		maxX = maxInt(maxX, b.BBox.X+b.BBox.Width)
		maxY = maxInt(maxY, b.BBox.Y+b.BBox.Height)
	}
	return translateTextBBox{X: minX, Y: minY, Width: maxInt(1, maxX-minX), Height: maxInt(1, maxY-minY)}
}

func titleGroupStyle() translateTextStyle {
	return translateTextStyle{Color: "#111111", FontWeight: "bold", Align: "left"}
}

func badgeGroupStyle() translateTextStyle {
	return translateTextStyle{Color: "#ffffff", BackgroundColor: "#111111", FontWeight: "bold", Align: "center", BorderRadius: 999}
}

func groupStyleForType(gt string, fallback translateTextStyle) translateTextStyle {
	switch gt {
	case groupTypeMainTitle:
		return titleGroupStyle()
	case groupTypeBadge, groupTypeBottomBadge:
		return badgeGroupStyle()
	default:
		if strings.TrimSpace(fallback.Color) == "" {
			fallback.Color = defaultTranslateTextColor
		}
		if strings.TrimSpace(fallback.Align) == "" {
			fallback.Align = "left"
		}
		return fallback
	}
}

func looksLikeTitleBadge(groups []translateTextGroup) bool {
	var title, badge, bottom bool
	for _, g := range groups {
		switch g.GroupType {
		case groupTypeMainTitle:
			title = true
		case groupTypeBadge:
			badge = true
		case groupTypeBottomBadge:
			bottom = true
		}
	}
	return title && badge && bottom
}

func looksLikeBadgeText(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	return strings.Contains(s, "/") || strings.Contains(s, "色") || strings.Contains(s, "黑") || strings.Contains(s, "white") || strings.Contains(s, "black")
}

func isDarkLabelStyle(style translateTextStyle) bool {
	bg := strings.TrimSpace(strings.ToLower(style.BackgroundColor))
	return bg == "#111111" || bg == "#000000" || bg == "#1f1f1f"
}

func clampGroupBBox(bb translateTextBBox, imageW, imageH int) translateTextBBox {
	if bb.Width < 1 {
		bb.Width = 1
	}
	if bb.Height < 1 {
		bb.Height = 1
	}
	if bb.X < 0 {
		bb.X = 0
	}
	if bb.Y < 0 {
		bb.Y = 0
	}
	if imageW > 0 && bb.X+bb.Width > imageW {
		bb.Width = maxInt(1, imageW-bb.X)
	}
	if imageH > 0 && bb.Y+bb.Height > imageH {
		bb.Height = maxInt(1, imageH-bb.Y)
	}
	return bb
}

func groupOrder(gt string) int {
	switch gt {
	case groupTypeMainTitle:
		return 10
	case groupTypeSubtitle:
		return 20
	case groupTypeBadge:
		return 30
	case groupTypeCornerLabel:
		return 40
	case groupTypeBottomBadge:
		return 50
	default:
		return 60
	}
}

func clampLines(lines []string, maxLines int) []string {
	if maxLines <= 0 || len(lines) <= maxLines {
		return lines
	}
	return lines[:maxLines]
}

func formatGroupID(n int) string {
	return "group_" + itoa(n)
}

func minPositive(a, b int) int {
	if a <= 0 {
		return b
	}
	if b <= 0 {
		return a
	}
	return minInt(a, b)
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func clampInt(v, minV, maxV int) int {
	if maxV > 0 && v > maxV {
		v = maxV
	}
	if v < minV {
		v = minV
	}
	return v
}

func clampFloat(v, minV, maxV float64) float64 {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}
