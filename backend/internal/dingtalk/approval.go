package dingtalk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"nwbd-dev-workstation/internal/config"
)

// ApprovalClient 是审批专用应用的客户端（04 项目策划流程：发起、撤销、读取状态）。
type ApprovalClient struct {
	cfg  config.Config
	http *http.Client

	mu      sync.Mutex
	token   string
	expires time.Time

	contractMu    sync.Mutex
	contractCache []ContractApproval
	contractAt    time.Time
}

// NewApprovalClient 构造审批客户端；未配置凭据时返回 nil。
func NewApprovalClient(cfg config.Config) *ApprovalClient {
	if !cfg.HasApprovalCredentials() {
		return nil
	}
	return &ApprovalClient{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.DingTalkTimeout},
	}
}

// Enabled 是否可用。
func (c *ApprovalClient) Enabled() bool {
	return c != nil
}

func (c *ApprovalClient) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.token != "" && time.Now().Before(c.expires) {
		return c.token, nil
	}

	body, _ := json.Marshal(map[string]string{
		"appKey": c.cfg.ApprovalClientID, "appSecret": c.cfg.ApprovalClientSecret,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.cfg.DingTalkAPIBase+"/v1.0/oauth2/accessToken", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("无法访问钉钉开放平台: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	var payload struct {
		AccessToken string `json:"accessToken"`
		ExpireIn    int    `json:"expireIn"`
		Code        string `json:"code"`
		Message     string `json:"message"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", fmt.Errorf("钉钉审批鉴权响应无法解析: %w", err)
	}
	if payload.AccessToken == "" {
		return "", fmt.Errorf("钉钉审批鉴权失败: %s %s", payload.Code, payload.Message)
	}

	c.token = payload.AccessToken
	ttl := payload.ExpireIn
	if ttl <= 0 {
		ttl = 7200
	}
	c.expires = time.Now().Add(time.Duration(ttl-300) * time.Second)
	return c.token, nil
}

// apiError 是钉钉 v1.0 的错误体。
type apiError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestid"`
}

func (e apiError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// IsAPIError 判断错误码，便于上层给出可读提示。
func IsAPIError(err error, code string) bool {
	var ae apiError
	if ok := asAPIError(err, &ae); ok {
		return ae.Code == code
	}
	return false
}

func asAPIError(err error, out *apiError) bool {
	for err != nil {
		if ae, ok := err.(apiError); ok {
			*out = ae
			return true
		}
		type unwrapper interface{ Unwrap() error }
		u, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

func (c *ApprovalClient) do(ctx context.Context, method, path string, body any, out any) error {
	raw, err := c.doRaw(ctx, method, path, body)
	if err != nil {
		return err
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("钉钉响应无法解析: %w", err)
		}
	}
	return nil
}

// doRaw 调用接口并返回原始响应体。
func (c *ApprovalClient) doRaw(ctx context.Context, method, path string, body any) ([]byte, error) {
	token, err := c.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.cfg.DingTalkAPIBase+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-acs-dingtalk-access-token", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("无法访问钉钉开放平台: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		var ae apiError
		if json.Unmarshal(raw, &ae) == nil && ae.Code != "" {
			return nil, ae
		}
		return nil, fmt.Errorf("钉钉接口返回 %d: %s", resp.StatusCode, summarize(raw))
	}
	return raw, nil
}

// FormValue 是发起审批时的一个表单控件值。
type FormValue struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name"`
	Value    string `json:"value"`
	ExtValue string `json:"extValue,omitempty"`
}

// StartInput 发起审批的入参。
type StartInput struct {
	ProcessCode     string
	OriginatorID    string
	DeptID          int64
	AgentID         int64
	FormValues      []FormValue
	ProcessInstance string
}

// StartProcess 发起审批实例，返回实例 ID。
func (c *ApprovalClient) StartProcess(ctx context.Context, in StartInput) (string, error) {
	body := map[string]any{
		"processCode":         in.ProcessCode,
		"originatorUserId":    in.OriginatorID,
		"deptId":              in.DeptID,
		"formComponentValues": in.FormValues,
	}
	if in.AgentID > 0 {
		body["microappAgentId"] = in.AgentID
	}
	var out struct {
		InstanceID string `json:"instanceId"`
	}
	if err := c.do(ctx, http.MethodPost, "/v1.0/workflow/processInstances", body, &out); err != nil {
		return "", err
	}
	if out.InstanceID == "" {
		return "", fmt.Errorf("钉钉未返回审批实例 ID")
	}
	return out.InstanceID, nil
}

// TerminateProcess 以系统身份撤销审批实例。
func (c *ApprovalClient) TerminateProcess(ctx context.Context, instanceID, remark string) error {
	body := map[string]any{
		"processInstanceId": instanceID,
		"isSystem":          true,
		"remark":            remark,
	}
	return c.do(ctx, http.MethodPost, "/v1.0/workflow/processInstances/terminate", body, nil)
}

// ProcessInstance 是审批实例的关键信息。
type ProcessInstance struct {
	ID           string
	Title        string
	Status       string // RUNNING / COMPLETED / TERMINATED
	Result       string // agree / refuse
	BusinessID   string
	OriginatorID string
	CreateTime   time.Time
	FinishTime   *time.Time
	FormValues   []FormValue
	Tasks        []InstanceTask
	Operations   []InstanceOperation
	CurrentNode  string
}

// InstanceTask 是审批节点任务。
type InstanceTask struct {
	TaskID     string
	UserID     string
	Status     string
	Result     string
	Activity   string
	CreatedAt  *time.Time
	FinishedAt *time.Time
}

// InstanceOperation 是操作记录。
type InstanceOperation struct {
	UserID   string
	Date     time.Time
	Type     string
	Result   string
	Remark   string
	ShowName string
}

// GetInstance 读取审批实例详情（含表单值、任务、操作记录）。
func (c *ApprovalClient) GetInstance(ctx context.Context, instanceID string) (*ProcessInstance, error) {
	var out struct {
		InstanceID          string `json:"processInstanceId"`
		Title               string `json:"title"`
		Status              string `json:"status"`
		Result              string `json:"result"`
		BusinessID          string `json:"businessId"`
		OriginatorUserID    string `json:"originatorUserId"`
		OriginatorDeptID    string `json:"originatorDeptId"`
		CreateTime          string `json:"createTime"`
		FinishTime          string `json:"finishTime"`
		FormComponentValues []struct {
			Name     string `json:"name"`
			ID       string `json:"id"`
			Value    string `json:"value"`
			ExtValue string `json:"extValue"`
		} `json:"formComponentValues"`
		Tasks []struct {
			TaskID     any    `json:"taskId"`
			UserID     string `json:"userId"`
			Status     string `json:"status"`
			Result     string `json:"result"`
			ActivityID string `json:"activityId"`
			CreateTime string `json:"createTime"`
			FinishTime string `json:"finishTime"`
		} `json:"tasks"`
		OperationRecords []struct {
			UserID   string `json:"userId"`
			Date     string `json:"date"`
			Type     string `json:"type"`
			Result   string `json:"result"`
			Remark   string `json:"remark"`
			ShowName string `json:"showName"`
		} `json:"operationRecords"`
	}
	path := "/v1.0/workflow/processInstances?processInstanceId=" + instanceID + "&includeFormComponentValues=true"
	raw, err := c.doRaw(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	// 钉钉这个接口有时把结果包在 result 里，有时直接返回在顶层
	var envelope struct {
		Result json.RawMessage `json:"result"`
	}
	if json.Unmarshal(raw, &envelope) == nil && len(envelope.Result) > 0 && len(envelope.Result) > 1 && envelope.Result[0] == '{' {
		raw = envelope.Result
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("钉钉响应无法解析: %w", err)
	}

	inst := &ProcessInstance{
		ID: out.InstanceID, Title: out.Title, Status: out.Status, Result: out.Result,
		BusinessID: out.BusinessID, OriginatorID: out.OriginatorUserID,
	}
	if inst.ID == "" {
		inst.ID = instanceID
	}
	inst.CreateTime = parseDingTime(out.CreateTime)
	if out.FinishTime != "" {
		t := parseDingTime(out.FinishTime)
		inst.FinishTime = &t
	}
	for _, f := range out.FormComponentValues {
		inst.FormValues = append(inst.FormValues, FormValue{ID: f.ID, Name: f.Name, Value: f.Value, ExtValue: f.ExtValue})
	}
	for _, t := range out.Tasks {
		task := InstanceTask{TaskID: fmt.Sprint(t.TaskID), UserID: t.UserID, Status: t.Status, Result: t.Result, Activity: t.ActivityID}
		if t.CreateTime != "" {
			ct := parseDingTime(t.CreateTime)
			task.CreatedAt = &ct
		}
		if t.FinishTime != "" {
			ft := parseDingTime(t.FinishTime)
			task.FinishedAt = &ft
		}
		inst.Tasks = append(inst.Tasks, task)
		if inst.CurrentNode == "" && strings.EqualFold(t.Status, "RUNNING") {
			inst.CurrentNode = t.ActivityID
		}
	}
	for _, op := range out.OperationRecords {
		inst.Operations = append(inst.Operations, InstanceOperation{
			UserID: op.UserID, Date: parseDingTime(op.Date), Type: op.Type,
			Result: op.Result, Remark: op.Remark, ShowName: op.ShowName,
		})
	}
	return inst, nil
}

// FormValueByName 取表单值。
func (p *ProcessInstance) FormValueByName(name string) string {
	for _, f := range p.FormValues {
		if f.Name == name {
			return f.Value
		}
	}
	return ""
}

// ListInstanceIDs 按模板与时间窗查实例 ID（单次 ≤120 天）。
func (c *ApprovalClient) ListInstanceIDs(ctx context.Context, processCode string, start, end time.Time) ([]string, error) {
	body := map[string]any{
		"processCode": processCode,
		"startTime":   start.UnixMilli(),
		"endTime":     end.UnixMilli(),
		"nextToken":   0,
		"maxResults":  20,
	}
	var out struct {
		Result struct {
			List []string `json:"list"`
		} `json:"result"`
	}
	if err := c.do(ctx, http.MethodPost, "/v1.0/workflow/processes/instanceIds/query", body, &out); err != nil {
		return nil, err
	}
	return out.Result.List, nil
}

// ContractApproval 是一张 03 销售合同签订单的摘要。
type ContractApproval struct {
	InstanceID   string
	Title        string
	ProjectCode  string
	ContractName string
	Customer     string
	Status       string
	BusinessID   string
}

// ListContractApprovals 拉取近 120 天的合同签订单，供「项目代号」下拉使用。
// 结果缓存 5 分钟：钉钉该接口限流较严（每分钟 100 次）。
func (c *ApprovalClient) ListContractApprovals(ctx context.Context) ([]ContractApproval, error) {
	c.contractMu.Lock()
	defer c.contractMu.Unlock()
	if len(c.contractCache) > 0 && time.Since(c.contractAt) < 5*time.Minute {
		return c.contractCache, nil
	}

	now := time.Now()
	ids, err := c.ListInstanceIDs(ctx, c.cfg.ContractProcessCode, now.AddDate(0, 0, -119), now)
	if err != nil {
		return nil, err
	}
	out := make([]ContractApproval, 0, len(ids))
	for _, id := range ids {
		inst, err := c.GetInstance(ctx, id)
		if err != nil {
			continue
		}
		code := inst.FormValueByName("项目代号")
		if strings.TrimSpace(code) == "" {
			continue
		}
		out = append(out, ContractApproval{
			InstanceID:   id,
			Title:        inst.Title,
			ProjectCode:  strings.TrimSpace(code),
			ContractName: inst.FormValueByName("合同名称"),
			Customer:     inst.FormValueByName("甲方单位"),
			Status:       inst.Status,
			BusinessID:   inst.BusinessID,
		})
	}
	if len(out) > 0 {
		c.contractCache = out
		c.contractAt = time.Now()
	}
	return out, nil
}

func parseDingTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04Z07:00", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, value); err == nil {
			return t
		}
	}
	return time.Time{}
}
