package imagerender

import "image/color"

// BBox is a pixel rectangle on the image.
type BBox struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// TextStyle hints for drawing translated text.
type TextStyle struct {
	Color           string `json:"color,omitempty"`
	BackgroundColor string `json:"backgroundColor,omitempty"`
	FontWeight      string `json:"fontWeight,omitempty"`
	Align           string `json:"align,omitempty"`
}

// TextBlock is one region to erase and redraw.
type TextBlock struct {
	ID       string
	Lines    []string
	FontSize int
	BBox     BBox
	Style    TextStyle
	Align    string
	Bold     bool
}

// Options controls erase and padding behavior.
type Options struct {
	EraseMode   string
	MaskPadding int
	TextPadding int
	LineHeight  float64
}

const (
	EraseBackgroundSample = "background_sample"
	EraseBlurFill         = "blur_fill"
	EraseOpenCVInpaint    = "opencv_inpaint"
	EraseAIInpaint        = "ai_inpaint"
	EraseAuto             = "auto"
)

func clampRect(x, y, w, h, imgW, imgH int) (int, int, int, int) {
	minH := 24
	minW := 40
	if w < minW {
		w = minW
	}
	if h < minH {
		h = minH
	}
	if imgW > 0 {
		if x < 0 {
			x = 0
		}
		if x+w > imgW {
			if w > imgW {
				w = imgW
				x = 0
			} else {
				x = imgW - w
			}
		}
		if x < 0 {
			x = 0
		}
	} else if x < 0 {
		x = 0
	}
	if imgH > 0 {
		if y < 0 {
			y = 0
		}
		if y+h > imgH {
			y = imgH - h
		}
		if y < 0 {
			y = 0
			if h > imgH {
				h = imgH
			}
		}
	} else if y < 0 {
		y = 0
	}
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return x, y, w, h
}

func expandRect(b BBox, pad, imgW, imgH int) BBox {
	x, y, w, h := b.X-pad, b.Y-pad, b.Width+pad*2, b.Height+pad*2
	x, y, w, h = clampRect(x, y, w, h, imgW, imgH)
	return BBox{X: x, Y: y, Width: w, Height: h}
}

func parseHexColor(s string, fallback color.RGBA) color.RGBA {
	s = trimLower(s)
	if s == "" {
		return fallback
	}
	if s[0] == '#' {
		s = s[1:]
	}
	if len(s) == 3 {
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	}
	if len(s) != 6 {
		return fallback
	}
	var r, g, b uint8
	if _, err := parseHexBytePair(s[0:2], &r); err != nil {
		return fallback
	}
	if _, err := parseHexBytePair(s[2:4], &g); err != nil {
		return fallback
	}
	if _, err := parseHexBytePair(s[4:6], &b); err != nil {
		return fallback
	}
	return color.RGBA{R: r, G: g, B: b, A: 255}
}

func parseHexBytePair(hex string, out *uint8) (int, error) {
	var v int
	for _, c := range hex {
		v <<= 4
		switch {
		case c >= '0' && c <= '9':
			v += int(c - '0')
		case c >= 'a' && c <= 'f':
			v += int(c - 'a' + 10)
		case c >= 'A' && c <= 'F':
			v += int(c - 'A' + 10)
		default:
			return 0, errInvalidColor
		}
	}
	*out = uint8(v)
	return 2, nil
}

var errInvalidColor = errorString("invalid color")

type errorString string

func (e errorString) Error() string { return string(e) }

func trimLower(s string) string {
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b = append(b, c)
	}
	return string(b)
}
