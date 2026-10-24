package integration

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/adminuser"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/testing/postgrestest"
	"github.com/trademind-ai/trademind/backend/internal/testing/safeenv"
)

func adminDeletionFixture(t *testing.T) (*adminuser.Service, []admin.AdminUser) {
	t.Helper()
	t.Setenv("APP_ENV", "test")
	_, ok, err := safeenv.TestDatabaseURLFromEnv()
	require.NoError(t, err)
	if !ok {
		t.Skip("TEST_DATABASE_URL is not set; PostgreSQL deletion concurrency requires an isolated test database")
	}
	harness := postgrestest.Require(t)
	harness.EmitMetadata(t)
	db := harness.DB
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.AdminMutationLock{}, &admin.RoleDefinition{}, &admin.UserStorePermission{}, &shop.Shop{}, &operationlog.OperationLog{}))
	tenant := time.Now().UnixNano()
	admins := []admin.AdminUser{
		{TenantID: tenant, Username: admin.NewInternalUsername(), Email: "delete-a@example.test", PasswordHash: "test-only", Role: adminperm.RoleAdmin, Status: admin.StatusActive, TokenVersion: 1},
		{TenantID: tenant, Username: admin.NewInternalUsername(), Email: "delete-b@example.test", PasswordHash: "test-only", Role: adminperm.RoleAdmin, Status: admin.StatusActive, TokenVersion: 1},
	}
	require.NoError(t, db.Create(&admins).Error)
	return &adminuser.Service{DB: db, OpLog: &operationlog.Service{DB: db}}, admins
}

func deletionContext(actor admin.AdminUser) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users", nil)
	c.Set(ctxkey.AdminID, actor.ID.String())
	c.Set(ctxkey.TenantID, actor.TenantID)
	c.Set(ctxkey.TraceID, uuid.NewString())
	return c
}

func TestConcurrentUserDeletionRetainsLastActiveAdministrator(t *testing.T) {
	for _, other := range []string{"delete", "disable", "demote"} {
		t.Run(other, func(t *testing.T) {
			svc, admins := adminDeletionFixture(t)
			start := make(chan struct{})
			results := make(chan error, 2)
			go func() {
				<-start
				results <- svc.Delete(deletionContext(admins[0]), admins[1].ID, &admins[0].ID)
			}()
			go func() {
				<-start
				if other == "delete" {
					results <- svc.Delete(deletionContext(admins[1]), admins[0].ID, &admins[1].ID)
					return
				}
				body := adminuser.UpdateBody{}
				if other == "disable" {
					status := admin.StatusDisabled
					body.Status = &status
				} else {
					role := adminperm.RoleReadonly
					body.Role = &role
				}
				_, err := svc.Update(deletionContext(admins[1]), admins[0].ID, body, &admins[1].ID)
				results <- err
			}()
			close(start)
			succeeded := 0
			for i := 0; i < 2; i++ {
				err := <-results
				if err == nil {
					succeeded++
				} else {
					require.EqualError(t, err, "必须保留至少一名启用的管理员")
				}
			}
			require.Equal(t, 1, succeeded)
			var active int64
			require.NoError(t, svc.DB.Model(&admin.AdminUser{}).Where("tenant_id = ? AND role = ? AND status = ?", admins[0].TenantID, adminperm.RoleAdmin, admin.StatusActive).Count(&active).Error)
			require.EqualValues(t, 1, active)
		})
	}
}

func TestConcurrentRoleDeletionAndAssignmentCannotLeaveDanglingMembership(t *testing.T) {
	for _, create := range []bool{true, false} {
		name := "existing_user"
		if create {
			name = "new_user"
		}
		t.Run(name, func(t *testing.T) {
			svc, admins := adminDeletionFixture(t)
			actor, target := admins[0], admins[1]
			require.NoError(t, svc.DB.Model(&target).Update("role", adminperm.RoleReadonly).Error)
			role := admin.RoleDefinition{TenantID: actor.TenantID, Code: "r_0123456789abcdef01234567", Name: "E2E deletion race", Enabled: true, PermissionsJSON: "[\"product.view\"]"}
			require.NoError(t, svc.DB.Create(&role).Error)
			start := make(chan struct{})
			results := make(chan error, 2)
			go func() {
				<-start
				results <- svc.DeleteRole(deletionContext(actor), role.Code, &actor.ID)
			}()
			go func() {
				<-start
				var err error
				if create {
					_, err = svc.Create(deletionContext(actor), adminuser.CreateBody{Email: "role-race@example.test", Password: "synthetic-password-123", Role: role.Code}, &actor.ID)
				} else {
					_, err = svc.Update(deletionContext(actor), target.ID, adminuser.UpdateBody{Role: &role.Code}, &actor.ID)
				}
				results <- err
			}()
			close(start)
			succeeded := 0
			for i := 0; i < 2; i++ {
				err := <-results
				if err == nil {
					succeeded++
				} else {
					require.Contains(t, []string{adminuser.ErrRoleHasMembers.Error(), "自定义角色不存在或已停用"}, err.Error())
				}
			}
			require.Equal(t, 1, succeeded, "either the role deletion or the assignment may commit, never both")
			var roles, members int64
			require.NoError(t, svc.DB.Model(&admin.RoleDefinition{}).Where("tenant_id = ? AND code = ?", actor.TenantID, role.Code).Count(&roles).Error)
			require.NoError(t, svc.DB.Model(&admin.AdminUser{}).Where("tenant_id = ? AND role = ?", actor.TenantID, role.Code).Count(&members).Error)
			require.Equal(t, roles, members, "a member must keep its role; deleted roles must have no live members")
		})
	}
}
