package imagerender

import (
	"image"
	"image/color"
	"testing"
)

func TestBuildBestTextPixelMaskLightTextOnGreyWall(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 400, 120))
	for y := 0; y < 120; y++ {
		for x := 0; x < 400; x++ {
			img.Set(x, y, color.RGBA{195, 192, 188, 255})
		}
	}
	for y := 18; y < 42; y++ {
		for x := 220; x < 380; x++ {
			if (x*2+y)%17 < 2 {
				img.Set(x, y, color.RGBA{248, 248, 248, 255})
			}
		}
	}
	rect := image.Rect(210, 12, 390, 48)
	mask, ok := buildBestTextPixelMask(img, rect, "light", 1)
	if !ok {
		t.Fatal("expected mask")
	}
	ratio := float64(countMaskPixels(mask)) / float64(rect.Dx()*rect.Dy())
	if ratio > 0.45 {
		t.Fatalf("mask ratio too high: %.3f", ratio)
	}
	if ratio < 0.002 {
		t.Fatalf("mask ratio too low: %.3f", ratio)
	}
}

func TestBuildBestTextPixelMaskDarkTextOnWhitePill(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 400, 120))
	for y := 0; y < 120; y++ {
		for x := 0; x < 400; x++ {
			img.Set(x, y, color.RGBA{210, 210, 210, 255})
		}
	}
	for y := 14; y < 38; y++ {
		for x := 280; x < 380; x++ {
			img.Set(x, y, color.RGBA{245, 245, 245, 255})
		}
	}
	for y := 20; y < 32; y++ {
		for x := 300; x < 360; x++ {
			if (x*3+y)%9 < 2 {
				img.Set(x, y, color.RGBA{35, 35, 35, 255})
			}
		}
	}
	rect := image.Rect(278, 12, 382, 40)
	mask, ok := buildBestTextPixelMask(img, rect, "dark", 1)
	if !ok {
		t.Fatal("expected mask")
	}
	ratio := float64(countMaskPixels(mask)) / float64(rect.Dx()*rect.Dy())
	if ratio > 0.45 {
		t.Fatalf("mask ratio too high: %.3f", ratio)
	}
}

func TestDetectTextPolarityFromImage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 120, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 120; x++ {
			img.Set(x, y, color.RGBA{240, 240, 240, 255})
		}
	}
	for y := 12; y < 28; y++ {
		for x := 20; x < 100; x++ {
			if x%5 < 2 {
				img.Set(x, y, color.RGBA{30, 30, 30, 255})
			}
		}
	}
	if got := detectTextPolarityFromImage(img, image.Rect(10, 8, 110, 32)); got != "dark" {
		t.Fatalf("polarity = %q, want dark", got)
	}
}
