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
