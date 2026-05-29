package settings

import (
	"context"
	"fmt"

	"github.com/trademind-ai/trademind/backend/internal/encrypt"
	"gorm.io/gorm"
)

// EnsureImageDefaults inserts the image settings group when keys are missing.
func EnsureImageDefaults(ctx context.Context, db *gorm.DB, enc *encrypt.Service) error {
	if db == nil {
		return nil
	}
	type def struct {
		key       string
		val       string
		encrypted bool
	}
	defs := []def{
		{"provider", "noop", false},
		{"provider_preset", "", false},
		{"image_task_default_provider", "", false},
		{"image_task_default_size", "1024x1024", false},
		{"image_task_default_quality", "standard", false},
		{"removebg_api_key", "", true},
		{"removebg_base_url", "", false},
		{"openai_image_base_url", "", false},
		{"openai_image_api_key", "", true},
		{"openai_image_model", "gpt-image-1", false},
		{"openai_image_size", "1024x1024", false},
		{"openai_image_quality", "standard", false},
		{"openai_image_background", "", false},
		{"comfyui_base_url", "http://127.0.0.1:8188", false},
		{"comfyui_api_key", "", true},
		{"comfyui_workflow_json", "", false},
		{"comfyui_prompt_node_id", "", false},
		{"comfyui_image_node_id", "", false},
		{"comfyui_output_node_id", "", false},
		{"comfyui_timeout_sec", "180", false},
		{"comfyui_poll_interval_seconds", "2", false},
		{"comfyui_max_poll_seconds", "180", false},
		{"dashscope_image_api_key", "", true},
		{"dashscope_image_base_url", "", false},
		{"dashscope_image_model", "wan2.7-image-pro", false},
		{"dashscope_image_size", "2K", false},
		{"dashscope_image_quality", "", false},
		{"volcengine_image_api_key", "", true},
		{"volcengine_image_base_url", "", false},
		{"volcengine_image_model", "", false},
		{"volcengine_image_size", "1024x1024", false},
		{"siliconflow_image_api_key", "", true},
		{"siliconflow_image_base_url", "", false},
		{"siliconflow_image_model", "", false},
		{"siliconflow_image_size", "1024x1024", false},
		{"hunyuan_image_api_key", "", true},
		{"hunyuan_image_base_url", "", false},
		{"hunyuan_image_model", "", false},
		{"timeout_sec", "60", false},

		// OCR Settings
		{"ocr_provider", "paddleocr", false},
		{"ocr_base_url", "", false},
		{"ocr_api_key", "", true},
		{"ocr_secret", "", true},
		{"ocr_timeout_sec", "30", false},
		{"ocr_min_confidence", "0.8", false},
		{"ocr_fallback_to_vision", "true", false},

		// Inpaint Settings
		{"erase_mode", "auto", false},
		{"ai_inpaint_comfyui_base_url", "http://127.0.0.1:8188", false},
		{"ai_inpaint_comfyui_workflow_json", "", false},
		{"ai_inpaint_comfyui_prompt_node_id", "", false},
		{"ai_inpaint_comfyui_image_node_id", "", false},
		{"ai_inpaint_comfyui_mask_node_id", "", false},
		{"ai_inpaint_comfyui_output_node_id", "", false},
	}
	for _, d := range defs {
		var n int64
		if err := db.WithContext(ctx).Model(&Setting{}).
			Where("tenant_id = ? AND group_key = ? AND item_key = ?", 0, "image", d.key).
			Count(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		val := d.val
		isEnc := d.encrypted
		if d.encrypted {
			if enc == nil {
				isEnc = false
				val = ""
			} else {
				ev, err := enc.Encrypt([]byte(val))
				if err != nil {
					return fmt.Errorf("settings image seed %s: %w", d.key, err)
				}
				val = ev
			}
		}
		row := Setting{
			TenantID:    0,
			GroupKey:    "image",
			ItemKey:     d.key,
			ItemValue:   val,
			ValueType:   "string",
			IsEncrypted: isEnc,
			Remark:      "",
		}
		if d.key == "comfyui_workflow_json" || d.key == "ai_inpaint_comfyui_workflow_json" {
			row.ValueType = "json"
		}
		if d.key == "comfyui_api_key" {
			row.Remark = "optional; encrypted when set"
		}
		if err := db.WithContext(ctx).Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}
