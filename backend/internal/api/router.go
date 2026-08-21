package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/encrypt"
	"github.com/trademind-ai/trademind/backend/internal/health"
	"github.com/trademind-ai/trademind/backend/internal/middleware"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/adminuser"
	"github.com/trademind-ai/trademind/backend/internal/modules/aioperationbatch"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiopsworkbench"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiproductimage"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiproducttext"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiprompt"
	"github.com/trademind-ai/trademind/backend/internal/modules/aitask"
	"github.com/trademind-ai/trademind/backend/internal/modules/alerting"
	"github.com/trademind-ai/trademind/backend/internal/modules/auth"
	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"github.com/trademind-ai/trademind/backend/internal/modules/collectbrowserprofile"
	"github.com/trademind-ai/trademind/backend/internal/modules/collectrule"
	"github.com/trademind-ai/trademind/backend/internal/modules/collectruleai"
	"github.com/trademind-ai/trademind/backend/internal/modules/configstatus"
	"github.com/trademind-ai/trademind/backend/internal/modules/customerchat"
	"github.com/trademind-ai/trademind/backend/internal/modules/customersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/demoseed"
	"github.com/trademind-ai/trademind/backend/internal/modules/douyinpreflight"
	"github.com/trademind-ai/trademind/backend/internal/modules/douyinruntime"
	"github.com/trademind-ai/trademind/backend/internal/modules/exportmod"
	"github.com/trademind-ai/trademind/backend/internal/modules/files"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventorysync"
	"github.com/trademind-ai/trademind/backend/internal/modules/observabilitymod"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationdashboard"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationtask"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/orderexception"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/pricing"
	"github.com/trademind-ai/trademind/backend/internal/modules/procurement"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productcheck"
	"github.com/trademind-ai/trademind/backend/internal/modules/productioncontrol"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/securitymod"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/modules/skucandidate"
	"github.com/trademind-ai/trademind/backend/internal/modules/storagepublic"
	"github.com/trademind-ai/trademind/backend/internal/modules/supplier"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter"
	"github.com/trademind-ai/trademind/backend/internal/modules/warehouse"
	"github.com/trademind-ai/trademind/backend/internal/modules/webhook"
	"github.com/trademind-ai/trademind/backend/internal/modules/worker"
	"github.com/trademind-ai/trademind/backend/internal/pkg/metrics"
	"github.com/trademind-ai/trademind/backend/internal/pkg/observability"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	aigate "github.com/trademind-ai/trademind/backend/internal/providers/ai"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	platformamazon "github.com/trademind-ai/trademind/backend/internal/providers/platform/amazon"
	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
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

type browserProfileCollectorGW struct {
	c *collect.CollectorClient
}

func (g browserProfileCollectorGW) OpenProfileLogin(ctx context.Context, profileKey, rawURL string) (string, error) {
	if g.c == nil {
		return "", fmt.Errorf("collector client unavailable")
	}
	return g.c.OpenBrowserProfileLogin(ctx, profileKey, rawURL)
}

func (g browserProfileCollectorGW) CheckProfileAccess(ctx context.Context, profileKey, rawURL string) (*collectbrowserprofile.CheckResultDTO, error) {
	if g.c == nil {
		return nil, fmt.Errorf("collector client unavailable")
	}
	out, err := g.c.CheckBrowserProfileAccess(ctx, profileKey, rawURL)
	if err != nil {
		return nil, err
	}
	return &collectbrowserprofile.CheckResultDTO{
		AccessStatus: out.AccessStatus,
		FinalURL:     out.FinalURL,
		ErrorCode:    out.ErrorCode,
		Message:      out.Message,
	}, nil
}

func (a collectRunnerAdapter) CustomRuleTest(ctx context.Context, rawURL string, options map[string]any) (*collectrule.RuleTestResultDTO, error) {
	raw, err := a.c.CustomRuleTest(ctx, rawURL, options)
	if err != nil {
		return nil, err
	}
	var extracted map[string]interface{}
	if len(raw.ExtractedFields) > 0 {
		_ = json.Unmarshal(raw.ExtractedFields, &extracted)
	}
	var qualityScore map[string]interface{}
	if len(raw.QualityScore) > 0 {
		_ = json.Unmarshal(raw.QualityScore, &qualityScore)
	}
	return &collectrule.RuleTestResultDTO{
		AccessStatus:    raw.AccessStatus,
		FinalURL:        raw.FinalURL,
		HTTPStatus:      raw.HTTPStatus,
		ExtractedFields: extracted,
		MissingFields:   raw.MissingFields,
		Warnings:        raw.Warnings,
		QualityScore:    qualityScore,
		ErrorCode:       raw.ErrorCode,
		Suggestion:      raw.Suggestion,
		Product:         raw.Product,
	}, nil
}

type collectRuleCreatorAdapter struct {
	svc *collectrule.Service
}

func (a collectRuleCreatorAdapter) CreateFromAI(c *gin.Context, body collectrule.CreateRuleBody, adminID *uuid.UUID) (*collectrule.RuleDetailDTO, error) {
	if a.svc == nil {
		return nil, fmt.Errorf("collect rules unavailable")
	}
	return a.svc.Create(c, body, adminID)
}

// Deps holds process-wide dependencies for HTTP handlers.
type Deps struct {
	Config          *config.Config
	DB              *gorm.DB
	Redis           *rdb.Client
	Encrypter       *encrypt.Service
	MigrationsReady bool
	// OpLog optional; when nil Register creates a default operation log service from DB.
	OpLog *operationlog.Service
	// Obs is the optional observability facade.
	Obs *observability.Observability
}

// Register mounts routes on the engine and returns services for optional async workers.
func Register(r gin.IRouter, dep *Deps) (*collect.Service, *imagetask.Service, *ordersync.Service, *customersync.Service, *productpublish.Service, *inventory.Service, *taskcenter.Service, *douyinruntime.Service, *webhook.Service, *files.Service, *securitymod.Service) {
	if dep == nil {
		dep = &Deps{}
	}
	platformp.Bootstrap()
	h := healthHandler(dep)
	r.GET("/health", h)
	r.GET("/api/v1/health", h)
	health.Register(r, &health.Deps{
		Config:          dep.Config,
		DB:              dep.DB,
		Redis:           dep.Redis,
		MigrationsReady: dep.MigrationsReady,
	})

	if dep.Obs != nil && dep.Config != nil && dep.Config.Observability.MetricsEnabled {
		metricsPath := strings.TrimSpace(dep.Config.Observability.MetricsPath)
		if metricsPath == "" {
			metricsPath = "/internal/metrics"
		}
		internal := r.Group(metricsPath)
		internal.Use(middleware.MetricsGuard(dep.Config.Observability.MetricsInternalOnly, dep.Config.Observability.MetricsAllowlistCIDRs))
		internal.GET("", observabilitymod.MetricsEndpoint(dep.Obs))
	}

	alertCooldown := 5 * time.Minute
	alertRecovery := true
	if dep.Config != nil {
		if dep.Config.Observability.AlertDefaultCooldownSecs > 0 {
			alertCooldown = time.Duration(dep.Config.Observability.AlertDefaultCooldownSecs) * time.Second
		}
		alertRecovery = dep.Config.Observability.AlertRecoveryEnabled
	}

	adminStore := &admin.Store{DB: dep.DB}
	var metricCatalog *metrics.Catalog
	if dep.Obs != nil {
		metricCatalog = dep.Obs.Catalog
	}
	sessionSvc := &auth.SessionService{Cfg: dep.Config, DB: dep.DB, Admins: adminStore, Metrics: metricCatalog}
	loginSvc := &auth.LoginService{Cfg: dep.Config, Admins: adminStore, Sessions: sessionSvc, Metrics: metricCatalog}
	settingsSvc := &settings.Service{DB: dep.DB, Encrypter: dep.Encrypter}
	opLogSvc := dep.OpLog
	if opLogSvc == nil {
		opLogSvc = &operationlog.Service{DB: dep.DB}
	}
	idempotencySvc := &idempotency.Service{DB: dep.DB}

	aiGateway := &aigate.Gateway{Settings: settingsSvc}

	authH := &auth.Handler{LoginSvc: loginSvc, Sessions: sessionSvc, Admins: adminStore, OpLog: opLogSvc, Redis: dep.Redis, Settings: settingsSvc, DB: dep.DB, Cfg: dep.Config}
	sessionH := &auth.SessionHandler{Cfg: dep.Config, Sessions: sessionSvc, OpLog: opLogSvc, DB: dep.DB}
	setH := &settings.Handler{Svc: settingsSvc, OpLog: opLogSvc, AIGateway: aiGateway, DB: dep.DB}
	opLogH := &operationlog.Handler{Svc: opLogSvc, DB: dep.DB}

	maxUp := int64(10 << 20)
	if dep.Config != nil {
		maxUp = dep.Config.MaxUploadBytes()
	}
	fileSvc := &files.Service{DB: dep.DB, Redis: dep.Redis, Settings: settingsSvc, MaxBytes: maxUp, Metrics: metricCatalog}
	fileH := &files.Handler{Svc: fileSvc}
	staticH := &files.StaticHandler{Settings: settingsSvc}

	collectorTimeout := 120 * time.Second
	if dep.Config != nil && dep.Config.CollectorTimeoutSeconds > 0 {
		collectorTimeout = time.Duration(dep.Config.CollectorTimeoutSeconds) * time.Second
	}
	collectorBase := "http://127.0.0.1:3100"
	if dep.Config != nil && dep.Config.CollectorBaseURL != "" {
		collectorBase = dep.Config.CollectorBaseURL
	}
	collectorToken := ""
	if dep.Config != nil {
		collectorToken = dep.Config.CollectorServiceToken
	}
	collectorClient := collect.NewCollectorClient(collectorBase, collectorTimeout, collectorToken)

	profileSvc := &collectbrowserprofile.Service{
		DB:        dep.DB,
		Collector: browserProfileCollectorGW{c: collectorClient},
		OpLog:     opLogSvc,
		Timeout:   collectorTimeout,
	}
	profileH := &collectbrowserprofile.Handler{Svc: profileSvc}

	collectRuleSvc := &collectrule.Service{
		DB:          dep.DB,
		OpLog:       opLogSvc,
		Runner:      collectRunnerAdapter{c: collectorClient},
		Profiles:    profileSvc,
		TestTimeout: collectorTimeout,
	}

	promptSvc := &aiprompt.Service{DB: dep.DB}
	aiTaskSvc := &aitask.Service{DB: dep.DB}
	imageTaskSvc := &imagetask.Service{
		DB:        dep.DB,
		OpLog:     opLogSvc,
		Settings:  settingsSvc,
		Files:     fileSvc,
		Redis:     dep.Redis,
		AIGateway: aiGateway,
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
		DB:          dep.DB,
		OpLog:       opLogSvc,
		Settings:    settingsSvc,
		Prompts:     promptSvc,
		AITasks:     aiTaskSvc,
		AIGateway:   aiGateway,
		Idempotency: idempotencySvc,
	}
	productH := &product.Handler{Svc: productSvc, Files: fileSvc}

	aiBatchSvc := &aioperationbatch.Service{
		DB:       dep.DB,
		Settings: settingsSvc,
		Products: productSvc,
		Image:    imageTaskSvc,
		OpLog:    opLogSvc,
	}
	aiBatchH := &aioperationbatch.Handler{Svc: aiBatchSvc}

	aiProductTextSvc := &aiproducttext.Service{
		DB:          dep.DB,
		Settings:    settingsSvc,
		Products:    productSvc,
		OpLog:       opLogSvc,
		Idempotency: idempotencySvc,
		Metrics:     metricCatalog,
	}
	aiProductTextH := &aiproducttext.Handler{Svc: aiProductTextSvc}

	aiProductImageSvc := &aiproductimage.Service{
		DB:          dep.DB,
		Settings:    settingsSvc,
		Products:    productSvc,
		Image:       imageTaskSvc,
		OpLog:       opLogSvc,
		Idempotency: idempotencySvc,
		Metrics:     metricCatalog,
	}
	aiProductImageH := &aiproductimage.Handler{Svc: aiProductImageSvc}

	promptH := &aiprompt.Handler{Svc: promptSvc}
	aiTaskH := &aitask.Handler{Svc: aiTaskSvc}

	collectSvc := &collect.Service{
		DB:       dep.DB,
		Products: productSvc,
		Rules:    collectRuleSvc,
		Profiles: profileSvc,
		OpLog:    opLogSvc,
		Client:   collectorClient,
		Redis:    dep.Redis,
	}
	if dep.Config != nil {
		_, tenantSource, tenantErr := dep.Config.ResolveRequestTenantID(0)
		env := config.NormalizeEnv(dep.Config.AppEnv)
		collectSvc.AllowLegacyTenantZero = tenantErr == nil &&
			tenantSource == security.AuthSourceLegacyDevZero &&
			(env == config.EnvDevelopment || env == config.EnvTest)
		collectSvc.QueueName = dep.Config.CollectQueueName
		collectSvc.QueueEnabled = dep.Config.CollectQueueEnabled
		collectSvc.BatchMaxURLs = dep.Config.CollectBatchMaxURLs
		collectSvc.CollectorTimeoutSeconds = dep.Config.CollectorTimeoutSeconds
		collectSvc.AutoRetryEnabled = dep.Config.CollectAutoRetryEnabled
		collectSvc.MaxAutoRetries = dep.Config.CollectMaxRetries
		collectSvc.RetryBaseDelaySec = dep.Config.CollectRetryBaseDelaySeconds
		collectSvc.RetryMaxDelaySec = dep.Config.CollectRetryMaxDelaySeconds
		collectSvc.TaskLeaseTimeoutSeconds = dep.Config.CollectTaskTimeoutSeconds
		collectSvc.Source1688BatchConcurrency = dep.Config.CollectBatchConcurrency1688
		collectSvc.Source1688BatchDelayMinMs = dep.Config.CollectBatchDelayMinMs1688
		collectSvc.Source1688BatchDelayMaxMs = dep.Config.CollectBatchDelayMaxMs1688
		collectSvc.BatchRetryOnBlocked = dep.Config.CollectBatchRetryOnBlocked
		collectSvc.BatchRetryOnTimeout = dep.Config.CollectBatchRetryOnTimeout
		collectSvc.Source1688BatchMaxRetries = dep.Config.CollectBatchMaxRetries1688
		collectSvc.Settings = settingsSvc
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
	if dep.Config != nil {
		shopSvc.AppEnv = dep.Config.AppEnv
	}
	productSvc.Shops = shopSvc
	platformtiktok.BindShops(shopSvc.TikTokShopsBridge())
	platformtiktok.BindPublishImages(newTikTokListingImageFetcher(settingsSvc))
	platformdouyin.BindShops(shopSvc.DouyinShopsBridge())
	platformdouyin.RegisterProvider()
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

	storagePublicSvc := &storagepublic.Service{Settings: settingsSvc, OpLog: opLogSvc}
	storagePublicH := &storagepublic.Handler{Svc: storagePublicSvc, OpLog: opLogSvc, DB: dep.DB}

	douyinPreflightSvc := &douyinpreflight.Service{
		DB:       dep.DB,
		Settings: settingsSvc,
		Shops:    shopSvc,
		Storage:  storagePublicSvc,
	}
	douyinPreflightH := &douyinpreflight.Handler{Svc: douyinPreflightSvc, OpLog: opLogSvc}
	douyinRuntimeSvc := &douyinruntime.Service{
		DB:        dep.DB,
		Settings:  settingsSvc,
		Preflight: douyinPreflightSvc,
		OpLog:     opLogSvc,
	}
	douyinRuntimeH := &douyinruntime.Handler{Svc: douyinRuntimeSvc}

	warehouseSvc := &warehouse.Service{DB: dep.DB}
	inventorySvc := &inventory.Service{
		DB:          dep.DB,
		Redis:       dep.Redis,
		Shops:       shopSvc,
		Settings:    settingsSvc,
		OpLog:       opLogSvc,
		Idempotency: idempotencySvc,
		Metrics:     metricCatalog,
		Warehouses:  warehouseSvc,
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
	warehouseH := &warehouse.Handler{Svc: warehouseSvc, OpLog: opLogSvc}
	supplierSvc := &supplier.Service{DB: dep.DB}
	supplierH := &supplier.Handler{Svc: supplierSvc, OpLog: opLogSvc}
	procurementSvc := &procurement.Service{
		DB: dep.DB, Warehouses: warehouseSvc, Suppliers: supplierSvc, Stock: inventory.WarehouseStockService{},
	}
	procurementH := &procurement.Handler{Svc: procurementSvc, OpLog: opLogSvc}

	orderSvc := &order.Service{DB: dep.DB, OpLog: opLogSvc, Shops: shopSvc, Settings: settingsSvc, Idempotency: idempotencySvc, Warehouses: warehouseSvc}
	orderH := &order.Handler{Svc: orderSvc, Inv: inventorySvc}

	orderSyncSvc := &ordersync.Service{
		DB:          dep.DB,
		Redis:       dep.Redis,
		Shops:       shopSvc,
		Orders:      orderSvc,
		Inventory:   inventorySvc,
		OpLog:       opLogSvc,
		Idempotency: idempotencySvc,
		Metrics:     metricCatalog,
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
		DB:          dep.DB,
		Redis:       dep.Redis,
		Settings:    settingsSvc,
		Prompts:     promptSvc,
		AITasks:     aiTaskSvc,
		AIGateway:   aiGateway,
		OpLog:       opLogSvc,
		Orders:      orderSvc,
		Shops:       shopSvc,
		Idempotency: idempotencySvc,
		AutoReplyQueueName: func() string {
			if dep.Config != nil {
				return strings.TrimSpace(dep.Config.CustomerAutoReplyQueueName)
			}
			return "customer:auto:reply:tasks"
		}(),
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
	customerChatSvc.AutoReplyDependenciesAvailable = func() bool {
		return customersync.CustomerMessageSyncWorkersRunning() && customersync.AutoReplyPollingSchedulerRunning()
	}
	customerSyncSvc.QueueEnabled = dep.Redis != nil && dep.Redis.Client != nil
	if dep.Config != nil {
		_, tenantSource, tenantErr := dep.Config.ResolveRequestTenantID(0)
		env := config.NormalizeEnv(dep.Config.AppEnv)
		allowLegacyTenantZero := tenantErr == nil && tenantSource == security.AuthSourceLegacyDevZero &&
			(env == config.EnvDevelopment || env == config.EnvTest)
		customerChatSvc.AllowLegacyTenantZero = allowLegacyTenantZero
		customerSyncSvc.AllowLegacyTenantZero = allowLegacyTenantZero
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
	productSvc.Readiness = func(ctx context.Context, req product.OperationReadinessRequest) (*product.OperationReadinessResult, error) {
		res, err := readinessSvc.CheckProductReadiness(ctx, productcheck.CheckProductReadinessRequest{
			ProductID: req.ProductID,
			Platform:  req.Platform,
			Mode:      req.Mode,
		})
		if err != nil {
			return nil, err
		}
		out := &product.OperationReadinessResult{
			Status:       res.Status,
			Result:       res.Result,
			CanPublish:   res.CanPublish,
			ErrorCount:   res.ErrorCount,
			WarningCount: res.WarningCount,
			Checks:       make([]product.OperationReadinessCheck, 0, len(res.Checks)),
		}
		for _, c := range res.Checks {
			out.Checks = append(out.Checks, product.OperationReadinessCheck{
				Group:      c.Group,
				Code:       c.Code,
				Level:      c.Level,
				Message:    c.Message,
				Suggestion: c.Suggestion,
			})
		}
		return out, nil
	}

	productPublishSvc := &productpublish.Service{
		DB:          dep.DB,
		Redis:       dep.Redis,
		Shops:       shopSvc,
		Settings:    settingsSvc,
		OpLog:       opLogSvc,
		Readiness:   readinessSvc,
		Idempotency: idempotencySvc,
	}
	if dep.Config != nil {
		productPublishSvc.Environment = dep.Config.AppEnv
	}
	writeControlSvc := &productioncontrol.Service{DB: dep.DB, Config: dep.Config, ProviderWriteGuard: func(ctx context.Context, shopID uuid.UUID) error {
		if guardErr := platformdouyin.GuardWorkerWithShop(ctx, shopID.String(), platformdouyin.FeatureProductDraft, true, false); guardErr != nil {
			return guardErr
		}
		return nil
	}}
	productPublishSvc.WriteControl = writeControlSvc
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
		productPublishSvc.BatchMaxProducts = dep.Config.PublishBatchMaxProducts
		productPublishSvc.BatchMaxTargets = dep.Config.PublishBatchMaxTargets
		productPublishSvc.BatchMaxTasks = dep.Config.PublishBatchMaxTasks
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
	v1.POST("/auth/refresh", sessionH.Refresh)

	authed := v1.Group("")
	authed.Use(middleware.BearerAuthWithDB(dep.Config, dep.DB, sessionSvc))
	authed.GET("/auth/profile", authH.Profile)
	authed.POST("/auth/logout", authH.Logout)
	authed.GET("/auth/sessions", sessionH.ListSessions)
	authed.DELETE("/auth/sessions/:id", sessionH.DeleteSession)
	authed.POST("/auth/sessions/revoke-others", sessionH.RevokeOthers)
	authed.POST("/auth/logout-all", sessionH.LogoutAll)
	authed.GET("/settings", setH.List)
	authed.PUT("/settings", setH.Put)
	authed.GET("/settings/integration-schemas", setH.IntegrationSchemas)
	authed.GET("/settings/integrations/overview", setH.IntegrationOverview)
	authed.POST("/settings/test-ai", setH.TestAI)
	authed.POST("/settings/test-image", imageTaskH.TestImage)
	authed.POST("/settings/test-ocr", imageTaskH.TestOCR)
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
	aiproducttext.Register(authed, aiProductTextH)
	aiproductimage.Register(authed, aiProductImageH)
	pricingSvc := &pricing.Service{DB: dep.DB, Settings: settingsSvc, OpLog: opLogSvc}
	pricingH := &pricing.Handler{Svc: pricingSvc}
	pricing.Register(authed, pricingH)
	collect.Register(authed, collectH)
	collectRuleAISvc := &collectruleai.Service{
		Settings:    settingsSvc,
		Prompts:     promptSvc,
		AIGateway:   aiGateway,
		Analyzer:    collectruleai.NewPageAnalyzer(collectorClient),
		Runner:      collectRunnerAdapter{c: collectorClient},
		Profiles:    profileSvc,
		Providers:   collectruleai.NewProviderResolver(collectSvc),
		Rules:       collectRuleCreatorAdapter{svc: collectRuleSvc},
		OpLog:       opLogSvc,
		TestTimeout: collectorTimeout,
	}
	collectRuleAIH := &collectruleai.Handler{Svc: collectRuleAISvc}
	collectruleai.Register(authed, collectRuleAIH)
	collectrule.Register(authed, collectRuleH)
	collectbrowserprofile.Register(authed, profileH)

	// 1688 采集浏览器登录态（与 /api/v1/collector/... 等价，便于前端与文档引用）
	collectorAlias := r.Group("/api/collector")
	collectorAlias.Use(middleware.BearerAuthWithDB(dep.Config, dep.DB, sessionSvc))
	collectorAlias.GET("/providers/1688/auth-status", collectH.Get1688AuthStatus)
	collectorAlias.POST("/providers/1688/open-login-browser", collectH.Open1688LoginBrowser)
	collectorAlias.GET("/providers/pinduoduo/auth-status", collectH.GetPinduoduoAuthStatus)
	collectorAlias.POST("/providers/pinduoduo/check-login", collectH.CheckPinduoduoLogin)
	collectorAlias.POST("/providers/pinduoduo/open-login-browser", collectH.OpenPinduoduoLoginBrowser)
	collectorAlias.POST("/providers/taobao_tmall/check-login", collectH.CheckTaobaoTmallLogin)
	collectorAlias.POST("/providers/taobao_tmall/open-login-browser", collectH.OpenTaobaoTmallLoginBrowser)
	productcheck.Register(authed, readinessH)
	order.Register(authed, orderH)
	skuCandH := &skucandidate.Handler{Svc: &skucandidate.Service{DB: dep.DB}}
	skucandidate.Register(authed, skuCandH)
	orderexception.Register(authed, excH)
	ordersync.Register(authed, orderSyncH)
	customersync.Register(authed, customerSyncH)
	customerchat.Register(authed, customerChatH)
	shop.RegisterPublic(v1, shopH)
	webhookRegistry := webhook.NewRegistry(dep.Config)
	// Register Douyin webhook signature verifier — loads app_secret from settings
	// (platform_douyin_shop group). If secret is missing the verifier is still
	// registered but Verify returns CodeVerifierNotConfigured.
	if settingsSvc != nil {
		appEnv := ""
		if dep.Config != nil {
			appEnv = dep.Config.AppEnv
		}
		if plain, err := settingsSvc.PlainByGroup(context.Background(), 0, "platform_douyin_shop"); err == nil {
			appSecret := plain["app_secret"]
			webhookRegistry.Register("douyin_shop", webhook.NewDouyinVerifierWithEnv(appSecret, appEnv))
			webhookRegistry.Register("douyin", webhook.NewDouyinVerifierWithEnv(appSecret, appEnv))
		} else {
			webhookRegistry.Register("douyin_shop", webhook.NewDouyinVerifierWithEnv("", appEnv))
			webhookRegistry.Register("douyin", webhook.NewDouyinVerifierWithEnv("", appEnv))
		}
	}
	webhookSvc := &webhook.Service{
		DB:          dep.DB,
		Idempotency: idempotencySvc,
		Verifiers:   webhookRegistry,
		Metrics:     metricCatalog,
		ShopResolver: &webhook.DBWebhookShopResolver{
			DB: dep.DB,
			AppEnv: func() string {
				if dep.Config != nil {
					return dep.Config.AppEnv
				}
				return ""
			}(),
		},
		OrderHandler: &ordersync.DouyinOrderWebhookHandler{
			DB:     dep.DB,
			Shops:  shopSvc,
			Orders: orderSvc,
		},
		AppEnv: "",
	}
	if dep.Config != nil {
		webhookSvc.MaxPayloadBytes = dep.Config.WebhookMaxBodyBytes()
		webhookSvc.MaxClockSkew = dep.Config.WebhookMaxClockSkew()
		webhookSvc.AppEnv = dep.Config.AppEnv
	}
	webhookH := &webhook.Handler{Svc: webhookSvc}
	webhook.Register(authed, webhookH)
	webhook.RegisterPublic(v1, webhookH)
	shop.Register(authed, shopH)
	storagepublic.Register(authed, storagePublicH)
	douyinpreflight.Register(authed, douyinPreflightH)
	douyinruntime.Register(authed, douyinRuntimeH)
	productpublish.Register(authed, productPublishH)
	inventory.Register(authed, inventoryH)
	warehouse.Register(authed, warehouseH)
	supplier.Register(authed, supplierH)
	procurement.Register(authed, procurementH)
	workerH := &worker.Handler{DB: dep.DB, Cfg: dep.Config}
	worker.Register(authed, workerH)

	operationTaskSvc := operationtask.NewAPIService(dep.DB)
	if dep.Config != nil {
		operationTaskSvc.Executor.AppEnv = dep.Config.AppEnv
	}
	resultSvc := &operationtask.ProductionResultService{DB: dep.DB}
	operationTaskSvc.SnapshotBuilder = productPublishSvc
	operationTaskSvc.Production = &operationtask.ProductionExecutionService{
		DB: dep.DB, Authorizer: operationTaskSvc.Authorizer, RetryAuth: operationTaskSvc.Authorizer,
		WriteGuard: writeControlSvc, Factory: productPublishSvc,
	}
	productPublishSvc.OperationResults = resultSvc
	productPublishSvc.ProductionOutbox = &operationtask.OutboxDispatcher{DB: dep.DB, Delivery: productPublishSvc}
	operationTaskH := &operationtask.Handler{Svc: operationTaskSvc}
	operationtask.Register(authed, operationTaskH)
	inventorySyncH := &inventorysync.Handler{Svc: inventorysync.NewAPIService(dep.DB)}
	inventorysync.Register(authed, inventorySyncH)

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
		AIProductText:  aiProductTextSvc,
	}
	tcH := &taskcenter.Handler{Svc: tcSvc}
	taskcenter.Register(authed, tcH)

	configStatusSvc := &configstatus.Service{
		DB:       dep.DB,
		Settings: settingsSvc,
		Redis:    dep.Redis,
		Config:   dep.Config,
		Shops:    shopSvc,
	}
	configStatusH := &configstatus.Handler{Svc: configStatusSvc}
	configstatus.Register(authed, configStatusH)

	dashSvc := &operationdashboard.Service{
		DB:              dep.DB,
		Inventory:       inventorySvc,
		TaskCenter:      tcSvc,
		OrderExceptions: excSvc,
		ConfigStatus:    configStatusSvc,
	}
	dashH := &operationdashboard.Handler{Svc: dashSvc}
	operationdashboard.Register(authed, dashH)

	aiOpsWorkbenchSvc := &aiopsworkbench.Service{
		DB:           dep.DB,
		ProductCheck: readinessSvc,
		TaskCenter:   tcSvc,
	}
	aiOpsWorkbenchH := &aiopsworkbench.Handler{Svc: aiOpsWorkbenchSvc}
	aiopsworkbench.Register(authed, aiOpsWorkbenchH)

	adminUserSvc := &adminuser.Service{DB: dep.DB, OpLog: opLogSvc}
	adminUserH := &adminuser.Handler{Svc: adminUserSvc}
	adminuser.Register(authed, adminUserH)

	secSvc := &securitymod.Service{DB: dep.DB, Cfg: dep.Config, OpLogs: opLogSvc, Metrics: metricCatalog}
	secH := &securitymod.Handler{Svc: secSvc, DB: dep.DB}
	securitymod.RegisterRoutes(authed, secH)

	exportSvc := &exportmod.Service{DB: dep.DB}
	exportH := &exportmod.Handler{Svc: exportSvc}
	exportmod.RegisterRoutes(authed, exportH)

	alertSvc := alerting.NewService(dep.DB, alertCooldown, alertRecovery)
	obsH := &observabilitymod.Handler{DB: dep.DB, Cfg: dep.Config, Obs: dep.Obs, Alert: alertSvc, OpLog: opLogSvc}
	observabilitymod.Register(authed, obsH)

	if dep.Config != nil && dep.Config.EnableDemoSeed && !config.IsProduction(dep.Config.AppEnv) {
		demoSeedSvc := &demoseed.Service{DB: dep.DB, OpLog: opLogSvc, AppEnv: dep.Config.AppEnv}
		demoSeedH := &demoseed.Handler{Svc: demoSeedSvc}
		demoseed.Register(authed, demoSeedH)
	}

	return collectSvc, imageTaskSvc, orderSyncSvc, customerSyncSvc, productPublishSvc, inventorySvc, tcSvc, douyinRuntimeSvc, webhookSvc, fileSvc, secSvc
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

		cmQEnabled := dep.Redis != nil && dep.Redis.Client != nil
		cmQName := "customer:message:sync:tasks"
		cmWConc := 1
		if dep.Config != nil {
			if strings.TrimSpace(dep.Config.CustomerMessageSyncQueueName) != "" {
				cmQName = strings.TrimSpace(dep.Config.CustomerMessageSyncQueueName)
			}
			cmWConc = dep.Config.CustomerMessageSyncWorkerConcurrency
			if cmWConc < 1 {
				cmWConc = 1
			}
		}
		cmq := customersync.BuildCustomerMessageSyncQueueHealthBlock(ctx, dep.Redis, cmQEnabled, cmQName, cmWConc)
		if cmQEnabled && (!cmq.RedisAvailable || !cmq.WorkerRunning) && checks["redis"] == "ok" {
			status = "degraded"
		}

		autoReplyQueueName := "customer:auto:reply:tasks"
		autoReplyWorkerConcurrency := 1
		if dep.Config != nil {
			if strings.TrimSpace(dep.Config.CustomerAutoReplyQueueName) != "" {
				autoReplyQueueName = strings.TrimSpace(dep.Config.CustomerAutoReplyQueueName)
			}
			autoReplyWorkerConcurrency = dep.Config.CustomerAutoReplyWorkerConcurrency
		}
		autoReplyQueue := customerchat.BuildAutoReplyQueueHealthBlock(ctx, dep.Redis, autoReplyQueueName, autoReplyWorkerConcurrency)
		autoReplyQueue.MessageSyncWorkerRunning = customersync.CustomerMessageSyncWorkersRunning()
		autoReplyQueue.SchedulerRunning = customersync.AutoReplyPollingSchedulerRunning()
		if (!autoReplyQueue.RedisAvailable || !autoReplyQueue.WorkerRunning || !autoReplyQueue.MessageSyncWorkerRunning || !autoReplyQueue.SchedulerRunning) && checks["redis"] == "ok" {
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
			"customerAutoReplyQueue":   autoReplyQueue,
			"productPublishQueue":      ppq,
			"inventorySyncQueue":       invq,
			"workers":                  workers,
			"timestamp":                time.Now().UTC().Format(time.RFC3339),
		})
	}
}
