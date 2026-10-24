package shop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type oauthPersistenceTxKey struct{}

func withOAuthPersistenceTx(ctx context.Context, tx *gorm.DB) context.Context {
	return context.WithValue(ctx, oauthPersistenceTxKey{}, tx)
}

func oauthPersistenceTx(ctx context.Context) (*gorm.DB, bool) {
	if ctx == nil {
		return nil, false
	}
	tx, ok := ctx.Value(oauthPersistenceTxKey{}).(*gorm.DB)
	return tx, ok && tx != nil
}

func oauthPersistenceDB(ctx context.Context, db *gorm.DB) *gorm.DB {
	if tx, ok := oauthPersistenceTx(ctx); ok {
		return tx.WithContext(ctx)
	}
	if db == nil {
		return nil
	}
	return db.WithContext(ctx)
}

// persistOAuthAuthorization commits credentials, provider metadata and success
// evidence together. Provider requests finish before this transaction starts.
func (s *Service) persistOAuthAuthorization(c *gin.Context, ctx context.Context, shopID uuid.UUID, body UpdateAuthBody, adminID *uuid.UUID, shopFields, tokenFields map[string]any, action string) (*ShopDetailDTO, error) {
	if s == nil || s.DB == nil || c == nil || c.Request == nil {
		return nil, fmt.Errorf("OAuth persistence unavailable")
	}
	if version, ok := oauthExpectedTokenVersion(c); !ok || version <= 0 {
		return nil, fmt.Errorf("OAuth actor security binding is required")
	}
	if _, ok := oauthExpectedShopTokenVersion(c.Request.Context()); !ok {
		return nil, fmt.Errorf("OAuth shop security binding is required")
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := withOAuthPersistenceTx(ctx, tx)
		txGin := c.Copy()
		txGin.Request = c.Request.WithContext(txCtx)
		if _, err := s.UpdateAuth(txGin, shopID, body, adminID); err != nil {
			return err
		}
		if len(shopFields) > 0 {
			if err := s.updateShopOAuthFields(txCtx, shopID, shopFields); err != nil {
				return err
			}
		}
		if len(tokenFields) > 0 {
			if err := s.updateShopAuthTokenOAuthFields(txCtx, shopID, tokenFields); err != nil {
				return err
			}
		}
		return s.writeShopAuditTx(tx, txGin, adminID, action, shopID.String(), "OAuth credentials and shop metadata saved")
	})
	if err != nil {
		return nil, err
	}
	return s.GetDetail(c, shopID)
}

// revalidateOAuthActorTx closes the gap between state validation and the end
// of a provider exchange. It must run inside the same transaction that writes
// shop credentials, while holding the actor row lock.
func (s *Service) revalidateOAuthActorTx(tx *gorm.DB, c *gin.Context, tenantID int64, shopID uuid.UUID, adminID *uuid.UUID) error {
	expectedVersion, hasExpectedVersion := oauthExpectedTokenVersion(c)
	actorID := uuid.Nil
	if c != nil {
		if raw, exists := c.Get(ctxkey.AdminID); exists {
			if value, ok := raw.(string); ok {
				actorID, _ = uuid.Parse(strings.TrimSpace(value))
			}
		}
	}
	if adminID != nil && *adminID != uuid.Nil {
		if actorID != uuid.Nil && actorID != *adminID {
			return fmt.Errorf("OAuth actor binding mismatch")
		}
		actorID = *adminID
	}
	if actorID == uuid.Nil {
		return fmt.Errorf("authenticated actor is unavailable")
	}
	var actor admin.AdminUser
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", actorID).First(&actor).Error; err != nil {
		return fmt.Errorf("resolve OAuth actor: %w", err)
	}
	if actor.TenantID != tenantID || !strings.EqualFold(strings.TrimSpace(actor.Status), admin.StatusActive) || (hasExpectedVersion && int64(actor.TokenVersion) != expectedVersion) {
		return fmt.Errorf("OAuth actor is no longer authorized")
	}

	role := strings.TrimSpace(strings.ToLower(actor.Role))
	if adminperm.IsCustomRole(role) {
		var definition admin.RoleDefinition
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id = ? AND code = ? AND enabled = ?", tenantID, role, true).First(&definition).Error; err != nil {
			return fmt.Errorf("OAuth actor custom role is no longer active: %w", err)
		}
		var permissions []string
		if err := json.Unmarshal([]byte(definition.PermissionsJSON), &permissions); err != nil {
			return fmt.Errorf("OAuth actor custom role is invalid: %w", err)
		}
		grants, err := adminperm.ValidateCustomPermissions(permissions)
		if err != nil || !containsPermission(grants, adminperm.PermStoreOperate) {
			return fmt.Errorf("OAuth actor no longer has shop authorization")
		}
	} else if !adminperm.HasPermission(role, adminperm.PermStoreOperate) {
		return fmt.Errorf("OAuth actor no longer has shop authorization")
	}
	if adminperm.IsCustomRole(role) || !strings.EqualFold(role, adminperm.RoleAdmin) {
		var grant admin.UserStorePermission
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND store_id = ? AND permission_scope IN ?", actorID, shopID, []string{admin.StorePermScopeOperate, admin.StorePermScopeManage}).
			First(&grant).Error; err != nil {
			return fmt.Errorf("OAuth actor no longer has shop authorization: %w", err)
		}
	}
	return nil
}

func containsPermission(perms []string, want string) bool {
	for _, perm := range perms {
		if strings.TrimSpace(perm) == want {
			return true
		}
	}
	return false
}
