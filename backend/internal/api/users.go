package api

import (
	"net/http"
	"strings"

	"nwbd-dev-workstation/internal/domain"
	"nwbd-dev-workstation/internal/store"
)

// effectivePerms 返回某人最终可访问的界面集合。
func effectivePerms(u *store.Person) []string {
	return domain.EffectivePerms(u.JobType, u.UIPerms)
}

func hasPerm(u *store.Person, key string) bool {
	return domain.HasPerm(effectivePerms(u), key)
}

// isBoss 部门经理：业务侧的最高权限（看全部项目、审批、管人员）。
//
// 只看岗位类型，不看钉钉管理员角色（role）。钉钉管理员只代表“能进后台”，
// 不代表业务上是部门经理；研发人员即使被设成钉钉管理员，也只能看自己的项目。
func isBoss(u *store.Person) bool {
	return u.JobType == "部门经理"
}

// requirePerm 校验界面权限，失败时写 403。
func (a *API) requirePerm(w http.ResponseWriter, r *http.Request, key string) (*store.Person, bool) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return nil, false
	}
	if !hasPerm(user, key) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "没有该页面的访问权限", nil)
		return nil, false
	}
	return user, true
}

// GET /api/users/managers 项目经理候选（任何能看项目的人都可以调用）。
func (a *API) listManagers(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requirePerm(w, r, "myproj"); !ok {
		return
	}
	people, err := a.store.ListPickableManagers(r.Context())
	if err != nil {
		a.fail(w, "读取项目经理失败", err)
		return
	}
	items := make([]map[string]any, 0, len(people))
	for i := range people {
		p := &people[i]
		items = append(items, map[string]any{
			"id":       p.ID,
			"name":     p.Name,
			"job_type": p.JobType,
			"title":    p.Title,
			"dept":     p.Dept,
		})
	}
	writeOK(w, map[string]any{"items": items, "total": len(items)})
}

// GET /api/users
func (a *API) listUsers(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "sysadmin")
	if !ok {
		return
	}
	if !isBoss(user) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "只有部门经理可以管理人员", nil)
		return
	}

	people, err := a.store.ListPeople(r.Context(), store.PersonFilter{
		Keyword: strings.TrimSpace(r.URL.Query().Get("keyword")),
		JobType: strings.TrimSpace(r.URL.Query().Get("job_type")),
		Status:  strings.TrimSpace(r.URL.Query().Get("status")),
	})
	if err != nil {
		a.fail(w, "读取人员失败", err)
		return
	}

	items := make([]map[string]any, 0, len(people))
	for i := range people {
		items = append(items, personView(&people[i]))
	}
	writeOK(w, map[string]any{"items": items, "total": len(items)})
}

func personView(p *store.Person) map[string]any {
	return map[string]any{
		"id":       p.ID,
		"name":     p.Name,
		"userid":   p.DingTalkUserID,
		"title":    p.Title,
		"dept":     p.Dept,
		"job_type": p.JobType,
		"level":    p.Level,
		"status":   p.Status,
		"role":     p.Role,
		"perms":    effectivePerms(p),
		"custom":   len(p.UIPerms) > 0,
		"avatar":   p.Avatar,
	}
}

// PATCH /api/users/{id}
func (a *API) updateUser(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "sysadmin")
	if !ok {
		return
	}
	if !isBoss(user) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "只有部门经理可以管理人员", nil)
		return
	}

	var payload struct {
		JobType *string   `json:"job_type"`
		Level   *string   `json:"level"`
		Status  *string   `json:"status"`
		Perms   *[]string `json:"perms"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "请求体不合法", nil)
		return
	}
	if payload.JobType != nil && !domain.IsValid(domain.JobTypes, *payload.JobType) {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_JOB_TYPE", "岗位类型取值不合法", nil)
		return
	}
	if payload.Level != nil && !domain.IsValid(domain.Levels, *payload.Level) {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_LEVEL", "级别取值不合法", nil)
		return
	}
	if payload.Status != nil && *payload.Status != "在职" && *payload.Status != "离职" {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_STATUS", "状态取值不合法", nil)
		return
	}
	if payload.Perms != nil {
		for _, key := range *payload.Perms {
			if _, exists := domain.PermLabels[key]; !exists {
				writeError(w, http.StatusUnprocessableEntity, "INVALID_PERM", "界面权限取值不合法："+key, nil)
				return
			}
		}
		// 不能取消自己的系统管理权限，避免把自己锁在门外
		if r.PathValue("id") == user.ID && !contains(*payload.Perms, "sysadmin") {
			writeError(w, http.StatusUnprocessableEntity, "SELF_LOCK", "不能取消自己的系统管理权限", nil)
			return
		}
	}

	updated, err := a.store.UpdatePerson(r.Context(), r.PathValue("id"), store.UpdatePersonInput{
		JobType: payload.JobType,
		Level:   payload.Level,
		Status:  payload.Status,
		UIPerms: payload.Perms,
	})
	if err != nil {
		a.fail(w, "更新人员失败", err)
		return
	}
	if updated == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "人员不存在", nil)
		return
	}
	a.log.Info("人员信息已更新", "actor", user.Name, "target", updated.Name,
		"job_type", updated.JobType, "level", updated.Level, "status", updated.Status)
	writeOK(w, personView(updated))
}

// POST /api/users/sync 手动触发一次通讯录同步。
func (a *API) syncUsers(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "sysadmin")
	if !ok {
		return
	}
	if !isBoss(user) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "只有部门经理可以同步通讯录", nil)
		return
	}
	result, err := a.org.Sync(r.Context())
	if err != nil {
		a.fail(w, "同步通讯录失败", err)
		return
	}
	writeOK(w, result)
}

func contains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}
