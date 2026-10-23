package integration

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/securitymod"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/testing/postgrestest"
	"github.com/trademind-ai/trademind/backend/internal/testing/safeenv"
)

func TestKeyRotationConcurrentStartsAreAudited(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	_, ok, err := safeenv.TestDatabaseURLFromEnv()
	require.NoError(t, err)
	if !ok {
		t.Skip("TEST_DATABASE_URL is not set; skipping PostgreSQL key rotation concurrency test")
	}
	db := postgrestest.Require(t).DB
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &securitymod.KeyRotationJob{}, &operationlog.OperationLog{}, &settings.Setting{}, &shop.Shop{}, &shop.ShopAuthToken{}))
	actors := []admin.AdminUser{
		{Username: admin.NewInternalUsername(), Email: "rotation-a@example.test", TenantID: 0, Role: "admin", Status: admin.StatusActive, PasswordHash: "test-only", TokenVersion: 1},
		{Username: admin.NewInternalUsername(), Email: "rotation-b@example.test", TenantID: 0, Role: "admin", Status: admin.StatusActive, PasswordHash: "test-only", TokenVersion: 1},
	}
	require.NoError(t, db.Create(&actors).Error)
	svc := &securitymod.Service{DB: db, Cfg: &config.Config{MasterKey: "0123456789abcdef0123456789abcdef", Auth: config.AuthConfig{AppMasterActiveKeyID: "active"}}, OpLogs: &operationlog.Service{DB: db}}
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, c.GetHeader("X-Test-Actor"))
		c.Set(ctxkey.TenantID, int64(0))
		c.Next()
	})
	securitymod.RegisterRoutes(r.Group("/api/v1"), &securitymod.Handler{Svc: svc, DB: db})

	const writers = 8
	start := make(chan struct{})
	results := make(chan *httptest.ResponseRecorder, writers)
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		actorID := actors[i%len(actors)].ID.String()
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/security/keys/rotation/start", strings.NewReader(`{"confirmPhrase":"ROTATE-KEYS-START"}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Test-Actor", actorID)
			r.ServeHTTP(w, req)
			results <- w
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	for w := range results {
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	}
	var jobs []securitymod.KeyRotationJob
	require.NoError(t, db.Find(&jobs).Error)
	require.Len(t, jobs, 1, "concurrent confirmations must share the active-key job")
	var logs []operationlog.OperationLog
	require.NoError(t, db.Where("action = ? AND status = ?", "key_rotation_start", "success").Find(&logs).Error)
	require.Len(t, logs, writers, "each accepted control must retain its evidence")
	for _, entry := range logs {
		require.Equal(t, jobs[0].ID.String(), entry.ResourceID)
		require.NotEmpty(t, entry.EntryHash)
	}
}
