package imagerender

import "testing"

func TestPerBlockEraseImageLimitAdaptive(t *testing.T) {
	imageArea := 900 * 900
	regionArea := 500 * 90
	limit := perBlockEraseImageLimit(regionArea, imageArea)
	want := float64(regionArea) / float64(imageArea) * MaxEraseMaskRegionCoverage
	if want > MaxEraseMaskRatioPerBlockCap {
		want = MaxEraseMaskRatioPerBlockCap
	}
	if limit < MaxEraseMaskRatioPerBlock {
		t.Fatalf("limit %.4f below floor %.4f", limit, MaxEraseMaskRatioPerBlock)
	}
	if limit < want-1e-9 {
		t.Fatalf("limit %.4f smaller than adaptive %.4f", limit, want)
	}
	// Production failure: ~2.47% image ratio with ~45% stroke coverage in a wide OCR box.
	imageRatio := 0.0247
	if imageRatio > limit+1e-9 {
		t.Fatalf("image ratio %.4f exceeds adaptive limit %.4f", imageRatio, limit)
	}
}

func TestPerBlockEraseImageLimitSmallBadge(t *testing.T) {
	imageArea := 900 * 900
	limit := perBlockEraseImageLimit(180*70, imageArea)
	if limit != MaxEraseMaskRatioPerBlock {
		t.Fatalf("small badge limit = %.4f, want floor %.4f", limit, MaxEraseMaskRatioPerBlock)
	}
}
