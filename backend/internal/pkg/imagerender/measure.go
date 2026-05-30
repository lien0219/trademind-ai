package imagerender

import (
	"unicode"

	"golang.org/x/image/font"
)

// MeasureTextWidth returns pixel width for text at the given font size.
func MeasureTextWidth(text string, fontSize int, cjk bool) (float64, error) {
	if fontSize <= 0 {
		fontSize = 14
	}
	face, err := NewFace(float64(fontSize), false)
	if err != nil {
		return estimateFallbackWidth(text, fontSize, cjk), err
	}
	defer face.Close()
	return float64(font.MeasureString(face, text).Ceil()), nil
}

func estimateFallbackWidth(text string, fontSize int, cjk bool) float64 {
	var w float64
	for _, r := range text {
		switch {
		case unicode.IsSpace(r):
			w += float64(fontSize) * 0.28
		case cjk || unicode.Is(unicode.Han, r):
			w += float64(fontSize) * 1.0
		default:
			w += float64(fontSize) * 0.55
		}
	}
	return w
}
