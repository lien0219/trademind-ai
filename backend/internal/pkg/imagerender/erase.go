package imagerender

import (
	"image"
	"image/color"
	"math"
	"strings"
)

func chooseEraseMode(mode string, region *image.RGBA, rect image.Rectangle) string {
	m := trimLower(mode)
	if m != "" && m != EraseAuto {
		return m
	}
	variance := borderColorVariance(region, rect)
	switch {
	case variance < 120:
		return EraseBackgroundSample
	case variance < 800:
		return EraseBlurFill
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

func eraseRegion(img *image.RGBA, rect image.Rectangle, mode string) {
	switch chooseEraseMode(mode, img, rect) {
	case EraseBlurFill:
		blurFillRegion(img, rect)
	case EraseOpenCVInpaint:
		inpaintRegion(img, rect, 6)
	default:
		sampleFillRegion(img, rect)
	}
}

func sampleFillRegion(img *image.RGBA, rect image.Rectangle) {
	fill := averageBorderColor(img, rect)
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			img.Set(x, y, fill)
		}
	}
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
	mask := make([][]bool, b.Dy())
	for i := range mask {
		mask[i] = make([]bool, b.Dx())
	}
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
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
