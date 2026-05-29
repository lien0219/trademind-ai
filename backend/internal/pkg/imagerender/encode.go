package imagerender

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"strings"

	"golang.org/x/image/webp"
)

// Encode encodes RGBA to png/jpeg (webp requested falls back to png for CGO-free builds).
func Encode(img *image.RGBA, format string) ([]byte, string, error) {
	if img == nil {
		return nil, "", fmt.Errorf("imagerender: nil image")
	}
	f := strings.TrimSpace(strings.ToLower(format))
	if f == "" {
		f = "webp"
	}
	switch f {
	case "jpeg", "jpg":
		return encodeJPEG(img)
	case "png", "webp":
		return encodePNG(img)
	default:
		return encodePNG(img)
	}
}

func encodePNG(img *image.RGBA) ([]byte, string, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, "", fmt.Errorf("imagerender: png encode: %w", err)
	}
	return buf.Bytes(), "image/png", nil
}

func encodeJPEG(img *image.RGBA) ([]byte, string, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 92}); err != nil {
		return nil, "", fmt.Errorf("imagerender: jpeg encode: %w", err)
	}
	return buf.Bytes(), "image/jpeg", nil
}

// Decode loads an image from bytes.
func Decode(data []byte) (image.Image, string, error) {
	if len(data) == 0 {
		return nil, "", fmt.Errorf("imagerender: empty data")
	}
	img, format, err := image.Decode(bytes.NewReader(data))
	if err == nil {
		return img, format, nil
	}
	img, err = webp.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("imagerender: decode: %w", err)
	}
	return img, "webp", nil
}
