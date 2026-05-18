package api

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/encrypt"
	"github.com/trademind-ai/trademind/backend/internal/middleware"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/aioperationbatch"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiprompt"
	"github.com/trademind-ai/trademind/backend/internal/modules/aitask"
	"github.com/trademind-ai/trademind/backend/internal/modules/auth"
	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"github.com/trademind-ai/trademind/backend/internal/modules/collectrule"
	"github.com/trademind-ai/trademind/backend/internal/modules/customerchat"
	"github.com/trademind-ai/trademind/backend/internal/modules/customersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/files"
	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationdashboard"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/orderexception"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productcheck"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter"
	"github.com/trademind-ai/trademind/backend/internal/modules/worker"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	aigate "github.com/trademind-ai/trademind/backend/internal/providers/ai"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	platformamazon "github.com/trademind-ai/trademind/backend/internal/providers/platform/amazon"
	platformlazada "github.com/trademind-ai/trademind/backend/internal/providers/platform/lazada"
	platformshopee "github.com/trademind-ai/trademind/backend/internal/providers/platform/shopee"
	platformtiktok "github.com/trademind-ai/trademind/backend/internal/providers/platform/tiktok"
	"github.com/trademind-ai/trademind/backend/internal/rdb"
	"gorm.io/gorm"
)

type collectRunnerAdapter struct {
	c *collect.CollectorClient
}

func (a collectRunnerAdapter) RunCollect(ctx context.Context, source, rawURL string, options map[string]any) (json.RawMessage, error) {
	out, err := a.c.Collect(ctx, source, rawURL, options)
	if err != nil {
		return nil, err
	}
	return out.ProductJSON, nil
}

// Deps holds process-wide dependencies for HTTP handlers.
type Deps struct {
	Config    *config.Config
	DB        *gorm.DB
	Redis     *rdb.Client
	Encrypter *encrypt.Service
	// OpLog optional; when nil Register creates a default operation log service from DB.
	OpLog *operationlog.Service
}

// Register mounts routes on the engine and returns services for optional async workers.
func Register(r gin.IRouter, dep *Deps) (*collect.Service, *imagetask.Service, *ordersync.Service, *customersync.Service, *productpublish.Service, *inventory.Service, *taskcenter.Service) {
	if dep == nil {
		dep = &Deps{}
	}
	platformp.Bootstrap()
	h := healthHandler(dep)
	r.GET("/health", h)
	r.GET("/api/v1/health", h)

	adminStore := &admin.Store{DB: dep.DB}
	loginSvc := &auth.LoginService{Cfg: dep.Config, Admins: adminStore}
	settingsSvc := &settings.Service{DB: dep.DB, Encrypter: dep.Encrypter}
	opLogSvc := dep.OpLog
	if opLogSvc == nil {
		opLogSvc = &operationlog.Service{DB: dep.DB}
	}

	authH := &auth.Handler{LoginSvc: loginSvc, Admins: adminStore, OpLog: opLogSvc, Redis: dep.Redis, Settings: settingsSvc}
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

	collectRuleSvc := &collectrule.Service{
		DB:          dep.DB,
		OpLog:       opLogSvc,
		Runner:      collectRunnerAdapter{c: collectorClient},
		TestTimeout: collectorTimeout,
	}

	promptSvc := &aiprompt.Service{DB: dep.DB}
	aiGateway := &aigate.Gateway{Settings: settingsSvc}
	aiTaskSvc := &aitask.Service{DB: dep.DB}
	imageTaskSvc := &imagetask.Service{
		DB:       dep.DB,
		OpLog:    opLogSvc,
		Settings: settingsSvc,
		Files:    fileSvc,
		Redis:    dep.Redis,
	}
	if dep.Config != nil {
		imageTaskSvc.QueueEnabled = dep.Config.ImageQueueEnabled
		imageTaskSvc.AutoRetryEnabled = dep.Config.ImageAutoRetryEnabled
		imageTaskSvc.MaxAutoRetries = dep.Config.ImageMaxRetries
		imageTaskSvc.RetryBaseDelaySec = dep.Config.ImageRetryBaseDelaySeconds
		imageTaskSvc.RetryMaxDelaySec = dep.Config.ImageRetryMaxDelaySeconds
		if strings.TrimSpace(dep.Config.ImageQueueName) != "" {
			imageTaskSvc.QueueName = strings.TrimSpace(dep.Config.ImageQueueName)
		} else {
			imageTaskSvc.QueueName = "image:tasks"
		}
		if dep.Config.ImageTaskTimeoutSeconds > 0 {
			imageTaskSvc.TaskTimeoutMax = time.Duration(dep.Config.ImageTaskTimeoutSeconds) * time.Second
		}
	}
	imageTaskH := &imagetask.Handler{Svc: imageTaskSvc}

	productSvc := &product.Service{
		DB:        dep.DB,
		OpLog:     opLogSvc,
		Settings:  settingsSvc,
		Prompts:   promptSvc,
		AITasks:   aiTaskSvc,
		AIGateway: aiGateway,
	}
	productH := &product.Handler{Svc: productSvc}

	aiBatchSvc := &aioperationbatch.Service{
		DB:       dep.DB,
		Settings: settingsSvc,
		Products: productSvc,
		Image:    imageTaskSvc,
		OpLog:    opLogSvc,
	}
	aiBatchH := &aioperationbatch.Handler{Svc: aiBatchSvc}

	promptH := &aiprompt.Handler{Svc: promptSvc}
	aiTaskH := &aitask.Handler{Svc: aiTaskSvc}

	collectSvc := &collect.Service{
		DB:       dep.DB,
		Products: productSvc,
		Rules:    collectRuleSvc,
		OpLog:    opLogSvc,
		Client:   collectorClient,
		Redis:    dep.Redis,
	}
	if dep.Config != nil {
		collectSvc.QueueName = dep.Config.CollectQueueName
		collectSvc.QueueEnabled = dep.Config.CollectQueueEnabled
		collectSvc.BatchMaxURLs = dep.Config.CollectBatchMaxURLs
		collectSvc.CollectorTimeoutSeconds = dep.Config.CollectorTimeoutSeconds
		collectSvc.AutoRetryEnabled = dep.Config.CollectAutoRetryEnabled
		collectSvc.MaxAutoRetries = dep.Config.CollectMaxRetries
		collectSvc.RetryBaseDelaySec = dep.Config.CollectRetryBaseDelaySeconds
		collectSvc.RetryMaxDelaySec = dep.Config.CollectRetryMaxDelaySeconds
		collectSvc.TaskLeaseTimeoutSeconds = dep.Config.CollectTaskTimeoutSeconds
	}
	collectH := &collect.Handler{Svc: collectSvc}
	collectRuleH := &collectrule.Handler{Svc: collectRuleSvc}

	shopSvc := &shop.Service{
		DB:        dep.DB,
		Encrypter: dep.Encrypter,
		OpLog:     opLogSvc,
		Redis:     dep.Redis,
		Settings:  settingsSvc,
	}
	platformtiktok.BindShops(shopSvc.TikTokShopsBridge())
	platformtiktok.BindPublishImages(newTikTokListingImageFetcher(settingsSvc))
	platformtiktok.RegisterProvider()
	platformshopee.BindShops(shopSvc.ShopeeShopsBridge())
	platformshopee.BindPublishImages(newTikTokListingImageFetcher(settingsSvc))
	platformshopee.RegisterProvider()
	platformlazada.BindShops(shopSvc.LazadaShopsBridge())
	platformlazada.BindPublishImages(newTikTokListingImageFetcher(settingsSvc))
	platformlazada.RegisterProvider()
	platformamazon.BindShops(shopSvc.AmazonShopsBridge())
	platformamazon.RegisterProvider()
	shopH := &shop.Handler{Svc: shopSvc}

	inventorySvc := &inventory.Service{
		DB:       dep.DB,
		Redis:    dep.Redis,
		Shops:    shopSvc,
		Settings: settingsSvc,
		OpLog:    opLogSvc,
	}
	if dep.Config != nil {
		inventorySvc.QueueEnabled = dep.Config.InventorySyncQueueEnabled
		if strings.TrimSpace(dep.Config.InventorySyncQueueName) != "" {
			inventorySvc.QueueName = strings.TrimSpace(dep.Config.InventorySyncQueueName)
		} else {
			inventorySvc.QueueName = "inventory:sync:tasks"
		}
		if dep.Config.InventorySyncTaskTimeoutSeconds > 0 {
			inventorySvc.TaskTimeout = time.Duration(dep.Config.InventorySyncTaskTimeoutSeconds) * time.Second
		}
	}
	inventoryH := &inventory.Handler{Svc: inventorySvc}

	orderSvc := &order.Service{DB: dep.DB, OpLog: opLogSvc, Shops: shopSvc, Settings: settingsSvc}
	orderH := &order.Handler{Svc: orderSvc, Inv: inventorySvc}

	orderSyncSvc := &ordersync.Service{
		DB:        dep.DB,
		Redis:     dep.Redis,
		Shops:     shopSvc,
		Orders:    orderSvc,
		Inventory: inventorySvc,
		OpLog:     opLogSvc,
	}
	if dep.Config != nil {
		orderSyncSvc.QueueEnabled = dep.Config.OrderSyncQueueEnabled
		if strings.TrimSpace(dep.Config.OrderSyncQueueName) != "" {
			orderSyncSvc.QueueName = strings.TrimSpace(dep.Config.OrderSyncQueueName)
		} else {
			orderSyncSvc.QueueName = "order:sync:tasks"
		}
		if dep.Config.OrderSyncTaskTimeoutSeconds > 0 {
			orderSyncSvc.TaskTimeout = time.Duration(dep.Config.OrderSyncTaskTimeoutSeconds) * time.Second
		}
	}
	orderSyncH := &ordersync.Handler{Svc: orderSyncSvc}

	excSvc := &orderexception.Service{
		DB:     dep.DB,
		Orders: orderSvc,
		Inv:    inventorySvc,
	}
	excCmd := &orderexception.Commands{
		Svc:    excSvc,
		Orders: orderSvc,
		Inv:    inventorySvc,
	}
	excH := &orderexception.Handler{
		Svc:   excSvc,
		Cmds:  excCmd,
		OpLog: opLogSvc,
	}

	customerChatSvc := &customerchat.Service{
		DB:        dep.DB,
		Settings:  settingsSvc,
		Prompts:   promptSvc,
		AITasks:   aiTaskSvc,
		AIGateway: aiGateway,
		OpLog:     opLogSvc,
		Orders:    orderSvc,
		Shops:     shopSvc,
	}
	customerChatH := &customerchat.Handler{Svc: customerChatSvc}

	customerSyncSvc := &customersync.Service{
		DB:           dep.DB,
		Redis:        dep.Redis,
		Shops:        shopSvc,
		Settings:     settingsSvc,
		CustomerChat: customerChatSvc,
		OpLog:        opLogSvc,
	}
	if dep.Config != nil {
		customerSyncSvc.QueueEnabled = dep.Config.CustomerMessageSyncQueueEnabled
		if strings.TrimSpace(dep.Config.CustomerMessageSyncQueueName) != "" {
			customerSyncSvc.QueueName = strings.TrimSpace(dep.Config.CustomerMessageSyncQueueName)
		} else {
			customerSyncSvc.QueueName = "customer:message:sync:tasks"
		}
		if dep.Config.CustomerMessageSyncTaskTimeoutSeconds > 0 {
			customerSyncSvc.TaskTimeout = time.Duration(dep.Config.CustomerMessageSyncTaskTimeoutSeconds) * time.Second
		}
	}
	customerSyncH := &customersync.Handler{Svc: customerSyncSvc}

	readinessSvc := &productcheck.Service{
		DB:       dep.DB,
		Settings: settingsSvc,
		Shops:    shopSvc,
	}

	productPublishSvc := &productpublish.Service{
		DB:        dep.DB,
		Redis:     dep.Redis,
		Shops:     shopSvc,
		Settings:  settingsSvc,
		OpLog:     opLogSvc,
		Readiness: readinessSvc,
	}
	if dep.Config != nil {
		productPublishSvc.QueueEnabled = dep.Config.ProductPublishQueueEnabled
		if strings.TrimSpace(dep.Config.ProductPublishQueueName) != "" {
			productPublishSvc.QueueName = strings.TrimSpace(dep.Config.ProductPublishQueueName)
		} else {
			productPublishSvc.QueueName = "product:publish:tasks"
		}
		if dep.Config.ProductPublishTaskTimeoutSeconds > 0 {
			productPublishSvc.TaskTimeout = time.Duration(dep.Config.ProductPublishTaskTimeoutSeconds) * time.Second
		}
	}
	productPublishH := &productpublish.Handler{Svc: productPublishSvc}
	readinessH := &productcheck.Handler{
		Svc:   readinessSvc,
		OpLog: opLogSvc,
	}

	r.GET("/static/*filepath", staticH.Serve)

	v1 := r.Group("/api/v1")
	v1.POST("/auth/login", authH.Login)
	v1.POST("/auth/register", authH.Register)
	v1.POST("/auth/send-email-code", authH.SendEmailCode)

	authed := v1.Group("")
	authed.Use(middleware.BearerAuth(dep.Config))
	authed.GET("/auth/profile", authH.Profile)
	authed.POST("/auth/logout", authH.Logout)
	authed.GET("/settings", setH.List)
	authed.PUT("/settings", setH.Put)
	authed.GET("/settings/integration-schemas", setH.IntegrationSchemas)
	authed.GET("/settings/integrations/overview", setH.IntegrationOverview)
	authed.POST("/settings/test-ai", setH.TestAI)
	authed.POST("/settings/test-storage", setH.TestStorage)
	authed.POST("/settings/test-platform-tiktok", setH.TestPlatformTikTok)
	authed.POST("/settings/test-email", setH.TestEmail)

	authed.GET("/operation-logs", opLogH.List)
	authed.POST("/files/upload", fileH.Upload)
	authed.GET("/files", fileH.List)
	authed.DELETE("/files/:id", fileH.Delete)

	aiprompt.Register(authed, promptH)
	aioperationbatch.Register(authed, aiBatchH)
	aitask.Register(authed, aiTaskH)
	imagetask.Register(authed, imageTaskH)
	product.Register(authed, productH)
	collect.Register(authed, collectH)
	collectrule.Register(authed, collectRuleH)
	productcheck.Register(authed, readinessH)
	order.Register(authed, orderH)
	orderexception.Register(authed, excH)
	ordersync.Register(authed, orderSyncH)
	customersync.Register(authed, customerSyncH)
	customerchat.Register(authed, customerChatH)
	shop.Register(authed, shopH)
	productpublish.Register(authed, productPublishH)
	inventory.Register(authed, inventoryH)
	workerH := &worker.Handler{DB: dep.DB, Cfg: dep.Config}
	worker.Register(authed, workerH)

	tcSvc := &taskcenter.Service{
		DB:             dep.DB,
		Cfg:            dep.Config,
		OpLog:          opLogSvc,
		Settings:       settingsSvc,
		Collect:        collectSvc,
		Image:          imageTaskSvc,
		OrderSync:      orderSyncSvc,
		CustomerSync:   customerSyncSvc,
		ProductPublish: productPublishSvc,
		Inventory:      inventorySvc,
	}
	tcH := &taskcenter.Handler{Svc: tcSvc}
	taskcenter.Register(authed, tcH)

	dashSvc := &operationdashboard.Service{
		DB:              dep.DB,
		Inventory:       inventorySvc,
		TaskCenter:      tcSvc,
		OrderExceptions: excSvc,
	}
	dashH := &operationdashboard.Handler{Svc: dashSvc}
	operationdashboard.Register(authed, dashH)

	return collectSvc, imageTaskSvc, orderSyncSvc, customerSyncSvc, productPublishSvc, inventorySvc, tcSvc
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
		queueEnabled := false
		queueName := "collect:tasks"
		workerConc := 2
		if dep.Config != nil {
			appEnv = dep.Config.AppEnv
			queueEnabled = dep.Config.CollectQueueEnabled
			if strings.TrimSpace(dep.Config.CollectQueueName) != "" {
				queueName = strings.TrimSpace(dep.Config.CollectQueueName)
			}
			workerConc = dep.Config.CollectWorkerConcurrency
			if workerConc < 1 {
				workerConc = 2
			}
		}
		cq := collect.BuildCollectQueueHealthBlock(ctx, dep.Redis, queueEnabled, queueName, workerConc)
		if queueEnabled && !cq.RedisAvailable && checks["redis"] == "ok" {
			status = "degraded"
		}

		imgQEnabled := false
		imgQName := "image:tasks"
		imgWConc := 2
		if dep.Config != nil {
			imgQEnabled = dep.Config.ImageQueueEnabled
			if strings.TrimSpace(dep.Config.ImageQueueName) != "" {
				imgQName = strings.TrimSpace(dep.Config.ImageQueueName)
			}
			imgWConc = dep.Config.ImageWorkerConcurrency
			if imgWConc < 1 {
				imgWConc = 2
			}
		}
		iq := imagetask.BuildImageQueueHealthBlock(ctx, dep.Redis, imgQEnabled, imgQName, imgWConc)
		if imgQEnabled && !iq.RedisAvailable && checks["redis"] == "ok" {
			status = "degraded"
		}

		osQEnabled := false
		osQName := "order:sync:tasks"
		osWConc := 1
		if dep.Config != nil {
			osQEnabled = dep.Config.OrderSyncQueueEnabled
			if strings.TrimSpace(dep.Config.OrderSyncQueueName) != "" {
				osQName = strings.TrimSpace(dep.Config.OrderSyncQueueName)
			}
			osWConc = dep.Config.OrderSyncWorkerConcurrency
			if osWConc < 1 {
				osWConc = 1
			}
		}
		osq := ordersync.BuildOrderSyncQueueHealthBlock(ctx, dep.Redis, osQEnabled, osQName, osWConc)
		if osQEnabled && !osq.RedisAvailable && checks["redis"] == "ok" {
			status = "degraded"
		}

		cmQEnabled := false
		cmQName := "customer:message:sync:tasks"
		cmWConc := 1
		if dep.Config != nil {
			cmQEnabled = dep.Config.CustomerMessageSyncQueueEnabled
			if strings.TrimSpace(dep.Config.CustomerMessageSyncQueueName) != "" {
				cmQName = strings.TrimSpace(dep.Config.CustomerMessageSyncQueueName)
			}
			cmWConc = dep.Config.CustomerMessageSyncWorkerConcurrency
			if cmWConc < 1 {
				cmWConc = 1
			}
		}
		cmq := customersync.BuildCustomerMessageSyncQueueHealthBlock(ctx, dep.Redis, cmQEnabled, cmQName, cmWConc)
		if cmQEnabled && !cmq.RedisAvailable && checks["redis"] == "ok" {
			status = "degraded"
		}

		ppQEnabled := false
		ppQName := "product:publish:tasks"
		ppWConc := 1
		if dep.Config != nil {
			ppQEnabled = dep.Config.ProductPublishQueueEnabled
			if strings.TrimSpace(dep.Config.ProductPublishQueueName) != "" {
				ppQName = strings.TrimSpace(dep.Config.ProductPublishQueueName)
			}
			ppWConc = dep.Config.ProductPublishWorkerConcurrency
			if ppWConc < 1 {
				ppWConc = 1
			}
		}
		ppq := productpublish.BuildProductPublishQueueHealthBlock(ctx, dep.Redis, ppQEnabled, ppQName, ppWConc)
		if ppQEnabled && !ppq.RedisAvailable && checks["redis"] == "ok" {
			status = "degraded"
		}

		invQEnabled := false
		invQName := "inventory:sync:tasks"
		invWConc := 1
		if dep.Config != nil {
			invQEnabled = dep.Config.InventorySyncQueueEnabled
			if strings.TrimSpace(dep.Config.InventorySyncQueueName) != "" {
				invQName = strings.TrimSpace(dep.Config.InventorySyncQueueName)
			}
			invWConc = dep.Config.InventorySyncWorkerConcurrency
			if invWConc < 1 {
				invWConc = 1
			}
		}
		invq := inventory.BuildInventorySyncQueueHealthBlock(ctx, dep.Redis, invQEnabled, invQName, invWConc)
		if invQEnabled && !invq.RedisAvailable && checks["redis"] == "ok" {
			status = "degraded"
		}

		workers := worker.BuildHealthWorkersBlock(ctx, dep.DB, dep.Config)
		if workers.Degraded {
			status = "degraded"
		}

		response.OK(c, gin.H{
			"status":                   status,
			"appEnv":                   appEnv,
			"checks":                   checks,
			"collectQueue":             cq,
			"imageQueue":               iq,
			"orderSyncQueue":           osq,
			"customerMessageSyncQueue": cmq,
			"productPublishQueue":      ppq,
			"inventorySyncQueue":       invq,
			"workers":                  workers,
			"timestamp":                time.Now().UTC().Format(time.RFC3339),
		})
	}
}
