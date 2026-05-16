package imagetask

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
)

func (s *Service) prepareReplaceBackgroundHints(ctx context.Context, task *ImageTask, hints map[string]any) map[string]any {
	next := map[string]any{}
	for k, v := range hints {
		next[k] = v
	}

	prompt := stringFromMap(next, "prompt")
	neg := stringFromMap(next, "negativePrompt")
	if neg == "" {
		neg = stringFromMap(next, "negative_prompt")
	}
	background := stringFromMap(next, "background")
	style := stringFromMap(next, "style")
	platform := stringFromMap(next, "platform")
	if platform == "" {
		platform = stringFromMap(next, "targetPlatform")
	}
	size := stringFromMap(next, "size")

	var b strings.Builder
	b.WriteString("Editing task: keep the product subject exactly as-is (same colors, shape, logos, and any text printed on the product). ")
	b.WriteString("Replace only the background with the described scene; do not invent new product features. ")
	b.WriteString("Output should be sharp, high-resolution ecommerce imagery suitable for marketplace main or detail images. ")
	b.WriteString("No watermarks, heavy blur, warping, or distorted geometry. ")
	b.WriteString("Replace the surrounding scene while preserving realistic contact shadows when appropriate. ")
	b.WriteString("Avoid adult, violent, illegal, counterfeit, or misleading content. ")

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

	if background != "" {
		fmt.Fprintf(&b, "Target background: %s. ", background)
	}
	if style != "" {
		fmt.Fprintf(&b, "Visual style: %s. ", style)
	}
	if platform != "" {
		fmt.Fprintf(&b, "Target platform: %s. ", platform)
	}
	if size != "" {
		fmt.Fprintf(&b, "Preferred output dimensions: %s. ", size)
	}
	if prompt != "" {
		fmt.Fprintf(&b, "Additional instructions: %s. ", prompt)
	}
	if neg != "" {
		fmt.Fprintf(&b, "Negative prompt: %s. ", neg)
	}

	next["assembled_prompt"] = strings.TrimSpace(b.String())
	return next
}
