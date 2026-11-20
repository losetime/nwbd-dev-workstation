package api

import (
	"net/http"

	"nwbd-dev-workstation/internal/store"
)

// devEnabled 测试身份切换是否可用（仅 local + 开关）。
func (a *API) devEnabled() bool { return a.cfg.DevActAsEnabled() }

// GET /api/dev/people 可切换的在职人员。
func (a *API) devPeople(w http.ResponseWriter, r *http.Request) {
	if !a.devEnabled() {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "接口不存在", nil)
		return
	}
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	people, err := a.store.ListPeople(r.Context(), store.PersonFilter{})
	if err != nil {
		a.fail(w, "读取人员失败", err)
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

// POST /api/dev/act-as 切换成指定用户（仅本地测试）。
func (a *API) devActAs(w http.ResponseWriter, r *http.Request) {
	if !a.devEnabled() {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "接口不存在", nil)
		return
	}
	actor, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	var payload struct {
		UserID string `json:"user_id"`
	}
	if err := decodeJSON(r, &payload); err != nil || payload.UserID == "" {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "缺少 user_id", nil)
		return
	}

	user, token, err := a.auth.IssueForUser(r.Context(), payload.UserID)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "ACT_AS_FAILED", err.Error(), nil)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     tokenCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(a.auth.TokenTTL().Seconds()),
		HttpOnly: true,
		Secure:   a.cookieSecure(),
		SameSite: http.SameSiteLaxMode,
	})
	a.log.Warn("测试身份切换", "from", actor.Name, "to", user.Name)
	a.writeAudit(r, "dev.act_as", "user", user.ID, map[string]any{"from": actor.Name, "to": user.Name})
	writeOK(w, map[string]any{"ok": true, "user": userView(user), "token": token})
}
