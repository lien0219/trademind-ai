package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"gorm.io/gorm"
)

func TestReadyRequiresRedisInDeployedProfiles(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

	for _, tc := range []struct {
		env       string
		wantCode  int
		wantRedis string
	}{
		{config.EnvDevelopment, http.StatusOK, "skipped"},
		{config.EnvStaging, http.StatusServiceUnavailable, "down"},
		{config.EnvProduction, http.StatusServiceUnavailable, "down"},
	} {
		t.Run(tc.env, func(t *testing.T) {
			r := gin.New()
			Register(r, &Deps{
				DB: db, MigrationsReady: true,
				Config: &config.Config{AppEnv: tc.env, StorageProvider: "s3", MasterKey: "test-only"},
			})
			ready := httptest.NewRecorder()
			r.ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
			require.Equal(t, tc.wantCode, ready.Code)
			var payload struct {
				Data struct {
					Checks map[string]string `json:"checks"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(ready.Body.Bytes(), &payload))
			require.Equal(t, "ok", payload.Data.Checks["database"])
			require.Equal(t, tc.wantRedis, payload.Data.Checks["redis"])

			live := httptest.NewRecorder()
			r.ServeHTTP(live, httptest.NewRequest(http.MethodGet, "/health/live", nil))
			require.Equal(t, http.StatusOK, live.Code)
		})
	}
}
