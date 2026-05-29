package imagetask

import (
	"math"
)

const (
	warningOCRCoordScaled   = "ocr_coord_scaled"
	warningOCRCoordMismatch = "ocr_coord_mismatch"
)

// translateCoordinateMeta records OCR vs render image coordinate mapping.
type translateCoordinateMeta struct {
	OCRImageWidth       int     `json:"ocrImageWidth"`
	OCRImageHeight      int     `json:"ocrImageHeight"`
	RenderImageWidth    int     `json:"renderImageWidth"`
	RenderImageHeight   int     `json:"renderImageHeight"`
	ScaleX              float64 `json:"scaleX"`
	ScaleY              float64 `json:"scaleY"`
	CoordScaleApplied   bool    `json:"coordScaleApplied"`
	BBoxCorrectionCount int     `json:"bboxCorrectionCount"`
	CoordMappingValid   bool    `json:"coordMappingValid"`
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

func scaleTranslateBBox(bb translateTextBBox, scaleX, scaleY float64) translateTextBBox {
	return translateTextBBox{
		X:      int(math.Round(float64(bb.X) * scaleX)),
		Y:      int(math.Round(float64(bb.Y) * scaleY)),
		Width:  maxInt(1, int(math.Round(float64(bb.Width)*scaleX))),
		Height: maxInt(1, int(math.Round(float64(bb.Height)*scaleY))),
	}
}

func scaleTranslateTextBlock(b translateTextBlock, scaleX, scaleY float64) translateTextBlock {
	out := b
	out.BBox = scaleTranslateBBox(b.BBox, scaleX, scaleY)
	if len(b.Polygon) > 0 {
		out.Polygon = make([]translateTextPoint, len(b.Polygon))
		for i, p := range b.Polygon {
			out.Polygon[i] = translateTextPoint{
				X: int(math.Round(float64(p.X) * scaleX)),
				Y: int(math.Round(float64(p.Y) * scaleY)),
			}
		}
	}
	return out
}

func scaleTranslateTextBlocks(blocks []translateTextBlock, scaleX, scaleY float64) []translateTextBlock {
	if len(blocks) == 0 || (scaleX == 1 && scaleY == 1) {
		return blocks
	}
	out := make([]translateTextBlock, len(blocks))
	for i, b := range blocks {
		out[i] = scaleTranslateTextBlock(b, scaleX, scaleY)
	}
	return out
}

func applyOCRCoordinateMapping(ocr *translateOCRResult, renderW, renderH int) (translateCoordinateMeta, error) {
	meta := translateCoordinateMeta{
		RenderImageWidth:  renderW,
		RenderImageHeight: renderH,
		ScaleX:            1,
		ScaleY:            1,
		CoordMappingValid: true,
	}
	if ocr == nil || len(ocr.Blocks) == 0 || renderW <= 0 || renderH <= 0 {
		return meta, nil
	}

	ocrW, ocrH := inferOCRImageExtents(ocr.Blocks)
	meta.OCRImageWidth = ocrW
	meta.OCRImageHeight = ocrH
	if ocrW <= 0 || ocrH <= 0 {
		meta.OCRImageWidth = renderW
		meta.OCRImageHeight = renderH
		return meta, nil
	}

	fitsRender := ocrW <= renderW && ocrH <= renderH
	scaleX := float64(renderW) / float64(ocrW)
	scaleY := float64(renderH) / float64(ocrH)
	meta.ScaleX = scaleX
	meta.ScaleY = scaleY

	var applyScaleX, applyScaleY float64
	switch {
	case ocrW > renderW || ocrH > renderH:
		applyScaleX = float64(renderW) / float64(ocrW)
		applyScaleY = float64(renderH) / float64(ocrH)
		if applyScaleX > applyScaleY {
			applyScaleX = applyScaleY
		} else {
			applyScaleY = applyScaleX
		}
	case fitsRender && ocrW < int(float64(renderW)*0.72) && ocrH < int(float64(renderH)*0.72):
		applyScaleX = scaleX
		applyScaleY = scaleY
	default:
		return meta, nil
	}

	needScale := math.Abs(applyScaleX-1) > 0.02 || math.Abs(applyScaleY-1) > 0.02
	if !needScale {
		return meta, nil
	}

	if applyScaleX < 0.45 || applyScaleX > 2.2 || applyScaleY < 0.45 || applyScaleY > 2.2 {
		meta.CoordMappingValid = false
		return meta, newTranslateErr(errCodeTranslateRenderFail,
			"OCR 坐标与渲染图片尺寸差异过大，无法安全映射，请更换图片或检查 OCR 服务")
	}
	if math.Abs(applyScaleX-applyScaleY) > 0.18 {
		meta.CoordMappingValid = false
		return meta, newTranslateErr(errCodeTranslateRenderFail,
			"OCR 坐标 X/Y 缩放比例不一致，无法安全映射到渲染图")
	}

	meta.ScaleX = applyScaleX
	meta.ScaleY = applyScaleY
	scaled := scaleTranslateTextBlocks(ocr.Blocks, applyScaleX, applyScaleY)
	meta.CoordScaleApplied = true
	meta.BBoxCorrectionCount = len(scaled)
	ocr.Blocks = clampOCRBlockBBoxes(scaled, renderW, renderH)
	return meta, nil
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
