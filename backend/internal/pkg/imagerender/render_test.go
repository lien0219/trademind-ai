package imagerender

import (
	"testing"

	"image"
	"image/color"
)

func TestRenderAndEncodeDrawsText(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 400, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 400; x++ {
			src.Set(x, y, color.RGBA{240, 240, 240, 255})
		}
	}
	// simulate dark text region
	for y := 40; y < 82; y++ {
		for x := 32; x < 182; x++ {
			src.Set(x, y, color.RGBA{30, 30, 30, 255})
		}
	}
	raw := make([]byte, 400*200*4)
	copy(raw, src.Pix)

	res, err := RenderAndEncode(src, raw, []TextBlock{{
		ID:       "block_1",
		Lines:    []string{"Metal Base"},
		FontSize: 22,
		BBox:     BBox{X: 32, Y: 40, Width: 150, Height: 42},
		Align:    "left",
	}}, Options{EraseMode: EraseBackgroundSample, MaskPadding: 4, TextPadding: 4, LineHeight: 1.15}, "png")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(res.Data) == 0 {
		t.Fatal("empty output")
	}
	if res.SourceSHA256 == res.OutputSHA256 {
		t.Fatal("output must differ from source")
	}
	if res.BlocksDrawn != 1 {
		t.Fatalf("blocks drawn = %d", res.BlocksDrawn)
	}
}

func TestChooseEraseMode(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{250, 250, 250, 255})
		}
	}
	rect := image.Rect(20, 20, 80, 60)
	mode := chooseEraseMode(EraseAuto, img, rect)
	if mode != EraseBackgroundSample {
		t.Fatalf("expected background_sample for flat region, got %s", mode)
	}
}

func TestImagesEqual(t *testing.T) {
	a := []byte{1, 2, 3}
	b := []byte{1, 2, 3}
	c := []byte{1, 2, 4}
	if !ImagesEqual(a, b) {
		t.Fatal("expected equal")
	}
	if ImagesEqual(a, c) {
		t.Fatal("expected not equal")
	}
}

func TestClampRectOutOfBoundsX(t *testing.T) {
	x, _, w, _ := clampRect(877, 10, 50, 40, 800, 800)
	if x+w > 800 || x < 0 {
		t.Fatalf("rect still out of bounds: x=%d w=%d", x, w)
	}
	if w < 40 {
		t.Fatalf("width too small: %d", w)
	}
}

func TestRenderAndEncodeOutOfBoundsBBoxDoesNotPanic(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 800, 800))
	for y := 0; y < 800; y++ {
		for x := 0; x < 800; x++ {
			src.Set(x, y, color.RGBA{240, 240, 240, 255})
		}
	}
	raw := make([]byte, len(src.Pix))
	copy(raw, src.Pix)
	_, err := RenderAndEncode(src, raw, []TextBlock{{
		ID:       "block_1",
		Lines:    []string{"Metal Base"},
		FontSize: 22,
		BBox:     BBox{X: 877, Y: 0, Width: 420, Height: 120},
		Style:    TextStyle{Color: "#111111", Align: "left"},
	}}, Options{EraseMode: EraseOpenCVInpaint, MaskPadding: 8, TextPadding: 4, LineHeight: 1.15}, "png")
	if err != nil {
		t.Fatalf("render should not panic or fail: %v", err)
	}
}
