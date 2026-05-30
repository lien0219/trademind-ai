package imagerender

import (
	"image"
	"image/color"
	"math"
	"sort"
)

func saturation(c color.RGBA) float64 {
	maxC := float64(intMax(int(c.R), intMax(int(c.G), int(c.B))))
	minC := float64(intMin(int(c.R), intMin(int(c.G), int(c.B))))
	if maxC <= 0 {
		return 0
	}
	return (maxC - minC) / maxC
}

func isDarkTextPixel(c color.RGBA, bg color.RGBA, bgLum float64) bool {
	lum := luminance(c)
	delta := colorDistance(c, bg)
	if delta < 28 {
		return false
	}
	sat := saturation(c)
	if lum < bgLum-18 && lum <= 165 {
		return true
	}
	if sat < 0.22 && lum < bgLum-12 && delta >= 42 {
		return true
	}
	return hasHighContrastEdge(c, bg, bgLum, true)
}

func isLightTextPixel(c color.RGBA, bg color.RGBA, bgLum float64) bool {
	lum := luminance(c)
	delta := colorDistance(c, bg)
	if delta < 28 {
		return false
	}
	if lum > bgLum+18 && lum >= 120 {
		return true
	}
	return hasHighContrastEdge(c, bg, bgLum, false)
}

func hasHighContrastEdge(c color.RGBA, bg color.RGBA, bgLum float64, dark bool) bool {
	lum := luminance(c)
	lumDelta := math.Abs(lum - bgLum)
	if lumDelta < 38 {
		return false
	}
	if dark {
		return lum < bgLum && deltaEdge(c, bg) >= 48
	}
	return lum > bgLum && deltaEdge(c, bg) >= 48
}

func deltaEdge(c, bg color.RGBA) float64 {
	return colorDistance(c, bg)
}

func buildEnhancedTextPixelMask(img *image.RGBA, rect image.Rectangle, polarity string) []bool {
	w, h := rect.Dx(), rect.Dy()
	mask := make([]bool, w*h)
	bgColor, bgLum := estimateRegionBackgroundColor(img, rect)
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

func filterMaskConnectedComponents(mask []bool, w, h, regionArea, imageArea int) []bool {
	if len(mask) != w*h || w <= 0 || h <= 0 {
		return mask
	}
	minNoise := 2
	maxRegion := int(float64(regionArea) * 0.35)
	if maxRegion < 8 {
		maxRegion = 8
	}
	maxImage := int(float64(imageArea) * MaxEraseMaskRatioPerBlock)
	if maxImage < maxRegion {
		maxRegion = maxImage
	}
	connected := dilateMask(mask, w, h, 1)
	labels := make([]int, len(connected))
	label := 0
	sizes := map[int]int{}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := y*w + x
			if !connected[idx] || labels[idx] != 0 {
				continue
			}
			label++
			size := floodFillLabel(connected, labels, w, h, x, y, label)
			sizes[label] = size
		}
	}
	out := make([]bool, len(mask))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := y*w + x
			if !mask[idx] {
				continue
			}
			l := labels[idx]
			if l == 0 {
				out[idx] = true
				continue
			}
			size := sizes[l]
			if size > maxRegion {
				continue
			}
			if size < minNoise {
				continue
			}
			out[idx] = true
		}
	}
	if countMaskPixels(out) < 4 {
		return mask
	}
	return out
}

func floodFillLabel(mask []bool, labels []int, w, h, sx, sy, label int) int {
	type pt struct{ x, y int }
	stack := []pt{{sx, sy}}
	count := 0
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		idx := p.y*w + p.x
		if p.x < 0 || p.y < 0 || p.x >= w || p.y >= h || !mask[idx] || labels[idx] != 0 {
			continue
		}
		labels[idx] = label
		count++
		stack = append(stack,
			pt{p.x + 1, p.y}, pt{p.x - 1, p.y},
			pt{p.x, p.y + 1}, pt{p.x, p.y - 1},
		)
	}
	return count
}

func medianRGB(samples [][3]uint8) color.RGBA {
	if len(samples) == 0 {
		return color.RGBA{128, 128, 128, 255}
	}
	rs := make([]int, len(samples))
	gs := make([]int, len(samples))
	bs := make([]int, len(samples))
	for i, s := range samples {
		rs[i] = int(s[0])
		gs[i] = int(s[1])
		bs[i] = int(s[2])
	}
	sort.Ints(rs)
	sort.Ints(gs)
	sort.Ints(bs)
	mid := len(samples) / 2
	return color.RGBA{
		R: uint8(rs[mid]),
		G: uint8(gs[mid]),
		B: uint8(bs[mid]),
		A: 255,
	}
}

func sampleNonWhiteMedianColor(img *image.RGBA, rect image.Rectangle, excludeMask []bool) color.RGBA {
	w, h := rect.Dx(), rect.Dy()
	var samples [][3]uint8
	add := func(x, y int) {
		if x < rect.Min.X || y < rect.Min.Y || x >= rect.Max.X || y >= rect.Max.Y {
			return
		}
		idx := (y-rect.Min.Y)*w + (x - rect.Min.X)
		if len(excludeMask) == w*h && excludeMask[idx] {
			return
		}
		c := img.RGBAAt(x, y)
		if luminance(c) > 210 {
			return
		}
		samples = append(samples, [3]uint8{c.R, c.G, c.B})
	}
	for x := rect.Min.X; x < rect.Max.X; x++ {
		add(x, rect.Min.Y)
		add(x, rect.Max.Y-1)
	}
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		add(rect.Min.X, y)
		add(rect.Max.X-1, y)
	}
	midY := (rect.Min.Y + rect.Max.Y) / 2
	for x := rect.Min.X + 2; x < rect.Max.X-2; x++ {
		add(x, midY)
	}
	if len(samples) < 4 {
		for y := rect.Min.Y + 2; y < rect.Max.Y-2; y++ {
			for x := rect.Min.X + 2; x < rect.Max.X-2; x++ {
				add(x, y)
				if len(samples) >= 32 {
					break
				}
			}
			if len(samples) >= 32 {
				break
			}
		}
	}
	if len(samples) == 0 {
		return sampleCapsuleBackgroundColor(img, rect, color.RGBA{30, 30, 30, 255})
	}
	return medianRGB(samples)
}

func renderMaskOverlay(base *image.RGBA, rect image.Rectangle, mask []bool) *image.RGBA {
	out := cloneRGBA(base)
	w, h := rect.Dx(), rect.Dy()
	if len(mask) != w*h {
		return out
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !mask[y*w+x] {
				continue
			}
			out.SetRGBA(rect.Min.X+x, rect.Min.Y+y, color.RGBA{R: 255, G: 40, B: 40, A: 200})
		}
	}
	return out
}

func intMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func intMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}
