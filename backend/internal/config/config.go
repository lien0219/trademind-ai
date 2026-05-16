package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds environment-driven settings for the API server.
type Config struct {
	AppEnv    string
	HTTPAddr  string
	MasterKey string
	DB        DBConfig
	Redis     RedisConfig
	JWTSecret string
	JWTExpHrs int

	// BootstrapAdminEmail / BootstrapAdminPhone / BootstrapAdminPassword seed the first admin when admin_users is empty (at least one contact required).
	BootstrapAdminEmail    string
	BootstrapAdminPhone    string
	BootstrapAdminPassword string

	// UploadMaxMB limits multipart image uploads (default 10 MB).
	UploadMaxMB int

	// CollectorBaseURL is the Node collector HTTP base (e.g. http://127.0.0.1:3100).
	CollectorBaseURL string
	// CollectorTimeoutSeconds caps outbound HTTP calls to the collector (default 60).
	CollectorTimeoutSeconds int

	// CollectQueueEnabled gates async collect jobs (Redis list + worker).
	CollectQueueEnabled bool
	// CollectWorkerConcurrency is the number of concurrent BRPOP consumers.
	CollectWorkerConcurrency int
	// CollectQueueName is the Redis list key for collect job payloads.
	CollectQueueName string
	// CollectBatchMaxURLs limits URLs per POST /collect/batches (default 50).
	CollectBatchMaxURLs int

	// Worker automatic retry (backoff via DB next_retry_at + scheduler LPUSH).
	CollectAutoRetryEnabled      bool
	CollectMaxRetries            int
	CollectRetryBaseDelaySeconds int
	CollectRetryMaxDelaySeconds  int

	// ImageQueueEnabled gates async image_tasks (Redis list + in-process worker).
	ImageQueueEnabled bool
	// ImageWorkerConcurrency is the number of concurrent BRPOP consumers for image tasks.
	ImageWorkerConcurrency int
	// ImageQueueName is the Redis list key for image task payloads (default image:tasks).
	ImageQueueName string
	// ImageTaskTimeoutSeconds caps per-task provider context timeout (0 = use settings image timeout only).
	ImageTaskTimeoutSeconds int

	// Image auto-retry: failed tasks enter retrying + next_retry_at; scheduler LPUSH after delay (requires IMAGE_QUEUE_ENABLED).
	ImageAutoRetryEnabled      bool
	ImageMaxRetries            int
	ImageRetryBaseDelaySeconds int
	ImageRetryMaxDelaySeconds  int

	// OrderSyncQueueEnabled gates async order sync jobs (Redis list + worker).
	OrderSyncQueueEnabled bool
	// OrderSyncQueueName is the Redis list key for order sync payloads (default order:sync:tasks).
	OrderSyncQueueName string
	// OrderSyncWorkerConcurrency is concurrent BRPOP consumers for order sync (default 1).
	OrderSyncWorkerConcurrency int
	// OrderSyncTaskTimeoutSeconds caps each Provider.SyncOrders context (default 120).
	OrderSyncTaskTimeoutSeconds int

	// CollectTaskTimeoutSeconds is the DB lease TTL for collect_tasks (worker reclaim).
	CollectTaskTimeoutSeconds int

	// Worker heartbeat / lease reclaim (multi-instance workers).
	WorkerHeartbeatEnabled            bool
	WorkerHeartbeatIntervalSeconds    int
	WorkerStaleAfterSeconds           int
	WorkerReaperEnabled               bool
	WorkerReaperIntervalSeconds       int
	WorkerLegacyRunningTimeoutSeconds int
}

// DBConfig selects PostgreSQL (default) or MySQL via GORM.
type DBConfig struct {
	Driver   string // postgres | mysql
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	Timezone string
}

// RedisConfig is used for cache and future queues.
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// Load reads configuration from environment variables (after optional .env in main).
func Load() (*Config, error) {
	cfg := &Config{
		AppEnv:    firstNonEmpty(os.Getenv("APP_ENV"), "development"),
		HTTPAddr:  firstNonEmpty(os.Getenv("APP_HTTP_ADDR"), ":8080"),
		MasterKey: os.Getenv("APP_MASTER_KEY"),
		DB: DBConfig{
			Driver:   strings.ToLower(strings.TrimSpace(firstNonEmpty(os.Getenv("DB_DRIVER"), "postgres"))),
			Host:     firstNonEmpty(os.Getenv("DB_HOST"), "127.0.0.1"),
			User:     os.Getenv("DB_USER"),
			Password: os.Getenv("DB_PASSWORD"),
			Name:     os.Getenv("DB_NAME"),
			Timezone: firstNonEmpty(os.Getenv("DB_TIMEZONE"), "UTC"),
		},
		Redis: RedisConfig{
			Addr:     firstNonEmpty(os.Getenv("REDIS_ADDR"), "127.0.0.1:6379"),
			Password: os.Getenv("REDIS_PASSWORD"),
		},
		JWTSecret: firstNonEmpty(os.Getenv("JWT_SECRET"), "change-me-in-development"),
		JWTExpHrs: atoiOrDefault(os.Getenv("JWT_EXPIRE_HOURS"), 168),

		BootstrapAdminEmail:    strings.TrimSpace(os.Getenv("ADMIN_BOOTSTRAP_EMAIL")),
		BootstrapAdminPhone:    strings.TrimSpace(os.Getenv("ADMIN_BOOTSTRAP_PHONE")),
		BootstrapAdminPassword: os.Getenv("ADMIN_BOOTSTRAP_PASSWORD"),

		UploadMaxMB: atoiOrDefault(os.Getenv("UPLOAD_MAX_MB"), 10),

		CollectorBaseURL:        strings.TrimRight(strings.TrimSpace(firstNonEmpty(os.Getenv("COLLECTOR_BASE_URL"), "http://127.0.0.1:3100")), "/"),
		CollectorTimeoutSeconds: atoiOrDefault(os.Getenv("COLLECTOR_TIMEOUT_SECONDS"), 60),

		CollectQueueEnabled:      envBool(os.Getenv("COLLECT_QUEUE_ENABLED"), true),
		CollectWorkerConcurrency: atoiOrDefault(os.Getenv("COLLECT_WORKER_CONCURRENCY"), 2),
		CollectQueueName: strings.TrimSpace(firstNonEmpty(
			os.Getenv("COLLECT_QUEUE_NAME"),
			"collect:tasks",
		)),
		CollectBatchMaxURLs: atoiOrDefault(os.Getenv("COLLECT_BATCH_MAX_URLS"), 50),

		CollectAutoRetryEnabled:      envBool(os.Getenv("COLLECT_AUTO_RETRY_ENABLED"), true),
		CollectMaxRetries:            atoiOrDefault(os.Getenv("COLLECT_MAX_RETRIES"), 3),
		CollectRetryBaseDelaySeconds: atoiOrDefault(os.Getenv("COLLECT_RETRY_BASE_DELAY_SECONDS"), 30),
		CollectRetryMaxDelaySeconds:  atoiOrDefault(os.Getenv("COLLECT_RETRY_MAX_DELAY_SECONDS"), 600),

		ImageQueueEnabled:      envBool(os.Getenv("IMAGE_QUEUE_ENABLED"), true),
		ImageWorkerConcurrency: atoiOrDefault(os.Getenv("IMAGE_WORKER_CONCURRENCY"), 2),
		ImageQueueName: strings.TrimSpace(firstNonEmpty(
			os.Getenv("IMAGE_QUEUE_NAME"),
			"image:tasks",
		)),
		ImageTaskTimeoutSeconds: atoiOrDefault(os.Getenv("IMAGE_TASK_TIMEOUT_SECONDS"), 120),

		ImageAutoRetryEnabled:      envBool(os.Getenv("IMAGE_AUTO_RETRY_ENABLED"), true),
		ImageMaxRetries:            atoiOrDefault(os.Getenv("IMAGE_MAX_RETRIES"), 2),
		ImageRetryBaseDelaySeconds: atoiOrDefault(os.Getenv("IMAGE_RETRY_BASE_DELAY_SECONDS"), 30),
		ImageRetryMaxDelaySeconds:  atoiOrDefault(os.Getenv("IMAGE_RETRY_MAX_DELAY_SECONDS"), 300),

		OrderSyncQueueEnabled: envBool(os.Getenv("ORDER_SYNC_QUEUE_ENABLED"), true),
		OrderSyncQueueName: strings.TrimSpace(firstNonEmpty(
			os.Getenv("ORDER_SYNC_QUEUE_NAME"),
			"order:sync:tasks",
		)),
		OrderSyncWorkerConcurrency:  atoiOrDefault(os.Getenv("ORDER_SYNC_WORKER_CONCURRENCY"), 1),
		OrderSyncTaskTimeoutSeconds: atoiOrDefault(os.Getenv("ORDER_SYNC_TASK_TIMEOUT_SECONDS"), 120),

		CollectTaskTimeoutSeconds: atoiOrDefault(os.Getenv("COLLECT_TASK_TIMEOUT_SECONDS"), 600),

		WorkerHeartbeatEnabled:            envBool(os.Getenv("WORKER_HEARTBEAT_ENABLED"), true),
		WorkerHeartbeatIntervalSeconds:    atoiOrDefault(os.Getenv("WORKER_HEARTBEAT_INTERVAL_SECONDS"), 10),
		WorkerStaleAfterSeconds:           atoiOrDefault(os.Getenv("WORKER_STALE_AFTER_SECONDS"), 30),
		WorkerReaperEnabled:               envBool(os.Getenv("WORKER_REAPER_ENABLED"), true),
		WorkerReaperIntervalSeconds:       atoiOrDefault(os.Getenv("WORKER_REAPER_INTERVAL_SECONDS"), 15),
		WorkerLegacyRunningTimeoutSeconds: atoiOrDefault(os.Getenv("WORKER_LEGACY_RUNNING_TIMEOUT_SECONDS"), 1800),
	}

	port, err := atoiOrError(os.Getenv("DB_PORT"), defaultDBPort(cfg.DB.Driver))
	if err != nil {
		return nil, fmt.Errorf("DB_PORT: %w", err)
	}
	cfg.DB.Port = port

	rdbNum, err := atoiOrError(os.Getenv("REDIS_DB"), 0)
	if err != nil {
		return nil, fmt.Errorf("REDIS_DB: %w", err)
	}
	cfg.Redis.DB = rdbNum

	switch cfg.DB.Driver {
	case "mysql", "postgres":
	default:
		return nil, fmt.Errorf("DB_DRIVER must be mysql or postgres, got %q", cfg.DB.Driver)
	}

	if strings.TrimSpace(cfg.DB.User) == "" {
		return nil, fmt.Errorf("DB_USER is required")
	}
	if strings.TrimSpace(cfg.DB.Name) == "" {
		return nil, fmt.Errorf("DB_NAME is required")
	}

	if cfg.AppEnv == "production" && cfg.JWTSecret == "change-me-in-development" {
		return nil, fmt.Errorf("JWT_SECRET must be set for production")
	}

	return cfg, nil
}

// MaxUploadBytes returns the max upload size in bytes from UploadMaxMB (fallback 10 MB).
func (c *Config) MaxUploadBytes() int64 {
	if c == nil {
		return 10 << 20
	}
	mb := c.UploadMaxMB
	if mb <= 0 {
		mb = 10
	}
	return int64(mb) << 20
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func atoiOrDefault(s string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || s == "" {
		return def
	}
	return n
}

func envBool(s string, def bool) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return def
	}
	switch s {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

func defaultDBPort(driver string) int {
	if driver == "mysql" {
		return 3306
	}
	return 5432
}

func atoiOrError(s string, def int) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return def, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	return n, nil
}
