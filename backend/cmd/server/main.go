package main

import (
	"log/slog"
	"os"

	"github.com/joho/godotenv"
	"github.com/trademind-ai/trademind/backend/internal/api"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/database"
	"github.com/trademind-ai/trademind/backend/internal/logger"
	"github.com/trademind-ai/trademind/backend/internal/middleware"
	"github.com/trademind-ai/trademind/backend/internal/rdb"
	"github.com/gin-gonic/gin"
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

	var redisClient *rdb.Client
	if rcl, err := rdb.Open(cfg); err != nil {
		log.Warn("redis_unavailable", "error", err)
	} else {
		redisClient = rcl
		defer func() { _ = redisClient.Close() }()
	}

	engine := gin.New()
	engine.Use(middleware.RequestID(), middleware.Recovery(log), middleware.AccessLog(log))

	api.Register(engine, &api.Deps{
		Config: cfg,
		DB:     db,
		Redis:  redisClient,
	})

	log.Info("server_listen", "addr", cfg.HTTPAddr)
	if err := engine.Run(cfg.HTTPAddr); err != nil {
		log.Error("server_exit", "error", err)
		os.Exit(1)
	}
}
