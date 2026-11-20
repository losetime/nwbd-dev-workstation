// Package logging 统一日志初始化。
//
// 服务输出到 stdout，由容器日志驱动收集。
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// Setup 初始化全局 logger，level 支持 debug/info/warn/error。
func Setup(level string) *slog.Logger {
	var slogLevel slog.Level
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "DEBUG":
		slogLevel = slog.LevelDebug
	case "WARN", "WARNING":
		slogLevel = slog.LevelWarn
	case "ERROR":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slogLevel})
	log := slog.New(handler)
	slog.SetDefault(log)
	return log
}
