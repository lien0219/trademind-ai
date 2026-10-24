package taskcenter

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestApplyTaskAlertStoreScopeKeepsStoreBranchesInsideTenantPredicate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:task_alert_store_scope_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	shopID := uuid.New()
	var rows []TaskAlert
	stmt := applyTaskAlertStoreScope(
		db.Session(&gorm.Session{DryRun: true}).Model(&TaskAlert{}).Where("tenant_id = ?", int64(41)),
		41,
		true,
		[]uuid.UUID{shopID},
	).Find(&rows).Statement

	query := strings.Join(strings.Fields(stmt.SQL.String()), " ")
	require.Contains(t, query, "WHERE tenant_id = ? AND (")
	groupStart := strings.Index(query, "AND (") + len("AND ")
	groupEnd := -1
	depth := 0
	for i := groupStart; i < len(query); i++ {
		switch query[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				groupEnd = i
				break
			}
		}
		if groupEnd >= 0 {
			break
		}
	}
	require.NotEqual(t, -1, groupEnd, query)
	// Every dynamically added store-specific OR must remain within the same
	// parenthesized predicate as the tenant-scoped task-source branches.
	require.Less(t, strings.Index(query, "order_sync_tasks"), groupEnd, query)
	require.Less(t, strings.Index(query, "FROM customer_failure_events WHERE shop_id IN"), groupEnd, query)
}
