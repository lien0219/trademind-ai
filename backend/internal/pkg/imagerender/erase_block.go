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
	polarity := resolveTextPolarity(img, rect, block)
	fill := pillEraseFillColor(img, rect, block, polarity, mask)
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

func ForceEraseTextMaskBounds(img *image.RGBA, blocks []TextBlock, imageArea int) EraseStats {
	if img == nil || len(blocks) == 0 {
		return EraseStats{}
	}
	b := img.Bounds()
	total := EraseStats{}
	for _, block := range blocks {
		eraseBox := block.EraseBBox
		if eraseBox.Width <= 0 || eraseBox.Height <= 0 {
			eraseBox = block.BBox
		}
		if eraseBox.Width <= 0 || eraseBox.Height <= 0 {
			continue
		}
		pad := block.ErasePadding
		if pad <= 0 {
			pad = 1
		}
		if pad > 2 {
			pad = 2
		}
		if isCapsuleBlockClass(block.BlockClass) {
			pad = max(pad, max(8, eraseBox.Height/3))
		}
		rect := sourceDecorCleanupRect(eraseBox, block.BlockClass, pad, b)
		if rect.Empty() {
			continue
		}
		dilate := block.MaskDilate
		if dilate < 2 {
			dilate = 2
		}
		var mask []bool
		var ok bool
		if isCapsuleBlockClass(block.BlockClass) {
			mask, ok = buildRobustPillTextMask(img, rect, block, dilate, imageArea)
		} else {
			mask, ok = buildRobustTextPixelMask(img, rect, block, dilate, imageArea)
		}
		if !ok || countMaskPixels(mask) < 4 {
			if fallback, fallbackOK := buildStrokeFallbackMask(img, rect, dilate, max(1, rect.Dx()*rect.Dy()), imageArea); fallbackOK {
				mask = fallback
				ok = true
			}
		}
		if !ok || countMaskPixels(mask) < 4 {
			continue
		}
		cleanRect, ok := maskBoundsRect(rect, mask, max(4, min(14, max(rect.Dx(), rect.Dy())/12)), b)
		if !ok || cleanRect.Empty() {
			continue
		}
		area := cleanRect.Dx() * cleanRect.Dy()
		if imageArea > 0 && float64(area)/float64(imageArea) > sourceDecorCleanupMaxRatio(block.BlockClass) {
			continue
		}
		sampleFillRegion(img, cleanRect)
		total.ErasePixels += area
		total.PatchPixels += area
		total.FlatFillRatio += float64(area)
	}
	if total.PatchPixels > 0 {
		total.FlatFillRatio = total.FlatFillRatio / float64(total.PatchPixels)
	}
	return total
}

func ForceEraseSourceBlockBounds(img *image.RGBA, blocks []TextBlock, imageArea int) EraseStats {
	if img == nil || len(blocks) == 0 {
		return EraseStats{}
	}
	b := img.Bounds()
	total := EraseStats{}
	for _, block := range blocks {
		eraseBox := block.EraseBBox
		if eraseBox.Width <= 0 || eraseBox.Height <= 0 {
			eraseBox = block.BBox
		}
		if eraseBox.Width <= 0 || eraseBox.Height <= 0 {
			continue
		}
		pad := block.ErasePadding
		if pad <= 0 {
			pad = 1
		}
		if pad > 3 {
			pad = 3
		}
		rect := sourceDecorCleanupRect(eraseBox, block.BlockClass, pad, b)
		if rect.Empty() {
			continue
		}
		area := rect.Dx() * rect.Dy()
		if imageArea > 0 && float64(area)/float64(imageArea) > sourceDecorCleanupMaxRatio(block.BlockClass) {
			continue
		}
		blendFillRegion(img, rect)
		total.ErasePixels += area
		total.PatchPixels += area
		total.FlatFillRatio += float64(area)
	}
	if total.PatchPixels > 0 {
		total.FlatFillRatio = total.FlatFillRatio / float64(total.PatchPixels)
	}
	return total
}

func sourceDecorCleanupRect(box BBox, blockClass string, pad int, bounds image.Rectangle) image.Rectangle {
	if isCapsuleBlockClass(blockClass) {
		xPad := max(pad, max(12, max(box.Height*3, box.Width/2)))
		yPad := max(pad, max(6, box.Height/2))
		return image.Rect(
			box.X-xPad,
			box.Y-yPad,
			box.X+box.Width+xPad,
			box.Y+box.Height+yPad,
		).Intersect(bounds)
	}
	pad = max(pad, max(4, min(18, box.Height/5)))
	expanded := expandEraseRect(box, pad, bounds.Dx(), bounds.Dy())
	return image.Rect(expanded.X, expanded.Y, expanded.X+expanded.Width, expanded.Y+expanded.Height).Intersect(bounds)
}

func sourceDecorCleanupMaxRatio(blockClass string) float64 {
	if isCapsuleBlockClass(blockClass) {
		return 0.12
	}
	return MaxEraseMaskRatioTotal
}

func maskBoundsRect(region image.Rectangle, mask []bool, pad int, bounds image.Rectangle) (image.Rectangle, bool) {
	w, h := region.Dx(), region.Dy()
	if w <= 0 || h <= 0 || len(mask) != w*h {
		return image.Rectangle{}, false
	}
	minX, minY := w, h
	maxX, maxY := -1, -1
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !mask[y*w+x] {
				continue
			}
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x > maxX {
				maxX = x
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	if maxX < minX || maxY < minY {
		return image.Rectangle{}, false
	}
	if pad < 0 {
		pad = 0
	}
	out := image.Rect(
		region.Min.X+minX-pad,
		region.Min.Y+minY-pad,
		region.Min.X+maxX+pad+1,
		region.Min.Y+maxY+pad+1,
	).Intersect(bounds)
	if out.Empty() {
		return image.Rectangle{}, false
	}
	return out, true
}
