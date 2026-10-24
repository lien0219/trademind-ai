// Package tenantmgmt owns tenant lifecycle writes. HTTP authentication must
// authenticate platform operators separately from tenant-local RBAC.
package tenantmgmt

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var codePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{2,47}$`)

type Service struct {
	DB    *gorm.DB
	Audit *operationlog.Service
}

type CreateTenantInput struct {
	Code        string
	Name        string
	Description string
}

func (s *Service) requireOperator(ctx context.Context, actor uuid.UUID) error {
	if s == nil || s.DB == nil || actor == uuid.Nil {
		return errors.New("platform operator authorization unavailable")
	}
	var row admin.PlatformOperator
	if err := s.DB.WithContext(ctx).Where("user_id = ? AND enabled = ?", actor, true).First(&row).Error; err != nil {
		return errors.New("platform operator required")
	}
	return nil
}

// Create provisions metadata atomically, with auditable actor attribution.
// Creating a tenant does not imply access to its business data or grant
// the caller membership in that tenant.
func (s *Service) Create(ctx context.Context, actor uuid.UUID, input CreateTenantInput) (*admin.Tenant, error) {
	if err := s.requireOperator(ctx, actor); err != nil {
		return nil, err
	}
	input.Code = strings.ToLower(strings.TrimSpace(input.Code))
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	if !codePattern.MatchString(input.Code) || input.Name == "" || len([]rune(input.Name)) > 128 || len([]rune(input.Description)) > 512 {
		return nil, errors.New("invalid tenant metadata")
	}
	if s.Audit == nil {
		return nil, errors.New("tenant audit unavailable")
	}
	row := &admin.Tenant{
		Code: input.Code, Name: input.Name, Description: input.Description,
		Status: admin.TenantActive, CreatedBy: &actor,
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.requireOperatorTx(ctx, tx, actor); err != nil {
			return err
		}
		if err := tx.Create(row).Error; err != nil {
			return fmt.Errorf("create tenant: %w", err)
		}
		return s.Audit.WriteTxContext(ctx, tx, operationlog.WriteOpts{
			TenantID: row.ID, AdminUserID: &actor,
			Action: "tenant.create", Resource: "tenant",
			ResourceID: fmt.Sprintf("%d", row.ID), Status: "success",
			Message: "tenant registered; no member privileges granted",
		})
	})
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (s *Service) requireOperatorTx(ctx context.Context, tx *gorm.DB, actor uuid.UUID) error {
	var row admin.PlatformOperator
	if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "SHARE"}).
		Where("user_id = ? AND enabled = ?", actor, true).First(&row).Error; err != nil {
		return errors.New("platform operator required")
	}
	return nil
}

// SetStatus is fail-closed: closed tenants cannot be reopened through this
// operation, and state changes must commit together with an audit record.
func (s *Service) SetStatus(ctx context.Context, actor uuid.UUID, id int64, status string) error {
	if err := s.requireOperator(ctx, actor); err != nil {
		return err
	}
	if id <= 0 || (status != admin.TenantActive && status != admin.TenantSuspended && status != admin.TenantClosed) {
		return errors.New("invalid tenant status")
	}
	if s.Audit == nil {
		return errors.New("tenant audit unavailable")
	}
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.requireOperatorTx(ctx, tx, actor); err != nil {
			return err
		}
		var row admin.Tenant
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "id = ?", id).Error; err != nil {
			return err
		}
		if row.Status == admin.TenantClosed && status != admin.TenantClosed {
			return errors.New("closed tenant cannot be reactivated")
		}
		before := row.Status
		if err := tx.Model(&row).Update("status", status).Error; err != nil {
			return err
		}
		return s.Audit.WriteTxContext(ctx, tx, operationlog.WriteOpts{
			TenantID: id, AdminUserID: &actor, Action: "tenant.status.update",
			Resource: "tenant", ResourceID: fmt.Sprintf("%d", id),
			Status: "success", Message: fmt.Sprintf("before=%s after=%s", before, status),
		})
	})
}
