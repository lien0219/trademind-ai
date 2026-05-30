package imagerender

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"strings"
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
	EraseBlocks          int
}

func countEraseBlocks(blocks []TextBlock) int {
	n := 0
	for _, b := range blocks {
		eb := b.EraseBBox
		if eb.Width <= 0 || eb.Height <= 0 {
			eb = b.BBox
		}
		if eb.Width > 0 && eb.Height > 0 {
			n++
		}
	}
	return n
}

// EraseDebugArtifacts holds intermediate PNG bytes for translate debug UI.
type EraseDebugArtifacts struct {
	OriginalPNG []byte
	MaskPNG     []byte
	ErasedPNG   []byte
}

// EraseRegionsWithDebug removes text and returns debug overlays.
func EraseRegionsWithDebug(src image.Image, blocks []TextBlock, opts Options) (*image.RGBA, EraseStats, string, *EraseDebugArtifacts, error) {
	if src == nil {
		return nil, EraseStats{}, "", nil, fmt.Errorf("imagerender: nil source")
	}
	if len(blocks) == 0 {
		return nil, EraseStats{}, "", nil, fmt.Errorf("imagerender: no text blocks")
	}
	original := ToRGBA(src)
	rgba := cloneRGBA(original)
	combinedMask := image.NewRGBA(rgba.Bounds())
	stats, usedErase, err := eraseTextBlocksWithMask(rgba, blocks, opts, combinedMask)
	if err != nil {
		return rgba, stats, usedErase, nil, err
	}
	debug := &EraseDebugArtifacts{}
	if origPNG, _, encErr := Encode(original, "png"); encErr == nil {
		debug.OriginalPNG = origPNG
	}
	if maskPNG, _, encErr := Encode(combinedMask, "png"); encErr == nil {
		debug.MaskPNG = maskPNG
	}
	if erasedPNG, _, encErr := Encode(rgba, "png"); encErr == nil {
		debug.ErasedPNG = erasedPNG
	}
	return rgba, stats, usedErase, debug, nil
}

// EraseRegions removes original text areas without drawing translations.
func EraseRegions(src image.Image, blocks []TextBlock, opts Options) (*image.RGBA, EraseStats, string, error) {
	rgba, stats, usedErase, _, err := EraseRegionsWithDebug(src, blocks, opts)
	return rgba, stats, usedErase, err
}

func eraseTextBlocksWithMask(rgba *image.RGBA, blocks []TextBlock, opts Options, combinedMask *image.RGBA) (EraseStats, string, error) {
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
	usePixelMask := strings.EqualFold(eraseMode, EraseTextPixelMask)
	for _, block := range blocks {
		if usePixelMask {
			blockStats, blockMask, err := eraseTextBlockPixelMaskWithMask(rgba, block, imageArea)
			if err != nil {
				return stats, usedErase, err
			}
			paintBlockMask(combinedMask, block, blockMask)
			if usedErase == "" {
				usedErase = EraseTextPixelMask
			}
			stats.ErasePixels += blockStats.ErasePixels
			stats.PatchPixels += blockStats.PatchPixels
			stats.BackgroundDeltaScore += blockStats.BackgroundDeltaScore
			stats.FlatFillRatio += blockStats.FlatFillRatio * float64(blockStats.PatchPixels)
			stats.LargePatchDetected = stats.LargePatchDetected || blockStats.LargePatchDetected
			continue
		}
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
	if usePixelMask && stats.ErasePixels > 0 {
		totalRatio := float64(stats.ErasePixels) / float64(imageArea)
		if totalRatio > MaxEraseMaskRatioTotal {
			return stats, usedErase, fmt.Errorf("%w: total ratio %.4f", ErrEraseMaskTooLarge, totalRatio)
		}
	}
	return stats, usedErase, nil
}

func paintBlockMask(combined *image.RGBA, block TextBlock, mask []bool) {
	if combined == nil || len(mask) == 0 {
		return
	}
	eraseBox := block.EraseBBox
	if eraseBox.Width <= 0 || eraseBox.Height <= 0 {
		eraseBox = block.BBox
	}
	pad := block.ErasePadding
	if pad <= 0 {
		pad = 1
	}
	if pad > 2 {
		pad = 2
	}
	expanded := expandEraseRect(eraseBox, pad, combined.Bounds().Dx(), combined.Bounds().Dy())
	rect := image.Rect(expanded.X, expanded.Y, expanded.X+expanded.Width, expanded.Y+expanded.Height)
	w, h := rect.Dx(), rect.Dy()
	if len(mask) != w*h {
		return
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !mask[y*w+x] {
				continue
			}
			combined.SetRGBA(rect.Min.X+x, rect.Min.Y+y, color.RGBA{R: 255, G: 40, B: 40, A: 255})
		}
	}
}

// DrawRegions draws translated text on an already-erased image.
func DrawRegions(rgba *image.RGBA, blocks []TextBlock, opts Options) (int, error) {
	if rgba == nil {
		return 0, fmt.Errorf("imagerender: nil canvas")
	}
	drawn := 0
	for _, block := range blocks {
		if len(block.Lines) == 0 {
			continue
		}
		if err := DrawText(rgba, block, opts); err != nil {
			return drawn, fmt.Errorf("imagerender: draw block %s: %w", block.ID, err)
		}
		drawn++
	}
	if drawn == 0 {
		return 0, fmt.Errorf("imagerender: nothing drawn")
	}
	return drawn, nil
}

func eraseTextBlocks(rgba *image.RGBA, blocks []TextBlock, opts Options) (EraseStats, string, error) {
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
	usePixelMask := strings.EqualFold(eraseMode, EraseTextPixelMask)
	for _, block := range blocks {
		if usePixelMask {
			blockStats, err := eraseTextBlockPixelMask(rgba, block, imageArea)
			if err != nil {
				return stats, usedErase, err
			}
			if usedErase == "" {
				usedErase = EraseTextPixelMask
			}
			stats.ErasePixels += blockStats.ErasePixels
			stats.PatchPixels += blockStats.PatchPixels
			stats.BackgroundDeltaScore += blockStats.BackgroundDeltaScore
			stats.FlatFillRatio += blockStats.FlatFillRatio * float64(blockStats.PatchPixels)
			stats.LargePatchDetected = stats.LargePatchDetected || blockStats.LargePatchDetected
			continue
		}
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
	if usePixelMask && stats.ErasePixels > 0 {
		totalRatio := float64(stats.ErasePixels) / float64(imageArea)
		if totalRatio > MaxEraseMaskRatioTotal {
			return stats, usedErase, fmt.Errorf("%w: total ratio %.4f", ErrEraseMaskTooLarge, totalRatio)
		}
	}
	return stats, usedErase, nil
}

// RenderAndEncode erases original text regions, draws translated text, and encodes output.
func RenderAndEncode(src image.Image, sourceBytes []byte, blocks []TextBlock, opts Options, outputFormat string) (*Result, error) {
	if src == nil {
		return nil, fmt.Errorf("imagerender: nil source")
	}
	if len(blocks) == 0 {
		return nil, fmt.Errorf("imagerender: no text blocks")
	}
	rgba, stats, usedErase, err := EraseRegions(src, blocks, opts)
	if err != nil {
		return nil, err
	}
	drawn, err := DrawRegions(rgba, blocks, opts)
	if err != nil {
		return nil, err
	}
	b := rgba.Bounds()
	imageArea := max(1, b.Dx()*b.Dy())
	data, ct, err := Encode(rgba, outputFormat)
	if err != nil {
		return nil, err
	}
	res := &Result{
		Data:                 data,
		ContentType:          ct,
		EraseMode:            usedErase,
		BlocksDrawn:          drawn,
		EraseBlocks:          countEraseBlocks(blocks),
		SourceSHA256:         SHA256Hex(sourceBytes),
		OutputSHA256:         SHA256Hex(data),
		EraseAreaRatio:       float64(stats.ErasePixels) / float64(imageArea),
		PatchAreaRatio:       float64(stats.PatchPixels) / float64(imageArea),
		BackgroundDeltaScore: stats.BackgroundDeltaScore,
		FlatFillRatio:        stats.FlatFillRatio / float64(max(1, stats.PatchPixels)),
		LargePatchDetected:   stats.LargePatchDetected || float64(stats.PatchPixels)/float64(imageArea) > MaxEraseMaskRatioTotal,
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
