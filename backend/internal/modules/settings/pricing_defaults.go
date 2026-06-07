package settings

import (
	"context"

	"gorm.io/gorm"
)

// EnsurePricingDefaults inserts publish pricing rule keys when missing (idempotent).
func EnsurePricingDefaults(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return nil
	}
	type def struct {
		key string
		val string
	}
	defs := []def{
		{"default_markup_type", "percent"},
		{"default_markup_percent", "30"},
		{"default_markup_amount", "0"},
		{"default_markup_multiplier", "1.5"},
		{"default_shipping_cost", "0"},
		{"default_shipping_cost_per_weight", "0"},
		{"default_platform_commission_percent", "0"},
		{"default_min_profit", "0"},
		{"default_exchange_rate", "1"},
		{"default_rounding_mode", ".99"},
		{"default_min_margin_percent", "10"},
		{"default_currency", "CNY"},
		{"enable_platform_pricing_rules", "true"},
		{"tiktok_markup_percent", "30"},
		{"shopee_markup_percent", "30"},
		{"lazada_markup_percent", "30"},
		{"amazon_markup_percent", "30"},
		{"batch_max_size", "500"},
	}
	for _, d := range defs {
		var n int64
		if err := db.WithContext(ctx).Model(&Setting{}).
			Where("tenant_id = ? AND group_key = ? AND item_key = ?", 0, "pricing", d.key).
			Count(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		row := Setting{
			TenantID:    0,
			GroupKey:    "pricing",
			ItemKey:     d.key,
			ItemValue:   d.val,
			ValueType:   "string",
			IsEncrypted: false,
		}
		if err := db.WithContext(ctx).Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}
