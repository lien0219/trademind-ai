package imagetask

import (
	"context"
)

func layoutSummaryOverflowFails(hints map[string]any) bool {
	if hints == nil {
		return true
	}
	if _, ok := hints["allowTextOverflow"]; ok {
		return !boolFromHints(hints, "allowTextOverflow", false)
	}
	return !boolFromHints(hints, "allowTextBoxExpand", false)
}

func logTranslateCoordMapping(ctx context.Context, s *Service, task *ImageTask, ocr *translateOCRResult, meta translateCoordinateMeta) {
	if s == nil || task == nil || ocr == nil {
		return
	}
	fields := map[string]any{
		"originalImageWidth":  meta.OriginalImageWidth,
		"originalImageHeight": meta.OriginalImageHeight,
		"ocrImageWidth":       meta.OCRImageWidth,
		"ocrImageHeight":      meta.OCRImageHeight,
		"renderImageWidth":    meta.RenderImageWidth,
		"renderImageHeight":   meta.RenderImageHeight,
		"cropOffsetX":         meta.CropOffsetX,
		"cropOffsetY":         meta.CropOffsetY,
		"cropRenderWidth":     meta.CropRenderWidth,
		"cropRenderHeight":    meta.CropRenderHeight,
		"scaleX":              meta.ScaleX,
		"scaleY":              meta.ScaleY,
		"aspectRatioDiff":     meta.AspectRatioDiff,
		"mappingMode":         meta.MappingMode,
		"mappingTier":         meta.MappingTier,
		"finalMappedBox":      formatMappedBoxLogs(meta),
	}
	if meta.Fallback != "" {
		fields["fallback"] = meta.Fallback
	}
	if meta.GroupRelayoutFallback {
		fields["groupRelayoutFallback"] = true
	}
	s.logTranslateAudit(ctx, task, "ai_image.translate_text.coord_mapped", "success", translateAuditMsg(task, fields))
}
