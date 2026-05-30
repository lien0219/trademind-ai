package imagetask

import (
	"math"
	"strings"
)

const (
	layoutTemplateProductRelayout = "product_relayout"
	groupTypeTopRightBadge        = "top_right_badge"
)

func applyProductRelayoutHints(hints map[string]any) map[string]any {
	return applyTranslateRenderHints(hints)
}

func resolveLayoutTemplate(hints map[string]any, groups []translateTextGroup) string {
	tpl := layoutTemplateFromHints(hints)
	if tpl == layoutTemplateAuto {
		if looksLikeTitleBadge(groups) {
			return layoutTemplateTitleBadge
		}
		if hasTopRightBadgeGroup(groups) {
			return layoutTemplateProductRelayout
		}
		return layoutTemplateProductRelayout
	}
	return tpl
}

func isProductRelayoutTemplate(template string) bool {
	return template == layoutTemplateProductRelayout || template == layoutTemplateTitleBadge
}

func hasTopRightBadgeGroup(groups []translateTextGroup) bool {
	for _, g := range groups {
		if g.GroupType == groupTypeTopRightBadge {
			return true
		}
	}
	return false
}

func mergeProximityTextGroups(groups []translateTextGroup, imageW, imageH int) []translateTextGroup {
	if len(groups) <= 1 || imageW <= 0 || imageH <= 0 {
		return groups
	}
	var out []translateTextGroup
	used := make([]bool, len(groups))
	for i := range groups {
		if used[i] {
			continue
		}
		gi := groups[i]
		if canMergeTopRightBadge(gi, imageW, imageH) {
			for j := i + 1; j < len(groups); j++ {
				if used[j] {
					continue
				}
				gj := groups[j]
				if shouldMergeAsTopRightBadge(gi, gj, imageW, imageH) {
					merged := mergeGroupsAsTopRightBadge(gi, gj)
					out = append(out, merged)
					used[i], used[j] = true, true
					break
				}
			}
		}
		if !used[i] {
			out = append(out, gi)
			used[i] = true
		}
	}
	return out
}

func canMergeTopRightBadge(g translateTextGroup, imageW, imageH int) bool {
	if g.GroupType != groupTypeMainTitle && g.GroupType != groupTypeBadge && g.GroupType != groupTypeSubtitle {
		return false
	}
	bb := g.BBox
	return bb.X+bb.Width/2 > int(float64(imageW)*0.52) && bb.Y < int(float64(imageH)*0.42)
}

func shouldMergeAsTopRightBadge(a, b translateTextGroup, imageW, imageH int) bool {
	if !canMergeTopRightBadge(a, imageW, imageH) && !canMergeTopRightBadge(b, imageW, imageH) {
		return false
	}
	top, bottom := a, b
	if a.BBox.Y > b.BBox.Y {
		top, bottom = b, a
	}
	vGap := bottom.BBox.Y - (top.BBox.Y + top.BBox.Height)
	maxGap := maxInt(48, maxInt(top.BBox.Height, bottom.BBox.Height)*2)
	if vGap < -maxInt(top.BBox.Height, bottom.BBox.Height)/2 || vGap > maxGap {
		return false
	}
	overlapX := horizontalOverlapRatio(top.BBox, bottom.BBox)
	if overlapX < 0.25 {
		xDist := absInt((top.BBox.X + top.BBox.Width/2) - (bottom.BBox.X + bottom.BBox.Width/2))
		if xDist > maxInt(80, imageW/8) {
			return false
		}
	}
	return fontSizeSimilar(top, bottom)
}

func horizontalOverlapRatio(a, b translateTextBBox) float64 {
	left := maxInt(a.X, b.X)
	right := minInt(a.X+a.Width, b.X+b.Width)
	if right <= left {
		return 0
	}
	overlap := float64(right - left)
	minW := float64(minInt(maxInt(1, a.Width), maxInt(1, b.Width)))
	return overlap / minW
}

func fontSizeSimilar(a, b translateTextGroup) bool {
	ha := a.BBox.Height
	hb := b.BBox.Height
	if ha <= 0 || hb <= 0 {
		return true
	}
	ratio := float64(ha) / float64(hb)
	return ratio >= 0.45 && ratio <= 2.2
}

func mergeGroupsAsTopRightBadge(a, b translateTextGroup) translateTextGroup {
	top, bottom := a, b
	if a.BBox.Y > b.BBox.Y {
		top, bottom = b, a
	}
	blocks := append([]translateTextBlock{}, top.Blocks...)
	blocks = append(blocks, bottom.Blocks...)
	bb := unionTextBBox(blocks)
	lines := buildGroupTranslatedLines(blocks)
	style := titleGroupStyle()
	if isDarkLabelStyle(bottom.Style) || bottom.GroupType == groupTypeBadge {
		style = badgeGroupStyle()
	}
	return translateTextGroup{
		ID:              "group_top_right_badge",
		GroupType:       groupTypeTopRightBadge,
		Blocks:          blocks,
		BBox:            bb,
		Style:           style,
		TranslatedLines: lines,
	}
}

func buildGroupTranslatedLines(blocks []translateTextBlock) []string {
	var lines []string
	for _, b := range blocks {
		t := selectDrawTextForBlock(b)
		if t != "" {
			lines = append(lines, t)
		}
	}
	return lines
}

func erasePaddingForBlockClass(class string) int {
	switch class {
	case blockClassTitle:
		return 2
	case blockClassBadge, blockClassColorBadge, blockClassSubtitle, blockClassPill:
		return 1
	default:
		return 1
	}
}

func erasePaddingForGroup(g translateTextGroup) int {
	switch g.GroupType {
	case groupTypeMainTitle, groupTypeTopRightBadge:
		if len(g.Blocks) > 0 && strings.TrimSpace(g.Blocks[0].BlockClass) == blockClassTitle {
			return 16
		}
		return 14
	case groupTypeBadge, groupTypeBottomBadge, groupTypeColorBadge:
		return 14
	default:
		return 8
	}
}

func expandEraseBBoxWithFactor(bb translateTextBBox, factor float64, imageW, imageH int) translateTextBBox {
	if factor <= 1 || bb.Width <= 0 || bb.Height <= 0 {
		return bb
	}
	cx := bb.X + bb.Width/2
	cy := bb.Y + bb.Height/2
	nw := int(math.Round(float64(bb.Width) * factor))
	nh := int(math.Round(float64(bb.Height) * factor))
	out := translateTextBBox{
		X:      cx - nw/2,
		Y:      cy - nh/2,
		Width:  nw,
		Height: nh,
	}
	return clampGroupBBox(out, imageW, imageH)
}

func layoutTopRightBadgeBBox(g translateTextGroup, imageW, imageH int) translateTextBBox {
	bb := unionTextBBox(g.Blocks)
	if imageW > 0 {
		minW := int(float64(imageW) * 0.22)
		maxW := int(float64(imageW) * 0.42)
		bb.Width = clampInt(maxInt(bb.Width, minW), bb.Width, maxW)
	}
	if bb.Height > 0 {
		bb.Height = int(math.Round(float64(bb.Height) * 1.15))
	}
	return clampGroupBBox(bb, imageW, imageH)
}
