package imagerender

import (
	"fmt"
	"image"
	"strings"
)

func eraseTextBlockPixelMaskWithMask(img *image.RGBA, block TextBlock, imageArea int) (EraseStats, []bool, error) {
	class := strings.TrimSpace(strings.ToLower(block.BlockClass))
	switch {
	case class == "title":
		return eraseTitleOrNormalBlockWithMask(img, block, imageArea, 2)
	case isCapsuleBlockClass(class):
		return erasePillBlockWithMask(img, block, imageArea)
	default:
		return eraseTitleOrNormalBlockWithMask(img, block, imageArea, 2)
	}
}

func eraseTitleOrNormalBlockWithMask(img *image.RGBA, block TextBlock, imageArea, inpaintRadius int) (EraseStats, []bool, error) {
	b := img.Bounds()
	eraseBox := block.EraseBBox
	if eraseBox.Width <= 0 || eraseBox.Height <= 0 {
		eraseBox = block.BBox
	}
	if eraseBox.Width <= 0 || eraseBox.Height <= 0 {
		return EraseStats{}, nil, nil
	}
	pad := block.ErasePadding
	if pad <= 0 {
		pad = 1
	}
	if pad > 2 {
		pad = 2
	}
	expanded := expandEraseRect(eraseBox, pad, b.Dx(), b.Dy())
	rect := image.Rect(expanded.X, expanded.Y, expanded.X+expanded.Width, expanded.Y+expanded.Height)
	regionArea := rect.Dx() * rect.Dy()
	dilate := block.MaskDilate
	if dilate <= 0 {
		dilate = 1
	}
	if dilate > 2 {
		dilate = 2
	}
	mask, ok := buildRobustTextPixelMask(img, rect, block, dilate, imageArea)
	n := countMaskPixels(mask)
	if !ok || n < 4 {
		strokeN := strokeMaskPixelCount(img, rect)
		if strokeN > 0 && maskWouldExceedLimits(strokeN, regionArea, imageArea) {
			return EraseStats{ErasePixels: n, PatchPixels: n, LargePatchDetected: true}, nil,
				fmt.Errorf("%w: block %s mask generation failed", ErrEraseMaskGenerationBad, block.ID)
		}
		return EraseStats{}, nil, nil
	}
	changed := n
	limit := perBlockEraseImageLimit(regionArea, imageArea)
	if imageArea > 0 && float64(changed)/float64(imageArea) > limit {
		return EraseStats{ErasePixels: changed, PatchPixels: changed, LargePatchDetected: true}, mask,
			fmt.Errorf("%w: block %s ratio %.4f", ErrEraseMaskTooLarge, block.ID, float64(changed)/float64(imageArea))
	}
	if regionArea > 0 && float64(changed)/float64(regionArea) > MaxEraseMaskRegionCoverage {
		return EraseStats{ErasePixels: changed, PatchPixels: changed, LargePatchDetected: true}, mask,
			fmt.Errorf("%w: block %s mask covers too much of detection region", ErrEraseMaskGenerationBad, block.ID)
	}
	changed = teleaInpaintMask(img, rect, mask, inpaintRadius)
	stats := EraseStats{
		ErasePixels: changed,
		PatchPixels: changed,
	}
	if regionArea > 0 {
		stats.FlatFillRatio = float64(changed) / float64(regionArea)
	}
	return stats, mask, nil
}

func erasePillBlockWithMask(img *image.RGBA, block TextBlock, imageArea int) (EraseStats, []bool, error) {
	b := img.Bounds()
	eraseBox := block.EraseBBox
	if eraseBox.Width <= 0 || eraseBox.Height <= 0 {
		eraseBox = block.BBox
	}
	if eraseBox.Width <= 0 || eraseBox.Height <= 0 {
		return EraseStats{}, nil, nil
	}
	pad := block.ErasePadding
	if pad <= 0 {
		pad = 1
	}
	if pad > 2 {
		pad = 2
	}
	expanded := expandEraseRect(eraseBox, pad, b.Dx(), b.Dy())
	rect := image.Rect(expanded.X, expanded.Y, expanded.X+expanded.Width, expanded.Y+expanded.Height)
	regionArea := rect.Dx() * rect.Dy()
	dilate := block.MaskDilate
	if dilate <= 0 {
		dilate = 1
	}
	if dilate > 2 {
		dilate = 2
	}
	mask, ok := buildRobustPillTextMask(img, rect, block, dilate, imageArea)
	n := countMaskPixels(mask)
	if !ok || n < 4 {
		strokeN := strokeMaskPixelCount(img, rect)
		if strokeN > 0 && maskWouldExceedLimits(strokeN, regionArea, imageArea) {
			return EraseStats{ErasePixels: n, PatchPixels: n, LargePatchDetected: true}, nil,
				fmt.Errorf("%w: pill block %s mask generation failed", ErrEraseMaskGenerationBad, block.ID)
		}
		return EraseStats{}, nil, nil
	}
	fill := sampleNonWhiteMedianColor(img, rect, mask)
	if strings.TrimSpace(block.Style.BackgroundColor) != "" {
		fill = parseHexColor(block.Style.BackgroundColor, fill)
	}
	changed := applyMaskFill(img, rect, mask, fill)
	limit := perBlockEraseImageLimit(regionArea, imageArea)
	if imageArea > 0 && float64(changed)/float64(imageArea) > limit {
		return EraseStats{ErasePixels: changed, PatchPixels: changed, LargePatchDetected: true}, mask,
			fmt.Errorf("%w: pill block %s ratio %.4f", ErrEraseMaskTooLarge, block.ID, float64(changed)/float64(imageArea))
	}
	stats := EraseStats{
		ErasePixels: changed,
		PatchPixels: changed,
	}
	if regionArea > 0 {
		stats.FlatFillRatio = float64(changed) / float64(regionArea)
	}
	return stats, mask, nil
}
