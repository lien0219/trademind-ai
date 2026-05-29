package imagerender

import (
	"image"
	"image/color"
	"strings"

	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// DrawText draws multi-line text inside bbox with alignment.
func DrawText(dst *image.RGBA, block TextBlock, opts Options) error {
	if dst == nil || len(block.Lines) == 0 || block.FontSize <= 0 {
		return nil
	}
	b := dst.Bounds()
	x, y, w, h := clampRect(block.BBox.X, block.BBox.Y, block.BBox.Width, block.BBox.Height, b.Dx(), b.Dy())
	rect := image.Rect(x, y, x+w, y+h)
	textColor := contrastTextColor(dst, rect, block.Style)
	if bg := strings.TrimSpace(block.Style.BackgroundColor); bg != "" {
		radius := effectiveBorderRadius(w, h, block.Style.BorderRadius)
		fillRoundedRect(dst, rect, radius, parseHexColor(bg, color.RGBA{R: 17, G: 17, B: 17, A: 255}))
		textColor = parseHexColor(block.Style.Color, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	}
	align := strings.TrimSpace(strings.ToLower(block.Align))
	if align == "" {
		align = strings.TrimSpace(strings.ToLower(block.Style.Align))
	}
	if align == "" {
		align = "left"
	}
	ratio := opts.LineHeight
	if ratio <= 0 {
		ratio = 1.15
	}
	face, err := NewFace(float64(block.FontSize), block.Bold || strings.EqualFold(block.Style.FontWeight, "bold"))
	if err != nil {
		return err
	}
	defer face.Close()

	lineH := lineAdvance(block.FontSize, ratio)
	totalH := lineH * len(block.Lines)
	startY := y + (h-totalH)/2
	if startY < y {
		startY = y
	}
	padding := opts.TextPadding
	if padding <= 0 {
		padding = 4
	}
	d := &font.Drawer{Dst: dst, Src: &image.Uniform{C: textColor}}
	for i, line := range block.Lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		d.Face = face
		advance := font.MeasureString(face, line)
		lineW := advance.Ceil()
		lineX := x + padding
		switch align {
		case "center", "centre":
			lineX = x + (w-lineW)/2
		case "right":
			lineX = x + w - lineW - padding
		}
		if lineX < x {
			lineX = x
		}
		lineY := startY + i*lineH + block.FontSize
		if lineY > b.Max.Y-1 {
			break
		}
		d.Dot = fixed.Point26_6{
			X: fixed.I(lineX),
			Y: fixed.I(lineY),
		}
		d.DrawString(line)
	}
	return nil
}

// effectiveBorderRadius caps pill/capsule radius so wide badges do not become solid circles.
func effectiveBorderRadius(w, h, requested int) int {
	if w <= 0 || h <= 0 {
		return 4
	}
	maxR := w
	if h < maxR {
		maxR = h
	}
	maxR /= 2
	if maxR < 1 {
		maxR = 1
	}
	if requested > 0 && requested < maxR {
		return requested
	}
	if w >= h*2 {
		return max(4, h/2)
	}
	if requested > maxR {
		requested = maxR
	}
	if requested <= 0 {
		return max(4, maxR)
	}
	return requested
}

func fillRoundedRect(dst *image.RGBA, rect image.Rectangle, radius int, c color.RGBA) {
	rect = rect.Intersect(dst.Bounds())
	if rect.Empty() {
		return
	}
	if radius <= 0 {
		radius = 1
	}
	if radius > rect.Dx()/2 {
		radius = rect.Dx() / 2
	}
	if radius > rect.Dy()/2 {
		radius = rect.Dy() / 2
	}
	r2 := radius * radius
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			cx := x
			if x < rect.Min.X+radius {
				cx = rect.Min.X + radius
			} else if x >= rect.Max.X-radius {
				cx = rect.Max.X - radius - 1
			}
			cy := y
			if y < rect.Min.Y+radius {
				cy = rect.Min.Y + radius
			} else if y >= rect.Max.Y-radius {
				cy = rect.Max.Y - radius - 1
			}
			dx, dy := x-cx, y-cy
			if dx*dx+dy*dy <= r2 {
				dst.SetRGBA(x, y, c)
			}
		}
	}
}

// ToRGBA converts any image to RGBA (copies if needed).
func ToRGBA(src image.Image) *image.RGBA {
	if src == nil {
		return image.NewRGBA(image.Rect(0, 0, 1, 1))
	}
	if rgba, ok := src.(*image.RGBA); ok {
		return rgba
	}
	b := src.Bounds()
	dst := image.NewRGBA(b)
	draw.Draw(dst, b, src, b.Min, draw.Src)
	return dst
}
