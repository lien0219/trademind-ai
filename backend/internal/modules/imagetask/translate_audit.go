package imagetask

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/imagerender"
)

func (s *Service) logTranslateAudit(ctx context.Context, task *ImageTask, action, status, msg string) {
	if s == nil || s.OpLog == nil || task == nil {
		return
	}
	var admin *uuid.UUID
	if task.CreatedBy != nil {
		admin = task.CreatedBy
	}
	_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
		AdminUserID: admin,
		Action:      action,
		Resource:    "image_task",
		ResourceID:  task.ID.String(),
		Status:      status,
		Message:     truncateRunes(msg, 2000),
	})
}

func translateAuditMsg(task *ImageTask, fields map[string]any) string {
	parts := []string{
		fmt.Sprintf("taskId=%s", task.ID.String()),
	}
	if task.ProductID != nil {
		parts = append(parts, fmt.Sprintf("productId=%s", task.ProductID.String()))
	}
	if task.SourceImageID != nil {
		parts = append(parts, fmt.Sprintf("sourceImageId=%s", task.SourceImageID.String()))
	}
	for k, v := range fields {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	return strings.Join(parts, " ")
}

type translateResultMeta struct {
	Translate     translateSummaryMeta      `json:"translate"`
	Layout        translateLayoutMeta       `json:"layout"`
	Verification  translateVerificationMeta `json:"verification"`
	RenderQuality translateRenderQuality    `json:"renderQuality,omitempty"`
}

type translateSummaryMeta struct {
	SourceLanguage        string `json:"sourceLanguage"`
	TargetLanguage        string `json:"targetLanguage"`
	TextBlocksCount       int    `json:"textBlocksCount"`
	TranslatedBlocksCount int    `json:"translatedBlocksCount"`
	RenderedBlocksCount   int    `json:"renderedBlocksCount"`
	VerifiedBlocksCount   int    `json:"verifiedBlocksCount"`
}

type translateLayoutMeta struct {
	RenderMode         string                    `json:"renderMode"`
	EraseMode          string                    `json:"eraseMode"`
	LayoutTemplate     string                    `json:"layoutTemplate,omitempty"`
	EraseAreaRatio     float64                   `json:"eraseAreaRatio,omitempty"`
	PatchAreaRatio     float64                   `json:"patchAreaRatio,omitempty"`
	BackgroundDelta    float64                   `json:"backgroundDeltaScore,omitempty"`
	FlatFillRatio      float64                   `json:"flatFillRatio,omitempty"`
	LargePatchDetected bool                      `json:"largePatchDetected,omitempty"`
	RetryStrategies    []string                  `json:"retryStrategies,omitempty"`
	AutoWrappedBlocks  int                       `json:"autoWrappedBlocks"`
	FontResizedBlocks  int                       `json:"fontResizedBlocks"`
	SimplifiedBlocks   int                       `json:"simplifiedBlocks"`
	OverflowBlocks     int                       `json:"overflowBlocks"`
	MinFontSizeUsed    int                       `json:"minFontSizeUsed"`
	Simulation         translateLayoutSimulation `json:"simulation,omitempty"`
	Warnings           []string                  `json:"warnings"`
}

type translateVerificationMeta struct {
	ImageChanged            bool    `json:"imageChanged"`
	TargetTextDetected      bool    `json:"targetTextDetected"`
	SourceTextMayRemain     bool    `json:"sourceTextMayRemain"`
	TranslatedTextOverflow  bool    `json:"translatedTextOverflow,omitempty"`
	TextOverlapDetected     bool    `json:"textOverlapDetected,omitempty"`
	ProductOverlapDetected  bool    `json:"productOverlapDetected,omitempty"`
	CommercialUsabilityLow  bool    `json:"commercialUsabilityLow,omitempty"`
	Confidence              float64 `json:"confidence"`
	OutputTextVerifyFailed  bool    `json:"outputTextVerifyFailed,omitempty"`
	OutputTextVerifySkipped bool    `json:"outputTextVerifySkipped,omitempty"`
	SourceTextRemainNearBox bool    `json:"sourceTextRemainNearBox,omitempty"`
}

func buildRenderBlocks(ocr *translateOCRResult, plans []translateBlockLayoutPlan) []imagerender.TextBlock {
	if ocr == nil {
		return nil
	}
	out := make([]imagerender.TextBlock, 0, len(plans))
	planIdx := 0
	for _, b := range ocr.Blocks {
		if strings.TrimSpace(b.TranslatedText) == "" {
			continue
		}
		if planIdx >= len(plans) {
			break
		}
		plan := plans[planIdx]
		planIdx++
		align := strings.TrimSpace(b.Style.Align)
		if align == "" {
			align = "left"
		}
		out = append(out, imagerender.TextBlock{
			ID:       b.ID,
			Lines:    append([]string(nil), plan.Lines...),
			FontSize: plan.FontSize,
			BBox: imagerender.BBox{
				X: plan.BBox.X, Y: plan.BBox.Y,
				Width: plan.BBox.Width, Height: plan.BBox.Height,
			},
			EraseBBox: imagerender.BBox{
				X: b.BBox.X, Y: b.BBox.Y,
				Width: b.BBox.Width, Height: b.BBox.Height,
			},
			Style: imagerender.TextStyle{
				Color:           b.Style.Color,
				BackgroundColor: b.Style.BackgroundColor,
				FontWeight:      b.Style.FontWeight,
				Align:           align,
				BorderRadius:    b.Style.BorderRadius,
			},
			Align: align,
			Bold:  strings.EqualFold(b.Style.FontWeight, "bold"),
		})
	}
	return out
}

func buildImageRenderBlocks(blocks []translateRenderBlock) []imagerender.TextBlock {
	out := make([]imagerender.TextBlock, 0, len(blocks))
	for _, b := range blocks {
		align := strings.TrimSpace(b.Style.Align)
		if align == "" {
			align = "left"
		}
		out = append(out, imagerender.TextBlock{
			ID:         b.ID,
			BlockClass: b.BlockClass,
			Lines:      append([]string(nil), b.Lines...),
			FontSize:   b.FontSize,
			BBox: imagerender.BBox{
				X: b.BBox.X, Y: b.BBox.Y,
				Width: b.BBox.Width, Height: b.BBox.Height,
			},
			EraseBBox: imagerender.BBox{
				X: b.EraseBBox.X, Y: b.EraseBBox.Y,
				Width: b.EraseBBox.Width, Height: b.EraseBBox.Height,
			},
			Style: imagerender.TextStyle{
				Color:           b.Style.Color,
				BackgroundColor: b.Style.BackgroundColor,
				FontWeight:      b.Style.FontWeight,
				Align:           align,
				BorderRadius:    b.Style.BorderRadius,
			},
			Align:        align,
			Bold:         strings.EqualFold(b.Style.FontWeight, "bold"),
			ErasePadding: b.ErasePadding,
			MaskDilate:   b.MaskDilate,
			TextPolarity: b.TextPolarity,
		})
	}
	return out
}
