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

func TestPillTextEraseDarkTextUsesLightCapsuleFill(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 360, 180))
	for y := 0; y < 180; y++ {
		for x := 0; x < 360; x++ {
			img.Set(x, y, color.RGBA{150, 160, 168, 255})
		}
	}
	for y := 52; y < 96; y++ {
		for x := 60; x < 320; x++ {
			img.Set(x, y, color.RGBA{238, 240, 242, 255})
		}
	}
	for y := 66; y < 80; y++ {
		for x := 120; x < 260; x++ {
			if x%14 < 3 {
				img.Set(x, y, color.RGBA{20, 20, 20, 255})
			}
		}
	}
	block := TextBlock{
		ID: "pill", BlockClass: "pill",
		EraseBBox:    BBox{X: 60, Y: 52, Width: 260, Height: 44},
		ErasePadding: 1, MaskDilate: 1, TextPolarity: "dark",
		Style: TextStyle{Color: "#111111"},
	}
	stats, err := eraseTextBlockPixelMask(img, block, 360*180)
	if err != nil {
		t.Fatalf("pillTextErase: %v", err)
	}
	if stats.ErasePixels <= 0 {
		t.Fatal("expected text pixels erased")
	}
	if got := luminance(img.RGBAAt(126, 70)); got < 210 {
		t.Fatalf("dark text should be filled with light capsule color, luminance=%.1f", got)
	}
}

func TestForceEraseSourceBlockBoundsRemovesPillBackground(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 360, 180))
	bg := color.RGBA{150, 160, 168, 255}
	for y := 0; y < 180; y++ {
		for x := 0; x < 360; x++ {
			img.Set(x, y, bg)
		}
	}
	for y := 52; y < 96; y++ {
		for x := 60; x < 320; x++ {
			img.Set(x, y, color.RGBA{238, 240, 242, 255})
		}
	}
	for y := 66; y < 80; y++ {
		for x := 120; x < 260; x++ {
			if x%14 < 3 {
				img.Set(x, y, color.RGBA{20, 20, 20, 255})
			}
		}
	}
	stats := ForceEraseSourceBlockBounds(img, []TextBlock{{
		ID: "pill", BlockClass: "pill",
		EraseBBox:    BBox{X: 120, Y: 62, Width: 140, Height: 24},
		ErasePadding: 1,
	}}, 900*900)
	if stats.ErasePixels <= 0 {
		t.Fatal("expected source bounds cleanup")
	}
	if got := img.RGBAAt(180, 70); colorDistance(got, bg) > 12 {
		t.Fatalf("pill background should be replaced by surrounding background, got=%v", got)
	}
	if got := img.RGBAAt(70, 70); colorDistance(got, bg) > 12 {
		t.Fatalf("left pill cap should also be cleaned, got=%v", got)
	}
}

func TestForceEraseSourceBlockBoundsAllowsLargeTopRightPillCleanup(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 987, 987))
	bg := color.RGBA{154, 162, 170, 255}
	for y := 0; y < 987; y++ {
		for x := 0; x < 987; x++ {
			img.Set(x, y, bg)
		}
	}
	for y := 180; y < 232; y++ {
		for x := 545; x < 965; x++ {
			img.Set(x, y, color.RGBA{58, 62, 66, 255})
		}
	}
	stats := ForceEraseSourceBlockBounds(img, []TextBlock{{
		ID: "pill", BlockClass: "pill",
		EraseBBox:    BBox{X: 601, Y: 185, Width: 357, Height: 67},
		ErasePadding: 1,
	}}, 987*987)
	if stats.ErasePixels <= 0 {
		t.Fatal("expected large pill cleanup")
	}
	if ratio := float64(stats.ErasePixels) / float64(987*987); ratio <= MaxEraseMaskRatioTotal || ratio > 0.12 {
		t.Fatalf("cleanup ratio should be allowed for pill only, got %.4f", ratio)
	}
	if got := img.RGBAAt(700, 205); colorDistance(got, bg) > 24 {
		t.Fatalf("old pill background should be removed, got=%v", got)
	}
}

func TestSourceDecorCleanupRectExpandsTitleEdges(t *testing.T) {
	rect := sourceDecorCleanupRect(
		BBox{X: 100, Y: 80, Width: 180, Height: 60},
		"title",
		1,
		image.Rect(0, 0, 400, 300),
	)
	if rect.Min.X >= 96 || rect.Min.Y >= 76 || rect.Max.X <= 284 || rect.Max.Y <= 144 {
		t.Fatalf("title cleanup rect not expanded enough: %v", rect)
	}
	if rect.Dx()*rect.Dy() > 240*110 {
		t.Fatalf("title cleanup rect expanded too much: %v", rect)
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
