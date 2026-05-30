package settings

import (
	"context"
	"fmt"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/encrypt"
	aigate "github.com/trademind-ai/trademind/backend/internal/providers/ai"
	"gorm.io/gorm"
)

// EnsureAIProviderDefaults inserts per-provider AI text settings keys when missing (idempotent).
func EnsureAIProviderDefaults(ctx context.Context, db *gorm.DB, enc *encrypt.Service) error {
	if db == nil {
		return nil
	}
	if err := migrateLegacyAIKeys(ctx, db, enc); err != nil {
		return err
	}
	type def struct {
		key       string
		val       string
		encrypted bool
	}
	defs := []def{
		{"provider", "openai_compatible", false},
		{"openai_api_key", "", true},
		{"openai_base_url", "", false},
		{"openai_model", "", false},
		{"openai_compatible_api_key", "", true},
		{"openai_compatible_base_url", "", false},
		{"openai_compatible_model", "", false},
		{"deepseek_api_key", "", true},
		{"deepseek_base_url", "", false},
		{"deepseek_model", "", false},
		{"qwen_api_key", "", true},
		{"qwen_base_url", "", false},
		{"qwen_model", "", false},
	}
	for _, d := range defs {
		var n int64
		if err := db.WithContext(ctx).Model(&Setting{}).
			Where("tenant_id = ? AND group_key = ? AND item_key = ?", 0, "ai", d.key).
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
					return fmt.Errorf("settings ai seed %s: %w", d.key, err)
				}
				val = ev
			}
		}
		row := Setting{
			TenantID:    0,
			GroupKey:    "ai",
			ItemKey:     d.key,
			ItemValue:   val,
			ValueType:   "string",
			IsEncrypted: isEnc,
		}
		if err := db.WithContext(ctx).Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func migrateLegacyAIKeys(ctx context.Context, db *gorm.DB, enc *encrypt.Service) error {
	if db == nil {
		return nil
	}
	var legacy Setting
	err := db.WithContext(ctx).
		Where("tenant_id = ? AND group_key = ? AND item_key = ?", 0, "ai", "api_key").
		First(&legacy).Error
	if err != nil {
		return nil
	}
	if strings.TrimSpace(legacy.ItemValue) == "" {
		return nil
	}

	pname := "openai_compatible"
	var providerRow Setting
	if err := db.WithContext(ctx).
		Where("tenant_id = ? AND group_key = ? AND item_key = ?", 0, "ai", "provider").
		First(&providerRow).Error; err == nil {
		if p := strings.TrimSpace(providerRow.ItemValue); p != "" {
			pname = aigate.NormalizeProviderName(p)
		}
	}

	if err := copyLegacyAISettingIfEmpty(ctx, db, legacy, aigate.ProviderAPIKeyKey(pname)); err != nil {
		return err
	}
	var legacyBase Setting
	if err := db.WithContext(ctx).
		Where("tenant_id = ? AND group_key = ? AND item_key = ?", 0, "ai", "base_url").
		First(&legacyBase).Error; err == nil {
		if err := copyLegacyAISettingIfEmpty(ctx, db, legacyBase, aigate.ProviderBaseURLKey(pname)); err != nil {
			return err
		}
	}
	var legacyModel Setting
	if err := db.WithContext(ctx).
		Where("tenant_id = ? AND group_key = ? AND item_key = ?", 0, "ai", "model").
		First(&legacyModel).Error; err == nil {
		if err := copyLegacyAISettingIfEmpty(ctx, db, legacyModel, aigate.ProviderModelKey(pname)); err != nil {
			return err
		}
	}
	_ = enc
	return nil
}

func copyLegacyAISettingIfEmpty(ctx context.Context, db *gorm.DB, legacy Setting, targetKey string) error {
	if strings.TrimSpace(legacy.ItemValue) == "" {
		return nil
	}
	var target Setting
	err := db.WithContext(ctx).
		Where("tenant_id = ? AND group_key = ? AND item_key = ?", 0, "ai", targetKey).
		First(&target).Error
	if err == nil && strings.TrimSpace(target.ItemValue) != "" {
		return nil
	}
	if err == nil {
		return db.WithContext(ctx).Model(&Setting{}).Where("id = ?", target.ID).Updates(map[string]any{
			"item_value":   legacy.ItemValue,
			"is_encrypted": legacy.IsEncrypted,
		}).Error
	}
	return db.WithContext(ctx).Create(&Setting{
		TenantID:    0,
		GroupKey:    "ai",
		ItemKey:     targetKey,
		ItemValue:   legacy.ItemValue,
		ValueType:   "string",
		IsEncrypted: legacy.IsEncrypted,
	}).Error
}
