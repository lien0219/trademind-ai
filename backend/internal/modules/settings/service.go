package settings

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/encrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Service orchestrates settings persistence and encryption.
type Service struct {
	DB        *gorm.DB
	Encrypter *encrypt.Service
}

// List returns all settings with sensitive values masked when encrypted.
func (s *Service) List(ctx context.Context) ([]Setting, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("settings: no db")
	}
	var rows []Setting
	if err := s.DB.WithContext(ctx).Order("tenant_id, group_key, item_key").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]Setting, len(rows))
	for i := range rows {
		out[i] = rows[i]
		if !out[i].IsEncrypted || strings.TrimSpace(out[i].ItemValue) == "" {
			continue
		}
		plain, err := s.decryptStored(out[i].ItemValue)
		if err != nil {
			out[i].ItemValue = encrypt.MaskSecret(out[i].ItemValue)
			continue
		}
		out[i].ItemValue = encrypt.MaskSecret(string(plain))
	}
	return out, nil
}

func (s *Service) decryptStored(stored string) ([]byte, error) {
	if s.Encrypter == nil {
		return nil, errors.New("no encrypter")
	}
	return s.Encrypter.Decrypt(stored)
}

// PutBulk upserts items by (tenantId, groupKey, itemKey).
func (s *Service) PutBulk(ctx context.Context, items []PutItem) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("settings: no db")
	}
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, it := range items {
			if err := s.putOne(tx, it); err != nil {
				return err
			}
		}
		return nil
	})
}

// PutItem is the API payload for create/update.
type PutItem struct {
	TenantID    int64
	GroupKey    string
	ItemKey     string
	ItemValue   string
	ValueType   string
	IsEncrypted bool
	Remark      string
}

func (s *Service) putOne(tx *gorm.DB, it PutItem) error {
	gk := strings.TrimSpace(it.GroupKey)
	ik := strings.TrimSpace(it.ItemKey)
	if gk == "" || ik == "" {
		return fmt.Errorf("settings: groupKey and itemKey required")
	}
	tenant := it.TenantID
	valType := strings.TrimSpace(it.ValueType)
	if valType == "" {
		valType = "string"
	}

	var cur Setting
	err := tx.Where("tenant_id = ? AND group_key = ? AND item_key = ?", tenant, gk, ik).First(&cur).Error
	exists := true
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			exists = false
		} else {
			return err
		}
	}

	val := it.ItemValue
	if it.IsEncrypted && encrypt.LooksMasked(val) {
		if !exists {
			return fmt.Errorf("settings: cannot create encrypted item %s/%s with masked value", gk, ik)
		}
		upd := map[string]any{
			"is_encrypted": it.IsEncrypted,
			"remark":       strings.TrimSpace(it.Remark),
		}
		if vt := strings.TrimSpace(it.ValueType); vt != "" {
			upd["value_type"] = vt
		}
		return tx.Model(&Setting{}).Where("id = ?", cur.ID).Updates(upd).Error
	}

	if it.IsEncrypted {
		if s.Encrypter == nil {
			return fmt.Errorf("settings: APP_MASTER_KEY required for encrypted item %s/%s", gk, ik)
		}
		enc, err := s.Encrypter.Encrypt([]byte(val))
		if err != nil {
			return fmt.Errorf("settings: encrypt %s/%s: %w", gk, ik, err)
		}
		val = enc
	}

	row := Setting{
		TenantID:    tenant,
		GroupKey:    gk,
		ItemKey:     ik,
		ItemValue:   val,
		ValueType:   valType,
		IsEncrypted: it.IsEncrypted,
		Remark:      strings.TrimSpace(it.Remark),
	}

	if exists {
		updates := map[string]any{
			"item_value":   row.ItemValue,
			"value_type":   row.ValueType,
			"is_encrypted": row.IsEncrypted,
			"remark":       row.Remark,
		}
		return tx.Model(&Setting{}).Where("id = ?", cur.ID).Updates(updates).Error
	}

	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "tenant_id"},
			{Name: "group_key"},
			{Name: "item_key"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"item_value", "value_type", "is_encrypted", "remark", "updated_at",
		}),
	}).Create(&row).Error
}
