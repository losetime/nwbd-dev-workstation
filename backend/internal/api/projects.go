package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"nwbd-dev-workstation/internal/domain"
	"nwbd-dev-workstation/internal/metrics"
	"nwbd-dev-workstation/internal/store"
)

// GET /api/projects
func (a *API) listProjects(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "myproj")
	if !ok {
		return
	}

	filter := store.ProjectFilter{
		Type:     strings.TrimSpace(r.URL.Query().Get("type")),
		Status:   strings.TrimSpace(r.URL.Query().Get("status")),
		RiskFlag: strings.TrimSpace(r.URL.Query().Get("risk_flag")),
		Keyword:  strings.TrimSpace(r.URL.Query().Get("keyword")),
	}
	// 部门经理看全部；其他人只看自己参与的项目
	if scope := strings.TrimSpace(r.URL.Query().Get("scope")); scope == "managed" {
		// 只列「我担任项目经理」的项目；部门经理仍看全部
		if !isBoss(user) {
			filter.ManagerID = user.ID
		}
	} else if !isBoss(user) {
		filter.ViewerID = user.ID
	}

	projects, err := a.store.ListProjects(r.Context(), filter)
	if err != nil {
		a.fail(w, "读取项目列表失败", err)
		return
	}

	items := make([]map[string]any, 0, len(projects))
	for i := range projects {
		items = append(items, projectView(&projects[i]))
	}
	writeOK(w, map[string]any{"items": items, "total": len(items)})
}

// POST /api/projects
func (a *API) createProject(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "myproj")
	if !ok {
		return
	}
	if !isBoss(user) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "只有部门经理可以新建项目", nil)
		return
	}

	var payload struct {
		Code           string   `json:"code"`
		Name           string   `json:"name"`
		Type           string   `json:"type"`
		Customer       string   `json:"customer"`
		ContractAmount *float64 `json:"contract_amount"`
		ManagerIDs     []string `json:"manager_ids"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "请求体不合法", nil)
		return
	}

	payload.Code = strings.TrimSpace(payload.Code)
	payload.Name = strings.TrimSpace(payload.Name)
	if payload.Code == "" || payload.Name == "" {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "项目编号与项目名称不能为空", nil)
		return
	}
	if !domain.IsValid(domain.ProjectTypes, payload.Type) {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_TYPE", "项目类型不合法", nil)
		return
	}
	if len(payload.ManagerIDs) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "至少选择一位项目经理", nil)
		return
	}

	// 校验项目经理：在职，且职位被判定为项目经理（部门经理可兼任）
	for _, id := range payload.ManagerIDs {
		p, err := a.store.GetPerson(r.Context(), id)
		if err != nil {
			a.fail(w, "读取项目经理失败", err)
			return
		}
		if p == nil || p.Status != "在职" {
			writeError(w, http.StatusUnprocessableEntity, "INVALID_MANAGER", "项目经理不存在或已离职", nil)
			return
		}
		if p.JobType != "项目经理" && p.JobType != "部门经理" {
			writeError(w, http.StatusUnprocessableEntity, "INVALID_MANAGER",
				fmt.Sprintf("%s 的职位不是项目经理，不能作为项目经理", p.Name), nil)
			return
		}
	}

	project, err := a.store.CreateProject(r.Context(), store.ProjectInput{
		Code:           payload.Code,
		Name:           payload.Name,
		Type:           payload.Type,
		Customer:       payload.Customer,
		ContractAmount: payload.ContractAmount,
		ManagerIDs:     payload.ManagerIDs,
		CreatedBy:      user.ID,
	}, domain.StageTemplates[payload.Type])
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeError(w, http.StatusConflict, "DUPLICATE_PROJECT", "项目编号或项目名称已存在", nil)
			return
		}
		a.fail(w, "新建项目失败", err)
		return
	}

	a.log.Info("项目已创建", "actor", user.Name, "project", project.Name, "type", project.Type)
	a.writeAudit(r, "project.create", "project", project.ID, map[string]any{
		"name": project.Name, "code": project.Code, "type": project.Type,
	})
	writeOK(w, projectView(project))
}

// GET /api/projects/{id}
func (a *API) getProject(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "myproj")
	if !ok {
		return
	}

	id := r.PathValue("id")
	project, err := a.store.GetProject(r.Context(), id)
	if err != nil {
		a.fail(w, "读取项目失败", err)
		return
	}
	if project == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "项目不存在", nil)
		return
	}
	if !isBoss(user) {
		member, err := a.store.IsProjectMember(r.Context(), id, user.ID)
		if err != nil {
			a.fail(w, "校验项目成员失败", err)
			return
		}
		if !member {
			writeError(w, http.StatusForbidden, "FORBIDDEN", "只能查看自己参与的项目", nil)
			return
		}
	}

	milestones, err := a.store.ListMilestones(r.Context(), id)
	if err != nil {
		a.fail(w, "读取里程碑失败", err)
		return
	}
	tasks, err := a.store.ListTasks(r.Context(), id)
	if err != nil {
		a.fail(w, "读取任务失败", err)
		return
	}
	members, err := a.store.ListMembers(r.Context(), id)
	if err != nil {
		a.fail(w, "读取成员失败", err)
		return
	}
	risks, err := a.store.ListRisks(r.Context(), id)
	if err != nil {
		a.fail(w, "读取风险失败", err)
		return
	}

	canManage, _ := a.store.IsProjectManager(r.Context(), id, user.ID)
	writeOK(w, map[string]any{
		"project":    projectView(project),
		"milestones": milestoneViews(milestones),
		"tasks":      taskViews(tasks),
		"members":    memberViews(members),
		"risks":      riskViews(risks),
		"can_manage": isBoss(user) || canManage,
	})
}

// canManageProject 部门经理，或本项目的项目经理。
func (a *API) canManageProject(r *http.Request, user *store.Person, projectID string) (bool, error) {
	if isBoss(user) {
		return true, nil
	}
	return a.store.IsProjectManager(r.Context(), projectID, user.ID)
}

// ---------- 视图 ----------

func projectView(p *store.Project) map[string]any {
	if p == nil {
		return map[string]any{}
	}
	progress := metrics.Progress(p.TaskTotal, p.TaskDone)
	done := 0
	if p.TaskDone > 0 {
		done = p.TaskDone
	}
	return map[string]any{
		"id":                   p.ID,
		"code":                 p.Code,
		"name":                 p.Name,
		"type":                 p.Type,
		"status":               p.Status,
		"customer":             p.Customer,
		"current_stage":        p.CurrentStage,
		"current_milestone":    p.CurrentMilestone,
		"milestone_status":     p.MilestoneStatus,
		"plan_start":           p.PlanStart,
		"plan_end":             p.PlanEnd,
		"actual_end":           p.ActualEnd,
		"contract_amount":      p.ContractAmount,
		"budget":               p.Budget,
		"cost_actual":          p.CostActual,
		"labor_budget":         p.LaborBudget,
		"labor_rate":           p.LaborRate,
		"risk_flag":            p.RiskFlag,
		"weekly_summary":       p.WeeklySummary,
		"summary_at":           p.SummaryAt,
		"remark":               p.Remark,
		"task_total":           p.TaskTotal,
		"task_done":            done,
		"open_risks":           p.OpenRisks,
		"member_count":         p.MemberCount,
		"managers":             p.Managers,
		"progress":             progress,
		"created_at":           p.CreatedAt,
		"contract_instance_id": p.ContractInstanceID,
		"contract_business_id": p.ContractBusinessID,
		"contract_status":      p.ContractStatus,
	}
}

func milestoneViews(items []store.Milestone) []map[string]any {
	now := time.Now()
	out := make([]map[string]any, 0, len(items))
	for i := range items {
		m := &items[i]
		out = append(out, map[string]any{
			"id":               m.ID,
			"stage":            m.Stage,
			"name":             m.Name,
			"plan_due":         m.PlanDue,
			"contract_due":     m.ContractDue,
			"actual_done":      m.ActualDone,
			"status":           m.Status,
			"reviewer":         m.Reviewer,
			"review_date":      m.ReviewDate,
			"version":          m.Version,
			"locked":           m.Locked,
			"plan_overdue":     metrics.IsMilestoneOverdue(m.Status, m.PlanDue, now),
			"contract_overdue": metrics.IsMilestoneOverdue(m.Status, m.ContractDue, now),
		})
	}
	return out
}

func taskViews(items []store.Task) []map[string]any {
	now := time.Now()
	out := make([]map[string]any, 0, len(items))
	for i := range items {
		t := &items[i]
		out = append(out, map[string]any{
			"id":             t.ID,
			"name":           t.Name,
			"stage":          t.Stage,
			"owner_id":       t.OwnerID,
			"owner_name":     t.OwnerName,
			"job_type":       t.JobType,
			"plan_start":     t.PlanStart,
			"plan_due":       t.PlanDue,
			"actual_done":    t.ActualDone,
			"status":         t.Status,
			"estimate_hours": t.EstimateHours,
			"actual_hours":   t.ActualHours,
			"subtask_total":  t.SubtaskTotal,
			"subtask_done":   t.SubtaskDone,
			"overdue":        metrics.IsTaskOverdue(t.Status, t.PlanDue, now),
			"delay_days":     metrics.DelayDays(t.PlanDue, t.ActualDone),
		})
	}
	return out
}

func memberViews(items []store.Member) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for i := range items {
		m := &items[i]
		out = append(out, map[string]any{
			"user_id":      m.UserID,
			"name":         m.Name,
			"job_type":     m.JobType,
			"title":        m.Title,
			"project_role": m.ProjectRole,
			"allocation":   m.Allocation,
			"status":       m.Status,
			"joined_at":    m.JoinedAt,
			"left_at":      m.LeftAt,
		})
	}
	return out
}

func riskViews(items []store.Risk) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for i := range items {
		r := &items[i]
		out = append(out, map[string]any{
			"id":            r.ID,
			"code":          r.Code,
			"title":         r.Title,
			"description":   r.Description,
			"type":          r.Type,
			"level":         r.Level,
			"impact":        r.Impact,
			"action":        r.Action,
			"owner_id":      r.OwnerID,
			"owner_name":    r.OwnerName,
			"target_date":   r.TargetDate,
			"resolved_date": r.ResolvedDate,
			"status":        r.Status,
			"identified_on": r.IdentifiedOn,
		})
	}
	return out
}

// writeAudit 写一条操作日志（失败只记日志，不影响主流程）。
func (a *API) writeAudit(r *http.Request, action, targetType, targetID string, detail map[string]any) {
	user := currentUser(r)
	actorID, actorName := "", "system"
	if user != nil {
		actorID, actorName = user.ID, user.Name
	}
	if err := a.store.AddAudit(r.Context(), actorID, actorName, action, targetType, targetID, detail); err != nil {
		a.log.Warn("写审计日志失败", "action", action, "error", err)
	}
}
