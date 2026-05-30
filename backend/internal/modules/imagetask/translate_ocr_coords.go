package imagetask

import (
	"fmt"
	"math"
)

const (
	warningOCRCoordScaled   = "ocr_coord_scaled"
	warningOCRCoordMismatch = "ocr_coord_mismatch"

	mappingModeDirect     = "direct"
	mappingModeNormalized = "normalized"
	mappingModeCrop       = "crop"
	mappingModeUnsafe     = "unsafe"

	mappingTierExact  = "exact"
	mappingTierSmall  = "small"
	mappingTierMedium = "medium"
	mappingTierLarge  = "large"

	coordSmallAspectDiffMax   = 0.03
	coordSmallScaleMin        = 0.5
	coordSmallScaleMax        = 2.0
	coordSmallScaleSkewMax    = 0.15
	coordMediumAspectDiffMax  = 0.08
	coordMediumScaleMin       = 0.3
	coordMediumScaleMax       = 3.0
	coordMediumScaleSkewMax   = 0.30
	coordMappingDirectEpsilon = 0.02
)

type mappedBoxLog struct {
	BlockID string `json:"blockId,omitempty"`
	Box     string `json:"box,omitempty"`
}

// translateCoordinateMeta records OCR vs render image coordinate mapping.
type translateCoordinateMeta struct {
	OriginalImageWidth    int            `json:"originalImageWidth"`
	OriginalImageHeight   int            `json:"originalImageHeight"`
	OCRImageWidth         int            `json:"ocrImageWidth"`
	OCRImageHeight        int            `json:"ocrImageHeight"`
	RenderImageWidth      int            `json:"renderImageWidth"`
	RenderImageHeight     int            `json:"renderImageHeight"`
	CropOffsetX           int            `json:"cropOffsetX"`
	CropOffsetY           int            `json:"cropOffsetY"`
	CropRenderWidth       int            `json:"cropRenderWidth"`
	CropRenderHeight      int            `json:"cropRenderHeight"`
	CropScaleX            float64        `json:"cropScaleX"`
	CropScaleY            float64        `json:"cropScaleY"`
	ScaleX                float64        `json:"scaleX"`
	ScaleY                float64        `json:"scaleY"`
	AspectRatioDiff       float64        `json:"aspectRatioDiff"`
	MappingMode           string         `json:"mappingMode"`
	MappingTier           string         `json:"mappingTier"`
	GroupRelayoutFallback bool           `json:"groupRelayoutFallback,omitempty"`
	Fallback              string         `json:"fallback,omitempty"`
	CoordScaleApplied     bool           `json:"coordScaleApplied"`
	BBoxCorrectionCount   int            `json:"bboxCorrectionCount"`
	CoordMappingValid     bool           `json:"coordMappingValid"`
	FinalMappedBoxes      []mappedBoxLog `json:"finalMappedBoxes,omitempty"`
}

type safeCoordMapperInput struct {
	OCRImageWidth       int
	OCRImageHeight      int
	RenderImageWidth    int
	RenderImageHeight   int
	OriginalImageWidth  int
	OriginalImageHeight int
	CropOffsetX         int
	CropOffsetY         int
	CropRenderWidth     int
	CropRenderHeight    int
	HasCrop             bool
}

func coordMappingUnsafeError() error {
	return newTranslateErr(errCodeTranslateRenderFail,
		"OCR 图与渲染图尺寸差异超过安全映射阈值，系统已删除旧结果和中间文件。请检查 OCR 输入图是否为原图/裁切图，或更换图片重试。")
}

func parseSafeCoordMapperInput(hints map[string]any, renderW, renderH, originalW, originalH, inferredOCRW, inferredOCRH int) safeCoordMapperInput {
	in := safeCoordMapperInput{
		RenderImageWidth:    renderW,
		RenderImageHeight:   renderH,
		OriginalImageWidth:  originalW,
		OriginalImageHeight: originalH,
		OCRImageWidth:       intFromAny(hints["ocrImageWidth"]),
		OCRImageHeight:      intFromAny(hints["ocrImageHeight"]),
		CropOffsetX:         intFromAny(hints["cropOffsetX"]),
		CropOffsetY:         intFromAny(hints["cropOffsetY"]),
		CropRenderWidth:     intFromAny(hints["cropRenderWidth"]),
		CropRenderHeight:    intFromAny(hints["cropRenderHeight"]),
	}
	if in.OriginalImageWidth <= 0 {
		in.OriginalImageWidth = renderW
	}
	if in.OriginalImageHeight <= 0 {
		in.OriginalImageHeight = renderH
	}
	if in.OCRImageWidth <= 0 {
		in.OCRImageWidth = inferredOCRW
	}
	if in.OCRImageHeight <= 0 {
		in.OCRImageHeight = inferredOCRH
	}
	if in.OCRImageWidth <= 0 {
		in.OCRImageWidth = in.OriginalImageWidth
	}
	if in.OCRImageHeight <= 0 {
		in.OCRImageHeight = in.OriginalImageHeight
	}
	if in.CropRenderWidth > 0 && in.CropRenderHeight > 0 {
		in.HasCrop = true
	}
	return in
}

func aspectRatioOf(w, h int) float64 {
	if w <= 0 || h <= 0 {
		return 0
	}
	return float64(w) / float64(h)
}

func aspectRatioDiff(aW, aH, bW, bH int) float64 {
	a := aspectRatioOf(aW, aH)
	b := aspectRatioOf(bW, bH)
	if a <= 0 || b <= 0 {
		return 0
	}
	return math.Abs(a-b) / b
}

func classifyCoordMappingTier(scaleX, scaleY, aspectDiff float64) string {
	if math.Abs(scaleX-1) <= coordMappingDirectEpsilon &&
		math.Abs(scaleY-1) <= coordMappingDirectEpsilon &&
		aspectDiff <= coordSmallAspectDiffMax {
		return mappingTierExact
	}
	if aspectDiff <= coordSmallAspectDiffMax &&
		scaleX >= coordSmallScaleMin && scaleX <= coordSmallScaleMax &&
		scaleY >= coordSmallScaleMin && scaleY <= coordSmallScaleMax &&
		math.Abs(scaleX-scaleY) <= coordSmallScaleSkewMax {
		return mappingTierSmall
	}
	if aspectDiff <= coordMediumAspectDiffMax &&
		scaleX >= coordMediumScaleMin && scaleX <= coordMediumScaleMax &&
		scaleY >= coordMediumScaleMin && scaleY <= coordMediumScaleMax &&
		math.Abs(scaleX-scaleY) <= coordMediumScaleSkewMax {
		return mappingTierMedium
	}
	return mappingTierLarge
}

func normalizedMapBBox(bb translateTextBBox, in safeCoordMapperInput, meta *translateCoordinateMeta) translateTextBBox {
	ocrW := float64(in.OCRImageWidth)
	ocrH := float64(in.OCRImageHeight)
	if ocrW <= 0 || ocrH <= 0 {
		return bb
	}
	nx := float64(bb.X) / ocrW
	ny := float64(bb.Y) / ocrH
	nw := float64(bb.Width) / ocrW
	nh := float64(bb.Height) / ocrH
	if in.HasCrop {
		meta.MappingMode = mappingModeCrop
		meta.CropScaleX = float64(in.CropRenderWidth) / ocrW
		meta.CropScaleY = float64(in.CropRenderHeight) / ocrH
		x := float64(in.CropOffsetX) + nx*float64(in.CropRenderWidth)
		y := float64(in.CropOffsetY) + ny*float64(in.CropRenderHeight)
		w := nw * float64(in.CropRenderWidth)
		h := nh * float64(in.CropRenderHeight)
		return translateTextBBox{
			X:      int(math.Round(x)),
			Y:      int(math.Round(y)),
			Width:  maxInt(1, int(math.Round(w))),
			Height: maxInt(1, int(math.Round(h))),
		}
	}
	meta.MappingMode = mappingModeNormalized
	return translateTextBBox{
		X:      int(math.Round(nx * float64(in.RenderImageWidth))),
		Y:      int(math.Round(ny * float64(in.RenderImageHeight))),
		Width:  maxInt(1, int(math.Round(nw*float64(in.RenderImageWidth)))),
		Height: maxInt(1, int(math.Round(nh*float64(in.RenderImageHeight)))),
	}
}

func bboxOutOfBoundsRatio(bb translateTextBBox, renderW, renderH int) float64 {
	if renderW <= 0 || renderH <= 0 || bb.Width <= 0 || bb.Height <= 0 {
		return 0
	}
	area := float64(bb.Width * bb.Height)
	if area <= 0 {
		return 0
	}
	x0 := maxInt(0, bb.X)
	y0 := maxInt(0, bb.Y)
	x1 := minInt(renderW, bb.X+bb.Width)
	y1 := minInt(renderH, bb.Y+bb.Height)
	if x1 <= x0 || y1 <= y0 {
		return 1
	}
	inside := float64((x1 - x0) * (y1 - y0))
	outside := area - inside
	if outside <= 0 {
		return 0
	}
	return outside / area
}

func bboxAreaAbnormal(bb translateTextBBox, renderW, renderH int) bool {
	if renderW <= 0 || renderH <= 0 {
		return false
	}
	area := float64(bb.Width * bb.Height)
	renderArea := float64(renderW * renderH)
	if area <= 0 {
		return true
	}
	if area > renderArea*0.45 {
		return true
	}
	if bb.Width > renderW*2 || bb.Height > renderH*2 {
		return true
	}
	return false
}

func applyOCRCoordinateMapping(ocr *translateOCRResult, renderW, renderH, originalW, originalH int, hints map[string]any) (translateCoordinateMeta, error) {
	meta := translateCoordinateMeta{
		OriginalImageWidth:  originalW,
		OriginalImageHeight: originalH,
		RenderImageWidth:    renderW,
		RenderImageHeight:   renderH,
		CropScaleX:          1,
		CropScaleY:          1,
		ScaleX:              1,
		ScaleY:              1,
		MappingMode:         mappingModeDirect,
		MappingTier:         mappingTierExact,
		CoordMappingValid:   true,
	}
	if originalW <= 0 {
		meta.OriginalImageWidth = renderW
	}
	if originalH <= 0 {
		meta.OriginalImageHeight = renderH
	}
	if ocr == nil || len(ocr.Blocks) == 0 || renderW <= 0 || renderH <= 0 {
		return meta, nil
	}

	inferredW, inferredH := inferOCRImageExtents(ocr.Blocks)
	in := parseSafeCoordMapperInput(hints, renderW, renderH, originalW, originalH, inferredW, inferredH)
	meta.OCRImageWidth = in.OCRImageWidth
	meta.OCRImageHeight = in.OCRImageHeight
	meta.CropOffsetX = in.CropOffsetX
	meta.CropOffsetY = in.CropOffsetY
	meta.CropRenderWidth = in.CropRenderWidth
	meta.CropRenderHeight = in.CropRenderHeight

	targetW, targetH := in.RenderImageWidth, in.RenderImageHeight
	if in.HasCrop {
		targetW, targetH = in.CropRenderWidth, in.CropRenderHeight
	}
	meta.ScaleX = float64(targetW) / float64(maxInt(1, in.OCRImageWidth))
	meta.ScaleY = float64(targetH) / float64(maxInt(1, in.OCRImageHeight))
	meta.AspectRatioDiff = aspectRatioDiff(in.OCRImageWidth, in.OCRImageHeight, targetW, targetH)

	tier := classifyCoordMappingTier(meta.ScaleX, meta.ScaleY, meta.AspectRatioDiff)
	meta.MappingTier = tier
	if tier == mappingTierExact && !in.HasCrop {
		meta.MappingMode = mappingModeDirect
		for _, b := range ocr.Blocks {
			meta.FinalMappedBoxes = append(meta.FinalMappedBoxes, mappedBoxLog{
				BlockID: b.ID,
				Box:     fmt.Sprintf("%d,%d,%dx%d", b.BBox.X, b.BBox.Y, b.BBox.Width, b.BBox.Height),
			})
		}
		return meta, nil
	}
	if tier == mappingTierLarge {
		meta.MappingMode = mappingModeUnsafe
		meta.CoordMappingValid = false
		return meta, coordMappingUnsafeError()
	}

	if tier == mappingTierMedium {
		meta.GroupRelayoutFallback = true
		meta.Fallback = "group_relayout"
	}

	mapped := make([]translateTextBlock, len(ocr.Blocks))
	maxOutside := 0.0
	for i, b := range ocr.Blocks {
		mapped[i] = b
		mapped[i].BBox = normalizedMapBBox(b.BBox, in, &meta)
		if len(b.Polygon) > 0 {
			mapped[i].Polygon = make([]translateTextPoint, len(b.Polygon))
			for j, p := range b.Polygon {
				pt := normalizedMapBBox(translateTextBBox{X: p.X, Y: p.Y, Width: 1, Height: 1}, in, &meta)
				mapped[i].Polygon[j] = translateTextPoint{X: pt.X, Y: pt.Y}
			}
		}
		if ratio := bboxOutOfBoundsRatio(mapped[i].BBox, renderW, renderH); ratio > maxOutside {
			maxOutside = ratio
		}
		if tier != mappingTierMedium && bboxAreaAbnormal(mapped[i].BBox, renderW, renderH) {
			meta.MappingMode = mappingModeUnsafe
			meta.CoordMappingValid = false
			return meta, coordMappingUnsafeError()
		}
		meta.FinalMappedBoxes = append(meta.FinalMappedBoxes, mappedBoxLog{
			BlockID: b.ID,
			Box:     fmt.Sprintf("%d,%d,%dx%d", mapped[i].BBox.X, mapped[i].BBox.Y, mapped[i].BBox.Width, mapped[i].BBox.Height),
		})
	}
	if tier != mappingTierMedium && maxOutside > 0.35 {
		meta.MappingMode = mappingModeUnsafe
		meta.CoordMappingValid = false
		return meta, coordMappingUnsafeError()
	}

	meta.CoordScaleApplied = true
	meta.BBoxCorrectionCount = len(mapped)
	ocr.Blocks = clampOCRBlockBBoxes(mapped, renderW, renderH)
	return meta, nil
}

func inferOCRImageExtents(blocks []translateTextBlock) (int, int) {
	maxX, maxY := 0, 0
	for _, b := range blocks {
		if b.BBox.Width <= 0 || b.BBox.Height <= 0 {
			continue
		}
		rx := b.BBox.X + b.BBox.Width
		ry := b.BBox.Y + b.BBox.Height
		if rx > maxX {
			maxX = rx
		}
		if ry > maxY {
			maxY = ry
		}
		for _, p := range b.Polygon {
			if p.X > maxX {
				maxX = p.X
			}
			if p.Y > maxY {
				maxY = p.Y
			}
		}
	}
	return maxX, maxY
}

func renderDimensionsFromBytes(data []byte) (int, int, error) {
	if len(data) == 0 {
		return 0, 0, nil
	}
	payload := payloadFromImageBytes(data, "", "")
	if payload == nil {
		return 0, 0, nil
	}
	return payload.Width, payload.Height, nil
}
