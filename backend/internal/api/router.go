package api

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/encrypt"
	"github.com/trademind-ai/trademind/backend/internal/middleware"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/auth"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"github.com/trademind-ai/trademind/backend/internal/rdb"
	"gorm.io/gorm"
)

// Deps holds process-wide dependencies for HTTP handlers.
type Deps struct {
	Config    *config.Config
	DB        *gorm.DB
	Redis     *rdb.Client
	Encrypter *encrypt.Service
}

// Register mounts routes on the engine.
func Register(r gin.IRouter, dep *Deps) {
	if dep == nil {
		dep = &Deps{}
	}
	h := healthHandler(dep)
	r.GET("/health", h)
	r.GET("/api/v1/health", h)

	adminStore := &admin.Store{DB: dep.DB}
	loginSvc := &auth.LoginService{Cfg: dep.Config, Admins: adminStore}
	authH := &auth.Handler{LoginSvc: loginSvc, Admins: adminStore}
	setH := &settings.Handler{Svc: &settings.Service{DB: dep.DB, Encrypter: dep.Encrypter}}

	v1 := r.Group("/api/v1")
	v1.POST("/auth/login", authH.Login)

	authed := v1.Group("")
	authed.Use(middleware.BearerAuth(dep.Config))
	authed.GET("/auth/profile", authH.Profile)
	authed.POST("/auth/logout", authH.Logout)
	authed.GET("/settings", setH.List)
	authed.PUT("/settings", setH.Put)
}

func healthHandler(dep *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		checks := gin.H{
			"database": "unknown",
			"redis":    "unknown",
		}

		if dep.DB != nil {
			sqlDB, err := dep.DB.DB()
			if err != nil || sqlDB.PingContext(ctx) != nil {
				checks["database"] = "down"
			} else {
				checks["database"] = "ok"
			}
		} else {
			checks["database"] = "down"
		}

		switch {
		case dep.Redis == nil:
			checks["redis"] = "skipped"
		default:
			if err := dep.Redis.Ping(ctx).Err(); err != nil {
				checks["redis"] = "down"
			} else {
				checks["redis"] = "ok"
			}
		}

		status := "up"
		if checks["database"] != "ok" {
			response.Fail(c, 503, response.CodeInternalError, "database unavailable")
			return
		}
		if checks["redis"] == "down" {
			status = "degraded"
		}

		appEnv := ""
		if dep.Config != nil {
			appEnv = dep.Config.AppEnv
		}

		response.OK(c, gin.H{
			"status":    status,
			"appEnv":    appEnv,
			"checks":    checks,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	}
}
