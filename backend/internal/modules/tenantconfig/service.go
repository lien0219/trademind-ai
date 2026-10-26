package tenantconfig

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrVersionConflict = errors.New("tenant setting version conflict")
var ErrRestoreNotFound = errors.New("tenant setting revision not found")

type Snapshot struct {
	Module    string          `json:"module"`
	Version   int64           `json:"version"`
	Values    json.RawMessage `json:"values"`
	Inherited bool            `json:"inherited"`
}

// Read always uses trusted tenant identity, never an HTTP-supplied tenant id.
// Missing settings inherit safe defaults; failed SQL reads never fall back.
func Read(ctx context.Context, db *gorm.DB, tenantID int64, module string) (Snapshot, error) {
	var out Snapshot
	defaults, err := Defaults(module)
	if err != nil {
		return out, err
	}
	if db == nil || tenantID <= 0 {
		return out, errors.New("tenant context unavailable")
	}
	base, err := json.Marshal(defaults)
	if err != nil {
		return out, err
	}
	out = Snapshot{Module: module, Version: 0, Values: base, Inherited: true}
	var row Setting
	err = db.WithContext(ctx).Where("tenant_id = ? AND module = ?", tenantID, module).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return out, nil
	}
	if err != nil {
		return Snapshot{}, err
	}
	validated, err := Validate(module, []byte(row.Values))
	if err != nil {
		return Snapshot{}, fmt.Errorf("invalid stored tenant configuration: %w", err)
	}
	return Snapshot{Module: module, Version: row.Version, Values: validated, Inherited: false}, nil
}

func ReadAI(ctx context.Context, db *gorm.DB, tenantID int64) (AI, int64, error) {
	s, err := Read(ctx, db, tenantID, ModuleAI)
	if err != nil {
		return AI{}, 0, err
	}
	var v AI
	if err := json.Unmarshal(s.Values, &v); err != nil {
		return AI{}, 0, err
	}
	return v, s.Version, nil
}
func ReadCollect(ctx context.Context, db *gorm.DB, tenantID int64) (Collect, int64, error) {
	s, err := Read(ctx, db, tenantID, ModuleCollect)
	if err != nil {
		return Collect{}, 0, err
	}
	var v Collect
	if err := json.Unmarshal(s.Values, &v); err != nil {
		return Collect{}, 0, err
	}
	return v, s.Version, nil
}

func CheckCollectSource(cfg Collect, source string) error {
	if len(cfg.AllowedSources) == 0 {
		return nil
	}
	for _, allowed := range cfg.AllowedSources {
		if allowed == source {
			return nil
		}
	}
	return errors.New("collect source is disabled for this tenant")
}

// Write serializes on the parent tenant row. Version 0 is a create; stale
// writers always receive 409. Every successful mutation appends immutable
// revision evidence and a chained operation audit in the SAME transaction.
func Write(ctx context.Context, db *gorm.DB, audit *operationlog.Service, tenantID int64, module string, raw []byte, expected int64, actor uuid.UUID, action string) (Snapshot, error) {
	if db == nil || audit == nil || tenantID <= 0 || actor == uuid.Nil || expected < 0 {
		return Snapshot{}, errors.New("tenant setting write unavailable")
	}
	normalized, err := Validate(module, raw)
	if err != nil {
		return Snapshot{}, err
	}
	var out Snapshot
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var tenant admin.Tenant
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND status = ?", tenantID, admin.TenantActive).Take(&tenant).Error; err != nil {
			return err
		}
		var current Setting
		err := tx.Where("tenant_id = ? AND module = ?", tenantID, module).Take(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if expected != 0 {
				return ErrVersionConflict
			}
			current = Setting{TenantID: tenantID, Module: module, Values: datatypes.JSON(normalized), Version: 1, UpdatedBy: actor}
			if err := tx.Create(&current).Error; err != nil {
				return err
			}
		} else {
			if err != nil {
				return err
			}
			if current.Version != expected {
				return ErrVersionConflict
			}
			current.Version++
			current.Values = datatypes.JSON(normalized)
			current.UpdatedBy = actor
			if err := tx.Save(&current).Error; err != nil {
				return err
			}
		}
		rev := Revision{ID: uuid.New(), TenantID: tenantID, Module: module, Version: current.Version, Values: datatypes.JSON(normalized), ActorID: actor, Action: action}
		if err := tx.Create(&rev).Error; err != nil {
			return err
		}
		if err := audit.WriteTxContext(ctx, tx, operationlog.WriteOpts{
			TenantID: tenantID, AdminUserID: &actor, Action: "tenant.settings." + action,
			Resource: "tenant_business_setting", ResourceID: module, Status: "success",
			Message: fmt.Sprintf("module=%s version=%d", module, current.Version),
		}); err != nil {
			return err
		}
		out = Snapshot{Module: module, Version: current.Version, Values: normalized, Inherited: false}
		return nil
	})
	return out, err
}

func History(ctx context.Context, db *gorm.DB, tenantID int64, module string) ([]Revision, error) {
	if _, err := Defaults(module); err != nil {
		return nil, err
	}
	var out []Revision
	err := db.WithContext(ctx).Where("tenant_id = ? AND module = ?", tenantID, module).Order("version DESC").Limit(50).Find(&out).Error
	return out, err
}

func Restore(ctx context.Context, db *gorm.DB, audit *operationlog.Service, tenantID int64, module string, targetVersion, expectedVersion int64, actor uuid.UUID) (Snapshot, error) {
	if targetVersion <= 0 {
		return Snapshot{}, ErrRestoreNotFound
	}
	var revision Revision
	err := db.WithContext(ctx).Where("tenant_id = ? AND module = ? AND version = ?", tenantID, module, targetVersion).Take(&revision).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Snapshot{}, ErrRestoreNotFound
	}
	if err != nil {
		return Snapshot{}, err
	}
	return Write(ctx, db, audit, tenantID, module, revision.Values, expectedVersion, actor, "restore")
}
