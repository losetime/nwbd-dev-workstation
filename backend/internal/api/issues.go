package api

import (
	"net/http"
	"strings"

	"nwbd-dev-workstation/internal/domain"
	"nwbd-dev-workstation/internal/store"
)

func issueView(it *store.Issue) map[string]any {
	return map[string]any{
		"id":              it.ID,
		"code":            it.Code,
		"project_id":      it.ProjectID,
		"project_name":    it.ProjectName,
		"task_id":         it.TaskID,
		"task_name":       it.TaskName,
		"title":           it.Title,
		"description":     it.Description,
		"reporter_id":     it.ReporterID,
		"reporter_name":   it.ReporterName,
		"severity":        it.Severity,
		"status":          it.Status,
		"handling":        it.Handling,
		"risk_id":         it.RiskID,
		"reported_on":     it.ReportedOn,
		"handled_by":      it.HandledBy,
		"handled_by_name": it.HandledByName,
		"handled_at":      it.HandledAt,
	}
}

// GET /api/issues?scope=mine|todo
func (a *API) listIssues(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "help")
	if !ok {
		return
	}
	scope := r.URL.Query().Get("scope")
	f := store.IssueFilter{}
	switch scope {
	case "todo":
		if isBoss(user) {
			// 部门经理只处理「项目经理」上报的困难
			f.All = true
			f.ReporterJobType = "项目经理"
		} else {
			// 项目经理只处理研发/测试/UI 等一线成员上报的（同项目）
			f.ManagerID = user.ID
			f.ExcludeReporterJobTypes = []string{"项目经理", "部门经理"}
		}
	case "handled":
		// 我处理过的：不限项目
		f.HandledID = user.ID
	case "archived":
		// 已归档：只限当前参与中的项目（部门经理看全部）
		f.ArchivedOnly = true
		if isBoss(user) {
			f.All = true
		} else {
			f.ProjectViewerID = user.ID
		}
	default: // mine
		f.ReporterID = user.ID
	}

	rows, err := a.store.ListIssues(r.Context(), f)
	if err != nil {
		a.fail(w, "读取困难上报失败", err)
		return
	}
	// 统计与筛选分开：统计是整段范围，表格按状态过滤
	pending, risked, archived := 0, 0, 0
	for i := range rows {
		switch rows[i].Status {
		case "待处理":
			pending++
		case "已转风险":
			risked++
		case "已归档":
			archived++
		}
	}

	want := strings.TrimSpace(r.URL.Query().Get("status"))
	items := make([]map[string]any, 0, len(rows))
	for i := range rows {
		if want != "" && rows[i].Status != want {
			continue
		}
		items = append(items, issueView(&rows[i]))
	}

	writeOK(w, map[string]any{
		"items": items,
		"stats": map[string]any{
			"total": pending + risked + archived, "pending": pending,
			"risked": risked, "archived": archived,
		},
	})
}

// POST /api/issues 上报困难（所有岗位可用；部门经理可报任意项目）。
func (a *API) createIssue(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "help")
	if !ok {
		return
	}
	var payload struct {
		ProjectID   string `json:"project_id"`
		TaskID      string `json:"task_id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Severity    string `json:"severity"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "请求体不合法", nil)
		return
	}
	payload.Title = strings.TrimSpace(payload.Title)
	if payload.ProjectID == "" || payload.Title == "" {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "项目与标题必填", nil)
		return
	}
	if payload.Severity == "" {
		payload.Severity = "中"
	}
	if !domain.IsValid(domain.IssueSeverities, payload.Severity) {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_SEVERITY", "困难级别只能是高/中/低", nil)
		return
	}

	project, err := a.store.GetProject(r.Context(), payload.ProjectID)
	if err != nil || project == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "项目不存在", nil)
		return
	}
	if project.Status == "已取消" {
		writeError(w, http.StatusUnprocessableEntity, "PROJECT_CANCELLED", "项目已取消，不能上报困难", nil)
		return
	}
	// 项目成员才能报；部门经理可对任意项目上报
	if !isBoss(user) {
		member, err := a.store.IsProjectMember(r.Context(), payload.ProjectID, user.ID)
		if err != nil {
			a.fail(w, "校验项目成员失败", err)
			return
		}
		if !member {
			writeError(w, http.StatusForbidden, "FORBIDDEN", "只有本项目参与中成员可以上报困难", nil)
			return
		}
	}

	taskID := strings.TrimSpace(payload.TaskID)
	if taskID != "" {
		task, err := a.store.GetTask(r.Context(), taskID)
		if err != nil || task == nil {
			writeError(w, http.StatusUnprocessableEntity, "INVALID_TASK", "关联任务不存在", nil)
			return
		}
		if task.ProjectID != payload.ProjectID {
			writeError(w, http.StatusUnprocessableEntity, "INVALID_TASK", "关联任务不属于该项目", nil)
			return
		}
	}

	id, err := a.store.CreateIssue(r.Context(), store.IssueInput{
		ProjectID: payload.ProjectID, TaskID: taskID, Title: payload.Title,
		Description: payload.Description, Severity: payload.Severity,
	}, user.ID, user.JobType)
	if err != nil {
		a.fail(w, "上报困难失败", err)
		return
	}
	a.writeAudit(r, "issue.create", "issue", id, map[string]any{"project": payload.ProjectID, "title": payload.Title})
	created, err := a.store.GetIssue(r.Context(), id)
	if err != nil {
		a.fail(w, "读取困难失败", err)
		return
	}
	writeOK(w, map[string]any{"issue": issueView(created)})
}

// POST /api/issues/{id}/archive 归档（需处理说明）。
func (a *API) archiveIssue(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "help")
	if !ok {
		return
	}
	issue, err := a.store.GetIssue(r.Context(), r.PathValue("id"))
	if err != nil || issue == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "困难记录不存在", nil)
		return
	}
	allowed, err := a.canManageProject(r, user, issue.ProjectID)
	if err != nil {
		a.fail(w, "校验项目权限失败", err)
		return
	}
	if !allowed {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "只有项目经理或部门经理可以处理困难", nil)
		return
	}
	if issue.Status != "待处理" {
		writeError(w, http.StatusConflict, "ISSUE_HANDLED", "该困难已处理", nil)
		return
	}

	var payload struct {
		Handling string `json:"handling"`
	}
	if err := decodeJSON(r, &payload); err != nil || strings.TrimSpace(payload.Handling) == "" {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "归档必须填写处理说明", nil)
		return
	}
	if err := a.store.HandleIssue(r.Context(), issue.ID, "已归档", strings.TrimSpace(payload.Handling), "", user.ID); err != nil {
		a.fail(w, "归档失败", err)
		return
	}
	a.writeAudit(r, "issue.archive", "issue", issue.ID, map[string]any{"handling": payload.Handling})
	updated, err := a.store.GetIssue(r.Context(), issue.ID)
	if err != nil {
		a.fail(w, "读取困难失败", err)
		return
	}
	writeOK(w, map[string]any{"issue": issueView(updated)})
}
