package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config 是服务运行所需的全部配置，来源为环境变量。
type Config struct {
	Env             string
	Version         string
	HTTPAddr        string
	DatabaseURL     string
	RedisURL        string
	LogLevel        string
	ShutdownTimeout time.Duration
	DBMaxConns      int32
	ReportInterval  time.Duration

	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	EncryptionKey   string
	PhoneHashKey    string
	OfflineLeaseKey string
	OTELEndpoint    string
	SMSMode         string
	SMSMockCode     string
}

// Load 读取环境变量并做最小校验。
func Load() (Config, error) {
	cfg := Config{
		Env:             getenv("APP_ENV", "dev"),
		Version:         getenv("APP_VERSION", "dev"),
		HTTPAddr:        getenv("HTTP_ADDR", ":8080"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		RedisURL:        getenv("REDIS_URL", "redis://localhost:6379/0"),
		LogLevel:        getenv("LOG_LEVEL", "info"),
		ShutdownTimeout: getDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
		DBMaxConns:      int32(getInt("DB_MAX_CONNS", 10)),
		ReportInterval:  getDuration("REPORT_INTERVAL", time.Hour),

		JWTSecret:       os.Getenv("JWT_SECRET"),
		AccessTokenTTL:  getDuration("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL: getDuration("REFRESH_TOKEN_TTL", 720*time.Hour),
		EncryptionKey:   os.Getenv("DATA_ENCRYPTION_KEY"),
		PhoneHashKey:    os.Getenv("PHONE_HASH_KEY"),
		OfflineLeaseKey: os.Getenv("OFFLINE_LEASE_KEY"),
		OTELEndpoint:    os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		SMSMode:         getenv("SMS_MODE", "mock"),
		SMSMockCode:     getenv("SMS_MOCK_CODE", "123456"),
	}

	if cfg.Env == "dev" {
		if cfg.JWTSecret == "" {
			cfg.JWTSecret = "dev-insecure-jwt-secret-change-me-please"
		}
		if cfg.EncryptionKey == "" {
			cfg.EncryptionKey = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
		}
		if cfg.PhoneHashKey == "" {
			cfg.PhoneHashKey = "dev-phone-hash-key-change-me"
		}
		if cfg.OfflineLeaseKey == "" {
			cfg.OfflineLeaseKey = "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXphYmNkZWY="
		}
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if cfg.DBMaxConns <= 0 {
		return Config{}, fmt.Errorf("DB_MAX_CONNS must be positive, got %d", cfg.DBMaxConns)
	}
	if cfg.ShutdownTimeout <= 0 {
		return Config{}, fmt.Errorf("SHUTDOWN_TIMEOUT must be positive, got %s", cfg.ShutdownTimeout)
	}
	if cfg.ReportInterval <= 0 {
		return Config{}, fmt.Errorf("REPORT_INTERVAL must be positive, got %s", cfg.ReportInterval)
	}
	if cfg.Env != "dev" {
		if len(cfg.JWTSecret) < 32 {
			return Config{}, errors.New("JWT_SECRET must be at least 32 characters outside dev")
		}
		if cfg.EncryptionKey == "" || cfg.PhoneHashKey == "" || cfg.OfflineLeaseKey == "" {
			return Config{}, errors.New("DATA_ENCRYPTION_KEY, PHONE_HASH_KEY and OFFLINE_LEASE_KEY are required outside dev")
		}
		if cfg.SMSMode == "mock" {
			return Config{}, errors.New("SMS_MODE=mock is not allowed outside dev")
		}
	}
	if cfg.AccessTokenTTL <= 0 || cfg.RefreshTokenTTL <= 0 {
		return Config{}, errors.New("token TTLs must be positive")
	}
	switch cfg.SMSMode {
	case "mock", "disabled":
	default:
		return Config{}, fmt.Errorf("unsupported SMS_MODE %q", cfg.SMSMode)
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
