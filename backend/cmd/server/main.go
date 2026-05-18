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
	"github.com/joho/godotenv"
	"github.com/trademind-ai/trademind/backend/internal/api"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/database"
	"github.com/trademind-ai/trademind/backend/internal/encrypt"
	"github.com/trademind-ai/trademind/backend/internal/logger"
	"github.com/trademind-ai/trademind/backend/internal/middleware"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiprompt"
	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"github.com/trademind-ai/trademind/backend/internal/modules/customersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskreaper"
	"github.com/trademind-ai/trademind/backend/internal/modules/worker"
	"github.com/trademind-ai/trademind/backend/internal/rdb"
)

func loadDotEnv() {
	paths := []string{".env", "../.env", "../../.env"}
	for _, p := range paths {
		if err := godotenv.Load(p); err == nil {
			return
		}
	}
}

func main() {
	loadDotEnv()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config_load_failed", "error", err)
		os.Exit(1)
	}

	log := logger.Init(cfg.AppEnv)
	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	db, err := database.Open(cfg)
	if err != nil {
		log.Error("database_init_failed", "error", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close(db) }()

	if err := database.AutoMigrate(db); err != nil {
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

	anSeedCtx, anSeedCancel := context.WithTimeout(context.Background(), 15*time.Second)
	if err := settings.EnsureAlertNotifyDefaults(anSeedCtx, db); err != nil {
		anSeedCancel()
		log.Error("alert_notify_settings_seed_failed", "error", err)
		os.Exit(1)
	}
	anSeedCancel()

	bootCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := admin.EnsureBootstrapAdmin(bootCtx, db, cfg, log); err != nil {
		cancel()
		log.Error("admin_bootstrap_failed", "error", err)
		os.Exit(1)
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
	engine.MaxMultipartMemory = cfg.MaxUploadBytes()
	engine.Use(middleware.RequestID(), middleware.Recovery(log), middleware.AccessLog(log))

	opLogSvc := &operationlog.Service{DB: db}
	collectSvc, imageTaskSvc, orderSyncSvc, customerSyncSvc, productPublishSvc, inventorySyncSvc, tcSvc := api.Register(engine, &api.Deps{
		Config:    cfg,
		DB:        db,
		Redis:     redisClient,
		Encrypter: enc,
		OpLog:     opLogSvc,
	})

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

	if cfg.CustomerMessageSyncQueueEnabled && redisClient != nil && customerSyncSvc != nil {
		qn := strings.TrimSpace(cfg.CustomerMessageSyncQueueName)
		if qn == "" {
			qn = "customer:message:sync:tasks"
		}
		customersync.StartWorker(workerCtx, &workerWG, log, customerSyncSvc, qn, cmWorkerConc, workerReg)
		log.Info("customer_message_sync_worker_started", "concurrency", cmWorkerConc, "queue", qn)
	} else if cfg.CustomerMessageSyncQueueEnabled && redisClient == nil {
		log.Warn("customer_message_sync_worker_skipped", "reason", "redis unavailable while CUSTOMER_MESSAGE_SYNC_QUEUE_ENABLED=true")
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
