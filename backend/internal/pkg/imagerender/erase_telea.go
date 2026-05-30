package imagerender

import (
	"image"
	"image/color"
	"math"
)

// teleaInpaintMask applies a Telea-style fast marching inpaint limited to mask pixels (radius in pixels).
func teleaInpaintMask(img *image.RGBA, rect image.Rectangle, mask []bool, radius int) int {
	w, h := rect.Dx(), rect.Dy()
	if w <= 0 || h <= 0 || len(mask) != w*h {
		return 0
	}
	if radius <= 0 {
		radius = 2
	}
	if radius > 4 {
		radius = 4
	}
	changed := countMaskPixels(mask)
	if changed == 0 {
		return 0
	}
	type node struct {
		x, y int
		dist float64
	}
	frontier := make([]node, 0, changed)
	dist := make([]float64, w*h)
	for i := range dist {
		dist[i] = math.MaxFloat64
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := y*w + x
			if !mask[idx] {
				continue
			}
			hasKnown := false
			for _, d := range [][2]int{{0, 1}, {0, -1}, {1, 0}, {-1, 0}} {
				nx, ny := x+d[0], y+d[1]
				if nx < 0 || ny < 0 || nx >= w || ny >= h || mask[ny*w+nx] {
					continue
				}
				hasKnown = true
				break
			}
			if hasKnown {
				dist[idx] = 0
				frontier = append(frontier, node{x, y, 0})
			}
		}
	}
	if len(frontier) == 0 {
		return maskedInpaintRegion(img, rect, mask, radius*2)
	}
	maxIter := radius * radius * 8
	if maxIter < 16 {
		maxIter = 16
	}
	for step := 0; step < maxIter && len(frontier) > 0; step++ {
		next := cloneRGBA(img)
		newFrontier := frontier[:0]
		for _, n := range frontier {
			idx := n.y*w + n.x
			if dist[idx] > float64(radius) {
				continue
			}
			var r, g, bl, weight float64
			for _, d := range [][2]int{{0, 1}, {0, -1}, {1, 0}, {-1, 0}, {1, 1}, {-1, 1}, {1, -1}, {-1, -1}} {
				nx, ny := n.x+d[0], n.y+d[1]
				if nx < 0 || ny < 0 || nx >= w || ny >= h {
					continue
				}
				nidx := ny*w + nx
				if mask[nidx] {
					continue
				}
				c := img.RGBAAt(rect.Min.X+nx, rect.Min.Y+ny)
				wgt := 1.0 / (1.0 + dist[idx])
				r += float64(c.R) * wgt
				g += float64(c.G) * wgt
				bl += float64(c.B) * wgt
				weight += wgt
			}
			if weight == 0 {
				continue
			}
			next.SetRGBA(rect.Min.X+n.x, rect.Min.Y+n.y, color.RGBA{
				R: uint8(r / weight),
				G: uint8(g / weight),
				B: uint8(bl / weight),
				A: 255,
			})
			for _, d := range [][2]int{{0, 1}, {0, -1}, {1, 0}, {-1, 0}} {
				nx, ny := n.x+d[0], n.y+d[1]
				if nx < 0 || ny < 0 || nx >= w || ny >= h || !mask[ny*w+nx] {
					continue
				}
				nidx := ny*w + nx
				nd := dist[idx] + 1
				if nd < dist[nidx] {
					dist[nidx] = nd
					newFrontier = append(newFrontier, node{nx, ny, nd})
				}
			}
		}
		*img = *next
		frontier = newFrontier
		if len(frontier) == 0 {
			break
		}
	}
	return changed
}
