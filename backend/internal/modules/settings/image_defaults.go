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
		{"removebg_api_key", "", true},
		{"openai_image_model", "", false},
		{"comfyui_base_url", "", false},
		{"comfyui_workflow_json", "{}", false},
		{"timeout_sec", "60", false},
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
		if d.key == "comfyui_workflow_json" {
			row.ValueType = "json"
		}
		if err := db.WithContext(ctx).Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}
