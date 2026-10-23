package aiprompt

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

func TestTenantAdminCannotReadOrMutatePlatformPrompts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &AIPrompt{}))
	user := admin.AdminUser{TenantID: 21, Username: admin.NewInternalUsername(), Role: adminperm.RoleAdmin, Status: admin.StatusActive}
	require.NoError(t, db.Create(&user).Error)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, user.ID.String())
		c.Set(ctxkey.TenantID, user.TenantID)
		c.Next()
	})
	h := &Handler{Svc: &Service{DB: db}}
	Register(r.Group("/api/v1"), h)

	list := httptest.NewRecorder()
	r.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/v1/ai/prompts", nil))
	require.Equal(t, http.StatusForbidden, list.Code, list.Body.String())

	create := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/prompts", bytes.NewBufferString(`{"code":"test","name":"Test"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(create, req)
	require.Equal(t, http.StatusForbidden, create.Code, create.Body.String())
	var count int64
	require.NoError(t, db.Model(&AIPrompt{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestPromptMutationRollsBackWhenAuditFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &AIPrompt{}, &operationlog.OperationLog{}))
	user := admin.AdminUser{TenantID: 0, Username: admin.NewInternalUsername(), Role: adminperm.RoleAdmin, Status: admin.StatusActive}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Exec(`CREATE TRIGGER reject_operation_log BEFORE INSERT ON operation_logs BEGIN SELECT RAISE(ABORT, 'sqlite rejected operation log insert'); END`).Error)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, user.ID.String())
		c.Set(ctxkey.TenantID, user.TenantID)
		c.Next()
	})
	h := &Handler{Svc: &Service{DB: db}, OpLog: &operationlog.Service{DB: db}}
	Register(r.Group("/api/v1"), h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/prompts", bytes.NewBufferString(`{"code":"audit-test","name":"Audit test","systemPrompt":"private prompt"}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	require.Equal(t, http.StatusInternalServerError, res.Code, res.Body.String())
	require.Contains(t, res.Body.String(), "ai prompt audit unavailable")
	require.NotContains(t, res.Body.String(), "sqlite rejected operation log insert", "database details must be sanitized")
	var count int64
	require.NoError(t, db.Model(&AIPrompt{}).Count(&count).Error)
	require.Zero(t, count)
}
