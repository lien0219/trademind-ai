package imagetask

import (
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/pkg/imagerender"
)

func resolveTranslateFinalStatus(
	score int,
	verify translateVerificationMeta,
	layout translateLayoutSummary,
	renderQuality translateRenderQuality,
	qualityRetried bool,
) string {
	if !verify.TargetTextDetected {
		return StatusFailedValidation
	}
	if verify.SourceTextMayRemain || verify.ProductOverlapDetected {
		return StatusFailedValidation
	}
	if score >= 85 {
		return StatusSuccess
	}
	if score >= 75 {
		return StatusSuccessWithReview
	}
	return StatusFailedValidation
}

func resolveTranslateFinalStatusForMode(
	score int,
	verify translateVerificationMeta,
	layout translateLayoutSummary,
	renderQuality translateRenderQuality,
	qualityRetried bool,
	renderMode string,
	renderBlocks []translateRenderBlock,
	renderRes *imagerender.Result,
	imageW, imageH int,
) string {
	if isPureTextReplaceMode(renderMode) {
		validation := validatePureTextReplace(verify, layout, renderQuality, renderBlocks, renderRes, imageW, imageH)
		return resolvePureTextFinalStatus(validation)
	}
	return resolveTranslateFinalStatus(score, verify, layout, renderQuality, qualityRetried)
}

func shouldQualityAutoRetry(score int, verify translateVerificationMeta, layout translateLayoutSummary, rq translateRenderQuality, renderMode string, validation pureTextValidationResult, qualityRetried bool) bool {
	if isPureTextReplaceMode(renderMode) {
		return shouldPureTextQualityAutoRetry(validation, qualityRetried)
	}
	if verify.SourceTextMayRemain || verify.ProductOverlapDetected {
		return false
	}
	if score >= 75 {
		return false
	}
	if score < 65 {
		return false
	}
	return true
}

type translateQualityRetryPlan struct {
	MaskDilateExtra      int
	ForcePillErase       bool
	UseShorterText       bool
	SecondaryInpaint     bool
	DrawTextOnlyRetry    bool
	ForcePureTextReplace bool
	ReduceFontSize       bool
}

func buildQualityRetryPlan(
	verify translateVerificationMeta,
	layout translateLayoutSummary,
	renderQuality translateRenderQuality,
) translateQualityRetryPlan {
	plan := translateQualityRetryPlan{}
	for _, w := range renderQuality.Warnings {
		switch w {
		case verifyWarningSourceTextRemain, warningEraseFailed:
			plan.MaskDilateExtra = 1
		case layoutWarningPatchVisible:
			plan.SecondaryInpaint = true
		}
	}
	if verify.SourceTextMayRemain {
		plan.MaskDilateExtra = maxInt(plan.MaskDilateExtra, 1)
	}
	if layout.OverflowBlocks > 0 {
		plan.UseShorterText = true
	}
	for _, w := range renderQuality.Warnings {
		if w == warningBadgeShapeAbnormal {
			plan.ForcePillErase = true
		}
	}
	return plan
}

func applyQualityRetryToImageBlocks(blocks []imagerender.TextBlock, plan translateQualityRetryPlan) []imagerender.TextBlock {
	out := cloneImageRenderBlocks(blocks)
	if plan.DrawTextOnlyRetry {
		for i := range out {
			out[i].MaskDilate = 0
			out[i].ErasePadding = 0
		}
		return out
	}
	for i := range out {
		if plan.MaskDilateExtra > 0 {
			out[i].MaskDilate = maxInt(1, out[i].MaskDilate+plan.MaskDilateExtra)
			if out[i].MaskDilate > 2 {
				out[i].MaskDilate = 2
			}
		}
	}
	return out
}

func applyQualityRetryToRenderBlocks(blocks []translateRenderBlock, ocr *translateOCRResult, plan translateQualityRetryPlan) []translateRenderBlock {
	out := make([]translateRenderBlock, len(blocks))
	copy(out, blocks)
	if plan.ReduceFontSize {
		for i := range out {
			if out[i].FontSize > 10 {
				out[i].FontSize = maxInt(10, int(float64(out[i].FontSize)*0.88))
			}
		}
	}
	if !plan.UseShorterText || ocr == nil {
		return out
	}
	byID := map[string]translateTextBlock{}
	for _, b := range ocr.Blocks {
		byID[b.ID] = b
	}
	for i := range out {
		src, ok := byID[out[i].ID]
		if !ok {
			continue
		}
		shorter := pickShorterTranslation(src, out[i].BlockClass)
		if shorter == "" {
			if isPureTextReplaceBlock(out[i]) {
				shorter = pickPureTextShorterFallback(src, out[i].BlockClass)
			}
			if shorter == "" {
				continue
			}
		}
		out[i].Lines = []string{shorter}
	}
	return out
}

func isPureTextReplaceBlock(b translateRenderBlock) bool {
	return b.MaskDilate > 0 || b.BlockClass != ""
}

func pickPureTextShorterFallback(b translateTextBlock, blockClass string) string {
	cands := pureTextTranslationCandidates(b, blockClass, 0)
	if len(cands) <= 1 {
		return ""
	}
	return cands[len(cands)-1]
}

func pickShorterTranslation(b translateTextBlock, blockClass string) string {
	candidates := translationCandidatesInPriority(b, blockClass)
	if len(candidates) <= 1 {
		return ""
	}
	shortest := candidates[len(candidates)-1]
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if len([]rune(c)) < len([]rune(shortest)) {
			shortest = c
		}
	}
	if strings.Contains(shortest, "Telescopic") || strings.Contains(shortest, "Universal Phone") && !strings.Contains(shortest, "Stand") {
		for _, alt := range []string{"Universal Stand", "For Phones", "Phone Stand"} {
			for _, c := range candidates {
				if c == alt {
					return alt
				}
			}
		}
	}
	return shortest
}

func isTranslateOutputUsable(status string) bool {
	switch status {
	case StatusSuccess, StatusSuccessWithWarnings, StatusSuccessWithReview:
		return true
	default:
		return false
	}
}

func isTranslateOutputSavableToProduct(status string) bool {
	return status == StatusSuccess || status == StatusSuccessWithWarnings
}
