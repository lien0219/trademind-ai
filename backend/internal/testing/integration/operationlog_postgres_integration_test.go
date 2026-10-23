package integration

import (
	"context"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/testing/postgrestest"
	"github.com/trademind-ai/trademind/backend/internal/testing/safeenv"
	"gorm.io/gorm"
)

// A transaction may append multiple audit rows while another transaction is
// waiting for the same partition. PostgreSQL's transaction lock must remain
// the only lock held while waiting; a process mutex would invert that order.
func TestOperationLogConcurrentMultiRowTransactionsDoNotDeadlock(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	_, ok, err := safeenv.TestDatabaseURLFromEnv()
	require.NoError(t, err)
	if !ok {
		t.Skip("TEST_DATABASE_URL is not set; skipping PostgreSQL audit concurrency test")
	}
	harness := postgrestest.Require(t)
	db := harness.DB
	require.NoError(t, db.AutoMigrate(&operationlog.OperationLog{}))
	logs := &operationlog.Service{DB: db}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	type writerContextKey struct{}
	firstWritten := make(chan struct{})
	secondWaiting := make(chan struct{})
	var signal sync.Once
	require.NoError(t, db.Callback().Raw().Before("gorm:raw").Register("operationlog:test_waiting_writer", func(tx *gorm.DB) {
		if tx.Statement.Context.Value(writerContextKey{}) == 2 && strings.Contains(tx.Statement.SQL.String(), "pg_advisory_xact_lock") {
			signal.Do(func() { close(secondWaiting) })
		}
	}))
	defer func() { require.NoError(t, db.Callback().Raw().Remove("operationlog:test_waiting_writer")) }()
	results := make(chan error, 2)
	write := func(tx *gorm.DB, action string) error {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("PUT", "/api/v1/settings", nil).WithContext(ctx)
		return logs.WriteTx(c, tx, operationlog.WriteOpts{TenantID: 77, AdminRole: "admin", Action: action, Resource: "settings", Status: "success"})
	}
	go func() {
		results <- db.WithContext(context.WithValue(ctx, writerContextKey{}, 1)).Transaction(func(tx *gorm.DB) error {
			if err := write(tx, "first.begin"); err != nil {
				return err
			}
			close(firstWritten)
			select {
			case <-secondWaiting:
			case <-ctx.Done():
				return ctx.Err()
			}
			return write(tx, "first.end")
		})
	}()
	go func() {
		select {
		case <-firstWritten:
		case <-ctx.Done():
			results <- ctx.Err()
			return
		}
		results <- db.WithContext(context.WithValue(ctx, writerContextKey{}, 2)).Transaction(func(tx *gorm.DB) error {
			return write(tx, "second.begin")
		})
	}()
	for i := 0; i < 2; i++ {
		require.NoError(t, <-results, "both audit transactions must commit without a lock inversion")
	}
	count, _, err := logs.VerifyChain(context.Background(), 77, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	require.NoError(t, err)
	require.Equal(t, 3, count)
}
