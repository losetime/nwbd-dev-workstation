package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"nwbd-dev-workstation/internal/domain"
	"nwbd-dev-workstation/internal/store"
)

// parseDate 解析 YYYY-MM-DD；空串返回 nil。
func parseDate(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// PATCH /api/projects/{id} 编辑项目信息（仅部门经理）。
func (a *API) updateProject(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "myproj")
	if !ok {
		return
	}
	if !isBoss(user) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "只有部门经理可以编辑项目信息", nil)
		return
	}
	id := r.PathValue("id")

	var payload struct {
		Name           *string  `json:"name"`
		Customer       *string  `json:"customer"`
		ContractAmount *float64 `json:"contract_amount"`
		PlanStart      *string  `json:"plan_start"`
		PlanEnd        *string  `json:"plan_end"`
		Budget         *float64 `json:"budget"`
		LaborBudget    *float64 `json:"labor_budget"`
		LaborRate      *float64 `json:"labor_rate"`
		Remark         *string  `json:"remark"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "请求体不合法", nil)
		return
	}
	if payload.Name != nil {
		name := strings.TrimSpace(*payload.Name)
		if name == "" {
			writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "项目名称不能为空", nil)
			return
		}
		payload.Name = &name
	}

	in := store.ProjectUpdate{
		Name:           payload.Name,
		Customer:       payload.Customer,
		ContractAmount: payload.ContractAmount,
		Budget:         payload.Budget,
		LaborBudget:    payload.LaborBudget,
		LaborRate:      payload.LaborRate,
		Remark:         payload.Remark,
	}
	if payload.PlanStart != nil {
		d, err := parseDate(*payload.PlanStart)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "INVALID_DATE", "计划开始日期格式应为 YYYY-MM-DD", nil)
			return
		}
		in.PlanStart = d
	}
	if payload.PlanEnd != nil {
		d, err := parseDate(*payload.PlanEnd)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "INVALID_DATE", "计划结束日期格式应为 YYYY-MM-DD", nil)
			return
		}
		in.PlanEnd = d
	}

	if err := a.store.UpdateProject(r.Context(), id, in); err != nil {
		a.fail(w, "保存项目信息失败", err)
		return
	}
	a.writeAudit(r, "project.update", "project", id, map[string]any{"actor": user.Name})
	project, _ := a.store.GetProject(r.Context(), id)
	writeOK(w, projectView(project))
}

// PATCH /api/projects/{id}/status 手动改项目状态（仅部门经理）。
func (a *API) updateProjectStatus(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "myproj")
	if !ok {
		return
	}
	if !isBoss(user) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "只有部门经理可以修改项目状态", nil)
		return
	}
	id := r.PathValue("id")

	var payload struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(r, &payload); err != nil || !domain.IsValid(domain.ProjectStatuses, payload.Status) {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_STATUS", "项目状态取值不合法", nil)
		return
	}
	if err := a.store.UpdateProjectStatus(r.Context(), id, payload.Status); err != nil {
		a.fail(w, "更新项目状态失败", err)
		return
	}
	a.writeAudit(r, "project.status", "project", id, map[string]any{"status": payload.Status, "actor": user.Name})
	project, _ := a.store.GetProject(r.Context(), id)
	writeOK(w, projectView(project))
}

// PATCH /api/projects/{id}/risk-flag 部门经理标注风险标识。
func (a *API) updateRiskFlag(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "myproj")
	if !ok {
		return
	}
	if !isBoss(user) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "只有部门经理可以标记风险标识", nil)
		return
	}
	var payload struct {
		RiskFlag string `json:"risk_flag"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "请求体不合法", nil)
		return
	}
	if payload.RiskFlag != "" && !domain.IsValid(domain.RiskFlags, payload.RiskFlag) {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_FLAG", "风险标识取值不合法", nil)
		return
	}
	id := r.PathValue("id")
	if err := a.store.UpdateRiskFlag(r.Context(), id, payload.RiskFlag); err != nil {
		a.fail(w, "更新风险标识失败", err)
		return
	}
	a.writeAudit(r, "project.risk_flag", "project", id, map[string]any{"flag": payload.RiskFlag, "actor": user.Name})
	project, _ := a.store.GetProject(r.Context(), id)
	writeOK(w, projectView(project))
}

// PUT /api/projects/{id}/managers 更换项目经理（仅部门经理）。
func (a *API) replaceManagers(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "myproj")
	if !ok {
		return
	}
	if !isBoss(user) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "只有部门经理可以更换项目经理", nil)
		return
	}
	var payload struct {
		ManagerIDs []string `json:"manager_ids"`
	}
	if err := decodeJSON(r, &payload); err != nil || len(payload.ManagerIDs) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "至少选择一位项目经理", nil)
		return
	}
	for _, id := range payload.ManagerIDs {
		p, err := a.store.GetPerson(r.Context(), id)
		if err != nil {
			a.fail(w, "读取项目经理失败", err)
			return
		}
		if p == nil || p.Status != "在职" || (p.JobType != "项目经理" && p.JobType != "部门经理") {
			writeError(w, http.StatusUnprocessableEntity, "INVALID_MANAGER", "项目经理必须是职位含「项目经理」的在职人员", nil)
			return
		}
	}

	projectID := r.PathValue("id")
	if err := a.store.ReplaceManagers(r.Context(), projectID, payload.ManagerIDs); err != nil {
		a.fail(w, "更换项目经理失败", err)
		return
	}
	a.writeAudit(r, "project.managers", "project", projectID, map[string]any{"managers": payload.ManagerIDs})
	project, _ := a.store.GetProject(r.Context(), projectID)
	writeOK(w, projectView(project))
}

// POST /api/projects/{id}/milestones/plan 批量填写节点计划。
func (a *API) saveMilestonePlan(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "myproj")
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
		writeError(w, http.StatusForbidden, "FORBIDDEN", "只有部门经理或本项目的项目经理可以填写节点计划", nil)
		return
	}

	var payload struct {
		Items []struct {
			ID          string `json:"id"`
			PlanDue     string `json:"plan_due"`
			ContractDue string `json:"contract_due"`
		} `json:"items"`
	}
	if err := decodeJSON(r, &payload); err != nil || len(payload.Items) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "缺少计划行", nil)
		return
	}

	plans := make([]store.MilestonePlanInput, 0, len(payload.Items))
	for _, item := range payload.Items {
		planDue, err := parseDate(item.PlanDue)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "INVALID_DATE", "计划完成日格式应为 YYYY-MM-DD", nil)
			return
		}
		contractDue, err := parseDate(item.ContractDue)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "INVALID_DATE", "合同要求完成日格式应为 YYYY-MM-DD", nil)
			return
		}
		plans = append(plans, store.MilestonePlanInput{ID: item.ID, PlanDue: planDue, ContractDue: contractDue})
	}

	updated, err := a.store.UpdateMilestonePlans(r.Context(), projectID, plans)
	if err != nil {
		a.fail(w, "保存节点计划失败", err)
		return
	}
	a.writeAudit(r, "milestone.plan", "project", projectID, map[string]any{"updated": updated, "actor": user.Name})
	writeOK(w, map[string]any{"updated": updated})
}

// PATCH /api/projects/{id}/milestones/{milestoneID} 单条里程碑编辑。
func (a *API) updateMilestone(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "myproj")
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
		writeError(w, http.StatusForbidden, "FORBIDDEN", "只有部门经理或本项目的项目经理可以编辑里程碑", nil)
		return
	}

	var payload struct {
		PlanDue     *string `json:"plan_due"`
		ContractDue *string `json:"contract_due"`
		Status      *string `json:"status"`
		ActualDone  *string `json:"actual_done"`
		Reviewer    *string `json:"reviewer"`
		ReviewDate  *string `json:"review_date"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "请求体不合法", nil)
		return
	}
	if payload.Status != nil && !domain.IsValid(domain.MilestoneStatuses, *payload.Status) {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_STATUS", "里程碑状态取值不合法", nil)
		return
	}

	in := store.MilestoneUpdate{Status: payload.Status, Reviewer: payload.Reviewer}
	if payload.PlanDue != nil {
		if in.PlanDue, err = parseDate(*payload.PlanDue); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "INVALID_DATE", "计划完成日格式应为 YYYY-MM-DD", nil)
			return
		}
	}
	if payload.ContractDue != nil {
		if in.ContractDue, err = parseDate(*payload.ContractDue); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "INVALID_DATE", "合同要求完成日格式应为 YYYY-MM-DD", nil)
			return
		}
	}
	if payload.ActualDone != nil {
		if in.ActualDone, err = parseDate(*payload.ActualDone); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "INVALID_DATE", "实际完成日格式应为 YYYY-MM-DD", nil)
			return
		}
	}
	if payload.ReviewDate != nil {
		if in.ReviewDate, err = parseDate(*payload.ReviewDate); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "INVALID_DATE", "评审日期格式应为 YYYY-MM-DD", nil)
			return
		}
	}

	if err := a.store.UpdateMilestone(r.Context(), projectID, r.PathValue("milestoneID"), in); err != nil {
		if errors.Is(err, store.ErrMilestoneLocked) {
			writeError(w, http.StatusConflict, "MILESTONE_LOCKED", "该阶段已锁定，修改请走变更单", nil)
			return
		}
		a.fail(w, "保存里程碑失败", err)
		return
	}
	a.writeAudit(r, "milestone.update", "project", projectID, map[string]any{"milestone": r.PathValue("milestoneID")})
	project, _ := a.store.GetProject(r.Context(), projectID)
	writeOK(w, projectView(project))
}

// taskSkillsOf 合并新式 skills 数组与旧式单技能字段（老客户端兼容）。
func taskSkillsOf(list []store.TaskSkill, legacyName string, legacyVersion *int) []store.TaskSkill {
	if len(list) > 0 {
		out := list
		normalized := normalizeSkills(&out)
		if normalized != nil {
			return *normalized
		}
		return []store.TaskSkill{}
	}
	name := strings.TrimSpace(legacyName)
	if name == "" {
		return []store.TaskSkill{}
	}
	return []store.TaskSkill{{Name: name, Version: legacyVersion}}
}

// DELETE /api/projects/{id} 软删除项目（仅部门经理）。
func (a *API) deleteProject(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "myproj")
	if !ok {
		return
	}
	if !isBoss(user) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "只有部门经理可以删除项目", nil)
		return
	}
	id := r.PathValue("id")
	project, err := a.store.GetProject(r.Context(), id)
	if err != nil || project == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "项目不存在", nil)
		return
	}
	if err := a.store.DeleteProject(r.Context(), id, user.ID); err != nil {
		a.fail(w, "删除项目失败", err)
		return
	}
	a.writeAudit(r, "project.delete", "project", id, map[string]any{"name": project.Name, "code": project.Code})
	writeOK(w, map[string]any{"ok": true})
}

// POST /api/projects/{id}/tasks 下发任务。
func (a *API) createTask(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "myproj")
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
		writeError(w, http.StatusForbidden, "FORBIDDEN", "只有部门经理或本项目的项目经理可以下发任务", nil)
		return
	}

	var payload struct {
		Name          string            `json:"name"`
		Description   string            `json:"description"`
		Acceptance    []string          `json:"acceptance"`
		Priority      string            `json:"priority"`
		Tags          []string          `json:"tags"`
		Skills        []store.TaskSkill `json:"skills"`
		SkillName     string            `json:"skill_name"`
		SkillVersion  *int              `json:"skill_version"`
		Stage         string            `json:"stage"`
		OwnerID       string            `json:"owner_id"`
		PlanStart     string            `json:"plan_start"`
		PlanDue       string            `json:"plan_due"`
		EstimateHours *float64          `json:"estimate_hours"`
		TaskTypeCode  string            `json:"task_type_code"`
		Approval      planApprovalInput `json:"approval"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "请求体不合法", nil)
		return
	}
	payload.Name = strings.TrimSpace(payload.Name)
	if payload.Name == "" || payload.OwnerID == "" {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "任务名称与负责人必填", nil)
		return
	}
	if payload.Priority == "" {
		payload.Priority = "P1"
	}
	if !domain.IsValid([]string{"P0", "P1", "P2"}, payload.Priority) {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_PRIORITY", "优先级取值不合法", nil)
		return
	}

	owner, err := a.store.GetPerson(r.Context(), payload.OwnerID)
	if err != nil || owner == nil {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_OWNER", "负责人不存在", nil)
		return
	}
	member, err := a.store.IsProjectMember(r.Context(), projectID, payload.OwnerID)
	if err != nil {
		a.fail(w, "校验项目成员失败", err)
		return
	}
	if !member {
		writeError(w, http.StatusUnprocessableEntity, "NOT_MEMBER", "负责人必须是本项目的参与成员", nil)
		return
	}

	start, err := parseDate(payload.PlanStart)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_DATE", "计划开始格式应为 YYYY-MM-DD", nil)
		return
	}
	due, err := parseDate(payload.PlanDue)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_DATE", "计划完成格式应为 YYYY-MM-DD", nil)
		return
	}
	if start != nil && due != nil && due.Before(*start) {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_RANGE", "计划完成不能早于计划开始", nil)
		return
	}

	hours := 0.0
	if payload.EstimateHours != nil {
		hours = *payload.EstimateHours
	}

	taskTypeCode := strings.TrimSpace(payload.TaskTypeCode)
	if taskTypeCode == "" {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_TASK_TYPE", "请选择任务类型", nil)
		return
	}
	taskType, err := a.store.GetTaskType(r.Context(), taskTypeCode)
	if err != nil {
		a.fail(w, "读取任务类型失败", err)
		return
	}
	if taskType == nil {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_TASK_TYPE", "任务类型不合法", nil)
		return
	}

	// 项目策划：只有部门经理可选；先发起钉钉审批，成功后再落本地任务
	instanceID, approvalStatus := "", ""
	if taskType.Code == store.TaskTypeProjectPlanCode {
		if !isBoss(user) {
			writeError(w, http.StatusForbidden, "FORBIDDEN", "只有部门经理可以下发「项目策划」任务", nil)
			return
		}
		project, err := a.store.GetProject(r.Context(), projectID)
		if err != nil || project == nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "项目不存在", nil)
			return
		}
		// 项目已关联合同签订单时直接复用，前端不用再选
		approvalIn := payload.Approval
		if strings.TrimSpace(project.ContractInstanceID) != "" {
			approvalIn.ContractInstanceID = project.ContractInstanceID
		}
		instanceID, err = a.startProjectPlanApproval(r.Context(), user, project, owner, approvalIn)
		if err != nil {
			a.log.Warn("发起项目策划审批失败", "project", projectID, "error", err)
			writeError(w, http.StatusUnprocessableEntity, "APPROVAL_START_FAILED", err.Error(), nil)
			return
		}
		approvalStatus = "审批中"
	}

	id, err := a.store.CreateTask(r.Context(), projectID, store.TaskInput{
		Name: payload.Name, Description: payload.Description, Acceptance: payload.Acceptance,
		Priority: payload.Priority, Tags: payload.Tags,
		Skills: taskSkillsOf(payload.Skills, payload.SkillName, payload.SkillVersion),
		Stage:  payload.Stage, OwnerID: payload.OwnerID, JobType: owner.JobType,
		PlanStart: start, PlanDue: due, EstimateHours: hours, CreatedBy: user.ID,
		TaskType: taskType.Name, TaskTypeCode: taskType.Code, ArchiveFolder: taskType.ArchiveFolder,
		ApprovalInstanceID: instanceID, ApprovalStatus: approvalStatus,
	})
	if err != nil {
		// 钉钉单已发起但本地落库失败：撤销审批，避免孤儿单
		if instanceID != "" && a.approval != nil {
			if revokeErr := a.approval.TerminateProcess(r.Context(), instanceID, "本地任务创建失败，自动撤销"); revokeErr != nil {
				a.log.Error("撤销审批失败，需人工处理", "instance", instanceID, "error", revokeErr)
			}
		}
		a.fail(w, "下发任务失败", err)
		return
	}
	a.writeAudit(r, "task.create", "task", id, map[string]any{
		"project": projectID, "name": payload.Name, "task_type": taskType.Name,
		"task_type_code": taskType.Code, "archive_folder": taskType.ArchiveFolder, "approval": instanceID,
	})
	writeOK(w, map[string]any{"id": id, "approval_instance_id": instanceID})
}

// PATCH /api/tasks/{id}/status 任务状态流转（负责人或项目经理）。
func (a *API) updateTaskStatus(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "myproj")
	if !ok {
		return
	}
	var payload struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(r, &payload); err != nil || !domain.IsValid(domain.TaskStatuses, payload.Status) {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_STATUS", "任务状态取值不合法", nil)
		return
	}

	task, err := a.store.GetTask(r.Context(), r.PathValue("id"))
	if err != nil || task == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "任务不存在", nil)
		return
	}
	// 延期/完成只能由任务负责人操作（项目经理只负责下发）
	if task.OwnerID != user.ID {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "任务状态由负责人操作，项目经理只能下发任务", nil)
		return
	}

	var done *time.Time
	if payload.Status == "已完成" {
		now := time.Now()
		done = &now
	}
	if err := a.store.UpdateTaskStatus(r.Context(), task.ID, payload.Status, done); err != nil {
		a.fail(w, "更新任务状态失败", err)
		return
	}
	a.writeAudit(r, "task.status", "task", task.ID, map[string]any{"status": payload.Status})
	writeOK(w, map[string]any{"ok": true})
}

// GET /api/projects/{id}/candidates 可加入项目的候选人。
func (a *API) listMemberCandidates(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "myproj")
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
		writeError(w, http.StatusForbidden, "FORBIDDEN", "只有部门经理或本项目的项目经理可以增加成员", nil)
		return
	}
	people, err := a.store.ListMemberCandidates(r.Context(), projectID)
	if err != nil {
		a.fail(w, "读取候选人失败", err)
		return
	}
	items := make([]map[string]any, 0, len(people))
	for i := range people {
		p := &people[i]
		items = append(items, map[string]any{
			"id": p.ID, "name": p.Name, "job_type": p.JobType, "dept": p.Dept, "title": p.Title,
		})
	}
	writeOK(w, map[string]any{"items": items, "total": len(items)})
}

// POST /api/projects/{id}/members 增加成员（仅部门经理）。
func (a *API) addMembers(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "myproj")
	if !ok {
		return
	}
	if !isBoss(user) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "只有部门经理可以增加成员", nil)
		return
	}
	projectID := r.PathValue("id")

	var payload struct {
		UserIDs []string `json:"user_ids"`
		Role    string   `json:"project_role"`
	}
	if err := decodeJSON(r, &payload); err != nil || len(payload.UserIDs) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "请选择成员", nil)
		return
	}
	if !domain.IsValid(domain.ProjectRoles, payload.Role) || payload.Role == "项目经理" {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_ROLE", "项目角色不合法（项目经理请用「更换项目经理」）", nil)
		return
	}
	if err := a.store.AddMembers(r.Context(), projectID, payload.UserIDs, payload.Role); err != nil {
		a.fail(w, "增加成员失败", err)
		return
	}
	a.writeAudit(r, "project.member.add", "project", projectID, map[string]any{"count": len(payload.UserIDs)})
	writeOK(w, map[string]any{"ok": true})
}

// PATCH /api/projects/{id}/members/{userID} 退出 / 恢复参与。
func (a *API) setMemberStatus(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "myproj")
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
		writeError(w, http.StatusForbidden, "FORBIDDEN", "只有部门经理或本项目的项目经理可以调整成员", nil)
		return
	}
	var payload struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(r, &payload); err != nil || (payload.Status != "参与中" && payload.Status != "已退出") {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_STATUS", "状态只能是参与中或已退出", nil)
		return
	}
	// 退出前检查：该成员在本项目下还有未完成任务时一律拒绝（前端弹提示框告知，不可强制）。
	if payload.Status == "已退出" {
		names, err := a.store.OpenTaskNames(r.Context(), projectID, r.PathValue("userID"))
		if err != nil {
			a.fail(w, "检查成员任务失败", err)
			return
		}
		if len(names) > 0 {
			preview := names
			if len(preview) > 3 {
				preview = preview[:3]
			}
			msg := fmt.Sprintf("该成员还有 %d 项未完成任务：%s", len(names), strings.Join(preview, "、"))
			if len(names) > 3 {
				msg += " 等"
			}
			msg += "，不能退出项目。请待任务完成或转派后再操作。"
			writeError(w, http.StatusConflict, "HAS_OPEN_TASKS", msg, nil)
			return
		}
	}
	if err := a.store.SetMemberStatus(r.Context(), projectID, r.PathValue("userID"), payload.Status); err != nil {
		a.fail(w, "更新成员状态失败", err)
		return
	}
	a.writeAudit(r, "project.member.status", "project", projectID,
		map[string]any{"user": r.PathValue("userID"), "status": payload.Status})
	writeOK(w, map[string]any{"ok": true})
}
