// Command contractsync 手动同步一条合同签订审批实例到项目台账。
//
// 用法：contractsync <processInstanceId>
// 用于补数据或排障，消费者会自动处理正常事件。
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"nwbd-dev-workstation/internal/config"
	"nwbd-dev-workstation/internal/contractsync"
	"nwbd-dev-workstation/internal/dingtalk"
	"nwbd-dev-workstation/internal/store"
)

func main() {
	if len(os.Args) < 2 || strings.TrimSpace(os.Args[1]) == "" {
		fmt.Fprintln(os.Stderr, "用法: contractsync <processInstanceId>")
		os.Exit(2)
	}
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	approval := dingtalk.NewApprovalClient(cfg)
	if approval == nil {
		fmt.Fprintln(os.Stderr, "缺少审批应用凭据")
		os.Exit(1)
	}
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := contractsync.Apply(ctx, store.New(pool), approval, log, strings.TrimSpace(os.Args[1])); err != nil {
		fmt.Fprintln(os.Stderr, "同步失败:", err)
		os.Exit(1)
	}
	fmt.Println("同步完成")
}
