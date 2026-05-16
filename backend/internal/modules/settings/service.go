package settings

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/encrypt"
	"github.com/trademind-ai/trademind/backend/internal/providers/email"
	"github.com/trademind-ai/trademind/backend/internal/providers/email/smtp"
	cosstorage "github.com/trademind-ai/trademind/backend/internal/providers/storage/cos"
	ossstorage "github.com/trademind-ai/trademind/backend/internal/providers/storage/oss"
	"github.com/trademind-ai/trademind/backend/internal/providers/storage/s3store"
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

// PlainByGroup returns plaintext values for one settings group (for internal connectivity checks).
func (s *Service) PlainByGroup(ctx context.Context, tenantID int64, groupKey string) (map[string]string, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("settings: no db")
	}
	gk := strings.TrimSpace(groupKey)
	if gk == "" {
		return nil, fmt.Errorf("settings: groupKey required")
	}
	var rows []Setting
	if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND group_key = ?", tenantID, gk).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]string, len(rows))
	for _, row := range rows {
		v := row.ItemValue
		if row.IsEncrypted && strings.TrimSpace(v) != "" {
			plain, err := s.decryptStored(v)
			if err != nil {
				return nil, fmt.Errorf("settings: decrypt %s/%s: %w", gk, row.ItemKey, err)
			}
			v = string(plain)
		}
		out[row.ItemKey] = v
	}
	return out, nil
}

// TestAIConnection calls the configured OpenAI-compatible chat/completions endpoint once.
func (s *Service) TestAIConnection(ctx context.Context) error {
	m, err := s.PlainByGroup(ctx, 0, "ai")
	if err != nil {
		return err
	}
	base := strings.TrimSpace(m["base_url"])
	if base == "" {
		return fmt.Errorf("ai base_url not configured")
	}
	base = strings.TrimRight(base, "/")
	apiKey := strings.TrimSpace(m["api_key"])
	if apiKey == "" {
		return fmt.Errorf("ai api_key not configured")
	}
	model := strings.TrimSpace(m["model"])
	if model == "" {
		model = "gpt-4o-mini"
	}
	timeout := 20 * time.Second
	if sec := strings.TrimSpace(m["timeout_sec"]); sec != "" {
		if n, err := strconv.Atoi(sec); err == nil && n > 0 && n <= 120 {
			timeout = time.Duration(n) * time.Second
		}
	}
	body := map[string]any{
		"model":       model,
		"max_tokens":  1,
		"temperature": 0,
		"messages": []map[string]string{
			{"role": "user", "content": "ping"},
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("ai request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ai provider returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// TestStorageConnection verifies local writability or S3-compat bucket HeadBucket access.
func (s *Service) TestStorageConnection(ctx context.Context) error {
	m, err := s.PlainByGroup(ctx, 0, "storage")
	if err != nil {
		return err
	}
	kind := strings.ToLower(strings.TrimSpace(m["kind"]))
	if kind == "" {
		kind = "local"
	}
	switch kind {
	case "local":
		root := strings.TrimSpace(m["local_root"])
		if root == "" {
			root = "data/uploads"
		}
		abs, err := filepath.Abs(root)
		if err != nil {
			return fmt.Errorf("storage local_root: %w", err)
		}
		if err := os.MkdirAll(abs, 0o755); err != nil {
			return fmt.Errorf("storage mkdir %q: %w", abs, err)
		}
		f, err := os.CreateTemp(abs, ".trademind-storage-test-*")
		if err != nil {
			return fmt.Errorf("storage write test: %w", err)
		}
		_ = f.Close()
		_ = os.Remove(f.Name())
		return nil
	case "cos":
		if err := cosstorage.TestConnectivity(ctx, m); err != nil {
			return err
		}
		return nil
	case "oss":
		if err := ossstorage.TestConnectivity(ctx, m); err != nil {
			return err
		}
		return nil
	case "s3", "r2", "minio":
		if err := s3store.TestConnectivity(ctx, kind, m); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("unsupported storage kind %q", kind)
	}
}

// TestEmailConnection verifies email configuration by sending a test email.
func (s *Service) TestEmailConnection(ctx context.Context, to string) error {
	m, err := s.PlainByGroup(ctx, 0, "email")
	if err != nil {
		return err
	}
	provider := strings.TrimSpace(m["provider"])
	if provider == "" || provider == "smtp" {
		port, _ := strconv.Atoi(m["smtp_port"])
		cfg := smtp.Config{
			Host:     m["smtp_host"],
			Port:     port,
			Username: m["smtp_username"],
			Password: m["smtp_password"],
			FromName: m["smtp_from_name"],
			From:     m["smtp_from"],
			UseTLS:   m["smtp_use_tls"] == "true",
			UseSSL:   m["smtp_use_ssl"] == "true",
		}
		if cfg.Host == "" || cfg.From == "" {
			return fmt.Errorf("incomplete email settings: need smtp_host and smtp_from")
		}
		p := smtp.NewProvider(cfg)
		return p.Send(ctx, email.SendEmailRequest{
			To:      to,
			Subject: "TradeMind Email Test",
			Content: "This is a test email from TradeMind.",
		})
	}
	return fmt.Errorf("unsupported email provider %q", provider)
}
