package imagetask

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"gorm.io/datatypes"
)

func (s *Service) prepareGenerateSceneHints(ctx context.Context, task *ImageTask, hints map[string]any) map[string]any {
	next := map[string]any{}
	for k, v := range hints {
		next[k] = v
	}

	userPrompt := stringFromMap(next, "prompt")
	scene := stringFromMap(next, "scene")
	style := stringFromMap(next, "style")
	platform := stringFromMap(next, "platform")
	size := stringFromMap(next, "size")
	background := stringFromMap(next, "background")
	srcURL := ""
	if task != nil {
		srcURL = strings.TrimSpace(task.SourceImageURL)
	}

	var b strings.Builder
	b.WriteString("Create a clean ecommerce-style product showcase image suitable for online marketplaces. ")
	b.WriteString("Do not include on-image text overlays, watermarks, or logos. ")
	b.WriteString("Avoid adult, violent, illegal, counterfeit, or misleading content; keep it safe for commerce. ")
	if platform != "" {
		fmt.Fprintf(&b, "Target platform vibe: %s. ", platform)
	}

	if s != nil && s.DB != nil && task != nil && task.ProductID != nil && *task.ProductID != uuid.Nil {
		var prod product.Product
		if err := s.DB.WithContext(ctx).First(&prod, "id = ?", task.ProductID).Error; err == nil {
			if t := strings.TrimSpace(prod.Title); t != "" {
				next["productTitle"] = t
				fmt.Fprintf(&b, "Product title: %s. ", t)
			}
			if t := strings.TrimSpace(prod.AITitle); t != "" {
				fmt.Fprintf(&b, "AI title suggestion: %s. ", t)
			}
			if d := strings.TrimSpace(prod.Description); d != "" {
				if len(d) > 400 {
					d = d[:400] + "…"
				}
				fmt.Fprintf(&b, "Description snippet: %s. ", d)
			}
			if a := attrsFromRawData(prod.RawData); a != "" {
				fmt.Fprintf(&b, "Structured attributes snippet: %s. ", a)
			}
			if c := categoryFromRawData(prod.RawData); c != "" {
				fmt.Fprintf(&b, "Category hint: %s. ", c)
			}
		}
	}

	if srcURL != "" {
		fmt.Fprintf(&b, "Feature the subject from this product photo (reference URL — model should preserve product identity): %s. ", srcURL)
	}
	if scene != "" {
		fmt.Fprintf(&b, "Desired scene / setting: %s. ", scene)
	}
	if style != "" {
		fmt.Fprintf(&b, "Visual style: %s. ", style)
	}
	if background != "" {
		fmt.Fprintf(&b, "Background direction: %s. ", background)
	}
	if userPrompt != "" {
		fmt.Fprintf(&b, "Additional creator instructions: %s. ", userPrompt)
	}
	if size != "" {
		fmt.Fprintf(&b, "Output framing similar to size %s. ", size)
	}

	next["assembled_prompt"] = strings.TrimSpace(b.String())
	return next
}

// attrsFromRawData extracts a bounded JSON-ish snapshot of attributes embedded in collector raw JSON.
func attrsFromRawData(raw datatypes.JSON) string {
	if len(raw) == 0 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return ""
	}
	attrKey := ""
	for _, k := range []string{"attributes", "attributeList", "params"} {
		if _, ok := m[k]; ok {
			attrKey = k
			break
		}
	}
	if attrKey == "" {
		return ""
	}
	bytes, err := json.Marshal(m[attrKey])
	if err != nil {
		return ""
	}
	out := strings.TrimSpace(string(bytes))
	if len(out) > 600 {
		out = out[:600] + "…"
	}
	return out
}

func categoryFromRawData(raw datatypes.JSON) string {
	if len(raw) == 0 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return ""
	}
	for _, k := range []string{"category", "categoryName", "category_name", "leafCategory"} {
		if s := stringifyAny(m[k]); s != "" {
			return s
		}
	}
	return ""
}

func stringifyAny(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	default:
		if v == nil {
			return ""
		}
		return strings.TrimSpace(fmt.Sprint(x))
	}
}
