package imagerender

import (
	"fmt"
	"image"
	"image/color"
	"strings"
)

// pillTextErase removes white/light text inside a capsule by median-fill, without inpainting the capsule background.
func pillTextErase(img *image.RGBA, block TextBlock, imageArea int) (EraseStats, error) {
	b := img.Bounds()
	eraseBox := block.EraseBBox
	if eraseBox.Width <= 0 || eraseBox.Height <= 0 {
		eraseBox = block.BBox
	}
	if eraseBox.Width <= 0 || eraseBox.Height <= 0 {
		return EraseStats{}, nil
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
	polarity := "light"
	if p := strings.TrimSpace(strings.ToLower(block.TextPolarity)); p == "light" || isWhiteTextStyle(block.Style) {
		polarity = "light"
	} else if p == "dark" {
		polarity = "dark"
	} else {
		polarity = detectTextPolarityFromImage(img, rect)
	}
	dilate := block.MaskDilate
	if dilate <= 0 {
		dilate = 1
	}
	if dilate > 2 {
		dilate = 2
	}
	mask, ok := buildPillTextMask(img, rect, polarity, dilate, imageArea)
	if !ok || countMaskPixels(mask) < 4 {
		return EraseStats{}, ErrEraseMaskEmpty
	}
	fill := sampleNonWhiteMedianColor(img, rect, mask)
	if strings.TrimSpace(block.Style.BackgroundColor) != "" {
		fill = parseHexColor(block.Style.BackgroundColor, fill)
	}
	changed := applyMaskFill(img, rect, mask, fill)
	regionArea := rect.Dx() * rect.Dy()
	limit := perBlockEraseImageLimit(regionArea, imageArea)
	if imageArea > 0 && float64(changed)/float64(imageArea) > limit {
		return EraseStats{ErasePixels: changed, PatchPixels: changed, LargePatchDetected: true},
			fmt.Errorf("%w: pill block %s ratio %.4f", ErrEraseMaskTooLarge, block.ID, float64(changed)/float64(imageArea))
	}
	stats := EraseStats{
		ErasePixels: changed,
		PatchPixels: changed,
	}
	if regionArea > 0 {
		stats.FlatFillRatio = float64(changed) / float64(regionArea)
	}
	return stats, nil
}

func buildPillTextMask(img *image.RGBA, rect image.Rectangle, polarity string, dilate, imageArea int) ([]bool, bool) {
	w, h := rect.Dx(), rect.Dy()
	regionArea := max(1, w*h)
	mask := buildEnhancedTextPixelMask(img, rect, polarity)
	if polarity != "light" {
		mask = buildEnhancedTextPixelMask(img, rect, "light")
	}
	mask = filterMaskConnectedComponents(mask, w, h, regionArea, imageArea)
	mask = dilateMask(mask, w, h, dilate)
	n := countMaskPixels(mask)
	if n < 4 {
		return nil, false
	}
	if float64(n)/float64(regionArea) > MaxEraseMaskRegionCoverage {
		return nil, false
	}
	if imageArea > 0 && float64(n)/float64(imageArea) > MaxEraseMaskRatioPerBlock {
		return nil, false
	}
	return mask, true
}

func buildWhiteTextMaskOnly(img *image.RGBA, rect image.Rectangle, dilate, imageArea int) ([]bool, bool) {
	return buildPillTextMask(img, rect, "light", dilate, imageArea)
}

func isPillBlockClass(class string) bool {
	return isCapsuleBlockClass(class)
}

func erasePillBlockClass(img *image.RGBA, block TextBlock, imageArea int) (EraseStats, error) {
	return pillTextErase(img, block, imageArea)
}

func eraseTitleOrNormalBlock(img *image.RGBA, block TextBlock, imageArea int, inpaintRadius int) (EraseStats, error) {
	b := img.Bounds()
	eraseBox := block.EraseBBox
	if eraseBox.Width <= 0 || eraseBox.Height <= 0 {
		eraseBox = block.BBox
	}
	if eraseBox.Width <= 0 || eraseBox.Height <= 0 {
		return EraseStats{}, nil
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
	polarity := resolveTextPolarity(img, rect, block)
	dilate := block.MaskDilate
	if dilate <= 0 {
		dilate = 1
	}
	if dilate > 2 {
		dilate = 2
	}
	mask, ok := buildFilteredTextPixelMask(img, rect, polarity, dilate, imageArea)
	if !ok || len(mask) == 0 {
		return EraseStats{}, ErrEraseMaskEmpty
	}
	changed := countMaskPixels(mask)
	regionArea := rect.Dx() * rect.Dy()
	limit := perBlockEraseImageLimit(regionArea, imageArea)
	if imageArea > 0 && float64(changed)/float64(imageArea) > limit {
		return EraseStats{ErasePixels: changed, PatchPixels: changed, LargePatchDetected: true},
			fmt.Errorf("%w: block %s ratio %.4f", ErrEraseMaskTooLarge, block.ID, float64(changed)/float64(imageArea))
	}
	if regionArea > 0 && float64(changed)/float64(regionArea) > MaxEraseMaskRegionCoverage {
		return EraseStats{ErasePixels: changed, PatchPixels: changed, LargePatchDetected: true},
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
	return stats, nil
}

func buildFilteredTextPixelMask(img *image.RGBA, rect image.Rectangle, polarity string, dilate, imageArea int) ([]bool, bool) {
	w, h := rect.Dx(), rect.Dy()
	regionArea := max(1, w*h)
	mask, ok := buildBestTextPixelMaskFiltered(img, rect, polarity, dilate, regionArea, imageArea)
	return mask, ok
}

func buildBestTextPixelMaskFiltered(img *image.RGBA, rect image.Rectangle, preferred string, dilate, regionArea, imageArea int) ([]bool, bool) {
	if dilate <= 0 {
		dilate = 1
	}
	if dilate > 2 {
		dilate = 2
	}
	w, h := rect.Dx(), rect.Dy()
	candidates := []string{preferred, invertPolarity(preferred), "dark", "light"}
	seen := map[string]bool{}
	var best []bool
	bestRatio := 1.0
	for _, pol := range candidates {
		pol = strings.TrimSpace(strings.ToLower(pol))
		if pol == "" || seen[pol] {
			continue
		}
		seen[pol] = true
		raw := buildEnhancedTextPixelMask(img, rect, pol)
		raw = filterMaskConnectedComponents(raw, w, h, regionArea, imageArea)
		raw = dilateMask(raw, w, h, dilate)
		n := countMaskPixels(raw)
		if n < 4 {
			continue
		}
		ratio := float64(n) / float64(regionArea)
		if ratio > MaxEraseMaskRegionCoverage {
			continue
		}
		if imageArea > 0 && float64(n)/float64(imageArea) > MaxEraseMaskRatioPerBlock {
			continue
		}
		if ratio < bestRatio {
			bestRatio = ratio
			best = raw
		}
	}
	if len(best) == 0 {
		return nil, false
	}
	return best, true
}

func SecondaryInpaintTextMask(img *image.RGBA, block TextBlock, imageArea int, radius int) (EraseStats, error) {
	return secondaryInpaintTextMask(img, block, imageArea, radius)
}

func secondaryInpaintTextMask(img *image.RGBA, block TextBlock, imageArea int, radius int) (EraseStats, error) {
	return eraseTitleOrNormalBlock(img, block, imageArea, radius)
}

func applyMaskFillColor(img *image.RGBA, rect image.Rectangle, mask []bool, fill color.RGBA) int {
	return applyMaskFill(img, rect, mask, fill)
}
