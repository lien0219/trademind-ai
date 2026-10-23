package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/api"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/database"
	"github.com/trademind-ai/trademind/backend/internal/encrypt"
	"github.com/trademind-ai/trademind/backend/internal/logger"
	"github.com/trademind-ai/trademind/backend/internal/middleware"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiprompt"
	"github.com/trademind-ai/trademind/backend/internal/modules/alerting"
	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"github.com/trademind-ai/trademind/backend/internal/modules/customerchat"
	"github.com/trademind-ai/trademind/backend/internal/modules/customersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/douyinruntime"
	"github.com/trademind-ai/trademind/backend/internal/modules/files"
	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/observabilitymod"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/securitymod"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskreaper"
	"github.com/trademind-ai/trademind/backend/internal/modules/webhook"
	"github.com/trademind-ai/trademind/backend/internal/modules/worker"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/logging"
	"github.com/trademind-ai/trademind/backend/internal/pkg/metrics"
	"github.com/trademind-ai/trademind/backend/internal/pkg/observability"
	"github.com/trademind-ai/trademind/backend/internal/pkg/runtimediag"
	securitypkg "github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"github.com/trademind-ai/trademind/backend/internal/pkg/tracing"
	"github.com/trademind-ai/trademind/backend/internal/rdb"
)

func main() {
	if err := loadDotEnv(); err != nil {
		slog.Error("env_load_failed", "error", err)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config_load_failed", "error", err)
		os.Exit(1)
	}

	log := logger.Init(cfg.AppEnv)
	obsCfg := cfg.Observability
	obs, err := observability.Init(observability.Config{
		Enabled:         obsCfg.Enabled,
		Mode:            obsCfg.Mode,
		Environment:     obsCfg.Environment,
		MetricsEnabled:  obsCfg.MetricsEnabled,
		MetricsPath:     obsCfg.MetricsPath,
		MetricsInternal: obsCfg.MetricsInternalOnly,
		TracingEnabled:  obsCfg.TracingEnabled,
		AlertingEnabled: obsCfg.AlertingEnabled,
		Logger: logging.Config{
			Format:         obsCfg.LogFormat,
			Level:          obsCfg.LogLevel,
			IncludeSource:  obsCfg.LogIncludeSource,
			MaxFieldLength: obsCfg.LogMaxFieldLength,
			Service:        obsCfg.OTELServiceName,
			Version:        obsCfg.OTELServiceVersion,
			Environment:    obsCfg.Environment,
			FailSafe:       true,
		},
		Tracer: tracing.Config{
			Enabled:       obsCfg.TracingEnabled,
			ServiceName:   obsCfg.OTELServiceName,
			Version:       obsCfg.OTELServiceVersion,
			Environment:   obsCfg.Environment,
			SampleRatio:   obsCfg.OTELTraceSampleRatio,
			ExportStdout:  cfg.AppEnv == "development" && obsCfg.TracingEnabled,
			OTLPEndpoint:  obsCfg.OTELExporterOTLPEndpoint,
			OTLPProtocol:  obsCfg.OTELExporterOTLPProtocol,
			OTLPHeaders:   obsCfg.OTELExporterOTLPHeaders,
			ExportTimeout: obsCfg.ExportTimeout(),
			QueueSize:     obsCfg.OTELExportQueueSize,
			BatchSize:     obsCfg.OTELExportBatchSize,
			RetryMax:      obsCfg.OTELExportRetryMax,
		},
	})
	if err != nil {
		log.Error("observability_init_failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		shCtx, shCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shCancel()
		_ = obs.Shutdown(shCtx)
	}()

	log.Info("config_loaded", "summary", cfg.RedactedSummary().String())
	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	db, err := database.Open(cfg)
	if err != nil {
		log.Error("database_init_failed", "error", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close(db) }()
	if sqlDB, sqlErr := db.DB(); sqlErr == nil {
		runtimediag.BindSamplingDB(sqlDB)
	}
	defer func() {
		shCtx, shCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer shCancel()
		runtimediag.Shutdown(shCtx)
	}()

	migrateCtx, migrateCancel := context.WithTimeout(context.Background(), time.Duration(cfg.MigrationLockTimeoutSeconds)*time.Second)
	defer migrateCancel()
	if cfg.MigrationRunOnStartup {
		if err := database.RunMigrateWithLock(migrateCtx, db, time.Duration(cfg.MigrationLockTimeoutSeconds)*time.Second, database.AutoMigrateProductionSchema); err != nil {
			log.Error("database_migrate_failed", "error", err)
			os.Exit(1)
		}
	} else if err := database.AutoMigrateProductionSchema(db); err != nil {
		log.Error("database_migrate_failed", "error", err)
		os.Exit(1)
	}

	seedCtx, seedCancel := context.WithTimeout(context.Background(), 15*time.Second)
	if err := aiprompt.EnsureDefaults(seedCtx, db); err != nil {
		seedCancel()
		log.Error("ai_prompt_seed_failed", "error", err)
		os.Exit(1)
	}
	seedCancel()

	enc, err := encrypt.NewService(cfg.MasterKey)
	if err != nil {
		log.Error("encrypt_init_failed", "error", err)
		os.Exit(1)
	}

	alertSeedCtx, alertSeedCancel := context.WithTimeout(context.Background(), 15*time.Second)
	if err := alerting.EnsureDefaultRules(alertSeedCtx, db); err != nil {
		alertSeedCancel()
		log.Error("alert_rules_seed_failed", "error", err)
		os.Exit(1)
	}
	alertSeedCancel()

	sloSeedCtx, sloSeedCancel := context.WithTimeout(context.Background(), 15*time.Second)
	if err := observabilitymod.EnsureDefaultSLOs(sloSeedCtx, db); err != nil {
		sloSeedCancel()
		log.Error("slo_seed_failed", "error", err)
		os.Exit(1)
	}
	sloSeedCancel()

	imgSeedCtx, imgSeedCancel := context.WithTimeout(context.Background(), 15*time.Second)
	if err := settings.EnsureImageDefaults(imgSeedCtx, db, enc); err != nil {
		imgSeedCancel()
		log.Error("image_settings_seed_failed", "error", err)
		os.Exit(1)
	}
	imgSeedCancel()

	aiBatchSeedCtx, aiBatchSeedCancel := context.WithTimeout(context.Background(), 15*time.Second)
	if err := settings.EnsureAIBatchDefaults(aiBatchSeedCtx, db); err != nil {
		aiBatchSeedCancel()
		log.Error("ai_batch_settings_seed_failed", "error", err)
		os.Exit(1)
	}
	aiBatchSeedCancel()

	aiProvSeedCtx, aiProvSeedCancel := context.WithTimeout(context.Background(), 15*time.Second)
	if err := settings.EnsureAIProviderDefaults(aiProvSeedCtx, db, enc); err != nil {
		aiProvSeedCancel()
		log.Error("ai_provider_settings_seed_failed", "error", err)
		os.Exit(1)
	}
	aiProvSeedCancel()

	stSeedCtx, stSeedCancel := context.WithTimeout(context.Background(), 15*time.Second)
	if err := settings.EnsureStorageDefaults(stSeedCtx, db); err != nil {
		stSeedCancel()
		log.Error("storage_settings_seed_failed", "error", err)
		os.Exit(1)
	}
	stSeedCancel()

	invSeedCtx, invSeedCancel := context.WithTimeout(context.Background(), 15*time.Second)
	if err := settings.EnsureInventoryDefaults(invSeedCtx, db); err != nil {
		invSeedCancel()
		log.Error("inventory_settings_seed_failed", "error", err)
		os.Exit(1)
	}
	invSeedCancel()

	tcSeedCtx, tcSeedCancel := context.WithTimeout(context.Background(), 15*time.Second)
	if err := settings.EnsureTaskcenterDefaults(tcSeedCtx, db); err != nil {
		tcSeedCancel()
		log.Error("taskcenter_settings_seed_failed", "error", err)
		os.Exit(1)
	}
	tcSeedCancel()

	dySeedCtx, dySeedCancel := context.WithTimeout(context.Background(), 15*time.Second)
	if err := settings.EnsureDouyinAlertDefaults(dySeedCtx, db); err != nil {
		dySeedCancel()
		log.Error("douyin_alert_settings_seed_failed", "error", err)
		os.Exit(1)
	}
	dySeedCancel()

	anSeedCtx, anSeedCancel := context.WithTimeout(context.Background(), 15*time.Second)
	if err := settings.EnsureAlertNotifyDefaults(anSeedCtx, db); err != nil {
		anSeedCancel()
		log.Error("alert_notify_settings_seed_failed", "error", err)
		os.Exit(1)
	}
	anSeedCancel()

	pricingSeedCtx, pricingSeedCancel := context.WithTimeout(context.Background(), 15*time.Second)
	if err := settings.EnsurePricingDefaults(pricingSeedCtx, db); err != nil {
		pricingSeedCancel()
		log.Error("pricing_settings_seed_failed", "error", err)
		os.Exit(1)
	}
	pricingSeedCancel()

	opLogSvc := &operationlog.Service{DB: db}
	bootCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := admin.EnsureBootstrapAdmin(bootCtx, db, cfg, log, opLogSvc); err != nil {
		cancel()
		log.Error("admin_bootstrap_failed", "error", err)
		os.Exit(1)
	}
	if _, err := admin.EnsurePerformanceBootstrap(bootCtx, db, cfg, log, opLogSvc); err != nil {
		cancel()
		log.Error("performance_bootstrap_failed", "error", err)
		os.Exit(1)
	}
	if cfg.AppEnv == config.EnvPerformance && cfg.RuntimeLimits.PerformanceTestMode {
		if ids, err := admin.PerformanceBootstrapUserIDs(bootCtx, db); err == nil {
			for _, id := range ids {
				adminperm.InvalidateUserPermissionCache(id)
			}
		}
	}
	cancel()

	var redisClient *rdb.Client
	if rcl, err := rdb.Open(cfg); err != nil {
		log.Warn("redis_unavailable", "error", err)
	} else {
		redisClient = rcl
		defer func() { _ = redisClient.Close() }()
	}

	engine := gin.New()
	if err := engine.SetTrustedProxies(cfg.Observability.HTTPTrustedProxyCIDRs); err != nil {
		log.Error("trusted_proxy_config_invalid", "error", err)
		os.Exit(1)
	}
	engine.MaxMultipartMemory = cfg.MaxUploadBytes()
	engine.Use(
		middleware.CORS(cfg),
		middleware.RequestID(),
		middleware.ContextCorrelation(),
		middleware.ObservabilityHTTP(obs),
		middleware.RateLimit(cfg),
		middleware.Recovery(log),
		middleware.AccessLog(log),
		securitypkg.SecurityHeaders(cfg),
		securitypkg.CSRFProtection(cfg),
	)

	collectSvc, imageTaskSvc, orderSyncSvc, customerSyncSvc, productPublishSvc, inventorySyncSvc, tcSvc, douyinRuntimeSvc, webhookSvc, fileSvc, secSvc := api.Register(engine, &api.Deps{
		Config:          cfg,
		DB:              db,
		Redis:           redisClient,
		Encrypter:       enc,
		OpLog:           opLogSvc,
		MigrationsReady: true,
		Obs:             obs,
	})
	api.RegisterProductionRoutes(engine, &api.Deps{Config: cfg, DB: db, Obs: obs})

	workerReg := worker.NewRegistryFromConfig(db, opLogSvc, cfg, log)

	workerConc := cfg.CollectWorkerConcurrency
	if workerConc < 1 {
		workerConc = 2
	}
	collect.ConfigureWorkerMonitor(cfg.CollectQueueEnabled, workerConc)

	imgWorkerConc := cfg.ImageWorkerConcurrency
	if imgWorkerConc < 1 {
		imgWorkerConc = 2
	}
	imagetask.ConfigureImageWorkerMonitor(cfg.ImageQueueEnabled, imgWorkerConc)

	osWorkerConc := cfg.OrderSyncWorkerConcurrency
	if osWorkerConc < 1 {
		osWorkerConc = 1
	}

	cmWorkerConc := cfg.CustomerMessageSyncWorkerConcurrency
	if cmWorkerConc < 1 {
		cmWorkerConc = 1
	}

	workerCtx, workerCancel := context.WithCancel(context.Background())
	var workerWG sync.WaitGroup

	if sqlDB, err := db.DB(); err == nil {
		observability.StartDBStatsCollector(workerCtx, &workerWG, log, sqlDB, obs.Catalog, "primary", 15*time.Second)
	} else {
		log.Warn("db_stats_collector_skipped", "error", err)
	}

	worker.StartStaleMarker(workerCtx, &workerWG, db, cfg, log)

	taskreaper.Start(workerCtx, &workerWG, taskreaper.Deps{
		Log:             log,
		DB:              db,
		Config:          cfg,
		Collect:         collectSvc,
		Image:           imageTaskSvc,
		Order:           orderSyncSvc,
		CustomerMessage: customerSyncSvc,
		ProductPublish:  productPublishSvc,
		InventorySync:   inventorySyncSvc,
	})

	taskcenter.StartAlertScanWorker(workerCtx, &workerWG, log, tcSvc, workerReg, cfg)
	douyinruntime.StartDouyinAlertScanWorker(workerCtx, &workerWG, log, douyinRuntimeSvc, workerReg, cfg)
	metricSamples := func() metrics.Snapshot {
		if obs == nil || obs.Metrics == nil {
			return metrics.Snapshot{TakenAt: time.Now().UTC(), Families: map[string]metrics.FamilySnapshot{}}
		}
		return obs.Metrics.SnapshotNow()
	}
	if cfg.Observability.AlertingEnabled {
		alertSvc := alerting.NewService(db, time.Duration(cfg.Observability.AlertDefaultCooldownSecs)*time.Second, cfg.Observability.AlertRecoveryEnabled)
		alerting.StartEvaluatorWorker(workerCtx, &workerWG, log, alertSvc, time.Minute, metricSamples)
		alerting.StartDeliveryWorker(workerCtx, &workerWG, log, alertSvc, 30*time.Second)
	}
	if cfg.Observability.Enabled && cfg.Observability.MetricsEnabled {
		observabilitymod.StartSLOEvaluatorWorker(workerCtx, &workerWG, log, db, obs.Catalog, time.Minute, metricSamples)
	}
	if webhookSvc != nil {
		webhook.StartWorker(workerCtx, &workerWG, log, webhookSvc, cfg, workerReg)
	}

	if cfg.CollectQueueEnabled && redisClient != nil && collectSvc != nil {
		collect.StartWorker(workerCtx, &workerWG, log, collectSvc, cfg.CollectQueueName, workerConc, workerReg)
		log.Info("collect_worker_started", "concurrency", workerConc, "queue", cfg.CollectQueueName)
		if cfg.CollectAutoRetryEnabled {
			collect.StartRetryScheduler(workerCtx, &workerWG, log, collectSvc, 5*time.Second)
			log.Info("collect_retry_scheduler_started", "interval_sec", 5)
		}
	} else if cfg.CollectQueueEnabled && redisClient == nil {
		log.Warn("collect_worker_skipped", "reason", "redis unavailable while COLLECT_QUEUE_ENABLED=true")
	}

	if cfg.ImageQueueEnabled && redisClient != nil && imageTaskSvc != nil {
		qn := strings.TrimSpace(cfg.ImageQueueName)
		if qn == "" {
			qn = "image:tasks"
		}
		imagetask.StartWorker(workerCtx, &workerWG, log, imageTaskSvc, qn, imgWorkerConc, workerReg)
		log.Info("image_task_worker_started", "concurrency", imgWorkerConc, "queue", qn)
		if cfg.ImageAutoRetryEnabled {
			imagetask.StartImageRetryScheduler(workerCtx, &workerWG, log, imageTaskSvc, 5*time.Second)
			log.Info("image_retry_scheduler_started", "interval_sec", 5)
		}
	} else if cfg.ImageQueueEnabled && redisClient == nil {
		log.Warn("image_task_worker_skipped", "reason", "redis unavailable while IMAGE_QUEUE_ENABLED=true")
	}

	if cfg.OrderSyncQueueEnabled && redisClient != nil && orderSyncSvc != nil {
		qn := strings.TrimSpace(cfg.OrderSyncQueueName)
		if qn == "" {
			qn = "order:sync:tasks"
		}
		ordersync.StartWorker(workerCtx, &workerWG, log, orderSyncSvc, qn, osWorkerConc, workerReg)
		log.Info("order_sync_worker_started", "concurrency", osWorkerConc, "queue", qn)
	} else if cfg.OrderSyncQueueEnabled && redisClient == nil {
		log.Warn("order_sync_worker_skipped", "reason", "redis unavailable while ORDER_SYNC_QUEUE_ENABLED=true")
	}

	if redisClient != nil && customerSyncSvc != nil {
		qn := strings.TrimSpace(cfg.CustomerMessageSyncQueueName)
		if qn == "" {
			qn = "customer:message:sync:tasks"
		}
		customersync.StartWorker(workerCtx, &workerWG, log, customerSyncSvc, qn, cmWorkerConc, workerReg)
		log.Info("customer_message_sync_worker_started", "concurrency", cmWorkerConc, "queue", qn)
	}
	if redisClient != nil && customerSyncSvc != nil && customerSyncSvc.CustomerChat != nil {
		customerchat.StartAutoReplyWorker(workerCtx, &workerWG, log, customerSyncSvc.CustomerChat, cfg.CustomerAutoReplyWorkerConcurrency)
		log.Info("customer_auto_reply_worker_started", "concurrency", cfg.CustomerAutoReplyWorkerConcurrency, "queue", cfg.CustomerAutoReplyQueueName)
		customersync.StartAutoReplyPollingScheduler(workerCtx, &workerWG, log, customerSyncSvc)
		log.Info("customer_auto_reply_poll_scheduler_started", "settings", "managed_in_admin")
	}

	if cfg.ProductPublishQueueEnabled && redisClient != nil && productPublishSvc != nil {
		ppWorkerConc := cfg.ProductPublishWorkerConcurrency
		if ppWorkerConc < 1 {
			ppWorkerConc = 1
		}
		ppQn := strings.TrimSpace(cfg.ProductPublishQueueName)
		if ppQn == "" {
			ppQn = "product:publish:tasks"
		}
		productpublish.StartWorker(workerCtx, &workerWG, log, productPublishSvc, ppQn, ppWorkerConc, workerReg)
		log.Info("product_publish_worker_started", "concurrency", ppWorkerConc, "queue", ppQn)
		if cfg.ProductionCapabilities.BackgroundWorkerEnabled {
			productpublish.StartProductionOutboxDispatcher(workerCtx, &workerWG, log, productPublishSvc, 2*time.Second)
			log.Info("operation_task_outbox_dispatcher_started", "interval_sec", 2)
		}
	} else if cfg.ProductPublishQueueEnabled && redisClient == nil {
		log.Warn("product_publish_worker_skipped", "reason", "redis unavailable while PRODUCT_PUBLISH_QUEUE_ENABLED=true")
	}

	if cfg.InventorySyncQueueEnabled && redisClient != nil && inventorySyncSvc != nil {
		invWorkerConc := cfg.InventorySyncWorkerConcurrency
		if invWorkerConc < 1 {
			invWorkerConc = 1
		}
		invQn := strings.TrimSpace(cfg.InventorySyncQueueName)
		if invQn == "" {
			invQn = "inventory:sync:tasks"
		}
		inventory.StartWorker(workerCtx, &workerWG, log, inventorySyncSvc, invQn, invWorkerConc, workerReg)
		log.Info("inventory_sync_worker_started", "concurrency", invWorkerConc, "queue", invQn)
	} else if cfg.InventorySyncQueueEnabled && redisClient == nil {
		log.Warn("inventory_sync_worker_skipped", "reason", "redis unavailable while INVENTORY_SYNC_QUEUE_ENABLED=true")
	}

	if redisClient != nil && fileSvc != nil {
		files.StartScanWorker(workerCtx, &workerWG, log, fileSvc, cfg, workerReg)
		log.Info("file_security_scan_worker_started")
	}
	if secSvc != nil {
		securitymod.StartReencryptWorker(workerCtx, &workerWG, log, secSvc, workerReg)
		log.Info("security_secret_reencrypt_worker_started")
	}

	srv := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: engine,
	}

	go func() {
		log.Info("server_listen", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server_exit", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("server_shutdown_begin")
	workerCancel()

	done := make(chan struct{})
	go func() {
		workerWG.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(60 * time.Second):
		log.Warn("collect_worker_shutdown_timeout")
	}

	collect.SetCollectWorkersRunning(false)
	imagetask.SetImageWorkersRunning(false)
	ordersync.SetOrderSyncWorkersRunning(false)
	customersync.SetCustomerMessageSyncWorkersRunning(false)
	productpublish.SetProductPublishWorkersRunning(false)
	inventory.SetInventorySyncWorkersRunning(false)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("server_shutdown_error", "error", err)
	}
	log.Info("server_shutdown_complete")
}
