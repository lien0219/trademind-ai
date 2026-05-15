package api

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/encrypt"
	"github.com/trademind-ai/trademind/backend/internal/middleware"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiprompt"
	"github.com/trademind-ai/trademind/backend/internal/modules/aitask"
	"github.com/trademind-ai/trademind/backend/internal/modules/auth"
	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"github.com/trademind-ai/trademind/backend/internal/modules/files"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	aigate "github.com/trademind-ai/trademind/backend/internal/providers/ai"
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

// Register mounts routes on the engine and returns collect service (for optional async worker).
func Register(r gin.IRouter, dep *Deps) *collect.Service {
	if dep == nil {
		dep = &Deps{}
	}
	h := healthHandler(dep)
	r.GET("/health", h)
	r.GET("/api/v1/health", h)

	adminStore := &admin.Store{DB: dep.DB}
	loginSvc := &auth.LoginService{Cfg: dep.Config, Admins: adminStore}
	settingsSvc := &settings.Service{DB: dep.DB, Encrypter: dep.Encrypter}
	opLogSvc := &operationlog.Service{DB: dep.DB}

	authH := &auth.Handler{LoginSvc: loginSvc, Admins: adminStore, OpLog: opLogSvc}
	setH := &settings.Handler{Svc: settingsSvc, OpLog: opLogSvc}
	opLogH := &operationlog.Handler{Svc: opLogSvc}

	maxUp := int64(10 << 20)
	if dep.Config != nil {
		maxUp = dep.Config.MaxUploadBytes()
	}
	fileSvc := &files.Service{DB: dep.DB, Settings: settingsSvc, MaxBytes: maxUp}
	fileH := &files.Handler{Svc: fileSvc}
	staticH := &files.StaticHandler{Settings: settingsSvc}

	collectorTimeout := 60 * time.Second
	if dep.Config != nil && dep.Config.CollectorTimeoutSeconds > 0 {
		collectorTimeout = time.Duration(dep.Config.CollectorTimeoutSeconds) * time.Second
	}
	collectorBase := "http://127.0.0.1:3100"
	if dep.Config != nil && dep.Config.CollectorBaseURL != "" {
		collectorBase = dep.Config.CollectorBaseURL
	}
	collectorClient := collect.NewCollectorClient(collectorBase, collectorTimeout)

	promptSvc := &aiprompt.Service{DB: dep.DB}
	aiGateway := &aigate.Gateway{Settings: settingsSvc}
	aiTaskSvc := &aitask.Service{DB: dep.DB}

	productSvc := &product.Service{
		DB:        dep.DB,
		OpLog:     opLogSvc,
		Settings:  settingsSvc,
		Prompts:   promptSvc,
		AITasks:   aiTaskSvc,
		AIGateway: aiGateway,
	}
	productH := &product.Handler{Svc: productSvc}
	promptH := &aiprompt.Handler{Svc: promptSvc}
	aiTaskH := &aitask.Handler{Svc: aiTaskSvc}

	collectSvc := &collect.Service{
		DB:       dep.DB,
		Products: productSvc,
		OpLog:    opLogSvc,
		Client:   collectorClient,
		Redis:    dep.Redis,
	}
	if dep.Config != nil {
		collectSvc.QueueName = dep.Config.CollectQueueName
		collectSvc.QueueEnabled = dep.Config.CollectQueueEnabled
	}
	collectH := &collect.Handler{Svc: collectSvc}

	r.GET("/static/*filepath", staticH.Serve)

	v1 := r.Group("/api/v1")
	v1.POST("/auth/login", authH.Login)

	authed := v1.Group("")
	authed.Use(middleware.BearerAuth(dep.Config))
	authed.GET("/auth/profile", authH.Profile)
	authed.POST("/auth/logout", authH.Logout)
	authed.GET("/settings", setH.List)
	authed.PUT("/settings", setH.Put)
	authed.POST("/settings/test-ai", setH.TestAI)
	authed.POST("/settings/test-storage", setH.TestStorage)

	authed.GET("/operation-logs", opLogH.List)
	authed.POST("/files/upload", fileH.Upload)
	authed.GET("/files", fileH.List)
	authed.DELETE("/files/:id", fileH.Delete)

	aiprompt.Register(authed, promptH)
	aitask.Register(authed, aiTaskH)
	product.Register(authed, productH)
	collect.Register(authed, collectH)
	return collectSvc
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
