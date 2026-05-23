package imagetask

import "strings"

// Extended AI image task types (production).
const (
	TaskTypeRemoveWatermark   = "remove_watermark"
	TaskTypeRemoveLogo        = "remove_logo"
	TaskTypeRemoveBadge       = "remove_badge"
	TaskTypeRemoveQRCode      = "remove_qrcode"
	TaskTypeCleanup           = "cleanup"
	TaskTypeEnhanceDetail     = "enhance_detail"
	TaskTypeGenerateMarketing = "generate_marketing"
	TaskTypeGenerateMainImage = "generate_main_image"
	TaskTypeBatchGenerateMain = "batch_generate_main"
	TaskTypeUpscale           = "upscale"
	TaskTypeScoreImage        = "score_image"
	TaskTypeSelectBestMain    = "select_best_main"
)

// IsCleanupTaskType reports inpaint / cleanup style tasks that edit the source image.
func IsCleanupTaskType(t string) bool {
	switch strings.TrimSpace(strings.ToLower(t)) {
	case TaskTypeRemoveWatermark, TaskTypeRemoveLogo, TaskTypeRemoveBadge,
		TaskTypeRemoveQRCode, TaskTypeCleanup, TaskTypeEnhanceDetail, TaskTypeUpscale:
		return true
	default:
		return false
	}
}

// IsGenerationTaskType reports tasks that synthesize new images.
func IsGenerationTaskType(t string) bool {
	switch strings.TrimSpace(strings.ToLower(t)) {
	case TaskTypeGenerateScene, TaskTypeGenerateMarketing, TaskTypeGenerateMainImage,
		TaskTypeBatchGenerateMain, TaskTypeReplaceBackground, TaskTypePosterGenerate:
		return true
	default:
		return false
	}
}

// IsScoringTaskType reports analysis-only tasks (no image output required).
func IsScoringTaskType(t string) bool {
	switch strings.TrimSpace(strings.ToLower(t)) {
	case TaskTypeScoreImage, TaskTypeSelectBestMain:
		return true
	default:
		return false
	}
}

// RequiresSourceImage reports whether a task must have a source image.
func RequiresSourceImage(t string) bool {
	tt := strings.TrimSpace(strings.ToLower(t))
	if IsScoringTaskType(tt) {
		return tt == TaskTypeScoreImage
	}
	if tt == TaskTypeSelectBestMain {
		return false
	}
	if tt == TaskTypeGenerateScene {
		return false // may omit for text-only providers
	}
	if tt == TaskTypeBatchGenerateMain {
		return false
	}
	return true
}
