package aiprompt

import (
	"context"
	"encoding/json"
	"strings"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const CodeProductTitleOptimize = "product_title_optimize"
const CodeProductDescriptionGenerate = "product_description_generate"

// EnsureDefaults creates built-in prompts when missing.
func EnsureDefaults(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return nil
	}
	if err := ensureProductTitleOptimize(ctx, db); err != nil {
		return err
	}
	return ensureProductDescriptionGenerate(ctx, db)
}

func ensureProductTitleOptimize(ctx context.Context, db *gorm.DB) error {
	schema, _ := json.Marshal(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"optimizedTitle": map[string]string{"type": "string"},
			"keywords": map[string]any{
				"type":  "array",
				"items": map[string]string{"type": "string"},
			},
			"reason": map[string]string{"type": "string"},
		},
		"required": []string{"optimizedTitle", "keywords", "reason"},
	})
	defaultSys := strings.TrimSpace(`You are an expert cross-border e-commerce copywriter.
Return ONLY valid JSON (no markdown fences) with keys: optimizedTitle (string), keywords (string array), reason (short string in the same language as the user's requested listing language).
The optimizedTitle must respect max length and platform style hints from the user message.`)
	defaultUser := strings.TrimSpace(`Optimize this product listing title.

Context:
- Current title: {{title}}
- Category: {{category}}
- Attributes / specs: {{attributes}}
- Target language: {{language}}
- Target platform: {{platform}}
- Max title length (characters): {{maxLength}}

Reply with JSON only.`)

	var count int64
	if err := db.WithContext(ctx).Model(&AIPrompt{}).Where("code = ?", CodeProductTitleOptimize).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	row := &AIPrompt{
		Code:         CodeProductTitleOptimize,
		Name:         "商品标题优化",
		Scene:        "product",
		Provider:     "",
		Model:        "",
		SystemPrompt: defaultSys,
		UserPrompt:   defaultUser,
		OutputSchema: datatypes.JSON(schema),
		Temperature:  0.4,
		MaxTokens:    800,
		Enabled:      true,
	}
	return db.WithContext(ctx).Create(row).Error
}

func ensureProductDescriptionGenerate(ctx context.Context, db *gorm.DB) error {
	schema, _ := json.Marshal(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"description": map[string]string{"type": "string"},
			"highlights": map[string]any{
				"type":  "array",
				"items": map[string]string{"type": "string"},
			},
			"specifications": map[string]any{
				"type":  "array",
				"items": map[string]string{"type": "string"},
			},
			"packageIncludes": map[string]any{
				"type":  "array",
				"items": map[string]string{"type": "string"},
			},
			"notes":  map[string]string{"type": "string"},
			"reason": map[string]string{"type": "string"},
		},
		"required": []string{"description", "highlights", "specifications", "packageIncludes", "notes", "reason"},
	})
	defaultSys := strings.TrimSpace(`You are an expert cross-border e-commerce copywriter for marketplace product detail pages.
Return ONLY valid JSON (no markdown fences) with exactly these keys: description (string), highlights (string array), specifications (string array), packageIncludes (string array), notes (string), reason (short string explaining choices, same language as description).

Rules:
- Base copy ONLY on facts present in the user message. Do not invent features, materials, certifications, or guarantees the product does not have.
- No exaggerated claims, medical claims, or policy-bypass language. Avoid hype words that platforms often restrict.
- Structure the detail page for cross-border sellers: cover Product Highlights, Specifications, Package Includes, and Notes where appropriate (you may weave these into description or use list fields).
- Default listing context in the user message uses English on TikTok Shop unless overridden; match the requested language and tone.
- Keep bullets concise; description can be several short paragraphs suitable for a PDP.`)
	defaultUser := strings.TrimSpace(`Generate a product detail page copy package.

Product context:
- Listing title (seller/current): {{title}}
- Original title (source): {{originalTitle}}
- AI-optimized title (if any): {{aiTitle}}
- Attributes / raw specs summary: {{attributes}}
- SKU lines: {{skus}}
- Target language: {{language}}
- Target platform: {{platform}}
- Tone: {{tone}}

Reply with JSON only using the schema from the system message.`)

	var count int64
	if err := db.WithContext(ctx).Model(&AIPrompt{}).Where("code = ?", CodeProductDescriptionGenerate).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	row := &AIPrompt{
		Code:         CodeProductDescriptionGenerate,
		Name:         "商品描述生成",
		Scene:        "product",
		Provider:     "",
		Model:        "",
		SystemPrompt: defaultSys,
		UserPrompt:   defaultUser,
		OutputSchema: datatypes.JSON(schema),
		Temperature:  0.45,
		MaxTokens:    2500,
		Enabled:      true,
	}
	return db.WithContext(ctx).Create(row).Error
}
