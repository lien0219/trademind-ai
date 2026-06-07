package imagetask

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/imagerender"
	imgprov "github.com/trademind-ai/trademind/backend/internal/providers/image"
	"gorm.io/gorm"
)

const (
	manualBaseOriginal = "original"
	manualBaseErased   = "erased"
	manualBaseResult   = "result"
)

type TranslateManualEditState struct {
	TaskID           string                     `json:"taskId"`
	OriginalImageURL string                     `json:"originalImageUrl,omitempty"`
	ErasedImageURL   string                     `json:"erasedImageUrl,omitempty"`
	ResultImageURL   string                     `json:"resultImageUrl,omitempty"`
	BaseImageURL     string                     `json:"baseImageUrl,omitempty"`
	ImageWidth       int                        `json:"imageWidth"`
	ImageHeight      int                        `json:"imageHeight"`
	Blocks           []TranslateManualEditBlock `json:"blocks"`
	Warnings         []string                   `json:"warnings,omitempty"`
}

type TranslateManualEditBlock struct {
	ID                     string            `json:"id"`
	SourceText             string            `json:"sourceText,omitempty"`
	Text                   string            `json:"text"`
	Lines                  []string          `json:"lines,omitempty"`
	BlockClass             string            `json:"blockClass,omitempty"`
	BBox                   translateTextBBox `json:"bbox"`
	EraseBBox              translateTextBBox `json:"eraseBBox"`
	FontSize               int               `json:"fontSize"`
	Color                  string            `json:"color,omitempty"`
	BackgroundColor        string            `json:"backgroundColor,omitempty"`
	FontWeight             string            `json:"fontWeight,omitempty"`
	Align                  string            `json:"align,omitempty"`
	BorderRadius           int               `json:"borderRadius,omitempty"`
	ErasePadding           int               `json:"erasePadding,omitempty"`
	MaskDilate             int               `json:"maskDilate,omitempty"`
	RemoveSourceBackground bool              `json:"removeSourceBackground"`
	Hidden                 bool              `json:"hidden,omitempty"`
}

type TranslateManualRenderRequest struct {
	BaseImage        string                     `json:"baseImage"`
	OutputFormat     string                     `json:"outputFormat"`
	Blocks           []TranslateManualEditBlock `json:"blocks"`
	Note             string                     `json:"note"`
	VerifyOutputText bool                       `json:"verifyOutputText"`
}

func (s *Service) GetTranslateManualEditState(ctx context.Context, id uuid.UUID) (*TranslateManualEditState, error) {
	task, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !IsTranslateTaskType(task.TaskType) {
		return nil, gorm.ErrRecordNotFound
	}
	out := imageTaskOutputMap(task)
	originalURL := firstNonEmptyString(stringFromAny(out["debugOriginalUrl"]), task.SourceImageURL)
	erasedURL := stringFromAny(out["debugErasedUrl"])
	resultURL := firstNonEmptyString(task.ResultURL, stringFromAny(out["debugFinalUrl"]), stringFromAny(out["resultUrl"]))
	baseURL := firstNonEmptyString(erasedURL, originalURL, resultURL)
	state := &TranslateManualEditState{
		TaskID:           task.ID.String(),
		OriginalImageURL: originalURL,
		ErasedImageURL:   erasedURL,
		ResultImageURL:   resultURL,
		BaseImageURL:     baseURL,
		Blocks:           manualBlocksFromOutput(out),
	}
	if state.BaseImageURL != "" {
		if payload, loadErr := s.loadTranslateImagePayload(ctx, state.BaseImageURL); loadErr == nil && payload != nil {
			state.ImageWidth = payload.Width
			state.ImageHeight = payload.Height
		} else {
			state.Warnings = append(state.Warnings, "base_image_not_loaded")
		}
	}
	if state.ImageWidth <= 0 || state.ImageHeight <= 0 {
		state.ImageWidth, state.ImageHeight = manualImageSizeFromOutput(out)
	}
	if len(state.Blocks) == 0 {
		state.Warnings = append(state.Warnings, "no_editable_text_blocks")
	}
	return state, nil
}

func (s *Service) ManualRenderTranslateTask(ctx context.Context, id uuid.UUID, req TranslateManualRenderRequest, editedBy *uuid.UUID) (*ImageTask, error) {
	task, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !IsTranslateTaskType(task.TaskType) {
		return nil, fmt.Errorf("manual render only supports translate_image_text tasks")
	}
	if len(req.Blocks) == 0 {
		return nil, fmt.Errorf("no editable blocks")
	}
	outObj := imageTaskOutputMap(task)
	baseURL := manualBaseURL(task, outObj, req.BaseImage)
	if baseURL == "" {
		return nil, fmt.Errorf("no base image url")
	}
	payload, err := s.loadTranslateImagePayload(ctx, baseURL)
	if err != nil || payload == nil || len(payload.RawBytes) == 0 {
		if err == nil {
			err = fmt.Errorf("empty base image")
		}
		return nil, fmt.Errorf("load base image: %w", err)
	}
	src, _, err := imagerender.Decode(payload.RawBytes)
	if err != nil {
		return nil, fmt.Errorf("decode base image: %w", err)
	}
	blocks := buildManualImageBlocks(req.Blocks, payload.Width, payload.Height)
	if len(blocks) == 0 {
		return nil, fmt.Errorf("no drawable blocks")
	}
	eraseBlocks := manualEraseBlocks(blocks)
	opts := imagerender.Options{
		EraseMode:              imagerender.EraseTextPixelMask,
		MaskPadding:            2,
		TextPadding:            6,
		LineHeight:             1.15,
		PureTextReplace:        true,
		ForceTextBoundsCleanup: true,
	}
	format := strings.TrimSpace(strings.ToLower(req.OutputFormat))
	if format == "" {
		format = "webp"
	}
	rgba := imagerender.ToRGBA(src)
	var stats imagerender.EraseStats
	usedErase := ""
	base := strings.TrimSpace(strings.ToLower(req.BaseImage))
	if base == "" {
		base = manualBaseOriginal
	}
	if base != manualBaseErased && len(eraseBlocks) > 0 {
		rgba, stats, usedErase, err = imagerender.EraseRegions(src, eraseBlocks, opts)
		if err != nil {
			fallbackOpts := opts
			fallbackOpts.EraseMode = imagerender.EraseOpenCVInpaint
			rgba, stats, usedErase, err = imagerender.EraseRegions(src, eraseBlocks, fallbackOpts)
			if err != nil {
				return nil, fmt.Errorf("erase source text: %w", err)
			}
		}
		bounds := rgba.Bounds()
		cleanup := imagerender.ForceEraseSourceBlockBounds(rgba, eraseBlocks, maxInt(1, bounds.Dx()*bounds.Dy()))
		stats = mergeEraseStats(stats, cleanup)
		if cleanup.ErasePixels > 0 && !strings.Contains(usedErase, "source_bounds_cleanup") {
			if usedErase == "" {
				usedErase = "source_bounds_cleanup"
			} else {
				usedErase += "+source_bounds_cleanup"
			}
		}
	}
	drawn, err := imagerender.DrawRegions(rgba, blocks, opts)
	if err != nil {
		return nil, fmt.Errorf("draw translated text: %w", err)
	}
	data, ct, err := imagerender.Encode(rgba, format)
	if err != nil {
		return nil, fmt.Errorf("encode manual render: %w", err)
	}
	res := &imgprov.ImageResult{
		RawPayload:         data,
		PayloadContentType: ct,
		Meta: map[string]any{
			"renderMode": "manual_edit",
			"eraseMode":  usedErase,
		},
	}
	finalURL, finalFID, storageKey, err := s.persistProviderResult(ctx, task, res, inputHints(task.Input))
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	outObj["manualEdit"] = map[string]any{
		"enabled":       true,
		"baseImage":     base,
		"baseImageUrl":  baseURL,
		"blocks":        req.Blocks,
		"blocksDrawn":   drawn,
		"eraseBlocks":   len(eraseBlocks),
		"eraseMode":     usedErase,
		"editedAt":      now.Format(time.RFC3339),
		"editedBy":      ptrUUIDStr(editedBy),
		"note":          truncateRunes(req.Note, 500),
		"verifySkipped": !req.VerifyOutputText,
	}
	outObj["renderMode"] = "manual_edit"
	outObj["finalQualityStatus"] = StatusSuccessWithReview
	outObj["resultUnavailable"] = false
	delete(outObj, "validationFailureReasons")
	delete(outObj, "pureTextValidation")
	delete(outObj, "validationMode")
	delete(outObj, "debugFinalUrl")
	imageArea := maxInt(1, payload.Width*payload.Height)
	outObj["layout"] = mergeManualLayoutMeta(outObj["layout"], usedErase, float64(stats.ErasePixels)/float64(imageArea))
	scoreJSON, _ := json.Marshal(map[string]any{
		"manualEdit": outObj["manualEdit"],
		"layout":     outObj["layout"],
	})
	if err := s.finalizeTaskSuccessWithStatus(ctx, task, finalURL, finalFID, storageKey, outObj, scoreJSON, false, StatusSuccessWithReview); err != nil {
		return nil, err
	}
	return s.GetByID(ctx, id)
}

func imageTaskOutputMap(task *ImageTask) map[string]any {
	if task == nil || len(task.Output) == 0 {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(task.Output, &out); err != nil || out == nil {
		return map[string]any{}
	}
	return out
}

func manualBaseURL(task *ImageTask, out map[string]any, base string) string {
	switch strings.TrimSpace(strings.ToLower(base)) {
	case manualBaseErased:
		return firstNonEmptyString(stringFromAny(out["debugErasedUrl"]), stringFromAny(out["debugOriginalUrl"]), task.SourceImageURL)
	case manualBaseResult:
		return firstNonEmptyString(task.ResultURL, stringFromAny(out["debugFinalUrl"]), stringFromAny(out["resultUrl"]), stringFromAny(out["debugErasedUrl"]), stringFromAny(out["debugOriginalUrl"]), task.SourceImageURL)
	default:
		return firstNonEmptyString(stringFromAny(out["debugOriginalUrl"]), task.SourceImageURL, stringFromAny(out["debugErasedUrl"]))
	}
}

func manualBlocksFromOutput(out map[string]any) []TranslateManualEditBlock {
	ocrBlocks := map[string]map[string]any{}
	if ocr, ok := out["ocr"].(map[string]any); ok {
		if arr, ok := ocr["blocks"].([]any); ok {
			for _, item := range arr {
				m, _ := item.(map[string]any)
				id := stringFromAny(m["id"])
				if id != "" {
					ocrBlocks[id] = m
				}
			}
		}
	}
	var blocks []TranslateManualEditBlock
	if arr, ok := out["blockClassifications"].([]any); ok {
		for i, item := range arr {
			m, _ := item.(map[string]any)
			if b := manualBlockFromMaps(i, m, ocrBlocks[stringFromAny(m["id"])]); b.ID != "" && strings.TrimSpace(b.Text) != "" {
				blocks = append(blocks, b)
			}
		}
	}
	if len(blocks) > 0 {
		return blocks
	}
	if ocr, ok := out["ocr"].(map[string]any); ok {
		if arr, ok := ocr["blocks"].([]any); ok {
			for i, item := range arr {
				m, _ := item.(map[string]any)
				if b := manualBlockFromMaps(i, m, nil); b.ID != "" && strings.TrimSpace(b.Text) != "" {
					blocks = append(blocks, b)
				}
			}
		}
	}
	return blocks
}

func manualBlockFromMaps(i int, summary, ocr map[string]any) TranslateManualEditBlock {
	id := firstNonEmptyString(stringFromAny(summary["id"]), fmt.Sprintf("manual_%d", i+1))
	class := firstNonEmptyString(stringFromAny(summary["blockClass"]), stringFromAny(ocr["blockClass"]))
	text := firstNonEmptyString(
		stringFromAny(summary["fixedShortTranslation"]),
		stringFromAny(summary["badgeTranslation"]),
		stringFromAny(summary["badge_translation"]),
		stringFromAny(summary["compactTranslation"]),
		stringFromAny(summary["compact_translation"]),
		stringFromAny(summary["standardTranslation"]),
		stringFromAny(summary["standard_translation"]),
		stringFromAny(ocr["drawText"]),
		stringFromAny(ocr["compactTranslation"]),
		stringFromAny(ocr["shortTranslatedText"]),
		stringFromAny(ocr["translatedText"]),
	)
	bbox := bboxFromAny(summary["layout_bbox"])
	if bbox.Width <= 0 || bbox.Height <= 0 {
		bbox = bboxFromAny(summary["bbox"])
	}
	if bbox.Width <= 0 || bbox.Height <= 0 {
		bbox = bboxFromAny(ocr["bbox"])
	}
	eraseBBox := bboxFromAny(summary["erase_bbox"])
	if eraseBBox.Width <= 0 || eraseBBox.Height <= 0 {
		eraseBBox = bboxFromAny(ocr["sourceBBox"])
	}
	if eraseBBox.Width <= 0 || eraseBBox.Height <= 0 {
		eraseBBox = bboxFromAny(ocr["bbox"])
	}
	if eraseBBox.Width <= 0 || eraseBBox.Height <= 0 {
		eraseBBox = bbox
	}
	style := map[string]any{}
	if raw, ok := ocr["style"].(map[string]any); ok {
		style = raw
	}
	fontSize := intFromAny(summary["fontSize"])
	if fontSize <= 0 {
		fontSize = manualDefaultFontSize(class, bbox)
	}
	align := firstNonEmptyString(stringFromAny(style["align"]), "left")
	if manualIsBadgeClass(class) {
		align = "center"
	}
	color := firstNonEmptyString(stringFromAny(style["color"]), "#111111")
	if isWhiteHex(color) {
		color = "#111111"
	}
	return TranslateManualEditBlock{
		ID:                     id,
		SourceText:             firstNonEmptyString(stringFromAny(summary["text"]), stringFromAny(ocr["text"])),
		Text:                   text,
		BlockClass:             class,
		BBox:                   bbox,
		EraseBBox:              eraseBBox,
		FontSize:               fontSize,
		Color:                  color,
		BackgroundColor:        "",
		FontWeight:             firstNonEmptyString(stringFromAny(style["fontWeight"]), manualDefaultFontWeight(class)),
		Align:                  align,
		BorderRadius:           0,
		ErasePadding:           2,
		MaskDilate:             2,
		RemoveSourceBackground: true,
	}
}

func buildManualImageBlocks(blocks []TranslateManualEditBlock, imageW, imageH int) []imagerender.TextBlock {
	out := make([]imagerender.TextBlock, 0, len(blocks))
	for i, b := range blocks {
		if b.Hidden {
			continue
		}
		lines := manualBlockLines(b)
		if len(lines) == 0 {
			continue
		}
		bbox := clampManualTranslateBBox(b.BBox, imageW, imageH)
		if bbox.Width <= 0 || bbox.Height <= 0 {
			continue
		}
		erase := clampManualTranslateBBox(b.EraseBBox, imageW, imageH)
		if erase.Width <= 0 || erase.Height <= 0 {
			erase = bbox
		}
		if !b.RemoveSourceBackground {
			erase = translateTextBBox{}
		}
		align := strings.TrimSpace(b.Align)
		if align == "" {
			align = "left"
		}
		fontSize := b.FontSize
		if fontSize <= 0 {
			fontSize = manualDefaultFontSize(b.BlockClass, bbox)
		}
		id := strings.TrimSpace(b.ID)
		if id == "" {
			id = fmt.Sprintf("manual_%d", i+1)
		}
		out = append(out, imagerender.TextBlock{
			ID:         id,
			BlockClass: b.BlockClass,
			Lines:      lines,
			FontSize:   fontSize,
			BBox:       imagerender.BBox{X: bbox.X, Y: bbox.Y, Width: bbox.Width, Height: bbox.Height},
			EraseBBox:  imagerender.BBox{X: erase.X, Y: erase.Y, Width: erase.Width, Height: erase.Height},
			Style: imagerender.TextStyle{
				Color:           firstNonEmptyString(b.Color, "#111111"),
				BackgroundColor: "",
				FontWeight:      firstNonEmptyString(b.FontWeight, manualDefaultFontWeight(b.BlockClass)),
				Align:           align,
				BorderRadius:    b.BorderRadius,
			},
			Align:        align,
			Bold:         strings.EqualFold(b.FontWeight, "bold") || manualIsTitleClass(b.BlockClass),
			ErasePadding: maxInt(1, b.ErasePadding),
			MaskDilate:   maxInt(1, b.MaskDilate),
			TextPolarity: "dark",
		})
	}
	return out
}

func manualEraseBlocks(blocks []imagerender.TextBlock) []imagerender.TextBlock {
	out := make([]imagerender.TextBlock, 0, len(blocks))
	for _, b := range blocks {
		if b.EraseBBox.Width <= 0 || b.EraseBBox.Height <= 0 {
			continue
		}
		out = append(out, b)
	}
	return out
}

func manualBlockLines(b TranslateManualEditBlock) []string {
	var lines []string
	for _, line := range b.Lines {
		if s := strings.TrimSpace(line); s != "" {
			lines = append(lines, s)
		}
	}
	if len(lines) > 0 {
		return lines
	}
	for _, line := range strings.Split(strings.ReplaceAll(b.Text, "\r\n", "\n"), "\n") {
		if s := strings.TrimSpace(line); s != "" {
			lines = append(lines, s)
		}
	}
	return lines
}

func clampManualTranslateBBox(b translateTextBBox, imageW, imageH int) translateTextBBox {
	if b.Width <= 0 || b.Height <= 0 {
		return translateTextBBox{}
	}
	if imageW > 0 {
		if b.X < 0 {
			b.Width += b.X
			b.X = 0
		}
		if b.X+b.Width > imageW {
			b.Width = imageW - b.X
		}
	}
	if imageH > 0 {
		if b.Y < 0 {
			b.Height += b.Y
			b.Y = 0
		}
		if b.Y+b.Height > imageH {
			b.Height = imageH - b.Y
		}
	}
	if b.Width < 1 || b.Height < 1 {
		return translateTextBBox{}
	}
	return b
}

func manualDefaultFontSize(class string, bbox translateTextBBox) int {
	if bbox.Height > 0 {
		fs := int(float64(bbox.Height) * 0.46)
		if manualIsTitleClass(class) {
			fs = int(float64(bbox.Height) * 0.55)
		}
		return clampInt(fs, 12, 56)
	}
	return 18
}

func manualDefaultFontWeight(class string) string {
	if manualIsTitleClass(class) || manualIsBadgeClass(class) {
		return "bold"
	}
	return ""
}

func manualIsTitleClass(class string) bool {
	c := strings.ToLower(strings.TrimSpace(class))
	return c == blockClassTitle || c == "main_title" || c == "title"
}

func manualIsBadgeClass(class string) bool {
	c := strings.ToLower(strings.TrimSpace(class))
	return strings.Contains(c, "badge") || c == blockClassPill || strings.Contains(c, "pill")
}

func bboxFromAny(v any) translateTextBBox {
	m, ok := v.(map[string]any)
	if !ok {
		return translateTextBBox{}
	}
	return translateTextBBox{
		X:      intFromAny(m["x"]),
		Y:      intFromAny(m["y"]),
		Width:  intFromAny(m["width"]),
		Height: intFromAny(m["height"]),
	}
}

func manualImageSizeFromOutput(out map[string]any) (int, int) {
	candidates := []map[string]any{}
	if coord, ok := out["coordinateMeta"].(map[string]any); ok {
		candidates = append(candidates, coord)
	}
	if ocr, ok := out["ocr"].(map[string]any); ok {
		if coord, ok := ocr["coordinateMeta"].(map[string]any); ok {
			candidates = append(candidates, coord)
		}
	}
	for _, c := range candidates {
		w := firstPositiveInt(intFromAny(c["renderImageWidth"]), intFromAny(c["originalImageWidth"]), intFromAny(c["ocrImageWidth"]))
		h := firstPositiveInt(intFromAny(c["renderImageHeight"]), intFromAny(c["originalImageHeight"]), intFromAny(c["ocrImageHeight"]))
		if w > 0 && h > 0 {
			return w, h
		}
	}
	return 0, 0
}

func mergeManualLayoutMeta(raw any, eraseMode string, eraseRatio float64) map[string]any {
	out := map[string]any{}
	if m, ok := raw.(map[string]any); ok {
		for k, v := range m {
			out[k] = v
		}
	}
	out["renderMode"] = "manual_edit"
	out["eraseMode"] = eraseMode
	out["eraseAreaRatio"] = eraseRatio
	out["manualEditable"] = true
	return out
}

func stringFromAny(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case json.RawMessage:
		return strings.TrimSpace(string(x))
	default:
		return ""
	}
}

func firstPositiveInt(values ...int) int {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}

func isWhiteHex(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "#fff", "#ffffff", "white":
		return true
	default:
		return false
	}
}
