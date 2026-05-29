package imagerender

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
)

// Result holds rendered image bytes and metadata.
type Result struct {
	Data                 []byte
	ContentType          string
	EraseMode            string
	BlocksDrawn          int
	SourceSHA256         string
	OutputSHA256         string
	EraseAreaRatio       float64
	PatchAreaRatio       float64
	BackgroundDeltaScore float64
	FlatFillRatio        float64
	LargePatchDetected   bool
	RetryStrategies      []string
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
	var stats EraseStats
	imageArea := max(1, b.Dx()*b.Dy())
	for _, block := range blocks {
		eraseBox := block.EraseBBox
		if eraseBox.Width <= 0 || eraseBox.Height <= 0 {
			eraseBox = block.BBox
		}
		if eraseBox.Width <= 0 || eraseBox.Height <= 0 {
			continue
		}
		pad := maskPad
		if block.ErasePadding > 0 {
			pad = block.ErasePadding
		}
		expanded := expandEraseRect(eraseBox, pad, b.Dx(), b.Dy())
		rect := image.Rect(expanded.X, expanded.Y, expanded.X+expanded.Width, expanded.Y+expanded.Height)
		chosen := chooseEraseMode(eraseMode, rgba, rect)
		if usedErase == "" {
			usedErase = chosen
		}
		blockStats := eraseRegion(rgba, rect, chosen)
		stats.ErasePixels += blockStats.ErasePixels
		stats.PatchPixels += blockStats.PatchPixels
		stats.BackgroundDeltaScore += blockStats.BackgroundDeltaScore
		stats.FlatFillRatio += blockStats.FlatFillRatio * float64(blockStats.PatchPixels)
		stats.LargePatchDetected = stats.LargePatchDetected || blockStats.LargePatchDetected
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
		Data:                 data,
		ContentType:          ct,
		EraseMode:            usedErase,
		BlocksDrawn:          drawn,
		SourceSHA256:         SHA256Hex(sourceBytes),
		OutputSHA256:         SHA256Hex(data),
		EraseAreaRatio:       float64(stats.ErasePixels) / float64(imageArea),
		PatchAreaRatio:       float64(stats.PatchPixels) / float64(imageArea),
		BackgroundDeltaScore: stats.BackgroundDeltaScore,
		FlatFillRatio:        stats.FlatFillRatio / float64(max(1, stats.PatchPixels)),
		LargePatchDetected:   stats.LargePatchDetected || float64(stats.PatchPixels)/float64(imageArea) > 0.08,
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
