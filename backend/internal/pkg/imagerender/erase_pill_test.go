package imagerender

import (
	"image"
	"image/color"
	"testing"
)

func TestPillTextErasePreservesCapsule(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 900, 900))
	for y := 0; y < 900; y++ {
		for x := 0; x < 900; x++ {
			img.Set(x, y, color.RGBA{235, 235, 235, 255})
		}
	}
	for y := 180; y < 228; y++ {
		for x := 550; x < 880; x++ {
			img.Set(x, y, color.RGBA{22, 22, 22, 255})
		}
	}
	for dx := 80; dx < 250; dx++ {
		if dx%5 < 2 {
			img.Set(550+dx, 200, color.RGBA{248, 248, 248, 255})
		}
	}
	block := TextBlock{
		ID: "pill", BlockClass: "badge",
		EraseBBox:    BBox{X: 550, Y: 180, Width: 330, Height: 48},
		ErasePadding: 1, MaskDilate: 1, TextPolarity: "light",
		Style: TextStyle{BackgroundColor: "#111111", Color: "#ffffff"},
	}
	stats, err := pillTextErase(img, block, 900*900)
	if err != nil {
		t.Fatalf("pillTextErase: %v", err)
	}
	if stats.ErasePixels <= 0 {
		t.Fatal("expected text pixels erased")
	}
	if float64(stats.ErasePixels)/float64(900*900) > MaxEraseMaskRatioPerBlock {
		t.Fatalf("erase ratio too high: %d", stats.ErasePixels)
	}
	corner := img.RGBAAt(558, 186)
	if luminance(corner) > 90 {
		t.Fatal("capsule corner should remain dark")
	}
}

func TestTitleUsesTeleaInpaintNotWholeRegion(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 400, 400))
	for y := 0; y < 400; y++ {
		for x := 0; x < 400; x++ {
			img.Set(x, y, color.RGBA{230, 230, 230, 255})
		}
	}
	for y := 48; y < 62; y++ {
		for x := 210; x < 310; x++ {
			if (x+y)%5 < 2 {
				img.Set(x, y, color.RGBA{18, 18, 18, 255})
			}
		}
	}
	block := TextBlock{
		ID: "t", BlockClass: "title",
		EraseBBox:    BBox{X: 200, Y: 38, Width: 120, Height: 34},
		ErasePadding: 1, MaskDilate: 1, TextPolarity: "dark",
	}
	stats, err := eraseTextBlockPixelMask(img, block, 400*400)
	if err != nil {
		t.Fatalf("erase: %v", err)
	}
	if stats.ErasePixels <= 0 {
		t.Fatal("expected mask erase")
	}
	if float64(stats.ErasePixels)/float64(400*400) > MaxEraseMaskRatioPerBlock {
		t.Fatalf("too much erased: %d", stats.ErasePixels)
	}
}
