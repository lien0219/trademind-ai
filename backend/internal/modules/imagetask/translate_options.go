package imagetask

import (
	"strings"
)

const (
	RenderModeDeterministic = "deterministic"
	RenderModeHybrid        = "hybrid"
	RenderModeAIEdit        = "ai_edit"

	errCodeImageNotChanged     = "IMAGE_NOT_CHANGED"
	errCodeImageTextNotApplied = "IMAGE_TEXT_NOT_APPLIED"
	errCodeOutputVerifyFailed  = "OUTPUT_TEXT_VERIFY_FAILED"
	errCodeTranslateRenderFail = "TRANSLATE_RENDER_FAILED"
	errCodeTranslateEraseFail  = "TRANSLATE_ERASE_FAILED"

	verifyWarningSourceTextRemain = "source_text_may_remain"
)

type translateRenderOptions struct {
	RenderMode       string
	EraseMode        string
	VerifyOutputText bool
	TextPadding      int
	MaskPadding      int
	OutputFormat     string
}

func renderModeFromHints(hints map[string]any) string {
	mode := strings.TrimSpace(strings.ToLower(stringFromMap(hints, "renderMode")))
	switch mode {
	case RenderModeDeterministic, RenderModeHybrid, RenderModeAIEdit:
		return mode
	case "":
		return RenderModeHybrid
	default:
		return RenderModeHybrid
	}
}

func parseTranslateRenderOptions(hints map[string]any) translateRenderOptions {
	opts := translateRenderOptions{
		RenderMode:       renderModeFromHints(hints),
		EraseMode:        strings.TrimSpace(strings.ToLower(stringFromMap(hints, "eraseMode"))),
		VerifyOutputText: boolFromHints(hints, "verifyOutputText", true),
		TextPadding:      intFromAny(hints["textPadding"]),
		MaskPadding:      intFromAny(hints["maskPadding"]),
		OutputFormat:     strings.TrimSpace(strings.ToLower(stringFromMap(hints, "outputFormat"))),
	}
	if opts.EraseMode == "" {
		opts.EraseMode = "auto"
	}
	if opts.TextPadding <= 0 {
		opts.TextPadding = 6
	}
	if opts.MaskPadding <= 0 {
		opts.MaskPadding = 12
	}
	if opts.OutputFormat == "" {
		opts.OutputFormat = "webp"
	}
	return opts
}

func effectiveEraseMode(renderOpts translateRenderOptions) string {
	if renderOpts.RenderMode == RenderModeHybrid {
		em := renderOpts.EraseMode
		if em == "" || em == "auto" {
			return "background_sample"
		}
		return em
	}
	if renderOpts.EraseMode == "" {
		return "auto"
	}
	return renderOpts.EraseMode
}

func assignOCRBlockIDs(blocks []translateTextBlock) {
	for i := range blocks {
		if strings.TrimSpace(blocks[i].ID) == "" {
			blocks[i].ID = formatBlockID(i + 1)
		}
	}
}

func formatBlockID(n int) string {
	return "block_" + itoa(n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func renderModeLabel(mode string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case RenderModeDeterministic:
		return "程序排版渲染"
	case RenderModeHybrid:
		return "AI 擦除 + 程序排版"
	case RenderModeAIEdit:
		return "AI 编辑实验模式"
	default:
		return mode
	}
}
