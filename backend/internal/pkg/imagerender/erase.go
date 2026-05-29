package imagerender

import (
	"image"
	"image/color"
	"math"
	"strings"
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
	case ErasePreciseMask, EraseBackgroundSample, EraseBlurFill, EraseOpenCVInpaint, EraseAIInpaint:
		return m
	}
	variance := borderColorVariance(region, rect)
	switch {
	case variance < 1600:
		return ErasePreciseMask
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
	switch chooseEraseMode(mode, img, rect) {
	case ErasePreciseMask:
		return preciseMaskEraseRegion(img, rect)
	case EraseBlurFill:
		blurFillRegion(img, rect)
	case EraseOpenCVInpaint:
		inpaintRegion(img, rect, 6)
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
	b := img.Bounds()
	var r, g, bl, n float64
	add := func(x, y int) {
		if x < b.Min.X || y < b.Min.Y || x >= b.Max.X || y >= b.Max.Y {
			return
		}
		c := img.RGBAAt(x, y)
		r += float64(c.R)
		g += float64(c.G)
		bl += float64(c.B)
		n++
	}
	for x := rect.Min.X; x < rect.Max.X; x++ {
		add(x, rect.Min.Y-1)
		add(x, rect.Max.Y)
	}
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		add(rect.Min.X-1, y)
		add(rect.Max.X, y)
	}
	if n == 0 {
		return color.RGBA{255, 255, 255, 255}
	}
	return color.RGBA{
		R: uint8(r / n),
		G: uint8(g / n),
		B: uint8(bl / n),
		A: 255,
	}
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
