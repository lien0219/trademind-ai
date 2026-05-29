package imagetask

import "math"

const (
	layoutWarningProductSubjectOverlap = "product_subject_overlap"
)

type translateLayoutSimulation struct {
	TextOverflowCount   int      `json:"textOverflowCount,omitempty"`
	CollisionCount      int      `json:"collisionCount,omitempty"`
	ProductOverlapCount int      `json:"productOverlapCount,omitempty"`
	AbnormalBadgeCount  int      `json:"abnormalBadgeCount,omitempty"`
	OverlapScore        float64  `json:"overlapScore,omitempty"`
	LayoutUnbalanced    bool     `json:"layoutUnbalanced,omitempty"`
	Warnings            []string `json:"warnings,omitempty"`
}

func simulateTranslateGroupLayouts(plans []translateGroupLayoutPlan, imageW, imageH int) translateLayoutSimulation {
	var sim translateLayoutSimulation
	for i, p := range plans {
		if p.Overflow || textLinesOverflow(p.Lines, p.FontSize, p.BBox) {
			sim.TextOverflowCount++
			sim.Warnings = appendUniqueCodeWarning(sim.Warnings, layoutWarningOverflow)
		}
		if isBadgePlan(p) && badgeExceedsHardLimit(p) {
			sim.AbnormalBadgeCount++
			sim.Warnings = appendUniqueCodeWarning(sim.Warnings, warningBadgeShapeAbnormal)
		}
		if likelyProductSubjectOverlap(p, imageW, imageH) {
			sim.ProductOverlapCount++
			sim.Warnings = appendUniqueCodeWarning(sim.Warnings, layoutWarningProductSubjectOverlap)
		}
		for j := i + 1; j < len(plans); j++ {
			if bboxOverlapRatio(p.BBox, plans[j].BBox) > 0.02 {
				sim.CollisionCount++
			}
		}
	}
	if sim.CollisionCount > 0 {
		sim.Warnings = appendUniqueCodeWarning(sim.Warnings, warningTextOverlap)
	}
	if layoutLooksUnbalanced(plans, imageW, imageH) {
		sim.LayoutUnbalanced = true
		sim.Warnings = appendUniqueCodeWarning(sim.Warnings, layoutWarningUnbalanced)
	}
	total := maxInt(1, len(plans))
	sim.OverlapScore = math.Min(1, float64(sim.CollisionCount+sim.ProductOverlapCount)/float64(total))
	return sim
}

func textLinesOverflow(lines []string, fontSize int, bb translateTextBBox) bool {
	if bb.Width <= 0 || bb.Height <= 0 || fontSize <= 0 {
		return false
	}
	for _, line := range lines {
		if estimateTextWidth(line, fontSize, mostlyCJK(line)) > float64(bb.Width)*1.04 {
			return true
		}
	}
	return lineBlockHeight(len(lines), fontSize, 1.15) > float64(bb.Height)*1.08
}

func isBadgePlan(p translateGroupLayoutPlan) bool {
	if p.GroupType != groupTypeBadge && p.GroupType != groupTypeBottomBadge {
		return false
	}
	return isDarkLabelStyle(p.Style)
}

func badgeExceedsHardLimit(p translateGroupLayoutPlan) bool {
	orig := p.OriginalBBox
	if orig.Width <= 0 || orig.Height <= 0 {
		orig = p.EraseBBox
	}
	if orig.Width <= 0 || orig.Height <= 0 {
		return false
	}
	maxW := int(math.Round(float64(orig.Width) * 1.35))
	maxH := int(math.Round(float64(orig.Height) * 1.25))
	return p.BBox.Width > maxW || p.BBox.Height > maxH
}

func likelyProductSubjectOverlap(p translateGroupLayoutPlan, imageW, imageH int) bool {
	if imageW <= 0 || imageH <= 0 || p.BBox.Width <= 0 || p.BBox.Height <= 0 {
		return false
	}
	if bboxOverlapRatio(p.BBox, p.EraseBBox) >= 0.18 {
		return false
	}
	center := translateTextBBox{
		X:      int(float64(imageW) * 0.28),
		Y:      int(float64(imageH) * 0.22),
		Width:  int(float64(imageW) * 0.44),
		Height: int(float64(imageH) * 0.56),
	}
	return bboxOverlapRatio(p.BBox, center) > 0.04
}

func layoutLooksUnbalanced(plans []translateGroupLayoutPlan, imageW, imageH int) bool {
	if len(plans) < 2 || imageW <= 0 || imageH <= 0 {
		return false
	}
	var area int
	for _, p := range plans {
		if p.BBox.Width <= 0 || p.BBox.Height <= 0 {
			continue
		}
		area += p.BBox.Width * p.BBox.Height
	}
	return float64(area)/float64(maxInt(1, imageW*imageH)) > 0.16
}

func bboxOverlapRatio(a, b translateTextBBox) float64 {
	ax1, ay1 := a.X+a.Width, a.Y+a.Height
	bx1, by1 := b.X+b.Width, b.Y+b.Height
	x0 := maxInt(a.X, b.X)
	y0 := maxInt(a.Y, b.Y)
	x1 := minInt(ax1, bx1)
	y1 := minInt(ay1, by1)
	if x1 <= x0 || y1 <= y0 {
		return 0
	}
	inter := (x1 - x0) * (y1 - y0)
	minArea := minInt(maxInt(1, a.Width*a.Height), maxInt(1, b.Width*b.Height))
	return float64(inter) / float64(minArea)
}
