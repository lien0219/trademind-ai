package imagerender

import (
	"image"
	"image/color"
	"testing"
)

func TestSampleFillCoversDarkText(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 200, 80))
	for y := 0; y < 80; y++ {
		for x := 0; x < 200; x++ {
			img.Set(x, y, color.RGBA{230, 230, 230, 255})
		}
	}
	for y := 20; y < 60; y++ {
		for x := 20; x < 180; x++ {
			img.Set(x, y, color.RGBA{20, 20, 20, 255})
		}
	}
	rect := image.Rect(20, 20, 180, 60)
	sampleFillRegion(img, rect)
	for y := 20; y < 60; y++ {
		for x := 20; x < 180; x++ {
			c := img.RGBAAt(x, y)
			lum := 0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)
			if lum < 150 {
				t.Fatalf("dark pixel remains at %d,%d lum=%.0f", x, y, lum)
			}
		}
	}
}
