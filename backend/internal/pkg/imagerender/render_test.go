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
		t.Fatalf("expected background_sample, got %s", mode)
	}
}
