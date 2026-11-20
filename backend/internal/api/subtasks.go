package api

import (
	"net/http"
	"strings"

	"nwbd-dev-workstation/internal/store"
)

// ownerTask 取任务并校验只有负责人能维护子任务。
func (a *API) ownerTask(w http.ResponseWriter, r *http.Request) (*store.Person, *store.BoardTask, bool) {
	user, ok := a.requirePerm(w, r, "mytask")
	if !ok {
		return nil, nil, false
	}
	task, err := a.store.GetBoardTask(r.Context(), r.PathValue("id"))
	if err != nil || task == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "任务不存在", nil)
		return nil, nil, false
	}
	if task.OwnerID != user.ID {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "只有任务负责人可以维护子任务", nil)
		return nil, nil, false
	}
	return user, task, true
}

// POST /api/my/tasks/{id}/subtasks 新增子任务。
func (a *API) createSubtask(w http.ResponseWriter, r *http.Request) {
	user, task, ok := a.ownerTask(w, r)
	if !ok {
		return
	}
	var payload struct {
		Title string `json:"title"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "请求体不合法", nil)
		return
	}
	title := strings.TrimSpace(payload.Title)
	if title == "" {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_TITLE", "子任务不能为空", nil)
		return
	}
	if len([]rune(title)) > 200 {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_TITLE", "子任务最多 200 字", nil)
		return
	}

	sub, err := a.store.CreateSubtask(r.Context(), task.ID, title, user.ID)
	if err != nil {
		a.fail(w, "新增子任务失败", err)
		return
	}
	writeOK(w, sub)
}

// PATCH /api/my/tasks/{id}/subtasks/{sid} 改标题或勾选。
func (a *API) updateSubtask(w http.ResponseWriter, r *http.Request) {
	user, task, ok := a.ownerTask(w, r)
	if !ok {
		return
	}
	var payload struct {
		Title string `json:"title"`
		Done  *bool  `json:"done"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "请求体不合法", nil)
		return
	}
	title := strings.TrimSpace(payload.Title)
	if title == "" && payload.Done == nil {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "没有要修改的内容", nil)
		return
	}
	if len([]rune(title)) > 200 {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_TITLE", "子任务最多 200 字", nil)
		return
	}

	sub, err := a.store.UpdateSubtask(r.Context(), task.ID, r.PathValue("sid"), title, payload.Done, user.ID)
	if err != nil {
		a.fail(w, "更新子任务失败", err)
		return
	}
	if sub == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "子任务不存在", nil)
		return
	}
	writeOK(w, sub)
}

// DELETE /api/my/tasks/{id}/subtasks/{sid} 删除子任务。
func (a *API) deleteSubtask(w http.ResponseWriter, r *http.Request) {
	_, task, ok := a.ownerTask(w, r)
	if !ok {
		return
	}
	if err := a.store.DeleteSubtask(r.Context(), task.ID, r.PathValue("sid")); err != nil {
		a.fail(w, "删除子任务失败", err)
		return
	}
	writeOK(w, map[string]bool{"ok": true})
}
