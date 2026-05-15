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

	// BootstrapAdminUsername / BootstrapAdminPassword seed the first admin when the table is empty.
	BootstrapAdminUsername string
	BootstrapAdminPassword string
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

		BootstrapAdminUsername: strings.TrimSpace(os.Getenv("ADMIN_BOOTSTRAP_USERNAME")),
		BootstrapAdminPassword: os.Getenv("ADMIN_BOOTSTRAP_PASSWORD"),
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
