package imagetask

import (
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/pkg/imagerender"
)

func buildTranslateRenderQuality(
	quality translateQualitySummary,
	layout translateLayoutSummary,
	verify translateVerificationMeta,
	opts translateRenderOptions,
	renderBlocks []translateRenderBlock,
	renderRes *imagerender.Result,
) translateRenderQuality {
	rq := translateRenderQuality{
		TextAppliedScore:         95,
		SourceTextRemovedScore:   88,
		LayoutScore:              86,
		StyleConsistencyScore:    82,
		ReadabilityScore:         88,
		ProductPreservationScore: 92,
		CommercialUsabilityScore: 85,
		Warnings:                 []string{},
	}
	if !verify.TargetTextDetected {
		rq.TextAppliedScore = 35
		rq.Warnings = appendUniqueCodeWarning(rq.Warnings, "text_not_applied")
	}
	if verify.SourceTextMayRemain {
		rq.SourceTextRemovedScore = 62
		rq.Warnings = appendUniqueCodeWarning(rq.Warnings, verifyWarningSourceTextRemain)
	}
	if layout.OverflowBlocks > 0 {
		rq.LayoutScore -= 28
		rq.ReadabilityScore -= 18
		rq.Warnings = appendUniqueCodeWarning(rq.Warnings, "text_overflow")
	}
	if !renderBlocksHaveCommercialTemplate(renderBlocks) && layout.LayoutTemplate == layoutTemplateTitleBadge {
		rq.LayoutScore -= 16
		rq.Warnings = appendUniqueCodeWarning(rq.Warnings, layoutWarningNotNatural)
	}
	if strings.EqualFold(effectiveEraseMode(opts), "background_sample") && !hasBadgeRenderBlock(renderBlocks) {
		rq.StyleConsistencyScore -= 14
		rq.Warnings = appendUniqueCodeWarning(rq.Warnings, layoutWarningPatchVisible)
	}
	if renderRes != nil {
		if renderRes.LargePatchDetected || renderRes.PatchAreaRatio > 0.08 || renderRes.FlatFillRatio > 0.72 {
			rq.ProductPreservationScore -= 24
			rq.StyleConsistencyScore -= 18
			rq.Warnings = appendUniqueCodeWarning(rq.Warnings, layoutWarningPatchVisible)
		}
		if renderRes.EraseAreaRatio > 0.12 {
			rq.ProductPreservationScore -= 18
			rq.Warnings = appendUniqueCodeWarning(rq.Warnings, "erase_area_too_large")
		}
	}
	if quality.LowConfidenceBlocksCount > 0 {
		rq.SourceTextRemovedScore -= 8
		rq.ReadabilityScore -= 6
	}
	if verify.Confidence > 0 && verify.Confidence < 0.7 {
		rq.TextAppliedScore -= 12
		rq.ReadabilityScore -= 10
	}
	rq.TextAppliedScore = clampScore(rq.TextAppliedScore)
	rq.SourceTextRemovedScore = clampScore(rq.SourceTextRemovedScore)
	rq.LayoutScore = clampScore(rq.LayoutScore)
	rq.StyleConsistencyScore = clampScore(rq.StyleConsistencyScore)
	rq.ReadabilityScore = clampScore(rq.ReadabilityScore)
	rq.ProductPreservationScore = clampScore(rq.ProductPreservationScore)
	rq.CommercialUsabilityScore = clampScore((rq.TextAppliedScore + rq.SourceTextRemovedScore + rq.LayoutScore + rq.StyleConsistencyScore + rq.ReadabilityScore + rq.ProductPreservationScore) / 6)
	if rq.CommercialUsabilityScore < 70 {
		rq.Warnings = appendUniqueCodeWarning(rq.Warnings, "commercial_usability_low")
	}
	rq.Passed = rq.TextAppliedScore >= 70 &&
		rq.SourceTextRemovedScore >= 70 &&
		rq.LayoutScore >= 70 &&
		rq.StyleConsistencyScore >= 70 &&
		rq.ReadabilityScore >= 70 &&
		rq.ProductPreservationScore >= 70 &&
		rq.CommercialUsabilityScore >= 75 &&
		len(blockingRenderWarnings(rq.Warnings)) == 0
	return rq
}

func hasBadgeRenderBlock(blocks []translateRenderBlock) bool {
	for _, b := range blocks {
		if b.GroupType == groupTypeBadge || b.GroupType == groupTypeBottomBadge {
			return true
		}
	}
	return false
}

func countStyleMismatchWarnings(warnings []string) int {
	n := 0
	for _, w := range warnings {
		switch w {
		case layoutWarningPatchVisible, layoutWarningNotNatural, "erase_area_too_large":
			n++
		}
	}
	return n
}

func blockingRenderWarnings(warnings []string) []string {
	var out []string
	for _, w := range warnings {
		switch w {
		case layoutWarningPatchVisible, layoutWarningNotNatural, verifyWarningSourceTextRemain, "text_overflow", "text_not_applied", "erase_area_too_large":
			out = append(out, w)
		}
	}
	return out
}

func clampScore(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func translateRenderAttemptModes(first, requested string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(m string) {
		m = strings.TrimSpace(strings.ToLower(m))
		if m == "" || seen[m] {
			return
		}
		seen[m] = true
		out = append(out, m)
	}
	add(first)
	if requested == "" || requested == "auto" || requested == "background_sample" || requested == "precise_mask" {
		add("precise_mask")
		add("blur_fill")
		add("opencv_inpaint")
	}
	return out
}

type translateRenderAttempt struct {
	Name    string
	Blocks  []imagerender.TextBlock
	Options imagerender.Options
	Modes   []string
}

func translateRenderAttempts(blocks []imagerender.TextBlock, opts imagerender.Options, first, requested string) []translateRenderAttempt {
	modes := translateRenderAttemptModes(first, requested)
	out := []translateRenderAttempt{{
		Name:    "normal_program_layout",
		Blocks:  cloneImageRenderBlocks(blocks),
		Options: opts,
		Modes:   modes,
	}}
	tight := cloneImageRenderBlocks(blocks)
	for i := range tight {
		if tight[i].ErasePadding > 1 {
			tight[i].ErasePadding = maxInt(1, tight[i].ErasePadding/2)
		}
	}
	out = append(out, translateRenderAttempt{
		Name:    "tighter_erase_bbox",
		Blocks:  tight,
		Options: opts,
		Modes:   []string{"precise_mask", "opencv_inpaint"},
	})
	compact := cloneImageRenderBlocks(blocks)
	for i := range compact {
		if compact[i].FontSize > 10 {
			compact[i].FontSize = maxInt(10, int(float64(compact[i].FontSize)*0.90))
		}
	}
	compactOpts := opts
	if compactOpts.LineHeight <= 0 || compactOpts.LineHeight > 1.08 {
		compactOpts.LineHeight = 1.08
	}
	out = append(out, translateRenderAttempt{
		Name:    "compact_font_layout",
		Blocks:  compact,
		Options: compactOpts,
		Modes:   []string{"precise_mask", "opencv_inpaint"},
	})
	return out
}

func cloneImageRenderBlocks(blocks []imagerender.TextBlock) []imagerender.TextBlock {
	out := make([]imagerender.TextBlock, len(blocks))
	copy(out, blocks)
	for i := range out {
		out[i].Lines = append([]string(nil), blocks[i].Lines...)
	}
	return out
}

func shouldRetryTranslateRender(warnings []string) bool {
	for _, w := range warnings {
		switch w {
		case layoutWarningPatchVisible, layoutWarningNotNatural, verifyWarningSourceTextRemain, "text_overflow", "text_not_applied", "erase_area_too_large":
			return true
		}
	}
	return false
}
