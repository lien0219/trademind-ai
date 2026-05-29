package imagetask

import (
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

func decodePayloadBytes(payload *translateImagePayload) ([]byte, error) {
	if payload == nil || len(payload.RawBytes) == 0 {
		return nil, fmt.Errorf("empty image payload")
	}
	return payload.RawBytes, nil
}

func inferBlockStyles(payload *translateImagePayload, blocks []translateTextBlock) {
	if payload == nil || len(payload.RawBytes) == 0 {
		return
	}
	img, _, err := image.Decode(bytesReader(payload.RawBytes))
	if err != nil {
		return
	}
	bounds := img.Bounds()
	for i := range blocks {
		bb := blocks[i].BBox
		if bb.Width <= 0 || bb.Height <= 0 {
			continue
		}
		if blocks[i].Style.Align == "" {
			blocks[i].Style.Align = "left"
		}
		if blocks[i].Style.Color != "" {
			continue
		}
		var rSum, gSum, bSum, n float64
		x0 := bb.X
		y0 := bb.Y
		x1 := bb.X + bb.Width
		y1 := bb.Y + bb.Height
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				if x < bounds.Min.X || y < bounds.Min.Y || x >= bounds.Max.X || y >= bounds.Max.Y {
					continue
				}
				c := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
				rSum += float64(c.R)
				gSum += float64(c.G)
				bSum += float64(c.B)
				n++
			}
		}
		if n == 0 {
			continue
		}
		lum := 0.299*rSum/n + 0.587*gSum/n + 0.114*bSum/n
		if lum > 140 {
			blocks[i].Style.Color = "#111111"
		} else {
			blocks[i].Style.Color = "#ffffff"
		}
	}
}
