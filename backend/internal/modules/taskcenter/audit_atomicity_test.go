package taskcenter

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/gorm"
)

func TestTaskAlertStatusChangeRollsBackWhenAuditCannotPersist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:taskcenter_audit_atomicity_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&TaskAlert{}, &operationlog.OperationLog{}))

	alert := TaskAlert{
		ID: uuid.New(), TenantID: 41, TaskType: TaskTypeCollect, SourceID: uuid.NewString(),
		FailureCategory: "test", Status: TaskAlertStatusOpen, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	require.NoError(t, db.Create(&alert).Error)
	require.NoError(t, db.Exec(`CREATE TRIGGER reject_operation_logs BEFORE INSERT ON operation_logs BEGIN SELECT RAISE(FAIL, 'audit unavailable'); END`).Error)

	ginContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	ginContext.Request = httptest.NewRequest("POST", "/api/v1/task-center/alerts/"+alert.ID.String()+"/handle", nil)
	ginContext.Set(ctxkey.TenantID, int64(41))
	service := &Service{DB: db, OpLog: &operationlog.Service{DB: db}}

	err = service.finishAlertStatusChange(context.Background(), ginContext, alert, TaskAlertStatusHandled, time.Now().UTC(), nil, "task_center.alert.handle")
	require.Error(t, err)

	var persisted TaskAlert
	require.NoError(t, db.First(&persisted, "id = ?", alert.ID).Error)
	require.Equal(t, TaskAlertStatusOpen, persisted.Status)
	var auditCount int64
	require.NoError(t, db.Model(&operationlog.OperationLog{}).Count(&auditCount).Error)
	require.Zero(t, auditCount)
}

func TestRetryFailureWritesFailedOutcomeWhenDependencyIsUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:taskcenter_retry_outcome_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&admin.AdminUser{}, &admin.RoleDefinition{}, &TaskFailureMark{},
		&collect.CollectTask{}, &operationlog.OperationLog{},
	))
	actorID := uuid.New()
	tenantID := int64(41)
	require.NoError(t, db.Create(&admin.AdminUser{
		Base: model.Base{ID: actorID}, TenantID: tenantID, Username: admin.NewInternalUsername(),
		PasswordHash: "test", Role: adminperm.RoleAdmin, Status: admin.StatusActive,
	}).Error)
	taskID := uuid.New()
	require.NoError(t, db.Create(&collect.CollectTask{
		HardDeleteBase: model.HardDeleteBase{ID: taskID}, TenantID: tenantID,
		Source: "test", SourceURL: "https://example.test/item", Status: collect.StatusFailed,
		ErrorMessage: "dependency unavailable", MaxRetries: 3,
	}).Error)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/api/v1/task-center/failures/collect/"+taskID.String()+"/retry", nil)
	c.Set(ctxkey.AdminID, actorID.String())
	c.Set(ctxkey.TenantID, tenantID)
	service := &Service{DB: db, OpLog: &operationlog.Service{DB: db}}

	err = service.RetryFailure(c, TaskTypeCollect, taskID)
	require.Error(t, err)
	var requested, failed int64
	require.NoError(t, db.Model(&operationlog.OperationLog{}).Where("action = ?", "task_center.retry.requested").Count(&requested).Error)
	require.NoError(t, db.Model(&operationlog.OperationLog{}).Where("action = ? AND status = ?", "task_center.retry.completed", "failed").Count(&failed).Error)
	require.Equal(t, int64(1), requested)
	require.Equal(t, int64(1), failed)
}

func TestAlertScanWritesFailedOutcomeWhenScanCannotReadSources(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:taskcenter_alert_scan_outcome_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&operationlog.OperationLog{}))

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/api/v1/task-center/alerts/scan", nil)
	service := &Service{DB: db, OpLog: &operationlog.Service{DB: db}}
	_, err = service.ScanAndGenerateTaskAlertsForScopeRequest(context.Background(), ListFailureParams{TenantID: 41, TenantScopeSet: true}, c)
	require.Error(t, err)
	var requested, failed int64
	require.NoError(t, db.Model(&operationlog.OperationLog{}).Where("action = ?", "task_center.alert.scan.requested").Count(&requested).Error)
	require.NoError(t, db.Model(&operationlog.OperationLog{}).Where("action = ? AND status = ?", "task_center.alert.scan.completed", "failed").Count(&failed).Error)
	require.Equal(t, int64(1), requested)
	require.Equal(t, int64(1), failed)
}

func TestAlertGenerateWritesFailedOutcomeWhenAlertPersistenceFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:taskcenter_alert_generate_outcome_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&admin.AdminUser{}, &admin.RoleDefinition{}, &TaskFailureMark{},
		&collect.CollectTask{}, &TaskAlert{}, &operationlog.OperationLog{},
	))
	actorID := uuid.New()
	tenantID := int64(41)
	require.NoError(t, db.Create(&admin.AdminUser{
		Base: model.Base{ID: actorID}, TenantID: tenantID, Username: admin.NewInternalUsername(),
		PasswordHash: "test", Role: adminperm.RoleAdmin, Status: admin.StatusActive,
	}).Error)
	taskID := uuid.New()
	require.NoError(t, db.Create(&collect.CollectTask{
		HardDeleteBase: model.HardDeleteBase{ID: taskID}, TenantID: tenantID,
		Source: "test", SourceURL: "https://example.test/item", Status: collect.StatusFailed,
		ErrorMessage: "upstream failed", MaxRetries: 3,
	}).Error)
	require.NoError(t, db.Exec(`CREATE TRIGGER reject_task_alerts BEFORE INSERT ON task_alerts BEGIN SELECT RAISE(FAIL, 'alert persistence unavailable'); END`).Error)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/api/v1/task-center/failures/collect/"+taskID.String()+"/generate-alert", nil)
	c.Set(ctxkey.AdminID, actorID.String())
	c.Set(ctxkey.TenantID, tenantID)
	service := &Service{DB: db, OpLog: &operationlog.Service{DB: db}}

	_, err = service.GenerateAlertForFailure(c, TaskTypeCollect, taskID)
	require.Error(t, err)
	var requested, failed int64
	require.NoError(t, db.Model(&operationlog.OperationLog{}).Where("action = ?", "task_center.alert.generate.requested").Count(&requested).Error)
	require.NoError(t, db.Model(&operationlog.OperationLog{}).Where("action = ? AND status = ?", "task_center.alert.generated", "failed").Count(&failed).Error)
	require.Equal(t, int64(1), requested)
	require.Equal(t, int64(1), failed)
}
