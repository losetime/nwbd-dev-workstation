// Command consumer 订阅钉钉审批事件（Stream 模式），把审批状态回写到任务。
//
// 使用审批专用应用（04 项目策划流程），与 file-manage 的旧应用互不影响。
package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	dingstream "github.com/open-dingtalk/dingtalk-stream-sdk-go/client"
	dingevent "github.com/open-dingtalk/dingtalk-stream-sdk-go/event"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/handler"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/payload"

	"nwbd-dev-workstation/internal/config"
	"nwbd-dev-workstation/internal/contractsync"
	"nwbd-dev-workstation/internal/dingtalk"
	"nwbd-dev-workstation/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	log := newLogger(cfg.LogLevel)

	if !cfg.ApprovalStreamEnabled || !cfg.HasApprovalCredentials() {
		log.Info("审批 Stream 未启用或缺少审批应用凭据，消费者退出")
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("连接数据库失败", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	st := store.New(pool)
	approval := dingtalk.NewApprovalClient(cfg)
	if approval == nil {
		log.Error("审批客户端不可用")
		os.Exit(1)
	}

	streamClient := dingstream.NewStreamClient(
		dingstream.WithAppCredential(dingstream.NewAppCredentialConfig(cfg.ApprovalClientID, cfg.ApprovalClientSecret)),
		dingstream.WithAutoReconnect(true),
	)
	streamClient.RegisterAllEventRouter(handlerFor(ctx, cfg, st, approval, log))

	log.Info("审批消费者启动", "process_code", cfg.ApprovalProcessCode)
	for {
		if ctx.Err() != nil {
			break
		}
		if err := streamClient.Start(ctx); err != nil {
			log.Error("连接钉钉 Stream 失败", "error", err)
			select {
			case <-ctx.Done():
			case <-time.After(5 * time.Second):
			}
			continue
		}
		log.Info("钉钉 Stream 长连接已建立")
		// Start 建立连接后即返回，保持进程存活直到收到退出信号
		<-ctx.Done()
	}
	streamClient.Close()
	log.Info("审批消费者已停止")
}

func handlerFor(
	parent context.Context, cfg config.Config, st *store.Store, approval *dingtalk.ApprovalClient, log *slog.Logger,
) handler.IFrameHandler {
	return func(ctx context.Context, df *payload.DataFrame) (*payload.DataFrameResponse, error) {
		header := dingevent.NewEventHeaderFromDataFrame(df)
		if header.EventId == "" {
			return dingevent.NewSuccessResponse()
		}
		if header.EventType != "bpms_instance_change" {
			return dingevent.NewSuccessResponse()
		}
		data := map[string]any{}
		if df.Data != "" {
			if err := json.Unmarshal([]byte(df.Data), &data); err != nil {
				log.Warn("审批事件体不是合法 JSON", "event", header.EventId, "error", err)
				return dingevent.NewSuccessResponse()
			}
		}
		processCode, _ := data["processCode"].(string)
		processCode = strings.TrimSpace(processCode)
		instanceID, _ := data["processInstanceId"].(string)
		if instanceID == "" {
			return dingevent.NewSuccessResponse()
		}
		switch processCode {
		case cfg.ApprovalProcessCode:
			return handleTaskApproval(ctx, st, approval, log, data, instanceID)
		case cfg.ContractProcessCode:
			return handleContractApproval(ctx, st, approval, log, data, instanceID)
		case cfg.ChangeProcessCode:
			return handleChangeApproval(ctx, st, approval, log, instanceID)
		default:
			return dingevent.NewSuccessResponse()
		}
	}
}

// handleTaskApproval 处理 04 项目策划审批的事件，回写任务状态。
func handleTaskApproval(
	ctx context.Context, st *store.Store, approval *dingtalk.ApprovalClient, log *slog.Logger,
	data map[string]any, instanceID string,
) (*payload.DataFrameResponse, error) {
	task, err := st.TaskByApprovalInstance(ctx, instanceID)
	if err != nil {
		log.Error("查询审批对应任务失败", "instance", instanceID, "error", err)
		return dingevent.NewLaterResponse()
	}
	if task == nil {
		return dingevent.NewSuccessResponse()
	}
	inst, err := approval.GetInstance(ctx, instanceID)
	if err != nil {
		log.Error("拉取审批详情失败", "instance", instanceID, "error", err)
		return dingevent.NewLaterResponse()
	}
	status := statusText(inst)
	if status != task.ApprovalStatus {
		if err := st.SetTaskApproval(ctx, task.ID, status, ""); err != nil {
			log.Error("回写审批状态失败", "task", task.ID, "error", err)
			return dingevent.NewLaterResponse()
		}
		_ = st.AddTaskEvent(ctx, task.ID, "approval", "钉钉审批："+status, "", "system", "info", task.AttemptCount)
		log.Info("审批状态已更新", "task", task.ID, "status", status, "instance", instanceID)
	}
	return dingevent.NewSuccessResponse()
}

// handleContractApproval 处理 03 销售合同签订的事件：通过后自动建项目，撤销/拒绝时打标。
// handleChangeApproval 处理 05 项目变更审批：回写状态，通过后把变更应用到项目。
func handleChangeApproval(
	ctx context.Context, st *store.Store, approval *dingtalk.ApprovalClient, log *slog.Logger, instanceID string,
) (*payload.DataFrameResponse, error) {
	c, err := st.ChangeByInstance(ctx, instanceID)
	if err != nil {
		log.Error("查询审批对应变更失败", "instance", instanceID, "error", err)
		return dingevent.NewLaterResponse()
	}
	if c == nil {
		return dingevent.NewSuccessResponse()
	}
	inst, err := approval.GetInstance(ctx, instanceID)
	if err != nil {
		log.Error("拉取变更审批详情失败", "instance", instanceID, "error", err)
		return dingevent.NewLaterResponse()
	}
	status := changeStatus(inst)
	if status == c.ApprovalStatus {
		return dingevent.NewSuccessResponse()
	}
	if err := st.SetChangeApproval(ctx, c.ID, status, status, inst.BusinessID); err != nil {
		log.Error("回写变更状态失败", "change", c.ID, "error", err)
		return dingevent.NewLaterResponse()
	}
	if status == "已通过" {
		if err := st.ApplyChangeToProject(ctx, c.ID); err != nil {
			log.Error("变更写回项目失败", "change", c.ID, "error", err)
			return dingevent.NewLaterResponse()
		}
		_ = st.AddAudit(ctx, "", "system", "change.apply", "change", c.ID, map[string]any{
			"project": c.ProjectID, "before": c.BeforeValue, "after": c.AfterValue,
		})
		log.Info("变更已通过并写回项目", "change", c.ID, "project", c.ProjectID)
	}
	return dingevent.NewSuccessResponse()
}

func changeStatus(inst *dingtalk.ProcessInstance) string {
	switch strings.ToUpper(inst.Status) {
	case "RUNNING":
		return "审批中"
	case "COMPLETED":
		if strings.EqualFold(inst.Result, "refuse") {
			return "已否决"
		}
		return "已通过"
	case "TERMINATED":
		return "已撤销"
	default:
		return inst.Status
	}
}

// handleContractApproval 处理 03 销售合同签订的事件：交给 contractsync 统一落库。
func handleContractApproval(
	ctx context.Context, st *store.Store, approval *dingtalk.ApprovalClient, log *slog.Logger,
	data map[string]any, instanceID string,
) (*payload.DataFrameResponse, error) {
	if err := contractsync.Apply(ctx, st, approval, log, instanceID); err != nil {
		log.Error("处理合同审批失败", "instance", instanceID, "error", err)
		return dingevent.NewLaterResponse()
	}
	return dingevent.NewSuccessResponse()
}

func statusText(inst *dingtalk.ProcessInstance) string {
	switch strings.ToUpper(inst.Status) {
	case "RUNNING":
		return "审批中"
	case "COMPLETED":
		if strings.EqualFold(inst.Result, "refuse") {
			return "已拒绝"
		}
		return "已通过"
	case "TERMINATED":
		return "已撤销"
	default:
		return inst.Status
	}
}

// newLogger 用标准库文本 handler，容器里直接看。
func newLogger(level string) *slog.Logger {
	var lv slog.Level
	switch strings.ToUpper(level) {
	case "DEBUG":
		lv = slog.LevelDebug
	case "WARN":
		lv = slog.LevelWarn
	case "ERROR":
		lv = slog.LevelError
	default:
		lv = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: lv}))
}
