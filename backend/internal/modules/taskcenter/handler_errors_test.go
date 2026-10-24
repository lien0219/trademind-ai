package taskcenter

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

func TestTaskFailureResponseUsesSafeStatusCodes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   int
	}{
		{name: "audit unavailable", err: errTaskAuditUnavailable, wantStatus: http.StatusServiceUnavailable, wantCode: response.CodeServiceUnavailable},
		{name: "permission denied", err: errTaskPermissionDenied, wantStatus: http.StatusForbidden, wantCode: response.CodeForbidden},
		{name: "tenant missing", err: errTaskTenantMissing, wantStatus: http.StatusForbidden, wantCode: response.CodeForbidden},
		{name: "out of scope resource", err: gorm.ErrRecordNotFound, wantStatus: http.StatusNotFound, wantCode: response.CodeNotFound},
		{name: "invalid request", err: errTaskInvalidRequest, wantStatus: http.StatusBadRequest, wantCode: response.CodeBadRequest},
		{name: "state conflict", err: errTaskConflict, wantStatus: http.StatusConflict, wantCode: response.CodeBadRequest},
		{name: "database failure", err: errors.New("database read failed"), wantStatus: http.StatusInternalServerError, wantCode: response.CodeInternalError},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/task-center/failures", nil)

			taskFailureResponse(c, tc.err)

			require.Equal(t, tc.wantStatus, recorder.Code)
			var body response.Envelope
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
			require.Equal(t, tc.wantCode, body.Code)
			if tc.name == "database failure" {
				require.NotContains(t, body.Message, "database read failed")
			}
		})
	}
}

func TestListFailuresDatabaseFailureReturnsServerError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:taskcenter_handler_db_failure_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.RoleDefinition{}, &admin.UserStorePermission{}, &shop.Shop{}))

	actorID := uuid.New()
	require.NoError(t, db.Create(&admin.AdminUser{
		Base: model.Base{ID: actorID}, TenantID: 41, Username: admin.NewInternalUsername(),
		PasswordHash: "test", Role: adminperm.RoleAdmin, Status: admin.StatusActive,
	}).Error)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, actorID.String())
		c.Set(ctxkey.TenantID, int64(41))
		c.Next()
	})
	Register(router.Group("/api/v1"), &Handler{Svc: &Service{DB: db}})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/task-center/failures", nil))
	require.Equal(t, http.StatusInternalServerError, recorder.Code, recorder.Body.String())
	var body response.Envelope
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, response.CodeInternalError, body.Code)
}

func TestTaskCenterRejectsMalformedMutationBodiesBeforeService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:taskcenter_invalid_body_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.RoleDefinition{}, &admin.UserStorePermission{}, &shop.Shop{}, &operationlog.OperationLog{}))

	actorID := uuid.New()
	require.NoError(t, db.Create(&admin.AdminUser{
		Base: model.Base{ID: actorID}, TenantID: 41, Username: admin.NewInternalUsername(),
		PasswordHash: "test", Role: adminperm.RoleAdmin, Status: admin.StatusActive,
	}).Error)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, actorID.String())
		c.Set(ctxkey.TenantID, int64(41))
		c.Next()
	})
	Register(router.Group("/api/v1"), &Handler{Svc: &Service{DB: db, OpLog: &operationlog.Service{DB: db}}})

	alertID := uuid.NewString()
	failureID := uuid.NewString()
	cases := []struct {
		name string
		path string
		body string
	}{
		{name: "ignore malformed JSON", path: "/failures/collect/" + failureID + "/ignore", body: `{"remark":`},
		{name: "handle malformed JSON", path: "/failures/collect/" + failureID + "/handle", body: `{"remark":`},
		{name: "notify malformed JSON", path: "/alerts/" + alertID + "/notify", body: `{"channels":`},
		{name: "notify null body", path: "/alerts/" + alertID + "/notify", body: `null`},
		{name: "notify missing channels", path: "/alerts/" + alertID + "/notify", body: `{}`},
		{name: "notify empty channels", path: "/alerts/" + alertID + "/notify", body: `{"channels":[]}`},
		{name: "notify null channels", path: "/alerts/" + alertID + "/notify", body: `{"channels":null}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/task-center"+tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(rec, req)
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		})
	}

	var auditCount int64
	require.NoError(t, db.Model(&operationlog.OperationLog{}).Count(&auditCount).Error)
	require.Zero(t, auditCount, "invalid requests must not start the protected action")
}
