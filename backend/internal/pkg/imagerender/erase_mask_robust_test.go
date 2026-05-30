package imagerender

import (
	"image"
	"image/color"
	"testing"
)

func makeTitleNearCapsuleFixture() (*image.RGBA, image.Rectangle, TextBlock) {
	img := image.NewRGBA(image.Rect(0, 0, 900, 900))
	for y := 0; y < 900; y++ {
		for x := 0; x < 900; x++ {
			img.Set(x, y, color.RGBA{235, 235, 235, 255})
		}
	}
	for y := 85; y < 125; y++ {
		for x := 620; x < 780; x++ {
			if (x+y)%6 < 2 {
				img.Set(x, y, color.RGBA{28, 28, 28, 255})
			}
		}
	}
	block := TextBlock{
		ID: "title", BlockClass: "title",
		EraseBBox: BBox{X: 610, Y: 80, Width: 180, Height: 50},
	}
	rect := image.Rect(611, 81, 791, 131)
	return img, rect, block
}

func TestBuildRobustTitleMaskDirect(t *testing.T) {
	img, rect, block := makeTitleNearCapsuleFixture()
	mask, ok := buildRobustTextPixelMask(img, rect, block, 1, 900*900)
	t.Logf("ok=%v pixels=%d", ok, countMaskPixels(mask))
	if !ok || countMaskPixels(mask) < 4 {
		t.Fatalf("robust mask failed")
	}
}

func TestRobustMaskTitleNearDarkCapsule(t *testing.T) {
	img, _, block := makeTitleNearCapsuleFixture()
	stats, _, err := eraseTextBlockPixelMaskWithMask(img, block, 900*900)
	if err != nil {
		t.Fatalf("erase: %v", err)
	}
	if stats.ErasePixels <= 0 {
		t.Fatalf("expected erase pixels, got %d", stats.ErasePixels)
	}
}

func TestEnhancedMaskAdaptiveDilate(t *testing.T) {
	img, rect, block := makeTitleNearCapsuleFixture()
	regionArea := rect.Dx() * rect.Dy()
	polarity := resolveTextPolarity(img, rect, block)
	raw := buildEnhancedTextPixelMaskForBlock(img, rect, polarity, block.BlockClass)
	if !maskWithinLimits(countMaskPixels(raw), regionArea, 900*900) {
		t.Fatalf("undilated enhanced mask should pass limits")
	}
	mask, ok := buildBestTextPixelMaskFilteredForBlock(img, rect, polarity, block.BlockClass, 1, regionArea, 900*900)
	if !ok || countMaskPixels(mask) < 4 {
		t.Fatalf("filtered mask should succeed with adaptive dilate, ok=%v n=%d", ok, countMaskPixels(mask))
	}
}
