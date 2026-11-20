package api

import (
	"net/http"
	"strings"
	"time"

	"nwbd-dev-workstation/internal/domain"
	"nwbd-dev-workstation/internal/metrics"
	"nwbd-dev-workstation/internal/store"
)

// taskView 是看板接口的任务结构。
func taskView(t *store.BoardTask) map[string]any {
	var result any
	if t.ResultSummary != "" || len(t.Artifacts) > 0 {
		result = map[string]any{
			"summary":     t.ResultSummary,
			"artifacts":   t.Artifacts,
			"agent":       t.ResultAgent,
			"finished_at": t.FinishedAt,
		}
	}
	// 进行中但长时间没有新事件 → 可能已中断
	stalled := false
	if t.Status == "进行中" {
		ref := t.UpdatedAt
		if t.LastEventAt != nil && t.LastEventAt.After(ref) {
			ref = *t.LastEventAt
		}
		stalled = time.Since(ref) > 15*time.Minute
	}
	return map[string]any{
		"id":                   t.ID,
		"project_id":           t.ProjectID,
		"project_name":         t.ProjectName,
		"stage":                t.Stage,
		"name":                 t.Name,
		"description":          t.Description,
		"acceptance":           t.Acceptance,
		"priority":             t.Priority,
		"tags":                 t.Tags,
		"links":                t.Links,
		"owner_id":             t.OwnerID,
		"owner_name":           t.OwnerName,
		"job_type":             t.JobType,
		"plan_start":           t.PlanStart,
		"plan_due":             t.PlanDue,
		"actual_done":          t.ActualDone,
		"status":               t.Status,
		"estimate_hours":       t.EstimateHours,
		"actual_hours":         t.ActualHours,
		"overdue":              metrics.IsTaskOverdue(t.Status, t.PlanDue, time.Now()),
		"result":               result,
		"skill":                skillView(t),
		"skills":               skillsView(t),
		"attempt_count":        t.AttemptCount,
		"last_event_at":        t.LastEventAt,
		"stalled":              stalled,
		"updated_at":           t.UpdatedAt,
		"created_at":           t.CreatedAt,
		"created_by":           t.CreatedBy,
		"creator_name":         t.CreatorName,
		"task_type":            t.TaskType,
		"task_type_code":       t.TaskTypeCode,
		"archive_folder":       t.ArchiveFolder,
		"subtask_total":        t.SubtaskTotal,
		"subtask_done":         t.SubtaskDone,
		"approval_instance_id": t.ApprovalInstanceID,
		"approval_status":      t.ApprovalStatus,
	}
}

func skillView(t *store.BoardTask) any {
	// 兼容字段：旧脚本只认 skill，取第一个绑定的技能。
	if len(t.Skills) > 0 {
		return map[string]any{"name": t.Skills[0].Name, "version": t.Skills[0].Version}
	}
	if t.SkillName == "" {
		return nil
	}
	return map[string]any{"name": t.SkillName, "version": t.SkillVersion}
}

// skillsView 任务绑定的多个技能（总是数组，空时返回 []）。
func skillsView(t *store.BoardTask) []store.TaskSkill {
	if t.Skills == nil {
		return []store.TaskSkill{}
	}
	return t.Skills
}

// normalizeSkills 规整技能列表：去空白、按名称去重、最多 10 个；nil 表示调用方未提交该字段。
func normalizeSkills(in *[]store.TaskSkill) *[]store.TaskSkill {
	if in == nil {
		return nil
	}
	out := make([]store.TaskSkill, 0, len(*in))
	seen := map[string]bool{}
	for _, sk := range *in {
		name := strings.TrimSpace(sk.Name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, store.TaskSkill{Name: name, Version: sk.Version})
		if len(out) >= 10 {
			break
		}
	}
	return &out
}

// GET /api/my/tasks
func (a *API) listMyTasks(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "mytask")
	if !ok {
		return
	}
	filter := store.BoardFilter{
		OwnerID: user.ID,
		Status:  strings.TrimSpace(r.URL.Query().Get("status")),
		Keyword: strings.TrimSpace(r.URL.Query().Get("keyword")),
		All:     isBoss(user) && r.URL.Query().Get("scope") == "all",
	}
	items, stats, err := a.store.ListBoardTasks(r.Context(), filter)
	if err != nil {
		a.fail(w, "读取任务失败", err)
		return
	}
	views := make([]map[string]any, 0, len(items))
	for i := range items {
		views = append(views, taskView(&items[i]))
	}
	writeOK(w, map[string]any{"items": views, "stats": stats})
}

// GET /api/my/tasks/{id}
func (a *API) getMyTask(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "mytask")
	if !ok {
		return
	}
	task, err := a.store.GetBoardTask(r.Context(), r.PathValue("id"))
	if err != nil {
		a.fail(w, "读取任务失败", err)
		return
	}
	if task == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "任务不存在", nil)
		return
	}
	canView, err := a.canViewTask(r, user, task)
	if err != nil {
		a.fail(w, "校验任务权限失败", err)
		return
	}
	if !canView {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "只能查看自己名下的任务", nil)
		return
	}
	events, err := a.store.ListTaskEvents(r.Context(), task.ID)
	if err != nil {
		a.fail(w, "读取任务时间线失败", err)
		return
	}
	artifacts, err := a.store.ListArtifacts(r.Context(), task.ID)
	if err != nil {
		a.fail(w, "读取产出失败", err)
		return
	}
	subtasks, err := a.store.ListSubtasks(r.Context(), task.ID)
	if err != nil {
		a.fail(w, "读取子任务失败", err)
		return
	}
	writeOK(w, map[string]any{
		"task":         taskView(task),
		"events":       events,
		"artifacts":    artifactsView(artifacts),
		"subtasks":     subtasks,
		"can_edit":     a.canEditTask(user, task),
		"can_operate":  task.OwnerID == user.ID,
		"is_owner":     task.OwnerID == user.ID,
		"creator_name": task.CreatorName,
	})
}

// canViewTask 负责人 / 本项目 PM / 部门经理。
func (a *API) canViewTask(r *http.Request, user *store.Person, task *store.BoardTask) (bool, error) {
	if isBoss(user) || task.OwnerID == user.ID {
		return true, nil
	}
	return a.store.IsProjectManager(r.Context(), task.ProjectID, user.ID)
}

// canEditTask 只有任务下发人可以编辑内容；历史旧数据没有下发人时由部门经理兜底。
func (a *API) canEditTask(user *store.Person, task *store.BoardTask) bool {
	if task.CreatedBy == "" {
		return isBoss(user)
	}
	return task.CreatedBy == user.ID
}

// PATCH /api/my/tasks/{id}/status
func (a *API) updateMyTaskStatus(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "mytask")
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
	task, err := a.store.GetBoardTask(r.Context(), r.PathValue("id"))
	if err != nil || task == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "任务不存在", nil)
		return
	}
	if task.OwnerID != user.ID {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "任务状态由负责人操作", nil)
		return
	}
	if payload.Status == "已取消" {
		manager, _ := a.store.IsProjectManager(r.Context(), task.ProjectID, user.ID)
		if !manager && !isBoss(user) {
			writeError(w, http.StatusForbidden, "FORBIDDEN", "取消任务需要项目经理操作", nil)
			return
		}
	}

	if err := a.store.UpdateTaskStatus(r.Context(), task.ID, payload.Status, nil); err != nil {
		a.fail(w, "更新任务状态失败", err)
		return
	}
	a.store.AddTaskEvent(r.Context(), task.ID, "status",
		"状态 "+task.Status+" → "+payload.Status, user.ID, user.Name, "info", task.AttemptCount)
	writeOK(w, map[string]any{"ok": true})
}

// POST /api/my/tasks/{id}/log 记录一条进展。
func (a *API) logMyTask(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "mytask")
	if !ok {
		return
	}
	var payload struct {
		Message string `json:"message"`
	}
	if err := decodeJSON(r, &payload); err != nil || strings.TrimSpace(payload.Message) == "" {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "进展内容不能为空", nil)
		return
	}
	task, err := a.store.GetBoardTask(r.Context(), r.PathValue("id"))
	if err != nil || task == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "任务不存在", nil)
		return
	}
	if task.OwnerID != user.ID && !isBoss(user) {
		manager, _ := a.store.IsProjectManager(r.Context(), task.ProjectID, user.ID)
		if !manager {
			writeError(w, http.StatusForbidden, "FORBIDDEN", "只能记录自己名下任务的进展", nil)
			return
		}
	}
	if err := a.store.AddTaskEvent(r.Context(), task.ID, "log", payload.Message, user.ID, user.Name, "info", task.AttemptCount); err != nil {
		a.fail(w, "记录进展失败", err)
		return
	}
	writeOK(w, map[string]any{"ok": true})
}

// POST /api/my/tasks/{id}/complete 提交结果并完成。
func (a *API) completeMyTask(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "mytask")
	if !ok {
		return
	}
	var payload struct {
		Summary   string   `json:"summary"`
		Artifacts []string `json:"artifacts"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "请求体不合法", nil)
		return
	}
	task, err := a.store.GetBoardTask(r.Context(), r.PathValue("id"))
	if err != nil || task == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "任务不存在", nil)
		return
	}
	if task.OwnerID != user.ID && !isBoss(user) {
		manager, _ := a.store.IsProjectManager(r.Context(), task.ProjectID, user.ID)
		if !manager {
			writeError(w, http.StatusForbidden, "FORBIDDEN", "只能完成自己名下的任务", nil)
			return
		}
	}

	artifacts := payload.Artifacts
	if artifacts == nil {
		artifacts = []string{}
	}
	if err := a.store.CompleteTask(r.Context(), task.ID, "已完成", payload.Summary, artifacts, user.Name); err != nil {
		a.fail(w, "提交结果失败", err)
		return
	}
	a.store.AddTaskEvent(r.Context(), task.ID, "done", "任务已完成", user.ID, user.Name, "success", task.AttemptCount)
	writeOK(w, map[string]any{"ok": true})
}

// PATCH /api/my/tasks/{id} 编辑任务内容。
func (a *API) updateMyTask(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "mytask")
	if !ok {
		return
	}
	task, err := a.store.GetBoardTask(r.Context(), r.PathValue("id"))
	if err != nil || task == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "任务不存在", nil)
		return
	}
	if !a.canEditTask(user, task) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "只有任务下发人可以编辑任务内容", nil)
		return
	}

	var payload struct {
		Name          *string            `json:"name"`
		Description   *string            `json:"description"`
		Acceptance    *[]string          `json:"acceptance"`
		Priority      *string            `json:"priority"`
		Tags          *[]string          `json:"tags"`
		Links         *[]store.TaskLink  `json:"links"`
		Skills        *[]store.TaskSkill `json:"skills"`
		PlanStart     *string            `json:"plan_start"`
		PlanDue       *string            `json:"plan_due"`
		EstimateHours *float64           `json:"estimate_hours"`
		Stage         *string            `json:"stage"`
		OwnerID       *string            `json:"owner_id"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "请求体不合法", nil)
		return
	}
	if payload.Priority != nil && !domain.IsValid([]string{"P0", "P1", "P2"}, *payload.Priority) {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_PRIORITY", "优先级取值不合法", nil)
		return
	}

	in := store.TaskEditInput{
		Name: payload.Name, Description: payload.Description, Acceptance: payload.Acceptance,
		Priority: payload.Priority, Tags: payload.Tags, Links: payload.Links,
		Skills:        normalizeSkills(payload.Skills),
		EstimateHours: payload.EstimateHours, Stage: payload.Stage,
	}
	if payload.PlanStart != nil {
		if in.PlanStart, err = parseDate(*payload.PlanStart); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "INVALID_DATE", "计划开始格式应为 YYYY-MM-DD", nil)
			return
		}
	}
	if payload.PlanDue != nil {
		if in.PlanDue, err = parseDate(*payload.PlanDue); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "INVALID_DATE", "计划完成格式应为 YYYY-MM-DD", nil)
			return
		}
	}

	// 改负责人 = 转派：必须是本项目参与中的成员。
	transferred := false
	newOwnerName := ""
	if payload.OwnerID != nil {
		ownerID := strings.TrimSpace(*payload.OwnerID)
		if ownerID != task.OwnerID {
			owner, err := a.store.GetPerson(r.Context(), ownerID)
			if err != nil || owner == nil {
				writeError(w, http.StatusUnprocessableEntity, "INVALID_OWNER", "负责人不存在", nil)
				return
			}
			member, err := a.store.IsProjectMember(r.Context(), task.ProjectID, ownerID)
			if err != nil {
				a.fail(w, "校验项目成员失败", err)
				return
			}
			if !member {
				writeError(w, http.StatusUnprocessableEntity, "NOT_MEMBER", "负责人必须是本项目的参与成员", nil)
				return
			}
			in.OwnerID = &ownerID
			in.OwnerJobType = &owner.JobType
			transferred = true
			newOwnerName = owner.Name
		}
	}

	if err := a.store.UpdateTaskContent(r.Context(), task.ID, in); err != nil {
		a.fail(w, "保存任务失败", err)
		return
	}
	a.store.AddTaskEvent(r.Context(), task.ID, "edit", "任务内容已更新", user.ID, user.Name, "info", task.AttemptCount)
	if transferred {
		a.store.AddTaskEvent(r.Context(), task.ID, "transfer",
			"任务转派："+task.OwnerName+" → "+newOwnerName, user.ID, user.Name, "info", task.AttemptCount)
		a.writeAudit(r, "task.transfer", "task", task.ID,
			map[string]any{"from": task.OwnerID, "to": *in.OwnerID})
	}
	writeOK(w, map[string]any{"ok": true, "transferred": transferred})
}
