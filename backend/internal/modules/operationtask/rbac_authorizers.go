package operationtask

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
)

type RBACAuthorizer struct {
	DB *gorm.DB
}

func NewRBACAuthorizer(db *gorm.DB) *RBACAuthorizer {
	return &RBACAuthorizer{DB: db}
}

func (a *RBACAuthorizer) CanRead(ctx context.Context, tenantID int64, actorID uuid.UUID) error {
	return a.require(ctx, tenantID, actorID, adminperm.PermOperationTaskAuditRead)
}

func (a *RBACAuthorizer) CanCreate(ctx context.Context, tenantID int64, actorID uuid.UUID) error {
	return a.require(ctx, tenantID, actorID, adminperm.PermOperationTaskEdit)
}

func (a *RBACAuthorizer) CanEdit(ctx context.Context, tenantID int64, actorID uuid.UUID, taskID uuid.UUID) error {
	if taskID == uuid.Nil {
		return ErrValidation
	}
	return a.require(ctx, tenantID, actorID, adminperm.PermOperationTaskEdit)
}

func (a *RBACAuthorizer) CanCancel(ctx context.Context, tenantID int64, actorID uuid.UUID, taskID uuid.UUID) error {
	if taskID == uuid.Nil {
		return ErrValidation
	}
	return a.require(ctx, tenantID, actorID, adminperm.PermOperationTaskEdit)
}

func (a *RBACAuthorizer) CanReview(ctx context.Context, tenantID int64, actorID uuid.UUID) error {
	return a.require(ctx, tenantID, actorID, adminperm.PermOperationTaskReview)
}

func (a *RBACAuthorizer) CanExecute(ctx context.Context, tenantID int64, actorID uuid.UUID, taskID uuid.UUID) error {
	if taskID == uuid.Nil {
		return ErrValidation
	}
	return a.require(ctx, tenantID, actorID, adminperm.PermOperationTaskExecute)
}

func (a *RBACAuthorizer) CanRetry(ctx context.Context, tenantID int64, actorID uuid.UUID, taskID uuid.UUID) error {
	if taskID == uuid.Nil {
		return ErrValidation
	}
	return a.require(ctx, tenantID, actorID, adminperm.PermOperationTaskRetry)
}

func (a *RBACAuthorizer) require(ctx context.Context, tenantID int64, actorID uuid.UUID, permission string) error {
	if a == nil || a.DB == nil || tenantID < 0 || actorID == uuid.Nil || strings.TrimSpace(permission) == "" {
		return ErrPermissionDenied
	}
	registry, memberships := adminperm.TenantSchemaState(a.DB)
	if registry || memberships {
		if !registry || !memberships || tenantID <= 0 {
			return ErrPermissionDenied
		}
		if err := adminperm.CheckTenantPermission(ctx, a.DB, tenantID, actorID, permission); err != nil {
			return ErrPermissionDenied
		}
		return nil
	}
	var user admin.AdminUser
	err := a.DB.WithContext(ctx).Select("id", "tenant_id", "role", "status").First(&user, "id = ?", actorID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrPermissionDenied
	}
	if err != nil {
		return stableError(err, ErrConflict)
	}
	if user.TenantID != tenantID || !strings.EqualFold(strings.TrimSpace(user.Status), admin.StatusActive) {
		return ErrPermissionDenied
	}
	if !adminperm.StrictHasPermission(user.Role, permission) {
		return ErrPermissionDenied
	}
	return nil
}
