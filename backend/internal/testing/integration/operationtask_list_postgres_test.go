//go:build integration || inventorypostgres

package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationtask"
	"github.com/trademind-ai/trademind/backend/internal/testing/postgrestest"
	"gorm.io/datatypes"
)

func TestOperationTaskListWithLatestSummariesPostgres(t *testing.T) {
	db := postgrestest.Require(t).DB
	require.NoError(t, operationtask.Migrate(db))

	const tenantID int64 = 71021
	task := operationtask.OperationTask{
		TenantID: tenantID, SourceType: operationtask.OperationTaskSourceManual,
		SourceReference: uuid.NewString(), TaskType: operationtask.OperationTaskTypeProductContent,
		Platform: operationtask.PlatformLocal, Title: "PostgreSQL list summary",
		Payload: datatypes.JSON([]byte(`{"title":"test"}`)), Status: operationtask.OperationTaskStatusSuggested,
		Priority: operationtask.OperationTaskPriorityNormal, Revision: 1,
	}
	require.NoError(t, operationtask.NewOperationTaskRepository(db).Create(context.Background(), &task))

	const latestHash = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	draft := operationtask.PlatformDraft{
		TenantID: tenantID, OperationTaskID: task.ID, Platform: task.Platform,
		AdapterMode: operationtask.AdapterModeSandbox, DraftVersion: 1,
		Payload: datatypes.JSON([]byte(`{"draft":{"title":"first"}}`)), PayloadHash: latestHash,
		Status: operationtask.PlatformDraftStatusEditable, ChangeReason: "PostgreSQL integration fixture",
	}
	require.NoError(t, operationtask.NewPlatformDraftRepository(db).CreateVersion(context.Background(), &draft))

	result, err := operationtask.NewOperationTaskRepository(db).ListWithLatestSummaries(context.Background(), operationtask.OperationTaskListParams{TenantID: tenantID, Limit: 10})
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	require.Equal(t, task.ID, result.Items[0].Task.ID)
	require.Equal(t, 1, result.Items[0].LatestDraftVersion)
	require.Empty(t, result.Items[0].LatestExecutionStatus)

	foreignTask := task
	foreignTask.ID = uuid.New()
	foreignTask.TenantID = tenantID + 1
	foreignTask.SourceReference = uuid.NewString()
	require.NoError(t, operationtask.NewOperationTaskRepository(db).Create(context.Background(), &foreignTask))
	result, err = operationtask.NewOperationTaskRepository(db).ListWithLatestSummaries(context.Background(), operationtask.OperationTaskListParams{TenantID: tenantID, Limit: 10})
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	require.Equal(t, task.ID, result.Items[0].Task.ID)
}
