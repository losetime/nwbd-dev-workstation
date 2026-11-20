package api

import (
	"net/http"
	"strings"
	"time"

	"nwbd-dev-workstation/internal/domain"
	"nwbd-dev-workstation/internal/store"
)

// riskView 单条风险的对外结构。
func riskView(r *store.Risk) map[string]any {
	if r == nil {
		return map[string]any{}
	}
	return map[string]any{
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
	}
}

// GET /api/risks 跨项目风险列表（我的风险）。
func (a *API) listRisks(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "myrisk")
	if !ok {
		return
	}
	f := store.RiskFilter{
		Status:  strings.TrimSpace(r.URL.Query().Get("status")),
		Level:   strings.TrimSpace(r.URL.Query().Get("level")),
		Keyword: strings.TrimSpace(r.URL.Query().Get("keyword")),
	}
	if !isBoss(user) {
		f.ViewerID = user.ID
	}
	rows, err := a.store.ListRiskRows(r.Context(), f)
	if err != nil {
		a.fail(w, "读取风险失败", err)
		return
	}

	items := make([]map[string]any, 0, len(rows))
	today := time.Now()
	for i := range rows {
		row := &rows[i]
		v := riskView(&row.Risk)
		v["project_id"] = row.ProjectID
		v["project_name"] = row.ProjectName
		v["project_code"] = row.ProjectCode
		v["overdue"] = row.Status != "已解决" && row.TargetDate != nil && row.TargetDate.Before(today)
		items = append(items, v)
	}

	// 统计按数据范围整体算，不受列表筛选影响
	scope := ""
	if !isBoss(user) {
		scope = user.ID
	}
	st, err := a.store.CountRiskStats(r.Context(), scope)
	if err != nil {
		a.fail(w, "读取风险统计失败", err)
		return
	}
	writeOK(w, map[string]any{
		"items": items,
		"stats": map[string]any{
			"total": st.Total, "open": st.Open, "high": st.High,
			"overdue": st.Overdue, "resolved": st.Resolved,
		},
	})
}

// POST /api/projects/{id}/risks 登记风险（项目经理 / 部门经理）。
func (a *API) createRisk(w http.ResponseWriter, r *http.Request) {
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
		writeError(w, http.StatusForbidden, "FORBIDDEN", "只有项目经理或部门经理可以登记风险", nil)
		return
	}
	project, err := a.store.GetProject(r.Context(), projectID)
	if err != nil || project == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "项目不存在", nil)
		return
	}
	if project.Status == "已取消" {
		writeError(w, http.StatusUnprocessableEntity, "PROJECT_CANCELLED", "项目已取消，不能登记新风险", nil)
		return
	}

	var payload struct {
		Title       string  `json:"title"`
		Description string  `json:"description"`
		Type        string  `json:"type"`
		Level       string  `json:"level"`
		Impact      string  `json:"impact"`
		Action      string  `json:"action"`
		OwnerID     string  `json:"owner_id"`
		TargetDate  *string `json:"target_date"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "请求体不合法", nil)
		return
	}
	payload.Title = strings.TrimSpace(payload.Title)
	if payload.Title == "" {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "风险标题必填", nil)
		return
	}
	if payload.Type == "" {
		payload.Type = "其他"
	}
	if payload.Level == "" {
		payload.Level = "中"
	}
	if !domain.IsValid(domain.RiskTypes, payload.Type) {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_TYPE", "风险类型取值不合法", nil)
		return
	}
	if !domain.IsValid(domain.RiskLevels, payload.Level) {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_LEVEL", "风险等级取值不合法", nil)
		return
	}

	in := store.RiskInput{
		Title: payload.Title, Description: payload.Description, Type: payload.Type, Level: payload.Level,
		Impact: payload.Impact, Action: payload.Action, OwnerID: strings.TrimSpace(payload.OwnerID),
	}
	if payload.TargetDate != nil {
		if in.TargetDate, err = parseDate(*payload.TargetDate); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "INVALID_DATE", "目标解决日格式应为 YYYY-MM-DD", nil)
			return
		}
	}
	if in.OwnerID != "" {
		member, err := a.store.IsProjectMember(r.Context(), projectID, in.OwnerID)
		if err != nil {
			a.fail(w, "校验项目成员失败", err)
			return
		}
		if !member {
			writeError(w, http.StatusUnprocessableEntity, "NOT_MEMBER", "责任人必须是本项目的参与成员", nil)
			return
		}
	}

	id, err := a.store.CreateRisk(r.Context(), projectID, in, user.ID)
	if err != nil {
		a.fail(w, "登记风险失败", err)
		return
	}
	a.writeAudit(r, "risk.create", "risk", id, map[string]any{"project": projectID, "title": payload.Title})
	created, err := a.store.GetRisk(r.Context(), id)
	if err != nil {
		a.fail(w, "读取风险失败", err)
		return
	}
	writeOK(w, map[string]any{"risk": riskView(created)})
}

// PATCH /api/risks/{id} 编辑风险 / 改状态。
func (a *API) updateRisk(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "myproj")
	if !ok {
		return
	}
	risk, err := a.store.GetRisk(r.Context(), r.PathValue("id"))
	if err != nil || risk == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "风险不存在", nil)
		return
	}
	allowed, err := a.canManageProject(r, user, risk.ProjectID)
	if err != nil {
		a.fail(w, "校验项目权限失败", err)
		return
	}
	if !allowed {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "只有项目经理或部门经理可以处理风险", nil)
		return
	}

	var payload struct {
		Title        *string `json:"title"`
		Description  *string `json:"description"`
		Type         *string `json:"type"`
		Level        *string `json:"level"`
		Impact       *string `json:"impact"`
		Action       *string `json:"action"`
		OwnerID      *string `json:"owner_id"`
		TargetDate   *string `json:"target_date"`
		Status       *string `json:"status"`
		ResolvedDate *string `json:"resolved_date"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "请求体不合法", nil)
		return
	}

	in := store.RiskUpdate{
		Title: payload.Title, Description: payload.Description, Impact: payload.Impact,
		Action: payload.Action, Type: payload.Type, Level: payload.Level, Status: payload.Status,
	}
	if payload.Title != nil && strings.TrimSpace(*payload.Title) == "" {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "风险标题不能为空", nil)
		return
	}
	if payload.Type != nil && !domain.IsValid(domain.RiskTypes, *payload.Type) {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_TYPE", "风险类型取值不合法", nil)
		return
	}
	if payload.Level != nil && !domain.IsValid(domain.RiskLevels, *payload.Level) {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_LEVEL", "风险等级取值不合法", nil)
		return
	}
	if payload.Status != nil && !domain.IsValid(domain.RiskStatuses, *payload.Status) {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_STATUS", "风险状态取值不合法", nil)
		return
	}
	if payload.TargetDate != nil {
		if in.TargetDate, err = parseDate(*payload.TargetDate); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "INVALID_DATE", "目标解决日格式应为 YYYY-MM-DD", nil)
			return
		}
	}
	if payload.ResolvedDate != nil {
		if in.ResolvedDate, err = parseDate(*payload.ResolvedDate); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "INVALID_DATE", "解决日期格式应为 YYYY-MM-DD", nil)
			return
		}
	}
	if payload.OwnerID != nil {
		ownerID := strings.TrimSpace(*payload.OwnerID)
		if ownerID != "" && ownerID != risk.OwnerID {
			member, err := a.store.IsProjectMember(r.Context(), risk.ProjectID, ownerID)
			if err != nil {
				a.fail(w, "校验项目成员失败", err)
				return
			}
			if !member {
				writeError(w, http.StatusUnprocessableEntity, "NOT_MEMBER", "责任人必须是本项目的参与成员", nil)
				return
			}
		}
		in.OwnerID = &ownerID
	}

	// 状态流转：解决必须填解决日期；已解决重开仅部门经理。
	if payload.Status != nil {
		next := *payload.Status
		if next == "已解决" && risk.Status != "已解决" && in.ResolvedDate == nil {
			writeError(w, http.StatusUnprocessableEntity, "RESOLVED_DATE_REQUIRED", "请填写解决日期", nil)
			return
		}
		if risk.Status == "已解决" && next != "已解决" && !isBoss(user) {
			writeError(w, http.StatusForbidden, "FORBIDDEN", "只有部门经理可以重开已解决的风险", nil)
			return
		}
	}

	if err := a.store.UpdateRisk(r.Context(), risk.ID, in, user.ID); err != nil {
		a.fail(w, "保存风险失败", err)
		return
	}
	action := "risk.update"
	if payload.Status != nil && *payload.Status == "已解决" && risk.Status != "已解决" {
		action = "risk.resolve"
	}
	status := risk.Status
	if payload.Status != nil {
		status = *payload.Status
	}
	a.writeAudit(r, action, "risk", risk.ID, map[string]any{"status": status})
	updated, err := a.store.GetRisk(r.Context(), risk.ID)
	if err != nil {
		a.fail(w, "读取风险失败", err)
		return
	}
	writeOK(w, map[string]any{"risk": riskView(updated)})
}
