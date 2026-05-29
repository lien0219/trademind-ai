package imagerender

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
)

// Result holds rendered image bytes and metadata.
type Result struct {
	Data         []byte
	ContentType  string
	EraseMode    string
	BlocksDrawn  int
	SourceSHA256 string
	OutputSHA256 string
}

// RenderAndEncode erases original text regions, draws translated text, and encodes output.
func RenderAndEncode(src image.Image, sourceBytes []byte, blocks []TextBlock, opts Options, outputFormat string) (*Result, error) {
	if src == nil {
		return nil, fmt.Errorf("imagerender: nil source")
	}
	if len(blocks) == 0 {
		return nil, fmt.Errorf("imagerender: no text blocks")
	}
	rgba := ToRGBA(src)
	b := rgba.Bounds()
	maskPad := opts.MaskPadding
	if maskPad <= 0 {
		maskPad = 8
	}
	eraseMode := opts.EraseMode
	if eraseMode == "" {
		eraseMode = EraseAuto
	}
	usedErase := ""
	for _, block := range blocks {
		if block.BBox.Width <= 0 || block.BBox.Height <= 0 {
			continue
		}
		expanded := expandRect(block.BBox, maskPad, b.Dx(), b.Dy())
		rect := image.Rect(expanded.X, expanded.Y, expanded.X+expanded.Width, expanded.Y+expanded.Height)
		chosen := chooseEraseMode(eraseMode, rgba, rect)
		if usedErase == "" {
			usedErase = chosen
		}
		eraseRegion(rgba, rect, chosen)
	}
	drawn := 0
	for _, block := range blocks {
		if len(block.Lines) == 0 {
			continue
		}
		if err := DrawText(rgba, block, opts); err != nil {
			return nil, fmt.Errorf("imagerender: draw block %s: %w", block.ID, err)
		}
		drawn++
	}
	if drawn == 0 {
		return nil, fmt.Errorf("imagerender: nothing drawn")
	}
	data, ct, err := Encode(rgba, outputFormat)
	if err != nil {
		return nil, err
	}
	res := &Result{
		Data:         data,
		ContentType:  ct,
		EraseMode:    usedErase,
		BlocksDrawn:  drawn,
		SourceSHA256: SHA256Hex(sourceBytes),
		OutputSHA256: SHA256Hex(data),
	}
	return res, nil
}

// SHA256Hex returns hex-encoded SHA256 of bytes.
func SHA256Hex(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// ImagesEqual compares SHA256 of two byte slices.
func ImagesEqual(a, b []byte) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	return SHA256Hex(a) == SHA256Hex(b)
}
