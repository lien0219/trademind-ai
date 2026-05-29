package imagerender

import (
	"image"
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
