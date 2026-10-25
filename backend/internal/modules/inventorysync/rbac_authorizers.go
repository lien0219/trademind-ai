package inventorysync

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

func (a *RBACAuthorizer) CanRunInventorySync(ctx context.Context, tenantID int64, actorID uuid.UUID, shopConnectionID uuid.UUID) error {
	if shopConnectionID == zeroUUID {
		return ErrValidation
	}
	if err := a.require(ctx, tenantID, actorID, adminperm.PermInventorySyncRun); err != nil {
		return err
	}
	return verifyShopConnection(ctx, a.DB, tenantID, shopConnectionID, PlatformDouyin)
}

func (a *RBACAuthorizer) CanRerunInventorySync(ctx context.Context, tenantID int64, actorID uuid.UUID, sourceRunID uuid.UUID) error {
	if sourceRunID == zeroUUID {
		return ErrValidation
	}
	if err := a.require(ctx, tenantID, actorID, adminperm.PermInventorySyncRerun); err != nil {
		return err
	}
	_, err := NewInventorySyncRunRepository(a.DB).GetByID(ctx, tenantID, sourceRunID)
	return err
}

func (a *RBACAuthorizer) CanResolveManualBinding(ctx context.Context, tenantID int64, actorID uuid.UUID, requestID uuid.UUID) error {
	if requestID == zeroUUID {
		return ErrValidation
	}
	if err := a.require(ctx, tenantID, actorID, adminperm.PermSKUBindingResolveManual); err != nil {
		return err
	}
	_, err := NewManualBindingRequestRepository(a.DB).GetByID(ctx, tenantID, requestID)
	return err
}

func (a *RBACAuthorizer) require(ctx context.Context, tenantID int64, actorID uuid.UUID, permission string) error {
	if a == nil || a.DB == nil || validateTenantID(tenantID) != nil || actorID == zeroUUID || strings.TrimSpace(permission) == "" {
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
		return stableError(err, ErrStateConflict)
	}
	if user.TenantID != tenantID || !strings.EqualFold(strings.TrimSpace(user.Status), admin.StatusActive) {
		return ErrPermissionDenied
	}
	if !adminperm.StrictHasPermission(user.Role, permission) {
		return ErrPermissionDenied
	}
	return nil
}
