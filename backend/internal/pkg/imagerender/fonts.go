package imagerender

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
)

var (
	fontOnce sync.Once
	fontData []byte
	fontErr  error
)

func resolveFontPath() string {
	if p := strings.TrimSpace(os.Getenv("TRANSLATE_FONT_PATH")); p != "" {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	candidates := []string{
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/opentype/noto/NotoSansCJKsc-Regular.otf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
	}
	if runtime.GOOS == "windows" {
		win := os.Getenv("WINDIR")
		if win == "" {
			win = `C:\Windows`
		}
		fontDir := filepath.Join(win, "Fonts")
		candidates = append([]string{
			filepath.Join(fontDir, "msyh.ttc"),
			filepath.Join(fontDir, "msyhbd.ttc"),
			filepath.Join(fontDir, "simhei.ttf"),
			filepath.Join(fontDir, "arial.ttf"),
		}, candidates...)
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	return ""
}

func loadFontBytes() ([]byte, error) {
	fontOnce.Do(func() {
		if path := resolveFontPath(); path != "" {
			fontData, fontErr = os.ReadFile(path)
			return
		}
		fontData = goregular.TTF
	})
	return fontData, fontErr
}

// NewFace creates a font.Face at the given pixel size.
func NewFace(size float64, bold bool) (font.Face, error) {
	if size < 8 {
		size = 8
	}
	data, err := loadFontBytes()
	if err != nil {
		return nil, fmt.Errorf("imagerender: load font: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("imagerender: empty font data")
	}
	collection, collErr := opentype.ParseCollection(data)
	if collErr == nil && collection != nil {
		ff, err := collection.Font(0)
		if err != nil {
			return nil, fmt.Errorf("imagerender: font index: %w", err)
		}
		return opentype.NewFace(ff, &opentype.FaceOptions{
			Size:    size,
			DPI:     72,
			Hinting: font.HintingFull,
		})
	}
	ff, err := opentype.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("imagerender: parse font: %w", err)
	}
	_ = bold // bold selection via separate font file could be added later
	return opentype.NewFace(ff, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
}

// FontPathForDebug returns the resolved font path or "embedded:goregular".
func FontPathForDebug() string {
	if p := resolveFontPath(); p != "" {
		return p
	}
	return "embedded:goregular"
}
