package imagerender

import (
	"image"
	"image/color"
	"sort"
	"strings"
)

func estimateMaskBackground(img *image.RGBA, rect image.Rectangle, polarity, blockClass string) (color.RGBA, float64) {
	class := strings.TrimSpace(strings.ToLower(blockClass))
	if polarity == "light" || isCapsuleBlockClass(class) {
		bg, lum := estimateRegionBackgroundColor(img, rect)
		return bg, lum
	}
	border := averageBorderColor(img, rect)
	borderLum := luminance(border)
	if borderLum >= 95 {
		return border, borderLum
	}
	light := sampleInteriorBackgroundColor(img, rect)
	if luminance(light) > borderLum+15 {
		return light, luminance(light)
	}
	return border, borderLum
}

func buildEnhancedTextPixelMaskForBlock(img *image.RGBA, rect image.Rectangle, polarity, blockClass string) []bool {
	w, h := rect.Dx(), rect.Dy()
	mask := make([]bool, w*h)
	bgColor, bgLum := estimateMaskBackground(img, rect, polarity, blockClass)
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			c := img.RGBAAt(x, y)
			idx := (y-rect.Min.Y)*w + (x - rect.Min.X)
			switch polarity {
			case "light":
				mask[idx] = isLightTextPixel(c, bgColor, bgLum)
			default:
				mask[idx] = isDarkTextPixel(c, bgColor, bgLum)
			}
		}
	}
	return mask
}

func maskWithinLimits(n, regionArea, imageArea int) bool {
	if n < 4 || regionArea <= 0 {
		return false
	}
	if float64(n)/float64(regionArea) > MaxEraseMaskRegionCoverage {
		return false
	}
	if imageArea > 0 && float64(n)/float64(imageArea) > MaxEraseMaskRatioPerBlock {
		return false
	}
	return true
}

// dilateMaskWithinLimits tries preferDilate down to 0 so sparse text masks are not rejected
// after dilation merges into an oversized blob.
func dilateMaskWithinLimits(mask []bool, w, h, regionArea, imageArea, preferDilate int) ([]bool, bool) {
	if len(mask) == 0 || w <= 0 || h <= 0 {
		return nil, false
	}
	if preferDilate < 0 {
		preferDilate = 0
	}
	if preferDilate > 2 {
		preferDilate = 2
	}
	for d := preferDilate; d >= 0; d-- {
		out := mask
		if d > 0 {
			out = dilateMask(mask, w, h, d)
		}
		if maskWithinLimits(countMaskPixels(out), regionArea, imageArea) {
			return out, true
		}
	}
	return nil, false
}

func cleanBorderBackgroundColor(img *image.RGBA, rect image.Rectangle) color.RGBA {
	rect = rect.Intersect(img.Bounds())
	if rect.Empty() {
		return color.RGBA{255, 255, 255, 255}
	}
	var lums []float64
	addLum := func(x, y int) {
		if x < rect.Min.X || y < rect.Min.Y || x >= rect.Max.X || y >= rect.Max.Y {
			return
		}
		c := img.RGBAAt(x, y)
		if luminance(c) > 245 {
			return
		}
		lums = append(lums, luminance(c))
	}
	for x := rect.Min.X; x < rect.Max.X; x++ {
		addLum(x, rect.Min.Y)
		addLum(x, rect.Max.Y-1)
	}
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		addLum(rect.Min.X, y)
		addLum(rect.Max.X-1, y)
	}
	if len(lums) == 0 {
		return sampleInteriorBackgroundColor(img, rect)
	}
	sorted := append([]float64(nil), lums...)
	sort.Float64s(sorted)
	med := sorted[len(sorted)/2]
	var r, g, bl, n float64
	add := func(x, y int) {
		if x < rect.Min.X || y < rect.Min.Y || x >= rect.Max.X || y >= rect.Max.Y {
			return
		}
		c := img.RGBAAt(x, y)
		if absFloat(luminance(c)-med) > 22 {
			return
		}
		r += float64(c.R)
		g += float64(c.G)
		bl += float64(c.B)
		n++
	}
	for x := rect.Min.X; x < rect.Max.X; x++ {
		add(x, rect.Min.Y)
		add(x, rect.Max.Y-1)
	}
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		add(rect.Min.X, y)
		add(rect.Max.X-1, y)
	}
	if n == 0 {
		return averageBorderColor(img, rect)
	}
	return color.RGBA{
		R: uint8(r / n),
		G: uint8(g / n),
		B: uint8(bl / n),
		A: 255,
	}
}

func buildBestTextPixelMaskFilteredForBlock(
	img *image.RGBA,
	rect image.Rectangle,
	preferred, blockClass string,
	dilate, regionArea, imageArea int,
) ([]bool, bool) {
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
		raw := buildEnhancedTextPixelMaskForBlock(img, rect, pol, blockClass)
		raw = filterMaskConnectedComponents(raw, w, h, regionArea, imageArea)
		candidate, ok := dilateMaskWithinLimits(raw, w, h, regionArea, imageArea, dilate)
		if !ok {
			continue
		}
		ratio := float64(countMaskPixels(candidate)) / float64(regionArea)
		if ratio < bestRatio {
			bestRatio = ratio
			best = candidate
		}
	}
	if len(best) == 0 {
		return nil, false
	}
	return best, true
}

func buildCapsuleWhiteTextMask(img *image.RGBA, rect image.Rectangle, dilate, regionArea, imageArea int) ([]bool, bool) {
	w, h := rect.Dx(), rect.Dy()
	mask := make([]bool, w*h)
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			lum := luminance(img.RGBAAt(x, y))
			if lum >= 155 {
				mask[(y-rect.Min.Y)*w+(x-rect.Min.X)] = true
			}
		}
	}
	mask = filterMaskConnectedComponents(mask, w, h, regionArea, imageArea)
	if out, ok := dilateMaskWithinLimits(mask, w, h, regionArea, imageArea, dilate); ok {
		return out, true
	}
	return nil, false
}

func buildStrokeFallbackMask(img *image.RGBA, rect image.Rectangle, dilate, regionArea, imageArea int) ([]bool, bool) {
	w, h := rect.Dx(), rect.Dy()
	bg := cleanBorderBackgroundColor(img, rect)
	mask := textStrokeMask(img, rect, bg)
	if out, ok := dilateMaskWithinLimits(mask, w, h, regionArea, imageArea, dilate); ok {
		return out, true
	}
	bgLum := luminance(bg)
	relaxed := make([]bool, w*h)
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			lum := luminance(img.RGBAAt(x, y))
			if absFloat(lum-bgLum) >= 28 {
				relaxed[(y-rect.Min.Y)*w+(x-rect.Min.X)] = true
			}
		}
	}
	relaxed = filterMaskConnectedComponents(relaxed, w, h, regionArea, imageArea)
	if out, ok := dilateMaskWithinLimits(relaxed, w, h, regionArea, imageArea, dilate); ok {
		return out, true
	}
	return nil, false
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func strokeMaskPixelCount(img *image.RGBA, rect image.Rectangle) int {
	bg := cleanBorderBackgroundColor(img, rect)
	return countMaskPixels(textStrokeMask(img, rect, bg))
}

func maskWouldExceedLimits(n, regionArea, imageArea int) bool {
	if regionArea <= 0 {
		return true
	}
	if float64(n)/float64(regionArea) > MaxEraseMaskRegionCoverage {
		return true
	}
	if imageArea > 0 && float64(n)/float64(imageArea) > MaxEraseMaskRatioPerBlock {
		return true
	}
	return false
}

func buildRelaxedContrastMask(img *image.RGBA, rect image.Rectangle) []bool {
	w, h := rect.Dx(), rect.Dy()
	bgLum := luminance(cleanBorderBackgroundColor(img, rect))
	mask := make([]bool, w*h)
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if absFloat(luminance(img.RGBAAt(x, y))-bgLum) >= 24 {
				mask[(y-rect.Min.Y)*w+(x-rect.Min.X)] = true
			}
		}
	}
	return mask
}

func buildRobustTextPixelMask(img *image.RGBA, rect image.Rectangle, block TextBlock, dilate, imageArea int) ([]bool, bool) {
	w, h := rect.Dx(), rect.Dy()
	if w <= 0 || h <= 0 {
		return nil, false
	}
	regionArea := max(1, w*h)
	if dilate <= 0 {
		dilate = 1
	}
	if dilate > 2 {
		dilate = 2
	}
	class := block.BlockClass
	polarity := resolveTextPolarity(img, rect, block)

	if mask, ok := buildBestTextPixelMaskFilteredForBlock(img, rect, polarity, class, dilate, regionArea, imageArea); ok {
		return mask, true
	}
	if mask, ok := buildBestTextPixelMask(img, rect, polarity, dilate); ok {
		if maskWithinLimits(countMaskPixels(mask), regionArea, imageArea) {
			return mask, true
		}
	}
	if strict, ok := buildStrictTextPixelMask(img, rect, polarity, dilate); ok {
		if maskWithinLimits(countMaskPixels(strict), regionArea, imageArea) {
			return strict, true
		}
	}
	for _, pol := range []string{polarity, invertPolarity(polarity), "dark", "light"} {
		raw := buildEnhancedTextPixelMaskForBlock(img, rect, pol, class)
		if out, ok := dilateMaskWithinLimits(raw, w, h, regionArea, imageArea, dilate); ok {
			return out, true
		}
	}
	for _, pol := range []string{polarity, invertPolarity(polarity)} {
		raw := buildTextPixelMask(img, rect, pol)
		if out, ok := dilateMaskWithinLimits(raw, w, h, regionArea, imageArea, dilate); ok {
			return out, true
		}
	}
	if isCapsuleBlockClass(class) {
		if mask, ok := buildCapsuleWhiteTextMask(img, rect, dilate, regionArea, imageArea); ok {
			return mask, true
		}
	}
	if mask, ok := buildStrokeFallbackMask(img, rect, dilate, regionArea, imageArea); ok {
		return mask, true
	}
	if out, ok := dilateMaskWithinLimits(buildRelaxedContrastMask(img, rect), w, h, regionArea, imageArea, dilate); ok {
		return out, true
	}
	return nil, false
}

func buildRobustPillTextMask(img *image.RGBA, rect image.Rectangle, block TextBlock, dilate, imageArea int) ([]bool, bool) {
	w, h := rect.Dx(), rect.Dy()
	regionArea := max(1, w*h)
	polarity := "light"
	if p := strings.TrimSpace(strings.ToLower(block.TextPolarity)); p == "light" || isWhiteTextStyle(block.Style) {
		polarity = "light"
	} else if p == "dark" {
		polarity = "dark"
	} else {
		polarity = detectTextPolarityFromImage(img, rect)
	}
	if mask, ok := buildPillTextMask(img, rect, polarity, dilate, imageArea); ok {
		return mask, true
	}
	if polarity != "light" {
		if mask, ok := buildPillTextMask(img, rect, "light", dilate, imageArea); ok {
			return mask, true
		}
	}
	if mask, ok := buildCapsuleWhiteTextMask(img, rect, dilate, regionArea, imageArea); ok {
		return mask, true
	}
	blockCopy := block
	blockCopy.BlockClass = "badge"
	if mask, ok := buildRobustTextPixelMask(img, rect, blockCopy, dilate, imageArea); ok {
		return mask, true
	}
	return buildStrokeFallbackMask(img, rect, dilate, regionArea, imageArea)
}
