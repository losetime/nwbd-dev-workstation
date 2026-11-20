package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"nwbd-dev-workstation/internal/dingtalk"
	"nwbd-dev-workstation/internal/store"
)

// changeView 变更申请对外结构。
func changeView(c *store.Change) map[string]any {
	if c == nil {
		return map[string]any{}
	}
	return map[string]any{
		"id":              c.ID,
		"code":            c.Code,
		"project_id":      c.ProjectID,
		"project_name":    c.ProjectName,
		"type":            c.Type,
		"source":          c.Source,
		"content":         c.Content,
		"impact":          c.Impact,
		"reason":          c.Reason,
		"budget_before":   c.BudgetBefore,
		"budget_after":    c.BudgetAfter,
		"plan_due_after":  c.PlanDueAfter,
		"material_path":   c.MaterialPath,
		"status":          c.Status,
		"approval_status": c.ApprovalStatus,
		"applicant_id":    c.ApplicantID,
		"applicant_name":  c.ApplicantName,
		"applied_on":      c.AppliedOn,
		"instance_id":     c.DtInstanceID,
		"business_id":     c.DtBusinessID,
		"before_value":    c.BeforeValue,
		"after_value":     c.AfterValue,
	}
}

// POST /api/projects/{id}/changes 发起项目变更（项目经理 / 部门经理）。
func (a *API) createChange(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "mychange")
	if !ok {
		return
	}
	projectID := r.PathValue("id")
	allowed, err := a.canManageProject(r, user, projectID)
	if err != nil {
		a.fail(w, "校验项目权限失败", err)
		return
	}
	if !allowed {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "只有项目经理或部门经理可以发起变更", nil)
		return
	}
	project, err := a.store.GetProject(r.Context(), projectID)
	if err != nil || project == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "项目不存在", nil)
		return
	}
	if a.approval == nil || !a.approval.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "APPROVAL_DISABLED", "未配置审批应用，无法发起变更", nil)
		return
	}
	if strings.TrimSpace(project.ContractInstanceID) == "" {
		writeError(w, http.StatusUnprocessableEntity, "NO_CONTRACT", "项目还没有关联合同签订单，无法发起变更", nil)
		return
	}

	var payload struct {
		Type         string   `json:"type"`
		Source       string   `json:"source"`
		Content      string   `json:"content"`
		Reason       string   `json:"reason"`
		Impact       string   `json:"impact"`
		BudgetBefore *float64 `json:"budget_before"`
		BudgetAfter  *float64 `json:"budget_after"`
		PlanDueAfter string   `json:"plan_due_after"`
		MaterialPath string   `json:"material_path"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "请求体不合法", nil)
		return
	}
	if !isValidChangeType(payload.Type) {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_TYPE", "变更类型只能是经费变更/计划变更/其他变更", nil)
		return
	}
	if payload.Source != "内部" && payload.Source != "外部" {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_SOURCE", "变更来源只能是内部或外部", nil)
		return
	}
	if strings.TrimSpace(payload.Content) == "" || strings.TrimSpace(payload.Reason) == "" || strings.TrimSpace(payload.Impact) == "" {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "变更内容、原因、影响范围必填", nil)
		return
	}
	planDue, err := parseDate(payload.PlanDueAfter)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_DATE", "变更后交付日期格式应为 YYYY-MM-DD", nil)
		return
	}

	before := map[string]any{"budget": project.Budget, "plan_end": project.PlanEnd}
	after := map[string]any{"budget": payload.BudgetAfter, "plan_end": planDue}
	beforeJSON, _ := json.Marshal(before)
	afterJSON, _ := json.Marshal(after)

	deptID, err := a.ding.UserDeptID(r.Context(), user.DingTalkUserID)
	if err != nil || deptID == 0 {
		if raw := strings.TrimSpace(user.DeptID); raw != "" {
			if parsed, convErr := strconv.ParseInt(raw, 10, 64); convErr == nil {
				deptID = parsed
			}
		}
	}
	if deptID == 0 {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_DEPT", "无法确定发起人的部门", nil)
		return
	}

	values := []dingtalk.FormValue{
		{ID: "FormRelateField_6VDER1RE3FOQ0", Name: "项目代号", Value: project.ContractInstanceID},
		{ID: "DDSelectField_6G5BUG02CE74F", Name: "变更来源", Value: payload.Source},
		{ID: "DDSelectField_XOJ388ETW1SEV", Name: "变更类型", Value: payload.Type},
		{ID: "TextareaField_AJISTB8KKYFZ7", Name: "变更内容", Value: strings.TrimSpace(payload.Content)},
		{ID: "TextareaField_UJZA98O3ZRPAH", Name: "变更原因", Value: strings.TrimSpace(payload.Reason)},
		{ID: "TextareaField_KEC6SDHD6S8QY", Name: "影响范围", Value: strings.TrimSpace(payload.Impact)},
	}
	if payload.BudgetBefore != nil {
		values = append(values, dingtalk.FormValue{ID: "MoneyField_WDWF5PG1974P4", Name: "项目实施预算（万元）", Value: fmt.Sprintf("%g", *payload.BudgetBefore)})
	}
	if payload.BudgetAfter != nil {
		values = append(values, dingtalk.FormValue{ID: "MoneyField_22K5P6SPAO9GM", Name: "变更后项目实施预算（万元）", Value: fmt.Sprintf("%g", *payload.BudgetAfter)})
	}
	if payload.PlanDueAfter != "" {
		values = append(values, dingtalk.FormValue{ID: "DDDateField_P9Y9XZNU34SUI", Name: "变更后计划交付日期", Value: payload.PlanDueAfter})
	}
	if path := strings.TrimSpace(payload.MaterialPath); path != "" {
		values = append(values, dingtalk.FormValue{ID: "TextField_6UOOT0UDNLFSE", Name: "变更材料存放位置（服务器路径）", Value: path})
	}

	instanceID, err := a.approval.StartProcess(r.Context(), dingtalk.StartInput{
		ProcessCode:  a.cfg.ChangeProcessCode,
		OriginatorID: user.DingTalkUserID,
		DeptID:       deptID,
		AgentID:      a.cfg.ApprovalAgentID,
		FormValues:   values,
	})
	if err != nil {
		a.log.Warn("发起变更审批失败", "project", projectID, "error", err)
		writeError(w, http.StatusUnprocessableEntity, "APPROVAL_START_FAILED", err.Error(), nil)
		return
	}

	id, err := a.store.CreateChange(r.Context(), store.ChangeInput{
		ProjectID: projectID, Type: payload.Type, Source: payload.Source,
		Content: strings.TrimSpace(payload.Content), Impact: strings.TrimSpace(payload.Impact),
		Reason: strings.TrimSpace(payload.Reason), BudgetBefore: payload.BudgetBefore,
		BudgetAfter: payload.BudgetAfter, PlanDueAfter: planDue,
		MaterialPath: strings.TrimSpace(payload.MaterialPath), ApplicantID: user.ID,
		DtInstanceID: instanceID, DtProcessCode: a.cfg.ChangeProcessCode,
		BeforeValue: string(beforeJSON), AfterValue: string(afterJSON),
	})
	if err != nil {
		if revokeErr := a.approval.TerminateProcess(r.Context(), instanceID, "本地变更创建失败，自动撤销"); revokeErr != nil {
			a.log.Error("撤销变更审批失败，需人工处理", "instance", instanceID, "error", revokeErr)
		}
		a.fail(w, "发起变更失败", err)
		return
	}
	a.writeAudit(r, "change.create", "change", id, map[string]any{"project": projectID, "instance": instanceID})
	created, err := a.store.GetChange(r.Context(), id)
	if err != nil {
		a.fail(w, "读取变更失败", err)
		return
	}
	writeOK(w, map[string]any{"change": changeView(created), "instance_id": instanceID})
}

func isValidChangeType(t string) bool {
	return t == "经费变更" || t == "计划变更" || t == "其他变更"
}

// GET /api/projects/{id}/changes
func (a *API) listProjectChanges(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "mychange")
	if !ok {
		return
	}
	projectID := r.PathValue("id")
	canView, err := a.canManageProject(r, user, projectID)
	if err != nil {
		a.fail(w, "校验项目权限失败", err)
		return
	}
	if !canView {
		member, err := a.store.IsProjectMember(r.Context(), projectID, user.ID)
		if err != nil {
			a.fail(w, "校验项目成员失败", err)
			return
		}
		if !member {
			writeError(w, http.StatusForbidden, "FORBIDDEN", "无权查看该项目变更", nil)
			return
		}
	}
	items, err := a.store.ListChanges(r.Context(), store.ChangeFilter{ProjectID: projectID})
	if err != nil {
		a.fail(w, "读取变更失败", err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for i := range items {
		out = append(out, changeView(&items[i]))
	}
	writeOK(w, map[string]any{"items": out})
}

// GET /api/changes 跨项目变更列表。
func (a *API) listChanges(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "mychange")
	if !ok {
		return
	}
	f := store.ChangeFilter{Status: strings.TrimSpace(r.URL.Query().Get("status"))}
	if !isBoss(user) {
		f.ViewerID = user.ID
	}
	items, err := a.store.ListChanges(r.Context(), f)
	if err != nil {
		a.fail(w, "读取变更失败", err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	pending := 0
	for i := range items {
		if items[i].ApprovalStatus == "审批中" {
			pending++
		}
		out = append(out, changeView(&items[i]))
	}

	// 发起变更：部门经理，或至少有一个自己担任项目经理的项目
	canCreate := isBoss(user)
	if !canCreate {
		if ids, err := a.store.ProjectIDsOfManager(r.Context(), user.ID); err != nil {
			a.fail(w, "校验发起权限失败", err)
			return
		} else if len(ids) > 0 {
			canCreate = true
		}
	}

	writeOK(w, map[string]any{
		"items":      out,
		"stats":      map[string]any{"total": len(items), "pending": pending},
		"can_create": canCreate,
	})
}

// GET /api/changes/{id} 变更详情（含实时审批状态）。
func (a *API) getChange(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "mychange")
	if !ok {
		return
	}
	c, err := a.store.GetChange(r.Context(), r.PathValue("id"))
	if err != nil || c == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "变更不存在", nil)
		return
	}
	allowed, err := a.canManageProject(r, user, c.ProjectID)
	if err != nil {
		a.fail(w, "校验项目权限失败", err)
		return
	}
	if !allowed {
		member, err := a.store.IsProjectMember(r.Context(), c.ProjectID, user.ID)
		if err != nil || !member {
			writeError(w, http.StatusForbidden, "FORBIDDEN", "无权查看该变更", nil)
			return
		}
	}
	var approval any
	if c.DtInstanceID != "" && a.approval != nil {
		inst, err := a.approval.GetInstance(r.Context(), c.DtInstanceID)
		if err == nil {
			ops := make([]map[string]any, 0, len(inst.Operations))
			for _, op := range inst.Operations {
				ops = append(ops, map[string]any{
					"user_id": op.UserID, "date": op.Date, "type": op.Type,
					"result": op.Result, "remark": op.Remark, "show_name": op.ShowName,
				})
			}
			approval = map[string]any{
				"instance_id": inst.ID, "title": inst.Title, "status": inst.Status,
				"result": inst.Result, "business_id": inst.BusinessID,
				"create_time": inst.CreateTime, "operations": ops,
				"display_status": changeStatusText(inst.Status, inst.Result),
			}
		}
	}
	writeOK(w, map[string]any{"change": changeView(c), "approval": approval})
}

// changeStatusText 钉钉实例状态 → 展示口径。
func changeStatusText(status, result string) string {
	switch strings.ToUpper(status) {
	case "RUNNING":
		return "审批中"
	case "COMPLETED":
		if strings.EqualFold(result, "refuse") {
			return "已否决"
		}
		return "已通过"
	case "TERMINATED":
		return "已撤销"
	default:
		return status
	}
}
