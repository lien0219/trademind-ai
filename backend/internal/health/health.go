package health

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"github.com/trademind-ai/trademind/backend/internal/rdb"
	"gorm.io/gorm"
)

// Deps holds dependencies for health probes.
type Deps struct {
	Config *config.Config
	DB     *gorm.DB
	Redis  *rdb.Client
	// MigrationsReady is set true after AutoMigrate succeeds in main.
	MigrationsReady bool
}

// Register mounts liveness and readiness routes.
func Register(r gin.IRouter, dep *Deps) {
	if r == nil {
		return
	}
	r.GET("/health/live", liveHandler())
	r.GET("/health/ready", readyHandler(dep))
}

func liveHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		response.OK(c, gin.H{
			"status":    "alive",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	}
}

func readyHandler(dep *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()

		checks := gin.H{}
		ready := true
		warnings := make([]string, 0, 4)

		if dep == nil || dep.DB == nil {
			checks["database"] = "down"
			ready = false
		} else {
			sqlDB, err := dep.DB.DB()
			if err != nil || sqlDB.PingContext(ctx) != nil {
				checks["database"] = "down"
				ready = false
			} else {
				checks["database"] = "ok"
			}
		}

		if dep != nil && dep.Redis != nil && dep.Redis.Client != nil {
			if err := dep.Redis.Ping(ctx).Err(); err != nil {
				checks["redis"] = "down"
				ready = false
			} else {
				checks["redis"] = "ok"
			}
		} else if dep != nil && dep.Config != nil && config.IsStagingOrProduction(dep.Config.AppEnv) {
			checks["redis"] = "down"
			ready = false
		} else {
			checks["redis"] = "skipped"
			warnings = append(warnings, "redis_not_configured")
		}

		if dep != nil && dep.MigrationsReady {
			checks["migrations"] = "ok"
		} else {
			checks["migrations"] = "pending"
			ready = false
		}

		if dep != nil && dep.Config != nil {
			checks["appEnv"] = dep.Config.AppEnv
			if !dep.Config.AllowsLocalStorageProvider() {
				checks["storage"] = "local_forbidden"
				ready = false
			} else {
				checks["storage"] = "ok"
			}
			if config.IsProduction(dep.Config.AppEnv) {
				if strings.TrimSpace(dep.Config.MasterKey) == "" {
					checks["masterKey"] = "missing"
					ready = false
				} else {
					checks["masterKey"] = "ok"
				}
				if dep.Config.EnableDemoSeed || dep.Config.EnableDevRoutes {
					checks["dangerousFeatures"] = "enabled"
					ready = false
				} else {
					checks["dangerousFeatures"] = "disabled"
				}
			}
		}

		status := "ready"
		code := http.StatusOK
		if !ready {
			status = "not_ready"
			code = http.StatusServiceUnavailable
		}
		payload := gin.H{
			"status":    status,
			"checks":    checks,
			"warnings":  warnings,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		if ready {
			response.OK(c, payload)
			return
		}
		response.JSON(c, code, response.CodeInternalError, "service not ready", payload)
	}
}
