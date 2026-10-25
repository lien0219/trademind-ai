package tenantmgmt

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/auth"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrTenantMustBeInactive = errors.New("tenant must be suspended or archived before deletion")
	ErrTenantCodeMismatch = errors.New("tenant confirmation code mismatch")
	ErrTenantAlreadyDeleted = errors.New("tenant already deleted")
)

// DeleteTenant is an audited logical removal. Child business records and
// legal/audit evidence are never cascaded; the ID and code are never reused.
func (s *Service) DeleteTenant(ctx context.Context, actor uuid.UUID, id int64, code string, version int64) error {
	if err := s.requireOperator(ctx, actor); err != nil { return err }
	if id <= 0 || version <= 0 || strings.TrimSpace(code) == "" { return errors.New("invalid tenant deletion request") }
	if s.Audit == nil { return errors.New("tenant deletion audit unavailable") }
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.requireOperatorTx(ctx, tx, actor); err != nil { return err }
		if err := adminperm.LockTenantMutation(tx, id); err != nil { return fmt.Errorf("lock tenant for deletion: %w", err) }
		var row admin.Tenant
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "id = ?", id).Error; err != nil { return err }
		if row.Code == admin.RootWorkspaceCode {return errors.New("Root workspace cannot be deleted")}
		if row.Status == admin.TenantDeleted { return ErrTenantAlreadyDeleted }
		if row.Version != version { return ErrTenantVersionConflict }
		if row.Code != code { return ErrTenantCodeMismatch }
		switch row.Status {
		case admin.TenantSuspended, admin.TenantArchived, admin.TenantClosed:
		default: return ErrTenantMustBeInactive
		}
		now := time.Now().UTC()
		res := tx.Model(&row).Where("version = ? AND status = ?", version, row.Status).Updates(map[string]any{
			"status": admin.TenantDeleted, "version": version+1, "deleted_at": now,
		})
		if res.Error != nil { return res.Error }
		if res.RowsAffected != 1 { return ErrTenantVersionConflict }
		if err := tx.Model(&auth.AuthSession{}).Where("tenant_id = ? AND status = ?", id, auth.SessionStatusActive).
			Updates(map[string]any{"status": auth.SessionStatusRevoked, "revoked_at": now, "revoke_reason": "tenant_deleted"}).Error; err != nil { return err }
		if err := tx.Model(&auth.AuthRefreshToken{}).Where("tenant_id = ? AND status IN ?", id, []string{auth.RefreshStatusActive, auth.RefreshStatusRotated}).
			Updates(map[string]any{"status": auth.RefreshStatusRevoked, "revoked_at": now, "revoke_reason": "tenant_deleted"}).Error; err != nil { return err }
		if err := tx.Model(&admin.TenantInvitation{}).Where("tenant_id = ? AND status = ?", id, admin.InvitationPending).
			Update("status", admin.InvitationCancelled).Error; err != nil { return err }
		return s.Audit.WriteTxContext(ctx, tx, operationlog.WriteOpts{
			TenantID: id, AdminUserID: &actor, Action: "tenant.delete", Resource: "tenant",
			ResourceID: fmt.Sprint(id), Status: "success",
			Message: fmt.Sprintf("logical deletion code=%s before=%s version=%d retained=true", row.Code, row.Status, version),
		})
	})
}
