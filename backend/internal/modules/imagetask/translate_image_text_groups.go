package imagetask

import (
	"math"
	"sort"
	"strings"
)

const (
	groupTypeMainTitle    = "main_title"
	groupTypeSubtitle     = "subtitle"
	groupTypeBadge        = "badge"
	groupTypeBottomBadge  = "bottom_badge"
	groupTypeColorBadge   = "color_badge"
	groupTypeCornerLabel  = "corner_label"
	groupTypeSmallCaption = "small_caption"
	groupTypeNormalText   = "normal_text"

	blockClassTitle        = "title"
	blockClassSubtitle     = "subtitle"
	blockClassBadge        = "badge"
	blockClassColorBadge   = "color_badge"
	blockClassSmallCaption = "small_caption"

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
	BlockClass   string
	Lines        []string
	FontSize     int
	BBox         translateTextBBox
	EraseBBox    translateTextBBox
	OriginalBBox translateTextBBox
	Style        translateTextStyle
	ErasePadding int
	Overflow     bool
	Downgraded   bool
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
		prominent := isProminentTitleCandidate(b, imageW, imageH)
		if !top || (!left && !prominent) || isDarkLabelStyle(b.Style) || looksLikeBadgeText(text) {
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
	if len(out) == 1 && isProminentTitleCandidate(blocks[out[0]], imageW, imageH) {
		return out
	}
	return nil
}

func classifyTranslateBlocks(blocks []translateTextBlock, imageW, imageH int) {
	for i := range blocks {
		blocks[i].BlockClass = classifyTranslateBlock(blocks[i], imageW, imageH)
	}
}

func classifyTranslateBlock(b translateTextBlock, imageW, imageH int) string {
	if isStrictBadgeCandidate(b, imageW, imageH) {
		if isColorBadgeText(b.Text) {
			return blockClassColorBadge
		}
		return blockClassBadge
	}
	if isProminentTitleCandidate(b, imageW, imageH) {
		return blockClassTitle
	}
	if imageH > 0 && b.BBox.Height > 0 && b.BBox.Height <= int(float64(imageH)*0.045) {
		return blockClassSmallCaption
	}
	if imageH > 0 && b.BBox.Y < int(float64(imageH)*0.34) {
		return blockClassSubtitle
	}
	return blockClassSmallCaption
}

func inferGroupType(b translateTextBlock, imageW, imageH int) string {
	text := strings.TrimSpace(b.Text)
	class := strings.TrimSpace(b.BlockClass)
	if class == "" {
		class = classifyTranslateBlock(b, imageW, imageH)
	}
	if class == blockClassBadge && (text == "手机 / 平板" || text == "手机/平板") {
		return groupTypeBadge
	}
	if (class == blockClassBadge || class == blockClassColorBadge) && (text == "牛奶白" || text == "暗夜黑") {
		return groupTypeBottomBadge
	}
	if class == blockClassBadge {
		if imageH > 0 && b.BBox.Y > int(float64(imageH)*0.58) {
			return groupTypeBottomBadge
		}
		return groupTypeBadge
	}
	if class == blockClassColorBadge {
		return groupTypeColorBadge
	}
	if class == blockClassSubtitle {
		return groupTypeSubtitle
	}
	if class == blockClassSmallCaption {
		return groupTypeSmallCaption
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
			tr = strings.TrimSpace(b.CompactTranslation)
		}
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
	origBBox := unionTextBBox(g.Blocks)
	if origBBox.Width <= 0 || origBBox.Height <= 0 {
		origBBox = g.BBox
	}
	eraseBBox := tightEraseBBoxForGroup(g, imageW, imageH)
	erasePad := 6
	downgraded := false
	blockClass := groupBlockClass(g)
	switch g.GroupType {
	case groupTypeMainTitle:
		lines = clampLines(lines, 2)
		bb = layoutMainTitleBBox(g, imageW, imageH, template)
		opts.LineHeightRatio = clampFloat(opts.LineHeightRatio, 1.05, 1.15)
		if opts.MaxFontSize > 0 {
			opts.MaxFontSize = minInt(opts.MaxFontSize, maxInt(28, imageW/18))
		}
		style = titleGroupStyle()
		erasePad = 8
	case groupTypeBadge, groupTypeBottomBadge:
		lines = []string{compactBadgeLine(g, strings.Join(lines, " / "))}
		bb = layoutBadgeBBox(g, lines[0], opts, imageW, imageH, template)
		style = badgeGroupStyle()
		erasePad = 6
	default:
		erasePad = 6
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	fontSize := fontSizeForGroup(g.GroupType, lines, bb, opts, imageW)
	overflow := false
	for attempt := 0; attempt < 4; attempt++ {
		overflow = false
		for _, line := range lines {
			if estimateTextWidth(line, fontSize, false) > float64(bb.Width) {
				overflow = true
				break
			}
		}
		if !overflow || fontSize <= opts.MinFontSize+1 {
			break
		}
		fontSize = maxInt(opts.MinFontSize, int(float64(fontSize)*0.88))
	}
	if (g.GroupType == groupTypeBadge || g.GroupType == groupTypeBottomBadge) && overflow {
		compact := compactBadgeLine(g, strings.Join(normalizeGroupLines(g), " / "))
		if compact != strings.Join(lines, " / ") && compact != "" {
			lines = []string{compact}
			fontSize = badgeFontSize(opts, imageW)
			for estimateTextWidth(compact, fontSize, false) > float64(bb.Width) && fontSize > maxInt(10, opts.MinFontSize-2) {
				fontSize--
			}
			overflow = estimateTextWidth(compact, fontSize, false) > float64(bb.Width)
		}
	}
	if (g.GroupType == groupTypeBadge || g.GroupType == groupTypeBottomBadge) && overflow {
		downgraded = true
		style = translateTextStyle{Color: defaultTranslateTextColor, Align: "left"}
		bb = origBBox
		blockClass = blockClassSmallCaption
		overflow = false
		for _, line := range lines {
			if estimateTextWidth(line, fontSize, false) > float64(bb.Width)*1.05 {
				overflow = true
				break
			}
		}
	}
	return translateGroupLayoutPlan{
		ID:           g.ID,
		GroupType:    g.GroupType,
		BlockClass:   blockClass,
		Lines:        lines,
		FontSize:     fontSize,
		BBox:         bb,
		EraseBBox:    eraseBBox,
		OriginalBBox: origBBox,
		Style:        style,
		ErasePadding: erasePad,
		Overflow:     overflow,
		Downgraded:   downgraded,
	}
}

func tightEraseBBoxForGroup(g translateTextGroup, imageW, imageH int) translateTextBBox {
	bb := unionTextBBox(g.Blocks)
	if bb.Width <= 0 || bb.Height <= 0 {
		bb = g.BBox
	}
	if g.GroupType == groupTypeMainTitle {
		bb.Width = int(math.Round(float64(bb.Width) * 1.12))
		bb.Height = int(math.Round(float64(bb.Height) * 1.18))
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
			t = strings.TrimSpace(b.CompactTranslation)
		}
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
	orig := unionTextBBox(g.Blocks)
	if orig.Width <= 0 {
		orig = g.BBox
	}
	bb := orig
	fs := badgeFontSize(opts, imageW)
	paddingX := clampInt(imageW/70, 10, 16)
	paddingY := clampInt(imageW/180, 4, 8)
	width := int(math.Ceil(estimateTextWidth(text, fs, false))) + paddingX*2
	height := int(math.Ceil(float64(fs)*1.15)) + paddingY*2
	maxH := orig.Height
	if maxH > 0 {
		maxH = int(math.Round(float64(orig.Height) * 1.25))
	}
	maxW := orig.Width
	if maxW > 0 {
		maxW = int(math.Round(float64(orig.Width) * 1.35))
	}
	bb.Width = clampInt(width, orig.Width, maxW)
	if maxH > 0 {
		bb.Height = clampInt(height, orig.Height, maxH)
	} else {
		bb.Height = maxInt(orig.Height, height)
	}
	_ = template
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
			BlockClass:   p.BlockClass,
			Lines:        append([]string(nil), p.Lines...),
			FontSize:     p.FontSize,
			BBox:         p.BBox,
			EraseBBox:    p.EraseBBox,
			OriginalBBox: p.OriginalBBox,
			Style:        p.Style,
			ErasePadding: p.ErasePadding,
		})
	}
	return out
}

type translateRenderBlock struct {
	ID           string
	GroupType    string
	BlockClass   string
	Lines        []string
	FontSize     int
	BBox         translateTextBBox
	EraseBBox    translateTextBBox
	OriginalBBox translateTextBBox
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
	return translateTextStyle{Color: "#ffffff", BackgroundColor: "#111111", FontWeight: "bold", Align: "center", BorderRadius: 0}
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
	return strings.Contains(s, "/") || strings.Contains(s, "／")
}

func isDarkLabelStyle(style translateTextStyle) bool {
	bg := strings.TrimSpace(strings.ToLower(style.BackgroundColor))
	return bg == "#111111" || bg == "#000000" || bg == "#1f1f1f"
}

func isWhiteTextStyle(style translateTextStyle) bool {
	c := strings.TrimSpace(strings.ToLower(style.Color))
	return c == "#ffffff" || c == "#fff" || c == "white"
}

func isStrictBadgeCandidate(b translateTextBlock, imageW, imageH int) bool {
	if !isDarkLabelStyle(b.Style) || !isWhiteTextStyle(b.Style) {
		return false
	}
	if b.BBox.Width <= 0 || b.BBox.Height <= 0 {
		return false
	}
	if imageH > 0 && b.BBox.Height > int(float64(imageH)*0.075) {
		return false
	}
	if b.BBox.Height > 64 {
		return false
	}
	ratio := float64(b.BBox.Width) / float64(maxInt(1, b.BBox.Height))
	return ratio >= 1.45 && ratio <= 8.5
}

func isColorBadgeText(s string) bool {
	s = strings.TrimSpace(s)
	switch s {
	case "牛奶白", "暗夜黑", "炫酷黑", "珍珠白", "曜石黑":
		return true
	default:
		return false
	}
}

func isProminentTitleCandidate(b translateTextBlock, imageW, imageH int) bool {
	text := strings.TrimSpace(b.Text)
	if text == "" || looksLikeBadgeText(text) || isDarkLabelStyle(b.Style) {
		return false
	}
	if imageH > 0 && b.BBox.Y > int(float64(imageH)*0.30) {
		return false
	}
	if imageH > 0 && b.BBox.Height >= int(float64(imageH)*0.055) {
		return true
	}
	if imageW > 0 && b.BBox.Width >= int(float64(imageW)*0.18) && len([]rune(text)) <= 8 {
		return true
	}
	return false
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
	case groupTypeColorBadge:
		return 45
	case groupTypeBottomBadge:
		return 50
	case groupTypeSmallCaption:
		return 55
	default:
		return 60
	}
}

func groupBlockClass(g translateTextGroup) string {
	switch g.GroupType {
	case groupTypeMainTitle:
		return blockClassTitle
	case groupTypeSubtitle:
		return blockClassSubtitle
	case groupTypeBadge, groupTypeBottomBadge:
		if len(g.Blocks) > 0 && strings.TrimSpace(g.Blocks[0].BlockClass) != "" {
			return g.Blocks[0].BlockClass
		}
		return blockClassBadge
	case groupTypeColorBadge:
		return blockClassColorBadge
	case groupTypeSmallCaption, groupTypeCornerLabel:
		return blockClassSmallCaption
	default:
		return blockClassSmallCaption
	}
}

func compactBadgeLine(g translateTextGroup, fallback string) string {
	for _, b := range g.Blocks {
		if s := strings.TrimSpace(b.CompactTranslation); s != "" {
			return s
		}
		if s := strings.TrimSpace(b.ShortTranslatedText); s != "" {
			return s
		}
	}
	return strings.TrimSpace(fallback)
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
