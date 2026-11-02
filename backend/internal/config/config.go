package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds environment-driven settings for the API server.
type Config struct {
	AppEnv         string
	AppName        string
	AppVersion     string
	HTTPAddr       string
	AdminPublicURL string
	APIPublicURL   string
	LogLevel       string
	MasterKey      string

	// Feature gates (production must disable dangerous flags).
	EnableSwagger        bool
	EnableDevRoutes      bool
	EnableDemoSeed       bool
	EnableDebugEndpoints bool
	DB                   DBConfig
	Redis                RedisConfig
	JWTSecret            string
	JWTExpHrs            int
	Auth                 AuthConfig
	Tenant               TenantConfig

	// BootstrapAdminEmail / BootstrapAdminPhone / BootstrapAdminPassword seed the first admin when admin_users is empty (at least one contact required).
	BootstrapAdminEmail    string
	BootstrapAdminPhone    string
	BootstrapAdminPassword string

	// UploadMaxMB limits multipart image uploads (default 10 MB).
	UploadMaxMB int

	// CollectorBaseURL is the Node collector HTTP base (e.g. http://127.0.0.1:3100).
	CollectorBaseURL string
	// CollectorTimeoutSeconds caps outbound HTTP calls to the collector (default 120).
	CollectorTimeoutSeconds int
	// CollectorServiceToken authenticates backend-to-collector HTTP calls.
	CollectorServiceToken string

	// CollectQueueEnabled gates async collect jobs (Redis list + worker).
	CollectQueueEnabled bool
	// CollectWorkerConcurrency is the number of concurrent ready-to-processing consumers.
	CollectWorkerConcurrency int
	// CollectQueueName is the Redis list key for collect job payloads.
	CollectQueueName string
	// CollectBatchMaxURLs limits URLs per POST /collect/batches (default 50).
	CollectBatchMaxURLs int

	// 1688 bulk collect throttling (conservative defaults; settings.collector can override).
	CollectBatchConcurrency1688 int
	CollectBatchDelayMinMs1688  int
	CollectBatchDelayMaxMs1688  int
	CollectBatchRetryOnBlocked  bool
	CollectBatchRetryOnTimeout  bool
	CollectBatchMaxRetries1688  int

	// Worker automatic retry (backoff via DB next_retry_at + scheduler LPUSH).
	CollectAutoRetryEnabled      bool
	CollectMaxRetries            int
	CollectRetryBaseDelaySeconds int
	CollectRetryMaxDelaySeconds  int

	// ImageQueueEnabled gates async image_tasks (Redis list + in-process worker).
	ImageQueueEnabled bool
	// ImageWorkerConcurrency is the number of concurrent ready-to-processing consumers for image tasks.
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
	// OrderSyncWorkerConcurrency is concurrent ready-to-processing consumers for order sync (default 1).
	OrderSyncWorkerConcurrency int
	// OrderSyncTaskTimeoutSeconds caps each Provider.SyncOrders context (default 120).
	OrderSyncTaskTimeoutSeconds int

	// CustomerMessageSyncQueueName is the Redis list key (default customer:message:sync:tasks).
	CustomerMessageSyncQueueName string
	// CustomerMessageSyncWorkerConcurrency is concurrent ready-to-processing consumers (default 1).
	CustomerMessageSyncWorkerConcurrency int
	// CustomerMessageSyncTaskTimeoutSeconds caps each Provider.PullMessages context (default 120).
	CustomerMessageSyncTaskTimeoutSeconds int
	CustomerAutoReplyQueueName            string
	CustomerAutoReplyWorkerConcurrency    int

	// ProductPublishQueueEnabled gates async product publish jobs (Redis list + worker).
	ProductPublishQueueEnabled bool
	// ProductPublishQueueName is the Redis list key for product publish payloads (default product:publish:tasks).
	ProductPublishQueueName string
	// ProductPublishWorkerConcurrency is concurrent ready-to-processing consumers (default 1).
	ProductPublishWorkerConcurrency int
	// ProductPublishTaskTimeoutSeconds caps publish worker lease TTL and provider timeout (default 180).
	ProductPublishTaskTimeoutSeconds int

	// Publish batch matrix limits.
	PublishBatchMaxProducts int
	PublishBatchMaxTargets  int
	PublishBatchMaxTasks    int

	// InventorySyncQueueEnabled gates outbound inventory_sync tasks via Redis LIST + worker (false = synchronous in API goroutine).
	InventorySyncQueueEnabled bool
	// InventorySyncQueueName Redis list key (default inventory:sync:tasks).
	InventorySyncQueueName string
	// InventorySyncWorkerConcurrency is concurrent ready-to-processing consumers (default 1).
	InventorySyncWorkerConcurrency int
	// InventorySyncTaskTimeoutSeconds caps worker lease TTL and provider context (default 120).
	InventorySyncTaskTimeoutSeconds int

	// CollectTaskTimeoutSeconds is the DB lease TTL for collect_tasks (worker reclaim).
	CollectTaskTimeoutSeconds int

	// Worker heartbeat / lease reclaim (multi-instance workers).
	WorkerHeartbeatEnabled            bool
	WorkerHeartbeatIntervalSeconds    int
	WorkerStaleAfterSeconds           int
	WorkerReaperEnabled               bool
	WorkerReaperIntervalSeconds       int
	WorkerLegacyRunningTimeoutSeconds int

	// Task alert scan worker (in-process ticker; not a Redis consumer).
	TaskAlertScanEnabled         bool
	TaskAlertScanIntervalSeconds int
	TaskAlertScanLookbackMinutes int
	TaskAlertScanLockTTLSeconds  int

	// StorageProvider is the fail-fast storage kind (STORAGE_PROVIDER env).
	// Allowed: local, cos, oss, s3, r2, minio. staging/production must not use local.
	StorageProvider string

	// CORS production settings.
	CORSAllowedOrigins   []string
	CORSAllowedMethods   []string
	CORSAllowedHeaders   []string
	CORSExposedHeaders   []string
	CORSAllowCredentials bool
	CORSMaxAge           int

	// Migration lock settings.
	MigrationRunOnStartup       bool
	MigrationLockTimeoutSeconds int

	// Webhook HTTP receiver (public POST /api/v1/webhooks/:platform/:eventType).
	WebhookMaxBodyKB                int
	WebhookMaxClockSkewSeconds      int
	WebhookEnableTestVerifier       bool
	WebhookWorkerIntervalSeconds    int
	DouyinWebhookTestShopBindingID  string
	EnableDouyinWebhookDemoFallback bool

	// Observability.
	Observability ObservabilityConfig
	// Performance, capacity, pagination, and limiting controls.
	RuntimeLimits RuntimeLimitsConfig
	// Production capability controls remain L0 until an explicit approval changes code and configuration.
	ProductionCapabilities ProductionCapabilityConfig
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

// RedisConfig is used for cache and active task queues.
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// Load reads configuration from process variables after main applies root .env defaults.
func Load() (*Config, error) {
	appEnv := NormalizeEnv(firstNonEmpty(os.Getenv("APP_ENV"), EnvDevelopment))
	cfg := &Config{
		AppEnv:               appEnv,
		AppName:              firstNonEmpty(os.Getenv("APP_NAME"), "TradeMind"),
		AppVersion:           strings.TrimSpace(os.Getenv("APP_VERSION")),
		HTTPAddr:             resolveHTTPAddr(),
		AdminPublicURL:       strings.TrimSpace(os.Getenv("ADMIN_PUBLIC_URL")),
		APIPublicURL:         strings.TrimSpace(os.Getenv("API_PUBLIC_URL")),
		LogLevel:             firstNonEmpty(os.Getenv("LOG_LEVEL"), defaultLogLevel(appEnv)),
		MasterKey:            os.Getenv("APP_MASTER_KEY"),
		EnableSwagger:        envBool(os.Getenv("ENABLE_SWAGGER"), appEnv != EnvProduction),
		EnableDevRoutes:      envBool(os.Getenv("ENABLE_DEV_ROUTES"), appEnv != EnvProduction && appEnv != EnvStaging),
		EnableDemoSeed:       envBool(os.Getenv("ENABLE_DEMO_SEED"), appEnv == EnvDevelopment || appEnv == EnvDemo),
		EnableDebugEndpoints: envBool(os.Getenv("ENABLE_DEBUG_ENDPOINTS"), appEnv != EnvProduction),
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
		Auth:      loadAuthConfig(appEnv),
		Tenant:    loadTenantConfig(appEnv),

		BootstrapAdminEmail:    strings.TrimSpace(os.Getenv("ADMIN_BOOTSTRAP_EMAIL")),
		BootstrapAdminPhone:    strings.TrimSpace(os.Getenv("ADMIN_BOOTSTRAP_PHONE")),
		BootstrapAdminPassword: os.Getenv("ADMIN_BOOTSTRAP_PASSWORD"),

		UploadMaxMB: atoiOrDefault(os.Getenv("UPLOAD_MAX_MB"), 10),

		CollectorBaseURL:        strings.TrimRight(strings.TrimSpace(firstNonEmpty(os.Getenv("COLLECTOR_BASE_URL"), "http://127.0.0.1:3100")), "/"),
		CollectorTimeoutSeconds: atoiOrDefault(os.Getenv("COLLECTOR_TIMEOUT_SECONDS"), 120),
		CollectorServiceToken:   strings.TrimSpace(os.Getenv("COLLECTOR_SERVICE_TOKEN")),

		CollectQueueEnabled:      envBool(os.Getenv("COLLECT_QUEUE_ENABLED"), true),
		CollectWorkerConcurrency: atoiOrDefault(os.Getenv("COLLECT_WORKER_CONCURRENCY"), 2),
		CollectQueueName: strings.TrimSpace(firstNonEmpty(
			os.Getenv("COLLECT_QUEUE_NAME"),
			"collect:tasks",
		)),
		CollectBatchMaxURLs: atoiOrDefault(os.Getenv("COLLECT_BATCH_MAX_URLS"), 50),

		CollectBatchConcurrency1688: atoiOrDefault(os.Getenv("COLLECT_BATCH_CONCURRENCY_1688"), 1),
		CollectBatchDelayMinMs1688:  atoiOrDefault(os.Getenv("COLLECT_BATCH_DELAY_MIN_MS_1688"), 1500),
		CollectBatchDelayMaxMs1688:  atoiOrDefault(os.Getenv("COLLECT_BATCH_DELAY_MAX_MS_1688"), 5000),
		CollectBatchRetryOnBlocked:  envBool(os.Getenv("COLLECT_BATCH_RETRY_ON_BLOCKED"), true),
		CollectBatchRetryOnTimeout:  envBool(os.Getenv("COLLECT_BATCH_RETRY_ON_TIMEOUT"), true),
		CollectBatchMaxRetries1688:  atoiOrDefault(os.Getenv("COLLECT_BATCH_MAX_RETRIES_1688"), 2),

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

		CustomerMessageSyncQueueName: strings.TrimSpace(firstNonEmpty(
			os.Getenv("CUSTOMER_MESSAGE_SYNC_QUEUE_NAME"),
			"customer:message:sync:tasks",
		)),
		CustomerMessageSyncWorkerConcurrency:  atoiOrDefault(os.Getenv("CUSTOMER_MESSAGE_SYNC_WORKER_CONCURRENCY"), 1),
		CustomerMessageSyncTaskTimeoutSeconds: atoiOrDefault(os.Getenv("CUSTOMER_MESSAGE_SYNC_TASK_TIMEOUT_SECONDS"), 120),
		CustomerAutoReplyQueueName: strings.TrimSpace(firstNonEmpty(
			os.Getenv("CUSTOMER_AUTO_REPLY_QUEUE_NAME"),
			"customer:auto:reply:tasks",
		)),
		CustomerAutoReplyWorkerConcurrency: atoiOrDefault(os.Getenv("CUSTOMER_AUTO_REPLY_WORKER_CONCURRENCY"), 1),

		ProductPublishQueueEnabled: envBool(os.Getenv("PRODUCT_PUBLISH_QUEUE_ENABLED"), true),
		ProductPublishQueueName: strings.TrimSpace(firstNonEmpty(
			os.Getenv("PRODUCT_PUBLISH_QUEUE_NAME"),
			"product:publish:tasks",
		)),
		ProductPublishWorkerConcurrency:  atoiOrDefault(os.Getenv("PRODUCT_PUBLISH_WORKER_CONCURRENCY"), 1),
		ProductPublishTaskTimeoutSeconds: atoiOrDefault(os.Getenv("PRODUCT_PUBLISH_TASK_TIMEOUT_SECONDS"), 180),

		PublishBatchMaxProducts: atoiOrDefault(os.Getenv("PUBLISH_BATCH_MAX_PRODUCTS"), 100),
		PublishBatchMaxTargets:  atoiOrDefault(os.Getenv("PUBLISH_BATCH_MAX_TARGETS"), 20),
		PublishBatchMaxTasks:    atoiOrDefault(os.Getenv("PUBLISH_BATCH_MAX_TASKS"), 300),

		InventorySyncQueueEnabled: envBool(os.Getenv("INVENTORY_SYNC_QUEUE_ENABLED"), true),
		InventorySyncQueueName: strings.TrimSpace(firstNonEmpty(
			os.Getenv("INVENTORY_SYNC_QUEUE_NAME"),
			"inventory:sync:tasks",
		)),
		InventorySyncWorkerConcurrency:  atoiOrDefault(os.Getenv("INVENTORY_SYNC_WORKER_CONCURRENCY"), 1),
		InventorySyncTaskTimeoutSeconds: atoiOrDefault(os.Getenv("INVENTORY_SYNC_TASK_TIMEOUT_SECONDS"), 120),

		CollectTaskTimeoutSeconds: atoiOrDefault(os.Getenv("COLLECT_TASK_TIMEOUT_SECONDS"), 600),

		WorkerHeartbeatEnabled:            envBool(os.Getenv("WORKER_HEARTBEAT_ENABLED"), true),
		WorkerHeartbeatIntervalSeconds:    atoiOrDefault(os.Getenv("WORKER_HEARTBEAT_INTERVAL_SECONDS"), 10),
		WorkerStaleAfterSeconds:           atoiOrDefault(os.Getenv("WORKER_STALE_AFTER_SECONDS"), 30),
		WorkerReaperEnabled:               envBool(os.Getenv("WORKER_REAPER_ENABLED"), true),
		WorkerReaperIntervalSeconds:       atoiOrDefault(os.Getenv("WORKER_REAPER_INTERVAL_SECONDS"), 15),
		WorkerLegacyRunningTimeoutSeconds: atoiOrDefault(os.Getenv("WORKER_LEGACY_RUNNING_TIMEOUT_SECONDS"), 1800),

		TaskAlertScanEnabled:         envBool(os.Getenv("TASK_ALERT_SCAN_ENABLED"), false),
		TaskAlertScanIntervalSeconds: atoiOrDefault(os.Getenv("TASK_ALERT_SCAN_INTERVAL_SECONDS"), 60),
		TaskAlertScanLookbackMinutes: atoiOrDefault(os.Getenv("TASK_ALERT_SCAN_LOOKBACK_MINUTES"), 120),
		TaskAlertScanLockTTLSeconds:  atoiOrDefault(os.Getenv("TASK_ALERT_SCAN_LOCK_TTL_SECONDS"), 120),

		StorageProvider: strings.ToLower(strings.TrimSpace(firstNonEmpty(os.Getenv("STORAGE_PROVIDER"), "local"))),

		CORSAllowedOrigins:   splitCSV(os.Getenv("CORS_ALLOWED_ORIGINS")),
		CORSAllowedMethods:   splitCSV(os.Getenv("CORS_ALLOWED_METHODS")),
		CORSAllowedHeaders:   splitCSV(os.Getenv("CORS_ALLOWED_HEADERS")),
		CORSExposedHeaders:   splitCSV(os.Getenv("CORS_EXPOSED_HEADERS")),
		CORSAllowCredentials: envBool(os.Getenv("CORS_ALLOW_CREDENTIALS"), true),
		CORSMaxAge:           atoiOrDefault(os.Getenv("CORS_MAX_AGE"), 43200),

		MigrationRunOnStartup:       envBool(os.Getenv("MIGRATION_RUN_ON_STARTUP"), true),
		MigrationLockTimeoutSeconds: atoiOrDefault(os.Getenv("MIGRATION_LOCK_TIMEOUT_SECONDS"), 120),

		WebhookMaxBodyKB:                atoiOrDefault(os.Getenv("WEBHOOK_MAX_BODY_KB"), 512),
		WebhookMaxClockSkewSeconds:      atoiOrDefault(os.Getenv("WEBHOOK_MAX_CLOCK_SKEW_SECONDS"), 300),
		WebhookEnableTestVerifier:       envBool(os.Getenv("WEBHOOK_ENABLE_TEST_VERIFIER"), false),
		WebhookWorkerIntervalSeconds:    atoiOrDefault(os.Getenv("WEBHOOK_WORKER_INTERVAL_SECONDS"), 3),
		DouyinWebhookTestShopBindingID:  strings.TrimSpace(os.Getenv("DOUYIN_WEBHOOK_TEST_SHOP_BINDING_ID")),
		EnableDouyinWebhookDemoFallback: envBool(os.Getenv("ENABLE_DOUYIN_WEBHOOK_DEMO_FALLBACK"), false),
	}
	cfg.Observability = LoadObservabilityConfig(cfg.AppEnv, cfg.AppName, cfg.AppVersion)
	cfg.RuntimeLimits = loadRuntimeLimitsConfig(cfg.AppEnv)
	cfg.ProductionCapabilities = loadProductionCapabilityConfig(cfg.AppEnv)
	// Test verifier must never run in production regardless of env flag.
	if IsProduction(cfg.AppEnv) {
		cfg.WebhookEnableTestVerifier = false
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

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if err := cfg.ProductionCapabilities.Validate(cfg.AppEnv); err != nil {
		return nil, err
	}

	return cfg, nil
}

func defaultLogLevel(appEnv string) string {
	if IsProduction(appEnv) {
		return "info"
	}
	return "debug"
}

func resolveHTTPAddr() string {
	if v := strings.TrimSpace(os.Getenv("APP_HTTP_ADDR")); v != "" {
		return v
	}
	host := strings.TrimSpace(os.Getenv("HTTP_HOST"))
	port := strings.TrimSpace(os.Getenv("HTTP_PORT"))
	if host != "" && port != "" {
		if strings.Contains(host, ":") {
			return host + ":" + port
		}
		return host + ":" + port
	}
	if port != "" {
		if strings.HasPrefix(port, ":") {
			return port
		}
		return ":" + port
	}
	return ":8080"
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

// WebhookMaxBodyBytes returns inbound webhook body limit (default 512 KiB).
func (c *Config) WebhookMaxBodyBytes() int64 {
	if c == nil {
		return 512 * 1024
	}
	kb := c.WebhookMaxBodyKB
	if kb <= 0 {
		kb = 512
	}
	return int64(kb) * 1024
}

// WebhookMaxClockSkew returns allowed |now - timestamp| window (default 300s).
func (c *Config) WebhookMaxClockSkew() time.Duration {
	sec := 300
	if c != nil && c.WebhookMaxClockSkewSeconds > 0 {
		sec = c.WebhookMaxClockSkewSeconds
	}
	return time.Duration(sec) * time.Second
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

func splitCSV(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
