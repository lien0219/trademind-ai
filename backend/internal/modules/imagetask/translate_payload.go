package imagetask

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strings"
)

func decodePayloadBytes(payload *translateImagePayload) ([]byte, error) {
	if payload == nil || len(payload.RawBytes) == 0 {
		return nil, fmt.Errorf("empty image payload")
	}
	return payload.RawBytes, nil
}

func inferBlockStyles(payload *translateImagePayload, blocks []translateTextBlock) {
	var img image.Image
	if payload != nil && len(payload.RawBytes) > 0 {
		if decoded, _, err := image.Decode(bytesReader(payload.RawBytes)); err == nil {
			img = decoded
		}
	}
	for i := range blocks {
		if blocks[i].Style.Align == "" {
			blocks[i].Style.Align = "left"
		}
		if img != nil {
			lum := averageBBoxLuminance(img, blocks[i].BBox)
			if lum < 70 {
				blocks[i].Style.Color = "#ffffff"
				blocks[i].Style.BackgroundColor = "#111111"
				blocks[i].Style.Align = "center"
				blocks[i].Style.FontWeight = "bold"
				blocks[i].Style.BorderRadius = 0
				continue
			}
			if lum < 135 && strings.TrimSpace(blocks[i].Style.Color) == "" {
				blocks[i].Style.Color = "#ffffff"
			}
		}
		if strings.TrimSpace(blocks[i].Style.Color) == "" {
			blocks[i].Style.Color = defaultTranslateTextColor
		}
	}
}

func averageBBoxLuminance(img image.Image, bb translateTextBBox) float64 {
	if img == nil || bb.Width <= 0 || bb.Height <= 0 {
		return 255
	}
	b := img.Bounds()
	x0 := maxInt(b.Min.X, bb.X)
	y0 := maxInt(b.Min.Y, bb.Y)
	x1 := minInt(b.Max.X, bb.X+bb.Width)
	y1 := minInt(b.Max.Y, bb.Y+bb.Height)
	var lum float64
	n := 0
	stepX := maxInt(1, (x1-x0)/24)
	stepY := maxInt(1, (y1-y0)/24)
	for y := y0; y < y1; y += stepY {
		for x := x0; x < x1; x += stepX {
			r, g, bl, _ := img.At(x, y).RGBA()
			lum += 0.299*float64(r>>8) + 0.587*float64(g>>8) + 0.114*float64(bl>>8)
			n++
		}
	}
	if n == 0 {
		return 255
	}
	return lum / float64(n)
}
