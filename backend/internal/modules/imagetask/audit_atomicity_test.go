package imagetask

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/gorm"
)

func TestRetryEnqueueRollsBackTaskWhenAuditFails(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:image_retry_audit_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&ImageTask{}, &operationlog.OperationLog{}))
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("test:fail_image_retry_audit", func(tx *gorm.DB) {
		if tx.Statement.Table == "operation_logs" {
			tx.AddError(errors.New("image retry audit unavailable"))
		}
	}))

	task := ImageTask{
		HardDeleteBase: model.HardDeleteBase{ID: uuid.New()},
		TenantID:       41,
		TaskType:       TaskTypeGenerateScene,
		Provider:       "test",
		Status:         StatusFailed,
	}
	require.NoError(t, db.Create(&task).Error)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/ai/image-tasks/"+task.ID.String()+"/retry", nil)
	svc := &Service{DB: db, OpLog: &operationlog.Service{DB: db}}
	err = svc.RetryEnqueue(c, task.ID)
	require.ErrorContains(t, err, "image retry audit unavailable")

	var stored ImageTask
	require.NoError(t, db.First(&stored, "id = ?", task.ID).Error)
	require.Equal(t, StatusFailed, stored.Status)
	require.Zero(t, stored.RetryCount)
}

func TestTaskStateTransitionFailsClosedWhenAuditServiceIsMissing(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:image_missing_audit_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&ImageTask{}))
	task := ImageTask{
		HardDeleteBase: model.HardDeleteBase{ID: uuid.New()},
		TenantID:       41,
		TaskType:       TaskTypeGenerateScene,
		Provider:       "test",
		Status:         StatusRunning,
	}
	require.NoError(t, db.Create(&task).Error)

	svc := &Service{DB: db}
	err = svc.updateTaskWithAudit(context.Background(), nil, &task, map[string]any{"status": StatusSuccess}, operationlog.WriteOpts{
		TenantID: 41, Action: "image.task.success", Resource: "image_task", Status: "success",
	})
	require.ErrorContains(t, err, "audit unavailable")
	var stored ImageTask
	require.NoError(t, db.First(&stored, "id = ?", task.ID).Error)
	require.Equal(t, StatusRunning, stored.Status)
}

func TestFinalizeImageFailedRollsBackTaskWhenAuditFails(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:image_finalize_audit_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&ImageTask{}, &operationlog.OperationLog{}))
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("test:fail_image_finalize_audit", func(tx *gorm.DB) {
		if tx.Statement.Table == "operation_logs" {
			tx.AddError(errors.New("image finalize audit unavailable"))
		}
	}))

	task := ImageTask{
		HardDeleteBase: model.HardDeleteBase{ID: uuid.New()},
		TenantID:       41,
		TaskType:       TaskTypeGenerateScene,
		Provider:       "test",
		Status:         StatusRunning,
	}
	require.NoError(t, db.Create(&task).Error)

	svc := &Service{DB: db, OpLog: &operationlog.Service{DB: db}}
	err = svc.finalizeImageFailed(context.Background(), nil, &task, "provider failed", false)
	require.ErrorContains(t, err, "image finalize audit unavailable")

	var stored ImageTask
	require.NoError(t, db.First(&stored, "id = ?", task.ID).Error)
	require.Equal(t, StatusRunning, stored.Status)
}
