package logx

import (
	"log/slog"
	"os"
	"strings"
)

// New 创建结构化日志器：开发环境用文本，其他环境用 JSON。
func New(level, env string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(level)}

	var handler slog.Handler
	if env == "dev" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}
	return slog.New(handler)
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
