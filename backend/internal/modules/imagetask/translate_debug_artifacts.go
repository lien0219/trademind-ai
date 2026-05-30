package imagetask

import (
	"context"
	"fmt"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/modules/files"
)

func (s *Service) uploadTranslateDebugPNG(
	ctx context.Context,
	task *ImageTask,
	filename string,
	data []byte,
) (url, key string) {
	if s == nil || s.Files == nil || task == nil || len(data) == 0 {
		return "", ""
	}
	objKey := fmt.Sprintf("ai-image/debug/%s/%s", task.ID.String(), filename)
	fr, err := s.Files.SaveProcessed(ctx, files.SaveProcessedOpts{
		OriginalName: filename,
		ObjectKey:    objKey,
		Data:         data,
		ContentType:  "image/png",
		CreatedBy:    task.CreatedBy,
	})
	if err != nil {
		return "", ""
	}
	return strings.TrimSpace(fr.PublicURL), strings.TrimSpace(fr.ObjectKey)
}

func buildTranslateDebugOutput(
	ctx context.Context,
	s *Service,
	task *ImageTask,
	originalBytes, maskBytes, erasedBytes, finalBytes []byte,
) map[string]any {
	if s == nil || task == nil {
		return nil
	}
	out := map[string]any{}
	if url, key := s.uploadTranslateDebugPNG(ctx, task, "original.png", originalBytes); key != "" {
		out["debugOriginalUrl"] = url
		out["debugOriginalKey"] = key
	}
	if url, key := s.uploadTranslateDebugPNG(ctx, task, "mask.png", maskBytes); key != "" {
		out["debugMaskUrl"] = url
		out["debugMaskKey"] = key
	}
	if url, key := s.uploadTranslateDebugPNG(ctx, task, "erased.png", erasedBytes); key != "" {
		out["debugErasedUrl"] = url
		out["debugErasedKey"] = key
	}
	if url, key := s.uploadTranslateDebugPNG(ctx, task, "final.png", finalBytes); key != "" {
		out["debugFinalUrl"] = url
		out["debugFinalKey"] = key
	}
	return out
}

func debugKeysFromOutput(m map[string]any) []string {
	if m == nil {
		return nil
	}
	var keys []string
	for _, k := range []string{"debugOriginalKey", "debugMaskKey", "debugErasedKey", "debugFinalKey"} {
		if v := strings.TrimSpace(stringFromMap(m, k)); v != "" {
			keys = append(keys, v)
		}
	}
	return keys
}

func attachDebugToOutput(outObj map[string]any, debug map[string]any) {
	if outObj == nil || debug == nil {
		return
	}
	for k, v := range debug {
		outObj[k] = v
	}
}
