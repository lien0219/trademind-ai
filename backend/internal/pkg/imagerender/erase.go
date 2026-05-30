package imagerender

import (
	"errors"
	"image"
	"image/color"
	"math"
	"strings"
)

var (
	ErrEraseMaskTooLarge      = errors.New("imagerender: erase mask area exceeds limit")
	ErrEraseMaskEmpty         = errors.New("imagerender: text pixel mask empty")
	ErrEraseMaskGenerationBad = errors.New("imagerender: text pixel mask generation failed")
)

type EraseStats struct {
	ErasePixels          int
	PatchPixels          int
	BackgroundDeltaScore float64
	FlatFillRatio        float64
	LargePatchDetected   bool
}

func chooseEraseMode(mode string, region *image.RGBA, rect image.Rectangle) string {
	m := trimLower(mode)
	switch m {
	case EraseTextPixelMask, ErasePreciseMask, EraseBackgroundSample, EraseBlurFill, EraseOpenCVInpaint, EraseAIInpaint:
		return m
	}
	variance := borderColorVariance(region, rect)
	switch {
	case variance < 1600:
		return EraseBackgroundSample
	default:
		return EraseOpenCVInpaint
	}
}

func borderColorVariance(img *image.RGBA, rect image.Rectangle) float64 {
	b := img.Bounds()
	samples := make([][3]float64, 0, 32)
	addSample := func(x, y int) {
		if x < b.Min.X || y < b.Min.Y || x >= b.Max.X || y >= b.Max.Y {
			return
		}
		c := img.RGBAAt(x, y)
		samples = append(samples, [3]float64{float64(c.R), float64(c.G), float64(c.B)})
	}
	for x := rect.Min.X; x < rect.Max.X; x++ {
		addSample(x, rect.Min.Y)
		if rect.Max.Y-1 > rect.Min.Y {
			addSample(x, rect.Max.Y-1)
		}
	}
	for y := rect.Min.Y + 1; y < rect.Max.Y-1; y++ {
		addSample(rect.Min.X, y)
		if rect.Max.X-1 > rect.Min.X {
			addSample(rect.Max.X-1, y)
		}
	}
	if len(samples) < 2 {
		return 0
	}
	var rSum, gSum, bSum float64
	for _, s := range samples {
		rSum += s[0]
		gSum += s[1]
		bSum += s[2]
	}
	n := float64(len(samples))
	rMean, gMean, bMean := rSum/n, gSum/n, bSum/n
	var vr, vg, vb float64
	for _, s := range samples {
		vr += (s[0] - rMean) * (s[0] - rMean)
		vg += (s[1] - gMean) * (s[1] - gMean)
		vb += (s[2] - bMean) * (s[2] - bMean)
	}
	return (vr + vg + vb) / (3 * n)
}

func eraseRegion(img *image.RGBA, rect image.Rectangle, mode string) EraseStats {
	rect = rect.Intersect(img.Bounds())
	if rect.Empty() {
		return EraseStats{}
	}
	chosen := chooseEraseMode(mode, img, rect)
	switch chosen {
	case EraseTextPixelMask:
		return textPixelMaskEraseRegion(img, rect, TextBlock{MaskDilate: 1}, false)
	case ErasePreciseMask:
		return preciseMaskEraseRegion(img, rect)
	case EraseBlurFill:
		blurFillRegion(img, rect)
	case EraseOpenCVInpaint:
		inpaintRegion(img, rect, 3)
	default:
		sampleFillRegion(img, rect)
	}
	area := rect.Dx() * rect.Dy()
	stats := EraseStats{ErasePixels: area, PatchPixels: area}
	stats.LargePatchDetected = chosen == EraseBackgroundSample && area > 0
	if chosen == EraseBackgroundSample {
		stats.FlatFillRatio = 1
	}
	return stats
}

func eraseTextBlockPixelMask(img *image.RGBA, block TextBlock, imageArea int) (EraseStats, error) {
	stats, _, err := eraseTextBlockPixelMaskWithMask(img, block, imageArea)
	return stats, err
}

func perBlockEraseImageLimit(regionArea, imageArea int) float64 {
	if imageArea <= 0 {
		return MaxEraseMaskRatioPerBlock
	}
	regionFrac := float64(regionArea) / float64(imageArea)
	adaptive := regionFrac * MaxEraseMaskRegionCoverage
	if adaptive > MaxEraseMaskRatioPerBlockCap {
		adaptive = MaxEraseMaskRatioPerBlockCap
	}
	if adaptive > MaxEraseMaskRatioPerBlock {
		return adaptive
	}
	return MaxEraseMaskRatioPerBlock
}

func isCapsuleBlockClass(class string) bool {
	switch strings.TrimSpace(strings.ToLower(class)) {
	case "subtitle", "badge", "color_badge", "pill":
		return true
	default:
		return false
	}
}

func textPixelMaskEraseRegion(img *image.RGBA, rect image.Rectangle, block TextBlock, _ bool) EraseStats {
	rect = rect.Intersect(img.Bounds())
	if rect.Empty() {
		return EraseStats{}
	}
	block.EraseBBox = BBox{X: rect.Min.X, Y: rect.Min.Y, Width: rect.Dx(), Height: rect.Dy()}
	b := img.Bounds()
	stats, err := eraseTextBlockPixelMask(img, block, max(1, b.Dx()*b.Dy()))
	if err != nil {
		return EraseStats{LargePatchDetected: true}
	}
	return stats
}

func inferTextPolarity(block TextBlock, bg color.RGBA) string {
	if isWhiteTextStyle(block.Style) && isDarkLabelStyle(block.Style) {
		return "light"
	}
	if isDarkTextStyle(block.Style) {
		return "dark"
	}
	if isWhiteTextStyle(block.Style) {
		return "light"
	}
	if luminance(bg) < 128 {
		return "light"
	}
	return "dark"
}

func isDarkTextStyle(style TextStyle) bool {
	c := strings.TrimSpace(strings.ToLower(style.Color))
	switch c {
	case "#111111", "#111", "#000000", "#000", "#1f1f1f", "black":
		return true
	default:
		return false
	}
}

func isDarkLabelStyle(style TextStyle) bool {
	bg := strings.TrimSpace(strings.ToLower(style.BackgroundColor))
	switch bg {
	case "#111111", "#111", "#000000", "#000", "#1f1f1f":
		return true
	default:
		return false
	}
}

func isWhiteTextStyle(style TextStyle) bool {
	c := strings.TrimSpace(strings.ToLower(style.Color))
	return c == "#ffffff" || c == "#fff" || c == "white"
}

func buildTextPixelMask(img *image.RGBA, rect image.Rectangle, polarity string) []bool {
	w, h := rect.Dx(), rect.Dy()
	mask := make([]bool, w*h)
	bgColor, bgLum := estimateRegionBackgroundColor(img, rect)
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			c := img.RGBAAt(x, y)
			lum := luminance(c)
			delta := colorDistance(c, bgColor)
			if delta < 30 {
				continue
			}
			switch polarity {
			case "light":
				if lum > bgLum+22 && lum >= 125 {
					mask[(y-rect.Min.Y)*w+x-rect.Min.X] = true
				}
			default:
				if lum < bgLum-22 && lum <= 160 {
					mask[(y-rect.Min.Y)*w+x-rect.Min.X] = true
				}
			}
		}
	}
	return mask
}

func estimateRegionBackgroundColor(img *image.RGBA, rect image.Rectangle) (color.RGBA, float64) {
	rect = rect.Intersect(img.Bounds())
	if rect.Empty() {
		return color.RGBA{255, 255, 255, 255}, 255
	}
	var darkR, darkG, darkB, darkN float64
	var lightR, lightG, lightB, lightN float64
	var midR, midG, midB, midN float64
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			c := img.RGBAAt(x, y)
			lum := luminance(c)
			switch {
			case lum < 85:
				darkR += float64(c.R)
				darkG += float64(c.G)
				darkB += float64(c.B)
				darkN++
			case lum > 175:
				lightR += float64(c.R)
				lightG += float64(c.G)
				lightB += float64(c.B)
				lightN++
			default:
				midR += float64(c.R)
				midG += float64(c.G)
				midB += float64(c.B)
				midN++
			}
		}
	}
	var bg color.RGBA
	switch {
	case darkN >= lightN && darkN >= midN && darkN > 0:
		bg = color.RGBA{R: uint8(darkR / darkN), G: uint8(darkG / darkN), B: uint8(darkB / darkN), A: 255}
	case lightN >= darkN && lightN >= midN && lightN > 0:
		bg = color.RGBA{R: uint8(lightR / lightN), G: uint8(lightG / lightN), B: uint8(lightB / lightN), A: 255}
	case midN > 0:
		bg = color.RGBA{R: uint8(midR / midN), G: uint8(midG / midN), B: uint8(midB / midN), A: 255}
	default:
		bg = averageBorderColor(img, rect)
	}
	return bg, luminance(bg)
}

func resolveTextPolarity(img *image.RGBA, rect image.Rectangle, block TextBlock) string {
	detected := detectTextPolarityFromImage(img, rect)
	hinted := strings.TrimSpace(strings.ToLower(block.TextPolarity))
	if hinted == "" {
		hinted = detectTextPolarityFromImage(img, rect)
	}
	if hinted == detected {
		return hinted
	}
	hintRatio := maskRatio(img, rect, hinted, block.MaskDilate)
	detRatio := maskRatio(img, rect, detected, block.MaskDilate)
	if hintRatio > 0.45 && detRatio <= hintRatio {
		return detected
	}
	if detRatio > 0.45 && hintRatio < detRatio {
		return hinted
	}
	return detected
}

func maskRatio(img *image.RGBA, rect image.Rectangle, polarity string, dilate int) float64 {
	mask := buildTextPixelMask(img, rect, polarity)
	if dilate <= 0 {
		dilate = 1
	}
	mask = dilateMask(mask, rect.Dx(), rect.Dy(), dilate)
	area := rect.Dx() * rect.Dy()
	if area <= 0 {
		return 0
	}
	return float64(countMaskPixels(mask)) / float64(area)
}

func detectTextPolarityFromImage(img *image.RGBA, rect image.Rectangle) string {
	bgLum := medianBorderLuminance(img, rect)
	if bgLum <= 0 {
		bgLum = luminance(averageBorderColor(img, rect))
	}
	dark, light := 0, 0
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			lum := luminance(img.RGBAAt(x, y))
			if lum < bgLum-32 {
				dark++
			} else if lum > bgLum+32 {
				light++
			}
		}
	}
	if dark == 0 && light > 0 {
		return "light"
	}
	if light == 0 && dark > 0 {
		return "dark"
	}
	if dark == 0 && light == 0 {
		return "dark"
	}
	if dark <= light {
		return "dark"
	}
	return "light"
}

func buildBestTextPixelMask(img *image.RGBA, rect image.Rectangle, preferred string, dilate int) ([]bool, bool) {
	if dilate <= 0 {
		dilate = 1
	}
	if dilate > 3 {
		dilate = 3
	}
	regionArea := max(1, rect.Dx()*rect.Dy())
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
		mask := buildTextPixelMask(img, rect, pol)
		mask = dilateMask(mask, rect.Dx(), rect.Dy(), dilate)
		n := countMaskPixels(mask)
		if n < 4 {
			continue
		}
		ratio := float64(n) / float64(regionArea)
		if ratio > 0.45 {
			continue
		}
		if ratio < bestRatio {
			bestRatio = ratio
			best = mask
		}
	}
	if len(best) == 0 {
		fallbackPol := detectTextPolarityFromImage(img, rect)
		if strict, ok := buildStrictTextPixelMask(img, rect, fallbackPol, dilate); ok {
			return strict, true
		}
		mask := buildTextPixelMask(img, rect, fallbackPol)
		mask = dilateMask(mask, rect.Dx(), rect.Dy(), dilate)
		n := countMaskPixels(mask)
		if n >= 4 && float64(n)/float64(regionArea) <= 0.45 {
			return mask, true
		}
	}
	if len(best) == 0 {
		return nil, false
	}
	return best, true
}

func buildStrictTextPixelMask(img *image.RGBA, rect image.Rectangle, preferred string, dilate int) ([]bool, bool) {
	if dilate <= 0 {
		dilate = 1
	}
	candidates := []string{preferred, invertPolarity(preferred), detectTextPolarityFromImage(img, rect)}
	seen := map[string]bool{}
	var best []bool
	bestRatio := 1.0
	regionArea := max(1, rect.Dx()*rect.Dy())
	for _, pol := range candidates {
		pol = strings.TrimSpace(strings.ToLower(pol))
		if pol == "" || seen[pol] {
			continue
		}
		seen[pol] = true
		mask := buildStrictTextPixelMaskForPolarity(img, rect, pol)
		mask = dilateMask(mask, rect.Dx(), rect.Dy(), dilate)
		n := countMaskPixels(mask)
		if n < 4 {
			continue
		}
		ratio := float64(n) / float64(regionArea)
		if ratio > 0.45 {
			continue
		}
		if ratio < bestRatio {
			bestRatio = ratio
			best = mask
		}
	}
	if len(best) == 0 {
		return nil, false
	}
	return best, true
}

func buildStrictTextPixelMaskForPolarity(img *image.RGBA, rect image.Rectangle, polarity string) []bool {
	w, h := rect.Dx(), rect.Dy()
	mask := make([]bool, w*h)
	bgColor, bgLum := estimateRegionBackgroundColor(img, rect)
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			c := img.RGBAAt(x, y)
			lum := luminance(c)
			delta := colorDistance(c, bgColor)
			if delta < 42 {
				continue
			}
			switch polarity {
			case "light":
				if lum > bgLum+38 && lum >= 140 {
					mask[(y-rect.Min.Y)*w+x-rect.Min.X] = true
				}
			default:
				if lum < bgLum-38 && lum <= 145 {
					mask[(y-rect.Min.Y)*w+x-rect.Min.X] = true
				}
			}
		}
	}
	return mask
}

func invertPolarity(p string) string {
	if p == "light" {
		return "dark"
	}
	return "light"
}

func shouldCapsuleFill(block TextBlock, polarity string) bool {
	return polarity == "light" || isCapsuleBlockClass(block.BlockClass)
}

func medianBorderLuminance(img *image.RGBA, rect image.Rectangle) float64 {
	b := img.Bounds()
	var lums []float64
	add := func(x, y int) {
		if x < b.Min.X || y < b.Min.Y || x >= b.Max.X || y >= b.Max.Y {
			return
		}
		lums = append(lums, luminance(img.RGBAAt(x, y)))
	}
	for x := rect.Min.X; x < rect.Max.X; x++ {
		add(x, rect.Min.Y)
		add(x, rect.Max.Y-1)
	}
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		add(rect.Min.X, y)
		add(rect.Max.X-1, y)
	}
	if len(lums) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range lums {
		sum += v
	}
	return sum / float64(len(lums))
}

func countMaskPixels(mask []bool) int {
	n := 0
	for _, v := range mask {
		if v {
			n++
		}
	}
	return n
}

func capsuleFillColor(img *image.RGBA, rect image.Rectangle, block TextBlock, borderBg color.RGBA, polarity string) color.RGBA {
	if bg := strings.TrimSpace(block.Style.BackgroundColor); bg != "" {
		return parseHexColor(bg, borderBg)
	}
	if polarity == "dark" {
		return sampleLightCapsuleBackgroundColor(img, rect, borderBg)
	}
	return sampleCapsuleBackgroundColor(img, rect, borderBg)
}

func sampleLightCapsuleBackgroundColor(img *image.RGBA, rect image.Rectangle, fallback color.RGBA) color.RGBA {
	b := img.Bounds()
	var r, g, bl, n float64
	add := func(x, y int) {
		if x < b.Min.X || y < b.Min.Y || x >= b.Max.X || y >= b.Max.Y {
			return
		}
		c := img.RGBAAt(x, y)
		lum := luminance(c)
		if lum < 150 {
			return
		}
		r += float64(c.R)
		g += float64(c.G)
		bl += float64(c.B)
		n++
	}
	midY := (rect.Min.Y + rect.Max.Y) / 2
	for x := rect.Min.X; x < rect.Max.X; x++ {
		add(x, rect.Min.Y)
		add(x, rect.Max.Y-1)
		add(x, midY)
	}
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		add(rect.Min.X, y)
		add(rect.Max.X-1, y)
	}
	if n == 0 {
		return fallback
	}
	return color.RGBA{
		R: uint8(r / n),
		G: uint8(g / n),
		B: uint8(bl / n),
		A: 255,
	}
}

func sampleCapsuleBackgroundColor(img *image.RGBA, rect image.Rectangle, fallback color.RGBA) color.RGBA {
	b := img.Bounds()
	var r, g, bl, n float64
	add := func(x, y int) {
		if x < b.Min.X || y < b.Min.Y || x >= b.Max.X || y >= b.Max.Y {
			return
		}
		c := img.RGBAAt(x, y)
		lum := luminance(c)
		if lum > 180 {
			return
		}
		r += float64(c.R)
		g += float64(c.G)
		bl += float64(c.B)
		n++
	}
	midY := (rect.Min.Y + rect.Max.Y) / 2
	for x := rect.Min.X; x < rect.Max.X; x++ {
		add(x, rect.Min.Y)
		add(x, rect.Max.Y-1)
		add(x, midY)
	}
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		add(rect.Min.X, y)
		add(rect.Max.X-1, y)
	}
	if n == 0 {
		return fallback
	}
	return color.RGBA{
		R: uint8(r / n),
		G: uint8(g / n),
		B: uint8(bl / n),
		A: 255,
	}
}

func applyMaskFill(img *image.RGBA, rect image.Rectangle, mask []bool, fill color.RGBA) int {
	w, h := rect.Dx(), rect.Dy()
	if len(mask) != w*h {
		return 0
	}
	changed := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !mask[y*w+x] {
				continue
			}
			img.SetRGBA(rect.Min.X+x, rect.Min.Y+y, fill)
			changed++
		}
	}
	return changed
}

func preciseMaskEraseRegion(img *image.RGBA, rect image.Rectangle) EraseStats {
	rect = rect.Intersect(img.Bounds())
	if rect.Empty() {
		return EraseStats{}
	}
	bg := averageBorderColor(img, rect)
	mask := textStrokeMask(img, rect, bg)
	mask = dilateMask(mask, rect.Dx(), rect.Dy(), 1)
	changed := maskedInpaintRegion(img, rect, mask, 5)
	area := rect.Dx() * rect.Dy()
	stats := EraseStats{
		ErasePixels: changed,
		PatchPixels: changed,
	}
	if area > 0 {
		stats.FlatFillRatio = float64(changed) / float64(area)
	}
	return stats
}

func textStrokeMask(img *image.RGBA, rect image.Rectangle, bg color.RGBA) []bool {
	w, h := rect.Dx(), rect.Dy()
	mask := make([]bool, w*h)
	bgLum := luminance(bg)
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			c := img.RGBAAt(x, y)
			delta := colorDistance(c, bg)
			lumDelta := math.Abs(luminance(c) - bgLum)
			if delta >= 54 || lumDelta >= 42 {
				mask[(y-rect.Min.Y)*w+x-rect.Min.X] = true
			}
		}
	}
	return mask
}

func dilateMask(mask []bool, w, h, radius int) []bool {
	if radius <= 0 || len(mask) == 0 {
		return mask
	}
	out := make([]bool, len(mask))
	copy(out, mask)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !mask[y*w+x] {
				continue
			}
			for dy := -radius; dy <= radius; dy++ {
				for dx := -radius; dx <= radius; dx++ {
					nx, ny := x+dx, y+dy
					if nx < 0 || ny < 0 || nx >= w || ny >= h {
						continue
					}
					out[ny*w+nx] = true
				}
			}
		}
	}
	return out
}

func maskedInpaintRegion(img *image.RGBA, rect image.Rectangle, mask []bool, iterations int) int {
	w, h := rect.Dx(), rect.Dy()
	if w <= 0 || h <= 0 || len(mask) != w*h {
		return 0
	}
	changed := 0
	for _, v := range mask {
		if v {
			changed++
		}
	}
	if changed == 0 {
		return 0
	}
	for iter := 0; iter < iterations; iter++ {
		next := cloneRGBA(img)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if !mask[y*w+x] {
					continue
				}
				var r, g, bl, n float64
				for _, d := range [][2]int{{0, 1}, {0, -1}, {1, 0}, {-1, 0}, {1, 1}, {-1, 1}, {1, -1}, {-1, -1}} {
					nx, ny := x+d[0], y+d[1]
					if nx < 0 || ny < 0 || nx >= w || ny >= h || mask[ny*w+nx] {
						continue
					}
					c := img.RGBAAt(rect.Min.X+nx, rect.Min.Y+ny)
					r += float64(c.R)
					g += float64(c.G)
					bl += float64(c.B)
					n++
				}
				if n == 0 {
					continue
				}
				next.SetRGBA(rect.Min.X+x, rect.Min.Y+y, color.RGBA{
					R: uint8(r / n),
					G: uint8(g / n),
					B: uint8(bl / n),
					A: 255,
				})
			}
		}
		*img = *next
	}
	return changed
}

func colorDistance(a, b color.RGBA) float64 {
	dr := float64(a.R) - float64(b.R)
	dg := float64(a.G) - float64(b.G)
	db := float64(a.B) - float64(b.B)
	return math.Sqrt(dr*dr + dg*dg + db*db)
}

func luminance(c color.RGBA) float64 {
	return 0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)
}

func sampleFillRegion(img *image.RGBA, rect image.Rectangle) {
	fill := textOverlayBackgroundColor(img, rect)
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			img.Set(x, y, fill)
		}
	}
}

// textOverlayBackgroundColor estimates the background behind a text overlay by sampling
// border pixels and preferring lighter (non-text) colors.
func textOverlayBackgroundColor(img *image.RGBA, rect image.Rectangle) color.RGBA {
	b := img.Bounds()
	var lightR, lightG, lightB, lightN float64
	var allR, allG, allB, allN float64
	add := func(x, y int) {
		if x < b.Min.X || y < b.Min.Y || x >= b.Max.X || y >= b.Max.Y {
			return
		}
		c := img.RGBAAt(x, y)
		lum := 0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)
		allR += float64(c.R)
		allG += float64(c.G)
		allB += float64(c.B)
		allN++
		if lum >= 110 {
			lightR += float64(c.R)
			lightG += float64(c.G)
			lightB += float64(c.B)
			lightN++
		}
	}
	for x := rect.Min.X; x < rect.Max.X; x++ {
		add(x, rect.Min.Y-1)
		add(x, rect.Min.Y-2)
		add(x, rect.Max.Y)
		add(x, rect.Max.Y+1)
	}
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		add(rect.Min.X-1, y)
		add(rect.Min.X-2, y)
		add(rect.Max.X, y)
		add(rect.Max.X+1, y)
	}
	if lightN > 0 {
		return color.RGBA{
			R: uint8(lightR / lightN),
			G: uint8(lightG / lightN),
			B: uint8(lightB / lightN),
			A: 255,
		}
	}
	if allN > 0 {
		return color.RGBA{
			R: uint8(allR / allN),
			G: uint8(allG / allN),
			B: uint8(allB / allN),
			A: 255,
		}
	}
	return color.RGBA{240, 240, 240, 255}
}

func averageBorderColor(img *image.RGBA, rect image.Rectangle) color.RGBA {
	rect = rect.Intersect(img.Bounds())
	if rect.Empty() {
		return color.RGBA{255, 255, 255, 255}
	}
	var r, g, bl, n float64
	add := func(x, y int) {
		if x < rect.Min.X || y < rect.Min.Y || x >= rect.Max.X || y >= rect.Max.Y {
			return
		}
		c := img.RGBAAt(x, y)
		lum := luminance(c)
		if lum > 245 {
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
		return sampleInteriorBackgroundColor(img, rect)
	}
	return color.RGBA{
		R: uint8(r / n),
		G: uint8(g / n),
		B: uint8(bl / n),
		A: 255,
	}
}

func sampleInteriorBackgroundColor(img *image.RGBA, rect image.Rectangle) color.RGBA {
	b := img.Bounds()
	rect = rect.Intersect(b)
	var lightR, lightG, lightB, lightN float64
	var allR, allG, allB, allN float64
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			c := img.RGBAAt(x, y)
			lum := luminance(c)
			if lum < 70 || lum > 245 {
				continue
			}
			allR += float64(c.R)
			allG += float64(c.G)
			allB += float64(c.B)
			allN++
			if lum >= 145 {
				lightR += float64(c.R)
				lightG += float64(c.G)
				lightB += float64(c.B)
				lightN++
			}
		}
	}
	if lightN > 0 {
		return color.RGBA{
			R: uint8(lightR / lightN),
			G: uint8(lightG / lightN),
			B: uint8(lightB / lightN),
			A: 255,
		}
	}
	if allN > 0 {
		return color.RGBA{
			R: uint8(allR / allN),
			G: uint8(allG / allN),
			B: uint8(allB / allN),
			A: 255,
		}
	}
	return color.RGBA{255, 255, 255, 255}
}

func blurFillRegion(img *image.RGBA, rect image.Rectangle) {
	b := img.Bounds()
	pad := 4
	src := image.Rect(
		max(b.Min.X, rect.Min.X-pad),
		max(b.Min.Y, rect.Min.Y-pad),
		min(b.Max.X, rect.Max.X+pad),
		min(b.Max.Y, rect.Max.Y+pad),
	)
	tmp := make([]color.RGBA, src.Dx()*src.Dy())
	idx := 0
	for y := src.Min.Y; y < src.Max.Y; y++ {
		for x := src.Min.X; x < src.Max.X; x++ {
			tmp[idx] = img.RGBAAt(x, y)
			idx++
		}
	}
	radius := 3
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			var r, g, bl, n float64
			for dy := -radius; dy <= radius; dy++ {
				for dx := -radius; dx <= radius; dx++ {
					sx, sy := x+dx, y+dy
					if sx < src.Min.X || sy < src.Min.Y || sx >= src.Max.X || sy >= src.Max.Y {
						continue
					}
					off := (sy-src.Min.Y)*src.Dx() + (sx - src.Min.X)
					c := tmp[off]
					r += float64(c.R)
					g += float64(c.G)
					bl += float64(c.B)
					n++
				}
			}
			if n == 0 {
				img.Set(x, y, averageBorderColor(img, rect))
				continue
			}
			img.Set(x, y, color.RGBA{
				R: uint8(r / n),
				G: uint8(g / n),
				B: uint8(bl / n),
				A: 255,
			})
		}
	}
}

func inpaintRegion(img *image.RGBA, rect image.Rectangle, iterations int) {
	b := img.Bounds()
	rect = rect.Intersect(b)
	if rect.Empty() {
		return
	}
	mask := make([][]bool, b.Dy())
	for i := range mask {
		mask[i] = make([]bool, b.Dx())
	}
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if x < b.Min.X || y < b.Min.Y || x >= b.Max.X || y >= b.Max.Y {
				continue
			}
			mask[y-b.Min.Y][x-b.Min.X] = true
		}
	}
	for iter := 0; iter < iterations; iter++ {
		next := cloneRGBA(img)
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				if !mask[y-b.Min.Y][x-b.Min.X] {
					continue
				}
				var r, g, bl, n float64
				for _, d := range [][2]int{{0, 1}, {0, -1}, {1, 0}, {-1, 0}, {1, 1}, {-1, 1}, {1, -1}, {-1, -1}} {
					nx, ny := x+d[0], y+d[1]
					if nx < b.Min.X || ny < b.Min.Y || nx >= b.Max.X || ny >= b.Max.Y {
						continue
					}
					if mask[ny-b.Min.Y][nx-b.Min.X] {
						continue
					}
					c := img.RGBAAt(nx, ny)
					r += float64(c.R)
					g += float64(c.G)
					bl += float64(c.B)
					n++
				}
				if n == 0 {
					continue
				}
				next.Set(x, y, color.RGBA{
					R: uint8(r / n),
					G: uint8(g / n),
					B: uint8(bl / n),
					A: 255,
				})
			}
		}
		*img = *next
	}
}

func cloneRGBA(src *image.RGBA) *image.RGBA {
	dst := image.NewRGBA(src.Bounds())
	copy(dst.Pix, src.Pix)
	return dst
}

func contrastTextColor(img *image.RGBA, rect image.Rectangle, style TextStyle) color.RGBA {
	if c := strings.TrimSpace(style.Color); c != "" {
		return parseHexColor(c, color.RGBA{R: 17, G: 17, B: 17, A: 255})
	}
	var lum float64
	n := 0
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			c := img.RGBAAt(x, y)
			lum += 0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)
			n++
		}
	}
	if n == 0 {
		return color.RGBA{17, 17, 17, 255}
	}
	if lum/float64(n) > 140 {
		return color.RGBA{17, 17, 17, 255}
	}
	return color.RGBA{255, 255, 255, 255}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func lineAdvance(fontSize int, ratio float64) int {
	if ratio <= 0 {
		ratio = 1.15
	}
	return int(math.Round(float64(fontSize) * ratio))
}
