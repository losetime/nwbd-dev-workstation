// Command api 启动 HTTP 服务：迁移数据库、提供登录与用户接口。
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"nwbd-dev-workstation/internal/api"
	"nwbd-dev-workstation/internal/auth"
	"nwbd-dev-workstation/internal/config"
	"nwbd-dev-workstation/internal/db"
	"nwbd-dev-workstation/internal/dingtalk"
	"nwbd-dev-workstation/internal/filemanage"
	"nwbd-dev-workstation/internal/logging"
	"nwbd-dev-workstation/internal/org"
	"nwbd-dev-workstation/internal/skills"
	"nwbd-dev-workstation/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("配置不合法", "error", err)
		os.Exit(1)
	}
	log := logging.Setup(cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := connectWithRetry(ctx, cfg.DatabaseURL, log)
	if err != nil {
		log.Error("数据库不可用", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		log.Error("数据库迁移失败", "error", err)
		os.Exit(1)
	}
	log.Info("数据库迁移完成")

	st := store.New(pool)
	client := dingtalk.NewClient(cfg)
	approvalClient := dingtalk.NewApprovalClient(cfg)
	authService := auth.NewService(cfg, st, client, log)
	skillsClient := skills.NewClient(cfg.SkillsBase, cfg.SkillsToken, cfg.SkillsTimeout)

	// 任务产出统一归档到文件管理（nwbd-file-manage），本项目不再使用 MinIO。
	files := filemanage.New(cfg.FileManageBase, cfg.FileManageToken, cfg.FileManageTimeout, log)
	if !files.Enabled() {
		log.Warn("未配置文件管理内部令牌，任务产出暂不能归档", "base", cfg.FileManageBase)
	}

	orgService := org.NewService(st, client, log)

	// 角色以钉钉管理员列表为准：启动即同步，之后定时刷新。
	authService.StartRoleSync(ctx, time.Duration(cfg.RoleSyncMinutes)*time.Minute)
	// 通讯录全量同步：保证没登录过的人也能被分配任务。
	orgService.StartSync(ctx, time.Duration(cfg.DirectorySyncMinutes)*time.Minute)

	handler := api.New(cfg, st, authService, orgService, files, skillsClient, client, approvalClient, log).Router()
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		// 产出上传/下载要经过文件管理转发，给足大文件时间
		ReadTimeout:  300 * time.Second,
		WriteTimeout: 300 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Info("API 已启动", "addr", cfg.HTTPAddr, "env", cfg.Environment)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("HTTP 服务异常退出", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("收到停止信号，正在关闭")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("优雅关闭失败", "error", err)
	}
	log.Info("API 已停止")
}

// connectWithRetry 等待数据库就绪，避免容器启动顺序造成的假失败。
func connectWithRetry(ctx context.Context, databaseURL string, log *slog.Logger) (*pgxpool.Pool, error) {
	var lastErr error
	for attempt := 1; attempt <= 30; attempt++ {
		pool, err := db.Connect(ctx, databaseURL)
		if err == nil {
			return pool, nil
		}
		lastErr = err
		log.Warn("数据库未就绪，稍后重试", "attempt", attempt, "error", err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
	return nil, lastErr
}
