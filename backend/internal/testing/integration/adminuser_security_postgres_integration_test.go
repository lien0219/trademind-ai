//go:build integration || inventorypostgres

package integration

import (
	"net/http"
	"net/http/httptest"
	"sync"
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
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"github.com/trademind-ai/trademind/backend/internal/testing/postgrestest"
	"github.com/trademind-ai/trademind/backend/internal/testing/safeenv"
)

func TestAdminUserConcurrentDemotionsRetainOneActiveAdministrator(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	_, ok, err := safeenv.TestDatabaseURLFromEnv()
	require.NoError(t, err)
	if !ok {
		t.Skip("TEST_DATABASE_URL is not set; skipping PostgreSQL last-administrator concurrency test")
	}

	harness := postgrestest.Require(t)
	harness.EmitMetadata(t)
	db := harness.DB
	require.NoError(t, db.AutoMigrate(
		&admin.AdminUser{}, &admin.AdminMutationLock{}, &admin.RoleDefinition{}, &admin.UserStorePermission{},
		&shop.Shop{}, &operationlog.OperationLog{},
	))
	tenantID := time.Now().UnixNano()
	admins := []admin.AdminUser{
		{Base: model.Base{ID: uuid.New()}, TenantID: tenantID, Username: admin.NewInternalUsername(), Email: "admin-a@example.test", PasswordHash: "test-only", Role: adminperm.RoleAdmin, Status: admin.StatusActive, TokenVersion: 1},
		{Base: model.Base{ID: uuid.New()}, TenantID: tenantID, Username: admin.NewInternalUsername(), Email: "admin-b@example.test", PasswordHash: "test-only", Role: adminperm.RoleAdmin, Status: admin.StatusActive, TokenVersion: 1},
	}
	require.NoError(t, db.Create(&admins).Error)

	type result struct{ err error }
	start := make(chan struct{})
	results := make(chan result, len(admins))
	var wait sync.WaitGroup
	for i := range admins {
		actor := admins[i]
		target := admins[1-i]
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request, _ = http.NewRequest(http.MethodPatch, "/api/v1/admin/users/"+target.ID.String(), nil)
			ctx.Set(ctxkey.AdminID, actor.ID.String())
			ctx.Set(ctxkey.TenantID, tenantID)
			ctx.Set(ctxkey.TraceID, uuid.NewString())
			role := adminperm.RoleReadonly
			_, updateErr := (&adminuser.Service{DB: db, OpLog: &operationlog.Service{DB: db}}).Update(ctx, target.ID, adminuser.UpdateBody{Role: &role}, &actor.ID)
			results <- result{err: updateErr}
		}()
	}
	close(start)
	wait.Wait()
	close(results)

	succeeded, rejected := 0, 0
	var failures []string
	for row := range results {
		if row.err == nil {
			succeeded++
		} else {
			rejected++
			failures = append(failures, row.err.Error())
		}
	}
	require.Equal(t, 1, succeeded, "one concurrent demotion may commit; rejected errors: %v", failures)
	require.Equal(t, 1, rejected, "the other demotion must preserve the last active administrator; rejected errors: %v", failures)
	var activeAdmins int64
	require.NoError(t, db.Model(&admin.AdminUser{}).
		Where("tenant_id = ? AND role = ? AND status = ?", tenantID, adminperm.RoleAdmin, admin.StatusActive).
		Count(&activeAdmins).Error)
	require.EqualValues(t, 1, activeAdmins)
}

func TestAdminUserConcurrentDisableAndDemotionRetainOneActiveAdministrator(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	_, ok, err := safeenv.TestDatabaseURLFromEnv()
	require.NoError(t, err)
	if !ok {
		t.Skip("TEST_DATABASE_URL is not set; skipping PostgreSQL last-administrator concurrency test")
	}
	harness := postgrestest.Require(t)
	harness.EmitMetadata(t)
	db := harness.DB
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.AdminMutationLock{}, &admin.RoleDefinition{}, &admin.UserStorePermission{}, &shop.Shop{}, &operationlog.OperationLog{}))
	tenantID := time.Now().UnixNano()
	admins := []admin.AdminUser{
		{Base: model.Base{ID: uuid.New()}, TenantID: tenantID, Username: admin.NewInternalUsername(), Email: "admin-disable-a@example.test", PasswordHash: "test-only", Role: adminperm.RoleAdmin, Status: admin.StatusActive, TokenVersion: 1},
		{Base: model.Base{ID: uuid.New()}, TenantID: tenantID, Username: admin.NewInternalUsername(), Email: "admin-disable-b@example.test", PasswordHash: "test-only", Role: adminperm.RoleAdmin, Status: admin.StatusActive, TokenVersion: 1},
	}
	require.NoError(t, db.Create(&admins).Error)
	type result struct{ err error }
	start := make(chan struct{})
	results := make(chan result, 2)
	var wait sync.WaitGroup
	for i := range admins {
		actor, target := admins[i], admins[1-i]
		wait.Add(1)
		go func(disable bool) {
			defer wait.Done()
			<-start
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request, _ = http.NewRequest(http.MethodPatch, "/api/v1/admin/users/"+target.ID.String(), nil)
			ctx.Set(ctxkey.AdminID, actor.ID.String())
			ctx.Set(ctxkey.TenantID, tenantID)
			ctx.Set(ctxkey.TraceID, uuid.NewString())
			body := adminuser.UpdateBody{}
			if disable {
				status := admin.StatusDisabled
				body.Status = &status
			} else {
				role := adminperm.RoleReadonly
				body.Role = &role
			}
			_, updateErr := (&adminuser.Service{DB: db, OpLog: &operationlog.Service{DB: db}}).Update(ctx, target.ID, body, &actor.ID)
			results <- result{err: updateErr}
		}(i == 0)
	}
	close(start)
	wait.Wait()
	close(results)
	succeeded := 0
	for row := range results {
		if row.err == nil {
			succeeded++
		}
	}
	require.Equal(t, 1, succeeded, "only one concurrent admin revocation may commit")
	var activeAdmins int64
	require.NoError(t, db.Model(&admin.AdminUser{}).Where("tenant_id = ? AND role = ? AND status = ?", tenantID, adminperm.RoleAdmin, admin.StatusActive).Count(&activeAdmins).Error)
	require.EqualValues(t, 1, activeAdmins)
}
