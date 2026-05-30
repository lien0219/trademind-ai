package imagetask

import "testing"

func TestResolveTranslateFinalStatusTiers(t *testing.T) {
	verify := translateVerificationMeta{TargetTextDetected: true}
	layout := translateLayoutSummary{}
	rq := translateRenderQuality{CommercialUsabilityScore: 88}

	if got := resolveTranslateFinalStatus(88, verify, layout, rq, false); got != StatusSuccess {
		t.Fatalf("88 = %q, want success", got)
	}
	if got := resolveTranslateFinalStatus(78, verify, layout, rq, false); got != StatusSuccessWithReview {
		t.Fatalf("78 = %q, want success_with_review", got)
	}
	if got := resolveTranslateFinalStatus(70, verify, layout, rq, true); got != StatusFailedValidation {
		t.Fatalf("70 after retry = %q, want failed_render_validation", got)
	}
	verify.SourceTextMayRemain = true
	if got := resolveTranslateFinalStatus(90, verify, layout, rq, false); got != StatusFailedValidation {
		t.Fatalf("source remain = %q, want failed_render_validation", got)
	}
}

func TestShouldQualityAutoRetry(t *testing.T) {
	verify := translateVerificationMeta{TargetTextDetected: true}
	layout := translateLayoutSummary{}
	rq := translateRenderQuality{}
	emptyValidation := pureTextValidationResult{HardPassed: true, OverallScore: 70}
	if !shouldQualityAutoRetry(70, verify, layout, rq, RenderModeRemoveTextThenRender, emptyValidation, false) {
		t.Fatal("score 70 should auto retry for relayout mode")
	}
	if shouldQualityAutoRetry(76, verify, layout, rq, RenderModeRemoveTextThenRender, emptyValidation, false) {
		t.Fatal("score 76 should not auto retry")
	}
	if shouldQualityAutoRetry(60, verify, layout, rq, RenderModeRemoveTextThenRender, emptyValidation, false) {
		t.Fatal("score 60 should not auto retry")
	}
	pureSoft := pureTextValidationResult{HardPassed: true, OverallScore: 68, TargetTextDetected: true}
	if !shouldPureTextQualityAutoRetry(pureSoft, false) {
		t.Fatal("pure text score 68 should allow retry")
	}
}

func TestTranslationBadgeDowngradeAlternates(t *testing.T) {
	b := translateTextBlock{
		Text:                  "折叠伸缩版/通用手机",
		FixedShortTranslation: "Universal Phone Stand",
		BadgeTranslation:      "Universal Phone Stand",
		CompactTranslation:    "Universal Stand",
		BlockClass:            blockClassBadge,
	}
	cands := translationCandidatesInPriority(b, blockClassBadge)
	foundStand := false
	for _, c := range cands {
		if c == "Universal Stand" {
			foundStand = true
		}
		if c == "Foldable Telescopic Version / Universal Phone" {
			t.Fatalf("long literal should not appear: %q", c)
		}
	}
	if !foundStand {
		t.Fatalf("missing Universal Stand in %v", cands)
	}
}
