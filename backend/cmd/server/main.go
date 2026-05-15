package main

import (
	"context"
	"log/slog"
	"os"
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

	api.Register(engine, &api.Deps{
		Config:    cfg,
		DB:        db,
		Redis:     redisClient,
		Encrypter: enc,
	})

	log.Info("server_listen", "addr", cfg.HTTPAddr)
	if err := engine.Run(cfg.HTTPAddr); err != nil {
		log.Error("server_exit", "error", err)
		os.Exit(1)
	}
}
