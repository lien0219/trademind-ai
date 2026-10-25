package operationtask_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationtask"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type operationTaskQueryCounter struct {
	logger.Interface
	queries atomic.Int64
}

func (l *operationTaskQueryCounter) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	l.queries.Add(1)
	l.Interface.Trace(ctx, begin, fc, err)
}

func TestAPIServiceCreateTaskIdempotencyAndTenantActorBoundary(t *testing.T) {
	db := openOperationTaskTestDB(t)
	tenantID := int64(101)
	actorID := createAdminUser(t, db, tenantID, admin.RoleAdmin, admin.StatusActive)
	svc := operationtask.NewAPIService(db)
	req := operationtask.CreateTaskRequest{
		SourceType:      operationtask.OperationTaskSourceManual,
		SourceReference: "product-1",
		TaskType:        operationtask.OperationTaskTypeProductContent,
		Platform:        operationtask.PlatformDouyin,
		Title:           "Review title",
		Summary:         "safe summary",
		Payload:         json.RawMessage(`{"title":"safe"}`),
		Priority:        operationtask.OperationTaskPriorityNormal,
	}
	actor := operationtask.APIActor{TenantID: tenantID, ActorID: actorID, Role: admin.RoleAdmin}
	created, err := svc.CreateTask(context.Background(), actor, req, "req-create-api", "idem-create-api")
	require.NoError(t, err)
	require.Equal(t, operationtask.OperationTaskStatusSuggested, created.Status)
	require.Equal(t, tenantID, actor.TenantID)
	require.NotNil(t, created.CreatedBy)
	require.Equal(t, actorID, *created.CreatedBy)

	replayed, err := svc.CreateTask(context.Background(), actor, req, "req-create-api-2", "idem-create-api")
	require.NoError(t, err)
	require.Equal(t, created.ID, replayed.ID)

	req.Payload = json.RawMessage(`{"title":"different"}`)
	_, err = svc.CreateTask(context.Background(), actor, req, "req-create-api-3", "idem-create-api")
	require.ErrorIs(t, err, operationtask.ErrIdemPayloadConflict)

	event, err := operationtask.NewOperationTaskEventRepository(db).GetLatestByTask(context.Background(), tenantID, created.ID)
	require.NoError(t, err)
	require.Equal(t, operationtask.OperationTaskEventTypeTaskCreated, event.EventType)
	require.NotContains(t, string(event.Metadata), "idem-create-api")
	require.Contains(t, string(event.Metadata), "idempotencyKeyHash")
}

func TestAPIServicePermissionDenialDoesNotCreateTask(t *testing.T) {
	db := openOperationTaskTestDB(t)
	tenantID := int64(101)
	readonlyID := createAdminUser(t, db, tenantID, "readonly", admin.StatusActive)
	svc := operationtask.NewAPIService(db)
	_, err := svc.CreateTask(context.Background(), operationtask.APIActor{TenantID: tenantID, ActorID: readonlyID, Role: "readonly"}, operationtask.CreateTaskRequest{
		SourceType: operationtask.OperationTaskSourceManual,
		TaskType:   operationtask.OperationTaskTypeProductContent,
		Platform:   operationtask.PlatformDouyin,
		Title:      "Denied",
		Payload:    json.RawMessage(`{"title":"safe"}`),
		Priority:   operationtask.OperationTaskPriorityNormal,
	}, "req-denied", "idem-denied")
	require.ErrorIs(t, err, operationtask.ErrPermissionDenied)

	out, err := operationtask.NewOperationTaskRepository(db).List(context.Background(), operationtask.OperationTaskListParams{TenantID: tenantID, Limit: 10})
	require.NoError(t, err)
	require.Empty(t, out.Items)
}

func TestAPIServiceGetTaskPropagatesRelatedRecordQueryFailure(t *testing.T) {
	db := openOperationTaskTestDB(t)
	tenantID := int64(101)
	actorID := createAdminUser(t, db, tenantID, admin.RoleAdmin, admin.StatusActive)
	svc := operationtask.NewAPIService(db)
	actor := operationtask.APIActor{TenantID: tenantID, ActorID: actorID, Role: admin.RoleAdmin}
	created, err := svc.CreateTask(context.Background(), actor, operationtask.CreateTaskRequest{
		SourceType: operationtask.OperationTaskSourceManual,
		TaskType:   operationtask.OperationTaskTypeProductContent,
		Platform:   operationtask.PlatformLocal,
		Title:      "Related query failure",
		Payload:    json.RawMessage(`{"title":"safe"}`),
		Priority:   operationtask.OperationTaskPriorityNormal,
	}, "req-related-query", "idem-related-query")
	require.NoError(t, err)
	require.NoError(t, db.Exec("DROP TABLE platform_drafts").Error)

	_, err = svc.GetTask(context.Background(), actor, created.ID)
	require.ErrorIs(t, err, operationtask.ErrConflict)
}

func TestAPIServiceListTasksUsesFixedQueryBudget(t *testing.T) {
	db := openOperationTaskTestDB(t)
	ctx := context.Background()
	tenantID := int64(101)
	actorID := createAdminUser(t, db, tenantID, admin.RoleAdmin, admin.StatusActive)
	var latestDataTaskID uuid.UUID
	for i := 0; i < 5; i++ {
		task := sampleTask(tenantID, uuid.NewString())
		task.SourceReference = uuid.NewString()
		require.NoError(t, operationtask.NewOperationTaskRepository(db).Create(ctx, &task))
		if i == 0 {
			latestDataTaskID = task.ID
		}
		draft := sampleDraft(task, 1, hash1)
		require.NoError(t, operationtask.NewPlatformDraftRepository(db).CreateVersion(ctx, &draft))
		approval := sampleApproval(task, draft, uuid.NewString())
		require.NoError(t, operationtask.NewApprovalRecordRepository(db).CreateDecision(ctx, &approval))
		attempt := sampleAttempt(task, draft, approval, 1, uuid.NewString())
		require.NoError(t, operationtask.NewExecutionAttemptRepository(db).CreateAttempt(ctx, &attempt))
		if i == 0 {
			latestDraft := sampleDraft(task, 2, hash2)
			require.NoError(t, operationtask.NewPlatformDraftRepository(db).CreateVersion(ctx, &latestDraft))
			latestApproval := sampleApproval(task, latestDraft, uuid.NewString())
			require.NoError(t, operationtask.NewApprovalRecordRepository(db).CreateDecision(ctx, &latestApproval))
			latestAttempt := sampleAttempt(task, latestDraft, latestApproval, 2, uuid.NewString())
			latestAttempt.Status = operationtask.ExecutionAttemptStatusFailed
			require.NoError(t, operationtask.NewExecutionAttemptRepository(db).CreateAttempt(ctx, &latestAttempt))
		}
	}
	foreignTask := sampleTask(202, uuid.NewString())
	require.NoError(t, operationtask.NewOperationTaskRepository(db).Create(ctx, &foreignTask))
	counter := &operationTaskQueryCounter{Interface: logger.Default.LogMode(logger.Silent)}
	countedDB := db.Session(&gorm.Session{Logger: counter})
	svc := operationtask.NewAPIService(countedDB)

	result, err := svc.ListTasks(ctx, operationtask.APIActor{TenantID: tenantID, ActorID: actorID, Role: admin.RoleAdmin}, operationtask.OperationTaskListParams{Limit: 50})
	require.NoError(t, err)
	require.Len(t, result.Items, 5)
	for _, item := range result.Items {
		if item.ID == latestDataTaskID {
			require.Equal(t, 2, item.LatestDraftVersion)
			require.Equal(t, operationtask.ExecutionAttemptStatusFailed, item.LatestExecutionStatus)
			continue
		}
		require.Equal(t, 1, item.LatestDraftVersion)
		require.Equal(t, operationtask.ExecutionAttemptStatusQueued, item.LatestExecutionStatus)
	}
	require.LessOrEqual(t, counter.queries.Load(), int64(4))
}

func TestOperationTaskHandlerRejectsUnknownDangerousFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openOperationTaskTestDB(t)
	tenantID := int64(101)
	actorID := createAdminUser(t, db, tenantID, admin.RoleAdmin, admin.StatusActive)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.TenantID, tenantID)
		c.Set(ctxkey.AdminID, actorID.String())
		c.Set(ctxkey.TraceID, uuid.NewString())
		c.Next()
	})
	operationtask.Register(r.Group("/api/v1"), &operationtask.Handler{Svc: operationtask.NewAPIService(db)})

	body := []byte(`{"sourceType":"manual","taskType":"product_content","platform":"douyin","title":"Unsafe","payload":{"title":"safe"},"priority":"normal","tenantId":202}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/operation-tasks", bytes.NewReader(body))
	req.Header.Set("Idempotency-Key", "idem-unknown-field")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	out, err := operationtask.NewOperationTaskRepository(db).List(context.Background(), operationtask.OperationTaskListParams{TenantID: tenantID, Limit: 10})
	require.NoError(t, err)
	require.Empty(t, out.Items)
}

func TestAPIExecuteResponseDoesNotExposePublished(t *testing.T) {
	db := openOperationTaskTestDB(t)
	tenantID := int64(101)
	actorID := createAdminUser(t, db, tenantID, admin.RoleAdmin, admin.StatusActive)
	svc := operationtask.NewAPIService(db)
	created, err := svc.CreateTask(context.Background(), operationtask.APIActor{TenantID: tenantID, ActorID: actorID, Role: admin.RoleAdmin}, operationtask.CreateTaskRequest{
		SourceType: operationtask.OperationTaskSourceManual,
		TaskType:   operationtask.OperationTaskTypeProductContent,
		Platform:   operationtask.PlatformLocal,
		Title:      "Local draft",
		Payload:    json.RawMessage(`{"draft":{"title":"safe"}}`),
		Priority:   operationtask.OperationTaskPriorityNormal,
	}, "req-create-local", "idem-create-local")
	require.NoError(t, err)
	preparing, err := operationtask.NewTaskTransitionService(db).Transition(context.Background(), operationtask.TaskTransitionInput{TenantID: tenantID, OperationTaskID: created.ID, ExpectedRevision: created.Revision, ToStatus: operationtask.OperationTaskStatusDraftPreparing, ActorType: operationtask.OperationTaskEventActorUser, ActorID: &actorID, RequestID: "req-prepare-local", Reason: "prepare local draft"})
	require.NoError(t, err)
	draft, err := svc.CreateInitialDraft(context.Background(), operationtask.APIActor{TenantID: tenantID, ActorID: actorID, Role: admin.RoleAdmin}, created.ID, operationtask.CreateDraftRequest{Payload: json.RawMessage(`{"draft":{"title":"safe"}}`), ExpectedTaskRevision: preparing.Revision}, "req-draft-local", "idem-draft-local")
	require.NoError(t, err)
	approved, err := svc.GetTask(context.Background(), operationtask.APIActor{TenantID: tenantID, ActorID: actorID, Role: admin.RoleAdmin}, created.ID)
	require.NoError(t, err)
	_, err = svc.Approve(context.Background(), operationtask.APIActor{TenantID: tenantID, ActorID: actorID, Role: admin.RoleAdmin}, created.ID, operationtask.ApprovalRequest{DraftVersion: draft.DraftVersion, DraftPayloadHash: draft.PayloadHash, ExpectedTaskRevision: approved.Revision}, "req-approve-local", "idem-approve-local")
	require.NoError(t, err)
	out, err := svc.Execute(context.Background(), operationtask.APIActor{TenantID: tenantID, ActorID: actorID, Role: admin.RoleAdmin}, created.ID, operationtask.ExecuteRequest{}, "req-api-exec", "idem-api-exec")
	require.NoError(t, err)
	require.NotNil(t, out)
	encoded, err := json.Marshal(out)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "published")
	require.NotContains(t, string(encoded), "listed")
	require.Equal(t, operationtask.OperationTaskStatusDraftWritten, out.TaskStatus)
}

func TestAPIExecuteRejectsStaleExpectedTaskRevision(t *testing.T) {
	db := openOperationTaskTestDB(t)
	task, _, _, _ := createApprovedExecutionTask(t, db)
	actorID := createAdminUser(t, db, task.TenantID, admin.RoleAdmin, admin.StatusActive)
	svc := operationtask.NewAPIService(db)
	_, err := svc.Execute(context.Background(), operationtask.APIActor{TenantID: task.TenantID, ActorID: actorID, Role: admin.RoleAdmin}, task.ID, operationtask.ExecuteRequest{ExpectedTaskRevision: task.Revision + 1}, "req-stale-execute", "idem-stale-execute")
	require.ErrorIs(t, err, operationtask.ErrRevisionConflict)
}

func TestAPIRetryValidatesFailedAttemptID(t *testing.T) {
	db := openOperationTaskTestDB(t)
	task, _, _, _ := createApprovedExecutionTask(t, db)
	actorID := createAdminUser(t, db, task.TenantID, admin.RoleAdmin, admin.StatusActive)
	port := newRecordingExecutionPort()
	svc := operationtask.NewAPIService(db)
	svc.Executor = operationtask.NewExecutionOrchestrator(db, operationtask.NewRBACAuthorizer(db), port)
	svc.Retry = operationtask.NewManualRetryService(db, svc.Executor, 3)

	succeeded, err := svc.Execute(context.Background(), operationtask.APIActor{TenantID: task.TenantID, ActorID: actorID, Role: admin.RoleAdmin}, task.ID, operationtask.ExecuteRequest{ExpectedTaskRevision: task.Revision}, "req-success-attempt", "idem-success-attempt")
	require.NoError(t, err)
	_, err = svc.RetryExecution(context.Background(), operationtask.APIActor{TenantID: task.TenantID, ActorID: actorID, Role: admin.RoleAdmin}, task.ID, operationtask.RetryRequest{FailedAttemptID: &succeeded.Attempt.ID, Reason: "retry failed fixture"}, "req-retry-nonfailed", "idem-retry-nonfailed")
	require.ErrorIs(t, err, operationtask.ErrStateConflict)
}

func TestAPIRetryAcceptsMatchingFailedAttemptID(t *testing.T) {
	db := openOperationTaskTestDB(t)
	task, _, _, _ := createApprovedExecutionTask(t, db)
	actorID := createAdminUser(t, db, task.TenantID, admin.RoleAdmin, admin.StatusActive)
	port := newRecordingExecutionPort()
	port.err = &operationtask.ExecutionDomainError{Category: operationtask.ExecutionErrorCategoryProviderTimeout, Code: "provider_timeout", SafeMessage: "Sandbox fixture timed out", Retryable: true}
	svc := operationtask.NewAPIService(db)
	svc.Executor = operationtask.NewExecutionOrchestrator(db, operationtask.NewRBACAuthorizer(db), port)
	svc.Retry = operationtask.NewManualRetryService(db, svc.Executor, 3)

	failed, err := svc.Execute(context.Background(), operationtask.APIActor{TenantID: task.TenantID, ActorID: actorID, Role: admin.RoleAdmin}, task.ID, operationtask.ExecuteRequest{ExpectedTaskRevision: task.Revision}, "req-failed-attempt", "idem-failed-attempt")
	require.Error(t, err)
	require.Equal(t, operationtask.ExecutionAttemptStatusFailed, failed.Attempt.Status)
	latest, err := operationtask.NewOperationTaskRepository(db).GetByID(context.Background(), task.TenantID, task.ID)
	require.NoError(t, err)
	port.err = nil
	retried, err := svc.RetryExecution(context.Background(), operationtask.APIActor{TenantID: task.TenantID, ActorID: actorID, Role: admin.RoleAdmin}, task.ID, operationtask.RetryRequest{FailedAttemptID: &failed.Attempt.ID, Reason: "retry failed fixture", ExpectedTaskRevision: latest.Revision}, "req-retry-failed", "idem-retry-failed")
	require.NoError(t, err)
	require.Equal(t, 2, retried.Attempt.AttemptNumber)
}
