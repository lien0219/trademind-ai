package imagetask

import (
	"math"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/pkg/imagerender"
)

const ValidationModePureTextReplace = "validatePureTextReplace"

const (
	hardFailSourceTextRemain      = "source_text_remain"
	hardFailTargetTextMissing     = "target_text_missing"
	hardFailTextOverflow          = "text_overflow"
	hardFailProductOcclusion      = "product_occlusion"
	hardFailExtraBackground       = "extra_background_layer"
	hardFailTranslatedOverlapOld  = "translated_overlap_old_text"
	warningPureTextStyleLow       = "pure_text_style_low"
	warningPureTextLayoutLow      = "pure_text_layout_low"
	warningPureTextReadabilityLow = "pure_text_readability_low"
	warningPureTextTitleDrift     = "pure_text_title_drift"
	warningPureTextBadgeDrift     = "pure_text_badge_drift"
	warningPureTextFontSizeOff    = "pure_text_font_size_off"
)

type pureTextValidationResult struct {
	ValidationMode               string   `json:"validationMode"`
	SourceTextRemainDetected     bool     `json:"sourceTextRemainDetected"`
	TargetTextDetected           bool     `json:"targetTextDetected"`
	TextOverflowCount            int      `json:"textOverflowCount"`
	ProductOcclusionRatio        float64  `json:"productOcclusionRatio"`
	ExtraBackgroundLayerDetected bool     `json:"extraBackgroundLayerDetected"`
	TranslatedTextOverlapOldText bool     `json:"translatedTextOverlapOldText"`
	HardFailures                 []string `json:"hardFailures,omitempty"`
	SoftWarnings                 []string `json:"softWarnings,omitempty"`
	OverallScore                 int      `json:"overallScore"`
	HardPassed                   bool     `json:"hardPassed"`
}

func validatePureTextReplace(
	verify translateVerificationMeta,
	layout translateLayoutSummary,
	rq translateRenderQuality,
	renderBlocks []translateRenderBlock,
	renderRes *imagerender.Result,
	imageW, imageH int,
) pureTextValidationResult {
	out := pureTextValidationResult{
		ValidationMode:     ValidationModePureTextReplace,
		TargetTextDetected: verify.TargetTextDetected,
		TextOverflowCount:  layout.OverflowBlocks,
		OverallScore:       rq.CommercialUsabilityScore,
	}
	if layout.Simulation.TextOverflowCount > out.TextOverflowCount {
		out.TextOverflowCount = layout.Simulation.TextOverflowCount
	}

	out.SourceTextRemainDetected = verify.SourceTextMayRemain
	if verify.SourceTextRemainNearBox {
		out.SourceTextRemainDetected = true
	}

	out.ProductOcclusionRatio = maxProductOcclusionRatio(renderBlocks, imageW, imageH)
	out.ExtraBackgroundLayerDetected = detectPureTextExtraBackground(renderBlocks, renderRes)
	out.TranslatedTextOverlapOldText = detectTranslatedOverlapOldText(verify, layout)

	if out.SourceTextRemainDetected {
		out.HardFailures = append(out.HardFailures, hardFailSourceTextRemain)
	}
	if !out.TargetTextDetected {
		out.HardFailures = append(out.HardFailures, hardFailTargetTextMissing)
	}
	if out.TextOverflowCount > 0 {
		out.HardFailures = append(out.HardFailures, hardFailTextOverflow)
	}
	if out.ProductOcclusionRatio > 0.02 {
		out.HardFailures = append(out.HardFailures, hardFailProductOcclusion)
	}
	if out.ExtraBackgroundLayerDetected {
		out.HardFailures = append(out.HardFailures, hardFailExtraBackground)
	}
	if out.TranslatedTextOverlapOldText {
		out.HardFailures = append(out.HardFailures, hardFailTranslatedOverlapOld)
	}

	if rq.StyleConsistencyScore > 0 && rq.StyleConsistencyScore < 70 {
		out.SoftWarnings = appendUniqueCodeWarning(out.SoftWarnings, warningPureTextStyleLow)
	}
	if rq.LayoutScore > 0 && rq.LayoutScore < 70 {
		out.SoftWarnings = appendUniqueCodeWarning(out.SoftWarnings, warningPureTextLayoutLow)
	}
	if rq.ReadabilityScore > 0 && rq.ReadabilityScore < 70 {
		out.SoftWarnings = appendUniqueCodeWarning(out.SoftWarnings, warningPureTextReadabilityLow)
	}
	for _, b := range renderBlocks {
		if titlePositionDriftExceeded(b) {
			out.SoftWarnings = appendUniqueCodeWarning(out.SoftWarnings, warningPureTextTitleDrift)
			break
		}
	}
	for _, b := range renderBlocks {
		if badgePositionDriftExceeded(b) {
			out.SoftWarnings = appendUniqueCodeWarning(out.SoftWarnings, warningPureTextBadgeDrift)
			break
		}
	}
	if fontSizeSlightlyOff(renderBlocks) {
		out.SoftWarnings = appendUniqueCodeWarning(out.SoftWarnings, warningPureTextFontSizeOff)
	}

	out.HardPassed = len(out.HardFailures) == 0
	if out.OverallScore <= 0 {
		out.OverallScore = computePureTextOverallScore(rq, len(out.SoftWarnings))
	}
	return out
}

func buildPureTextRenderQuality(
	quality translateQualitySummary,
	layout translateLayoutSummary,
	verify translateVerificationMeta,
	opts translateRenderOptions,
	renderBlocks []translateRenderBlock,
	renderRes *imagerender.Result,
	imageW, imageH int,
) translateRenderQuality {
	rq := translateRenderQuality{
		TextAppliedScore:         92,
		SourceTextRemovedScore:   90,
		LayoutScore:              84,
		StyleConsistencyScore:    82,
		ReadabilityScore:         86,
		ProductPreservationScore: 90,
		Warnings:                 []string{},
	}
	if !verify.TargetTextDetected {
		rq.TextAppliedScore = 30
		rq.Warnings = appendUniqueCodeWarning(rq.Warnings, "text_not_applied")
	}
	if verify.SourceTextMayRemain || verify.SourceTextRemainNearBox {
		rq.SourceTextRemovedScore = 40
		rq.Warnings = appendUniqueCodeWarning(rq.Warnings, warningPureTextSourceNotErased)
	}
	if layout.OverflowBlocks > 0 || layout.Simulation.TextOverflowCount > 0 {
		rq.LayoutScore -= 35
		rq.ReadabilityScore -= 20
		rq.Warnings = appendUniqueCodeWarning(rq.Warnings, layoutWarningOverflow)
	}
	if maxProductOcclusionRatio(renderBlocks, imageW, imageH) > 0.02 {
		rq.ProductPreservationScore -= 28
		rq.LayoutScore -= 12
	}
	if detectPureTextExtraBackground(renderBlocks, renderRes) {
		rq.StyleConsistencyScore -= 40
		rq.Warnings = appendUniqueCodeWarning(rq.Warnings, warningPureTextExtraBackground)
	}
	if detectTranslatedOverlapOldText(verify, layout) {
		rq.LayoutScore -= 25
		rq.Warnings = appendUniqueCodeWarning(rq.Warnings, warningPureTextOverlap)
	}
	if quality.LowConfidenceBlocksCount > 0 {
		rq.SourceTextRemovedScore -= 6
	}
	if verify.Confidence > 0 && verify.Confidence < 0.7 {
		rq.TextAppliedScore -= 10
	}
	rq.TextAppliedScore = clampScore(rq.TextAppliedScore)
	rq.SourceTextRemovedScore = clampScore(rq.SourceTextRemovedScore)
	rq.LayoutScore = clampScore(rq.LayoutScore)
	rq.StyleConsistencyScore = clampScore(rq.StyleConsistencyScore)
	rq.ReadabilityScore = clampScore(rq.ReadabilityScore)
	rq.ProductPreservationScore = clampScore(rq.ProductPreservationScore)
	rq.CommercialUsabilityScore = computePureTextOverallScore(rq, 0)

	validation := validatePureTextReplace(verify, layout, rq, renderBlocks, renderRes, imageW, imageH)
	for _, w := range validation.SoftWarnings {
		rq.Warnings = appendUniqueCodeWarning(rq.Warnings, w)
	}
	for _, f := range validation.HardFailures {
		switch f {
		case hardFailSourceTextRemain:
			rq.Warnings = appendUniqueCodeWarning(rq.Warnings, warningPureTextSourceNotErased)
		case hardFailTargetTextMissing:
			rq.Warnings = appendUniqueCodeWarning(rq.Warnings, "text_not_applied")
		case hardFailTextOverflow:
			rq.Warnings = appendUniqueCodeWarning(rq.Warnings, layoutWarningOverflow)
		case hardFailExtraBackground:
			rq.Warnings = appendUniqueCodeWarning(rq.Warnings, warningPureTextExtraBackground)
		case hardFailTranslatedOverlapOld:
			rq.Warnings = appendUniqueCodeWarning(rq.Warnings, warningPureTextOverlap)
		case hardFailProductOcclusion:
			rq.Warnings = appendUniqueCodeWarning(rq.Warnings, layoutWarningProductSubjectOverlap)
		}
	}
	rq.CommercialUsabilityScore = validation.OverallScore
	rq.Passed = validation.HardPassed && validation.OverallScore >= 75
	return rq
}

func computePureTextOverallScore(rq translateRenderQuality, softWarningCount int) int {
	score := (rq.TextAppliedScore + rq.SourceTextRemovedScore + rq.LayoutScore +
		rq.StyleConsistencyScore + rq.ReadabilityScore + rq.ProductPreservationScore) / 6
	score -= softWarningCount * 3
	return clampScore(score)
}

func resolvePureTextFinalStatus(validation pureTextValidationResult) string {
	if !validation.HardPassed {
		return StatusFailedValidation
	}
	if validation.OverallScore >= 75 {
		return StatusSuccess
	}
	if validation.OverallScore >= 60 {
		return StatusSuccessWithReview
	}
	return StatusFailedValidation
}

func pureTextHardFailureFromValidation(v pureTextValidationResult) bool {
	return !v.HardPassed
}

func validatePureTextRenderStyles(blocks []translateRenderBlock) bool {
	for _, b := range blocks {
		if strings.TrimSpace(b.Style.BackgroundColor) != "" {
			return false
		}
		if b.Style.BorderRadius > 0 {
			return false
		}
	}
	return true
}

func detectPureTextExtraBackground(blocks []translateRenderBlock, renderRes *imagerender.Result) bool {
	if !validatePureTextRenderStyles(blocks) {
		return true
	}
	if detectBadgeShapeAbnormal(blocks) {
		return true
	}
	if renderRes != nil && (renderRes.LargePatchDetected || renderRes.PatchAreaRatio > 0.045) {
		return true
	}
	return false
}

func detectTranslatedOverlapOldText(verify translateVerificationMeta, layout translateLayoutSummary) bool {
	if verify.SourceTextMayRemain || verify.SourceTextRemainNearBox {
		if verify.TextOverlapDetected || layout.Simulation.CollisionCount > 0 {
			return true
		}
	}
	return false
}

func maxProductOcclusionRatio(blocks []translateRenderBlock, imageW, imageH int) float64 {
	if imageW <= 0 || imageH <= 0 {
		return 0
	}
	center := productSubjectBBox(imageW, imageH)
	maxR := 0.0
	for _, b := range blocks {
		if b.BBox.Width <= 0 || b.BBox.Height <= 0 {
			continue
		}
		plan := translateGroupLayoutPlan{
			GroupType:    b.GroupType,
			BBox:         b.BBox,
			EraseBBox:    b.EraseBBox,
			OriginalBBox: b.OriginalBBox,
			Style:        b.Style,
		}
		if !likelyProductSubjectOverlap(plan, imageW, imageH) {
			continue
		}
		r := bboxOverlapRatio(b.BBox, center)
		if r > maxR {
			maxR = r
		}
	}
	return maxR
}

func productSubjectBBox(imageW, imageH int) translateTextBBox {
	return translateTextBBox{
		X:      int(float64(imageW) * 0.28),
		Y:      int(float64(imageH) * 0.22),
		Width:  int(float64(imageW) * 0.44),
		Height: int(float64(imageH) * 0.56),
	}
}

func titlePositionDriftExceeded(b translateRenderBlock) bool {
	if b.BlockClass != blockClassTitle && b.GroupType != groupTypeMainTitle {
		return false
	}
	return positionDriftRatio(b) > 0.35
}

func badgePositionDriftExceeded(b translateRenderBlock) bool {
	if !isCapsuleBlockClassForRender(b.BlockClass) &&
		b.BlockClass != blockClassBadge && b.BlockClass != blockClassColorBadge {
		return false
	}
	return positionDriftRatio(b) > 0.20
}

func positionDriftRatio(b translateRenderBlock) float64 {
	orig := b.OriginalBBox
	if orig.Width <= 0 || orig.Height <= 0 {
		orig = b.EraseBBox
	}
	if orig.Width <= 0 || orig.Height <= 0 {
		return 0
	}
	diag := math.Sqrt(float64(orig.Width*orig.Width + orig.Height*orig.Height))
	if diag <= 0 {
		return 0
	}
	ocx := orig.X + orig.Width/2
	ocy := orig.Y + orig.Height/2
	dcx := b.BBox.X + b.BBox.Width/2
	dcy := b.BBox.Y + b.BBox.Height/2
	dx := float64(dcx - ocx)
	dy := float64(dcy - ocy)
	return math.Sqrt(dx*dx+dy*dy) / diag
}

func fontSizeSlightlyOff(blocks []translateRenderBlock) bool {
	for _, b := range blocks {
		if b.FontSize <= 0 || b.OriginalBBox.Height <= 0 {
			continue
		}
		ratio := float64(b.FontSize) / float64(b.OriginalBBox.Height)
		if ratio < 0.35 || ratio > 1.45 {
			return true
		}
	}
	return false
}

func pureTextHardFailureReasonLabel(code string) string {
	switch code {
	case hardFailSourceTextRemain:
		return "原中文残留"
	case hardFailTargetTextMissing:
		return "新英文未渲染"
	case hardFailTextOverflow:
		return "英文溢出"
	case hardFailProductOcclusion:
		return "遮挡商品主体"
	case hardFailExtraBackground:
		return "出现额外背景层"
	case hardFailTranslatedOverlapOld:
		return "英文与旧中文重叠"
	default:
		return code
	}
}

func buildPureTextQualityRetryPlan(
	verify translateVerificationMeta,
	layout translateLayoutSummary,
	renderQuality translateRenderQuality,
	validation pureTextValidationResult,
) translateQualityRetryPlan {
	plan := translateQualityRetryPlan{}
	if validation.SourceTextRemainDetected || verify.SourceTextMayRemain {
		plan.MaskDilateExtra = maxInt(plan.MaskDilateExtra, 1)
		plan.SecondaryInpaint = true
		plan.ForcePillErase = true
		return plan
	}
	if !validation.TargetTextDetected {
		plan.DrawTextOnlyRetry = true
		return plan
	}
	if validation.TextOverflowCount > 0 || layout.OverflowBlocks > 0 {
		plan.UseShorterText = true
		plan.ReduceFontSize = true
		return plan
	}
	if validation.ExtraBackgroundLayerDetected {
		plan.ForcePureTextReplace = true
		return plan
	}
	for _, w := range renderQuality.Warnings {
		switch w {
		case verifyWarningSourceTextRemain, warningPureTextSourceNotErased, warningEraseFailed:
			plan.MaskDilateExtra = maxInt(plan.MaskDilateExtra, 1)
			plan.SecondaryInpaint = true
			plan.ForcePillErase = true
		case layoutWarningOverflow:
			plan.UseShorterText = true
			plan.ReduceFontSize = true
		case warningPureTextExtraBackground, warningBadgeShapeAbnormal:
			plan.ForcePureTextReplace = true
		}
	}
	return plan
}

func shouldPureTextQualityAutoRetry(validation pureTextValidationResult, qualityRetried bool) bool {
	if qualityRetried {
		return false
	}
	if !validation.HardPassed {
		return validation.SourceTextRemainDetected ||
			!validation.TargetTextDetected ||
			validation.TextOverflowCount > 0 ||
			validation.ExtraBackgroundLayerDetected
	}
	if validation.OverallScore >= 75 {
		return false
	}
	return validation.OverallScore >= 60
}
