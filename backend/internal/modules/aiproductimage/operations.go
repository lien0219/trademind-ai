package aiproductimage

import (
	"errors"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
	imgprov "github.com/trademind-ai/trademind/backend/internal/providers/image"
)

var (
	errSelectOperation      = errors.New("请选择处理方式")
	errUnsupportedOperation = errors.New("不支持的处理类型")
)

// operationToTaskType maps batch operation to imagetask task type.
func operationToTaskType(op string) string {
	switch strings.TrimSpace(op) {
	case OpQualityCheck:
		return imagetask.TaskTypeScoreImage
	case OpRemoveWatermark:
		return imagetask.TaskTypeRemoveWatermark
	case OpRemoveLogo:
		return imagetask.TaskTypeRemoveLogo
	case OpWhiteBackground:
		return imagetask.TaskTypeRemoveBackground
	case OpOptimizeBackground:
		return imagetask.TaskTypeReplaceBackground
	case OpTranslateText:
		return imagetask.TaskTypeTranslateImageText
	case OpSelectBestMain:
		return imagetask.TaskTypeSelectBestMain
	default:
		return ""
	}
}

func normalizeOperationTypes(raw []string) ([]string, error) {
	if len(raw) == 0 {
		return nil, errSelectOperation
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(raw))
	for _, op := range raw {
		op = strings.TrimSpace(strings.ToLower(op))
		if operationToTaskType(op) == "" {
			return nil, errUnsupportedOperation
		}
		if _, ok := seen[op]; ok {
			continue
		}
		seen[op] = struct{}{}
		out = append(out, op)
	}
	return out, nil
}

func imagetaskApplyMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case ApplySetMain:
		return "main"
	case ApplyAddDetail:
		return "detail"
	case ApplySaveToGallery:
		return "ai_generated"
	default:
		return "ai_generated"
	}
}

// resolveGenerationTaskType picks imagetask type; white_background uses replace_background when
// the configured provider supports it but not remove_background (e.g. dashscope_image).
func resolveGenerationTaskType(provider, operationType string) string {
	base := operationToTaskType(operationType)
	if operationType != OpWhiteBackground {
		return base
	}
	prov := strings.TrimSpace(strings.ToLower(provider))
	if prov == "" || prov == "noop" {
		return base
	}
	if imgprov.SupportsTask(prov, imagetask.TaskTypeRemoveBackground) {
		return imagetask.TaskTypeRemoveBackground
	}
	if imgprov.SupportsTask(prov, imagetask.TaskTypeReplaceBackground) {
		return imagetask.TaskTypeReplaceBackground
	}
	return base
}
