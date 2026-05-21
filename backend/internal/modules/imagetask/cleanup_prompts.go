package imagetask

import (
	"fmt"
	"strings"
)

func cleanupPromptForTaskType(taskType, userPrompt, negativePrompt string) string {
	base := strings.TrimSpace(userPrompt)
	neg := strings.TrimSpace(negativePrompt)
	var instruction string
	switch strings.TrimSpace(strings.ToLower(taskType)) {
	case TaskTypeRemoveWatermark:
		instruction = "Remove all watermarks from this product image. Keep the product intact, natural edges, and original composition. Fill removed areas seamlessly."
	case TaskTypeRemoveLogo:
		instruction = "Remove all logos and brand marks from this product image while preserving the product and background quality."
	case TaskTypeRemoveBadge:
		instruction = "Remove corner badges, stickers, promotional labels and corner overlays from this product image. Preserve the product details."
	case TaskTypeRemoveQRCode:
		instruction = "Remove QR codes and barcode overlays from this product image. Do not alter the main product."
	case TaskTypeCleanup:
		instruction = "Comprehensively clean this product image: remove watermarks, logos, badges, stickers, QR codes, and clutter. Keep the product sharp and natural."
	case TaskTypeEnhanceDetail:
		instruction = "Enhance this product detail image: improve clarity, reduce noise, remove distractions, keep accurate colors and product details."
	case TaskTypeUpscale:
		instruction = "Upscale and sharpen this product image for ecommerce listing. Improve clarity and detail without changing the product shape."
	default:
		instruction = "Improve this product image for ecommerce listing."
	}
	if base != "" {
		instruction = instruction + " " + base
	}
	if neg != "" {
		instruction = instruction + " Avoid: " + neg + "."
	}
	instruction += " Output a clean professional ecommerce product photo."
	return instruction
}

func generationPromptForTaskType(taskType, userPrompt, negativePrompt, styleTemplate string) string {
	base := strings.TrimSpace(userPrompt)
	neg := strings.TrimSpace(negativePrompt)
	style := strings.TrimSpace(styleTemplate)
	var instruction string
	switch strings.TrimSpace(strings.ToLower(taskType)) {
	case TaskTypeGenerateMarketing:
		instruction = "Create a compelling ecommerce marketing image based on the product. Highlight selling points with professional composition."
		if style != "" {
			instruction += " Style: " + style + "."
		}
	case TaskTypeGenerateMainImage, TaskTypeBatchGenerateMain:
		instruction = "Generate a high-converting ecommerce main product image on a clean background. Center the product, professional lighting, listing-ready."
		if style != "" {
			instruction += " Style: " + style + "."
		}
	case TaskTypePosterGenerate:
		instruction = "Create a marketing poster layout for this product."
	default:
		instruction = "Generate a professional ecommerce product image."
	}
	if base != "" {
		instruction = instruction + " " + base
	}
	if neg != "" {
		instruction = instruction + " Avoid: " + neg + "."
	}
	return instruction
}

func prepareCleanupHints(task *ImageTask, hints map[string]any) map[string]any {
	if hints == nil {
		hints = map[string]any{}
	}
	prompt := strings.TrimSpace(task.Prompt)
	if prompt == "" {
		prompt = stringFromMap(hints, "prompt")
	}
	neg := strings.TrimSpace(task.NegativePrompt)
	if neg == "" {
		neg = stringFromMap(hints, "negativePrompt")
	}
	assembled := cleanupPromptForTaskType(task.TaskType, prompt, neg)
	hints["assembled_prompt"] = assembled
	hints["prompt"] = assembled
	return hints
}

func prepareGenerationHints(task *ImageTask, hints map[string]any) map[string]any {
	if hints == nil {
		hints = map[string]any{}
	}
	prompt := strings.TrimSpace(task.Prompt)
	if prompt == "" {
		prompt = stringFromMap(hints, "prompt")
	}
	neg := strings.TrimSpace(task.NegativePrompt)
	if neg == "" {
		neg = stringFromMap(hints, "negativePrompt")
	}
	style := stringFromMap(hints, "style")
	if style == "" {
		style = stringFromMap(hints, "styleTemplate")
	}
	assembled := generationPromptForTaskType(task.TaskType, prompt, neg, style)
	hints["assembled_prompt"] = assembled
	hints["prompt"] = assembled
	return hints
}

func selectModeFromHints(hints map[string]any) string {
	mode := strings.TrimSpace(stringFromMap(hints, "selectMode"))
	if mode == "" {
		mode = strings.TrimSpace(stringFromMap(hints, "mode"))
	}
	switch strings.ToLower(mode) {
	case "score_only", "recommend", "auto_set":
		return strings.ToLower(mode)
	default:
		return "recommend"
	}
}

func autoSaveFromHints(hints map[string]any) bool {
	if hints == nil {
		return false
	}
	if v, ok := hints["autoSave"].(bool); ok {
		return v
	}
	if v, ok := hints["autoSaveToProduct"].(bool); ok {
		return v
	}
	return false
}

func autoSetMainFromHints(hints map[string]any) bool {
	if hints == nil {
		return false
	}
	if v, ok := hints["autoSetMain"].(bool); ok {
		return v
	}
	if v, ok := hints["autoSetBestMain"].(bool); ok {
		return v
	}
	return false
}

func resultCountFromHints(hints map[string]any, def int) int {
	n := intFromAny(hints["resultCount"])
	if n <= 0 {
		n = intFromAny(hints["count"])
	}
	if n <= 0 {
		return def
	}
	if n > 8 {
		return 8
	}
	return n
}

func imageTypeFromApplyMode(mode string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "main", "set_main", "best_main":
		return "main"
	case "marketing":
		return "marketing"
	case "detail", "set_detail":
		return "detail"
	default:
		return "ai_generated"
	}
}

func fmtScoreSummary(score map[string]any) string {
	if score == nil {
		return ""
	}
	overall := score["overallScore"]
	return fmt.Sprintf("overallScore=%v", overall)
}
