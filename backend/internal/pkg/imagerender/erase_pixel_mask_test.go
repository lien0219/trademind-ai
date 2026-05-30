package imagerender

import (
	"image"
	"image/color"
	"testing"
)

func TestTextPixelMaskEraseOnlyStrokes(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 900, 900))
	for y := 0; y < 900; y++ {
		for x := 0; x < 900; x++ {
			img.Set(x, y, color.RGBA{230, 230, 230, 255})
		}
	}
	for y := 610; y < 650; y++ {
		for x := 640; x < 790; x++ {
			if (x*5+y*7)%23 < 1 {
				img.Set(x, y, color.RGBA{20, 20, 20, 255})
			}
		}
	}
	block := TextBlock{
		ID: "t1", BlockClass: "title",
		EraseBBox:    BBox{X: 610, Y: 608, Width: 180, Height: 44},
		ErasePadding: 1, MaskDilate: 1, TextPolarity: "dark",
	}
	stats, err := eraseTextBlockPixelMask(img, block, 900*900)
	if err != nil {
		t.Fatalf("erase: %v", err)
	}
	if stats.ErasePixels <= 0 {
		t.Fatal("expected erased pixels")
	}
	if float64(stats.ErasePixels)/float64(900*900) > MaxEraseMaskRatioPerBlock {
		t.Fatalf("erase ratio too high: %d", stats.ErasePixels)
	}
}

func TestTextPixelMaskEraseAreaLimitPerBlock(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{230, 230, 230, 255})
		}
	}
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			if (x+y)%2 == 0 {
				img.Set(x, y, color.RGBA{20, 20, 20, 255})
			}
		}
	}
	block := TextBlock{
		ID: "big", BlockClass: "title",
		EraseBBox:    BBox{X: 0, Y: 0, Width: 100, Height: 100},
		ErasePadding: 0, MaskDilate: 1, TextPolarity: "dark",
	}
	_, err := eraseTextBlockPixelMask(img, block, 100*100)
	if err == nil {
		t.Fatal("expected erase area limit error")
	}
}

func TestTextPixelMaskBadgeFill(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 900, 900))
	for y := 0; y < 900; y++ {
		for x := 0; x < 900; x++ {
			img.Set(x, y, color.RGBA{240, 240, 240, 255})
		}
	}
	for y := 180; y < 228; y++ {
		for x := 550; x < 880; x++ {
			img.Set(x, y, color.RGBA{18, 18, 18, 255})
		}
	}
	for x := 600; x < 820; x++ {
		if x%9 < 2 {
			img.Set(x, 200, color.RGBA{250, 250, 250, 255})
		}
	}
	block := TextBlock{
		ID: "badge", BlockClass: "badge",
		EraseBBox:    BBox{X: 550, Y: 180, Width: 330, Height: 48},
		ErasePadding: 1, MaskDilate: 1, TextPolarity: "light",
		Style: TextStyle{BackgroundColor: "#111111", Color: "#ffffff"},
	}
	_, _, _, err := EraseRegions(img, []TextBlock{block}, Options{EraseMode: EraseTextPixelMask})
	if err != nil {
		t.Fatalf("erase: %v", err)
	}
	if luminance(img.RGBAAt(560, 185)) > 80 {
		t.Fatal("capsule background should remain dark at corner")
	}
}
