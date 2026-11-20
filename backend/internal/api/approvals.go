package api

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"nwbd-dev-workstation/internal/dingtalk"
	"nwbd-dev-workstation/internal/domain"
	"nwbd-dev-workstation/internal/store"
)

// GET /api/approvals/contracts 03 销售合同签订单列表（项目代号下拉）。
func (a *API) listContractApprovals(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requirePerm(w, r, "myproj"); !ok {
		return
	}
	if a.approval == nil || !a.approval.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "APPROVAL_DISABLED", "未配置审批应用凭据", nil)
		return
	}
	items, err := a.approval.ListContractApprovals(r.Context())
	if err != nil {
		a.fail(w, "读取合同签订单失败", err)
		return
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].ProjectCode < items[j].ProjectCode })
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		out = append(out, map[string]any{
			"instance_id":   it.InstanceID,
			"title":         it.Title,
			"project_code":  it.ProjectCode,
			"contract_name": it.ContractName,
			"customer":      it.Customer,
			"status":        it.Status,
			"business_id":   it.BusinessID,
		})
	}
	writeOK(w, map[string]any{"items": out})
}

// planApprovalInput 是前端提交的发起参数。
type planApprovalInput struct {
	ContractInstanceID string `json:"contract_instance_id"`
	Source             string `json:"source"`
}

// startProjectPlanApproval 发起 04 项目策划审批，返回实例 ID。
//
// 发起人 = 当前下发任务的人；项目经理 = 任务的负责人。
func (a *API) startProjectPlanApproval(
	ctx context.Context, user *store.Person, project *store.Project, owner *store.Person, in planApprovalInput,
) (string, error) {
	if a.approval == nil || !a.approval.Enabled() {
		return "", fmt.Errorf("未配置审批应用，无法发起钉钉审批")
	}
	if strings.TrimSpace(in.ContractInstanceID) == "" {
		return "", fmt.Errorf("请选择项目代号（合同签订单）")
	}
	if !domain.IsValid([]string{"中标合同", "公司内部立项", "顾客直接委托", "其他"}, in.Source) {
		return "", fmt.Errorf("请选择项目来源")
	}
	if owner.DingTalkUserID == "" {
		return "", fmt.Errorf("负责人缺少钉钉账号，无法发起审批")
	}

	// 校验合同单存在且填了项目代号
	contract, err := a.approval.GetInstance(ctx, in.ContractInstanceID)
	if err != nil {
		return "", fmt.Errorf("读取合同签订单失败: %w", err)
	}
	if strings.TrimSpace(contract.FormValueByName("项目代号")) == "" {
		return "", fmt.Errorf("所选合同签订单还没有填写项目代号，请先补合同单")
	}

	deptID, err := a.ding.UserDeptID(ctx, user.DingTalkUserID)
	if err != nil || deptID == 0 {
		if raw := strings.TrimSpace(user.DeptID); raw != "" {
			if parsed, convErr := strconv.ParseInt(raw, 10, 64); convErr == nil {
				deptID = parsed
			}
		}
	}
	if deptID == 0 {
		return "", fmt.Errorf("无法确定发起人的部门，无法发起审批")
	}

	values := []dingtalk.FormValue{
		{ID: "TextField_QSQO7H1M8VORC", Name: "项目名称", Value: project.Name},
		{ID: "FormRelateField_PRJCODE01", Name: "项目代号", Value: in.ContractInstanceID},
		{ID: "DDSelectField_00PFDBI1H2K6V", Name: "项目来源", Value: in.Source},
		{ID: "TextField_38IYET7WMQ247", Name: "客户单位", Value: project.Customer},
		{ID: "InnerContactField_DVTNEYU0B6SYC", Name: "项目经理", Value: `["` + owner.DingTalkUserID + `"]`},
	}
	return a.approval.StartProcess(ctx, dingtalk.StartInput{
		ProcessCode:  a.cfg.ApprovalProcessCode,
		OriginatorID: user.DingTalkUserID,
		DeptID:       deptID,
		AgentID:      a.cfg.ApprovalAgentID,
		FormValues:   values,
	})
}

// approvalStatus 把钉钉实例状态映射成本地展示状态。
func approvalStatus(inst *dingtalk.ProcessInstance) string {
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

// syncTaskApproval 拉一次实例详情并回写任务状态。
func (a *API) syncTaskApproval(ctx context.Context, task *store.BoardTask) (map[string]any, error) {
	if task.ApprovalInstanceID == "" {
		return nil, nil
	}
	inst, err := a.approval.GetInstance(ctx, task.ApprovalInstanceID)
	if err != nil {
		return nil, err
	}
	status := approvalStatus(inst)
	if status != task.ApprovalStatus {
		_ = a.store.SetTaskApproval(ctx, task.ID, status, "")
	}
	tasks := make([]map[string]any, 0, len(inst.Tasks))
	for _, t := range inst.Tasks {
		tasks = append(tasks, map[string]any{
			"task_id": t.TaskID, "user_id": t.UserID, "status": t.Status,
			"result": t.Result, "activity_id": t.Activity,
		})
	}
	ops := make([]map[string]any, 0, len(inst.Operations))
	for _, op := range inst.Operations {
		ops = append(ops, map[string]any{
			"user_id": op.UserID, "date": op.Date, "type": op.Type,
			"result": op.Result, "remark": op.Remark, "show_name": op.ShowName,
		})
	}
	return map[string]any{
		"instance_id": inst.ID,
		"title":       inst.Title,
		"status":      status,
		"raw_status":  inst.Status,
		"result":      inst.Result,
		"business_id": inst.BusinessID,
		"create_time": inst.CreateTime,
		"tasks":       tasks,
		"operations":  ops,
	}, nil
}

// GET /api/tasks/{id}/approval 查询任务关联审批的实时状态。
func (a *API) getTaskApproval(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "mytask")
	if !ok {
		return
	}
	task, err := a.store.GetBoardTask(r.Context(), r.PathValue("id"))
	if err != nil || task == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "任务不存在", nil)
		return
	}
	canView, err := a.canViewTask(r, user, task)
	if err != nil || !canView {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "无权查看该任务", nil)
		return
	}
	if a.approval == nil || task.ApprovalInstanceID == "" {
		writeOK(w, map[string]any{"approval": nil})
		return
	}
	view, err := a.syncTaskApproval(r.Context(), task)
	if err != nil {
		a.fail(w, "读取审批状态失败", err)
		return
	}
	writeOK(w, map[string]any{"approval": view})
}
