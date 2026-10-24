package adminuser

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
)

func TestDeleteUserSoftDeletesRevokesAndAudits(t *testing.T) {
	svc, actor, target := securityFixture(t)
	grant := admin.UserStorePermission{UserID: target.ID, StoreID: uuid.New(), PermissionScope: "view"}
	require.NoError(t, svc.DB.Create(&grant).Error)
	w := roleRequest(t, svc, actor, http.MethodDelete, "/api/v1/admin/users/"+target.ID.String(), nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.ErrorIs(t, svc.DB.First(&admin.AdminUser{}, "id = ?", target.ID).Error, gorm.ErrRecordNotFound)
	var deleted admin.AdminUser
	require.NoError(t, svc.DB.Unscoped().First(&deleted, "id = ?", target.ID).Error)
	require.True(t, deleted.DeletedAt.Valid)
	require.Equal(t, admin.StatusDisabled, deleted.Status)
	require.Equal(t, target.TokenVersion+1, deleted.TokenVersion)
	var count int64
	require.NoError(t, svc.DB.Model(&admin.UserStorePermission{}).Where("user_id = ?", target.ID).Count(&count).Error)
	require.Zero(t, count)
	var audit operationlog.OperationLog
	require.NoError(t, svc.DB.Where("action = ? AND resource_id = ? AND status = ?", "user.delete", target.ID, "success").First(&audit).Error)
	require.Equal(t, actor.TenantID, audit.TenantID)
	require.NotContains(t, audit.Message, target.PasswordHash)
	w = roleRequest(t, svc, actor, http.MethodDelete, "/api/v1/admin/users/"+target.ID.String(), nil)
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

func TestDeleteUserGuardsAndAuditRollback(t *testing.T) {
	for _, scenario := range []string{"self", "cross_tenant", "permission", "audit_failure", "storage_failure"} {
		t.Run(scenario, func(t *testing.T) {
			svc, actor, target := securityFixture(t)
			grant := admin.UserStorePermission{UserID: target.ID, StoreID: uuid.New(), PermissionScope: "view"}
			require.NoError(t, svc.DB.Create(&grant).Error)
			want := http.StatusBadRequest
			switch scenario {
			case "self":
				target = actor
			case "cross_tenant":
				require.NoError(t, svc.DB.Model(&target).Update("tenant_id", actor.TenantID+1).Error)
				want = http.StatusNotFound
			case "permission":
				require.NoError(t, svc.DB.Model(&actor).Update("role", adminperm.RoleOperator).Error)
				want = http.StatusForbidden
			case "audit_failure":
				failAuditInsert(t, svc.DB)
				want = http.StatusInternalServerError
			case "storage_failure":
				require.NoError(t, svc.DB.Callback().Delete().Before("gorm:delete").Register("test:reject_user_delete", func(tx *gorm.DB) {
					if tx.Statement.Table == "admin_users" {
						tx.AddError(gorm.ErrInvalidTransaction)
					}
				}))
				want = http.StatusInternalServerError
			}
			w := roleRequest(t, svc, actor, http.MethodDelete, "/api/v1/admin/users/"+target.ID.String(), nil)
			require.Equal(t, want, w.Code, w.Body.String())
			var current admin.AdminUser
			require.NoError(t, svc.DB.First(&current, "id = ?", target.ID).Error)
			require.Equal(t, target.TokenVersion, current.TokenVersion)
			require.Equal(t, target.Status, current.Status)
			require.NoError(t, svc.DB.First(&admin.UserStorePermission{}, "id = ?", grant.ID).Error, "failed deletion must retain store grants")
			if scenario != "audit_failure" {
				var audit operationlog.OperationLog
				require.NoError(t, svc.DB.Where("action = ? AND status <> ?", "user.delete", "success").First(&audit).Error)
			}
		})
	}
}

func TestDeleteUserPreservesLastActiveAdministrator(t *testing.T) {
	svc, lastAdmin, priorActor := securityFixture(t)
	// The domain check also protects requests whose actor was authorized before
	// a concurrent revocation. HTTP authorization alone cannot protect this race.
	err := svc.Delete(securityContext(priorActor), lastAdmin.ID, &priorActor.ID)
	require.EqualError(t, err, "必须保留至少一名启用的管理员")
	require.NoError(t, svc.DB.First(&admin.AdminUser{}, "id = ?", lastAdmin.ID).Error)
}

func TestDeleteRoleGuardsAndAuditRollback(t *testing.T) {
	for _, scenario := range []string{"success", "has_members", "system", "cross_tenant", "permission", "audit_failure"} {
		t.Run(scenario, func(t *testing.T) {
			svc, actor, target := securityFixture(t)
			role := admin.RoleDefinition{TenantID: actor.TenantID, Code: "r_0123456789abcdef01234567", Name: "Test role", PermissionsJSON: "[\"product.view\"]", Enabled: true}
			require.NoError(t, svc.DB.Create(&role).Error)
			code, want := role.Code, http.StatusOK
			switch scenario {
			case "has_members":
				require.NoError(t, svc.DB.Model(&target).Update("role", role.Code).Error)
				want = http.StatusConflict
			case "system":
				code, want = adminperm.RoleAdmin, http.StatusBadRequest
			case "cross_tenant":
				require.NoError(t, svc.DB.Model(&role).Update("tenant_id", actor.TenantID+1).Error)
				want = http.StatusNotFound
			case "permission":
				require.NoError(t, svc.DB.Model(&actor).Update("role", adminperm.RoleReadonly).Error)
				want = http.StatusForbidden
			case "audit_failure":
				failAuditInsert(t, svc.DB)
				want = http.StatusInternalServerError
			}
			w := roleRequest(t, svc, actor, http.MethodDelete, "/api/v1/admin/roles/"+code, nil)
			require.Equal(t, want, w.Code, w.Body.String())
			err := svc.DB.First(&admin.RoleDefinition{}, "id = ?", role.ID).Error
			if scenario == "success" {
				require.ErrorIs(t, err, gorm.ErrRecordNotFound)
				var audit operationlog.OperationLog
				require.NoError(t, svc.DB.Where("action = ? AND resource_id = ? AND status = ?", "role.delete", role.Code, "success").First(&audit).Error)
				w = roleRequest(t, svc, actor, http.MethodDelete, "/api/v1/admin/roles/"+code, nil)
				require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
