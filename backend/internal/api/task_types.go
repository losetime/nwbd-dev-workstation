package api

import (
	"net/http"
	"strings"

	"nwbd-dev-workstation/internal/store"
)

// taskTypePayload 新建 / 编辑任务类型的请求体。
type taskTypePayload struct {
	Name          string                `json:"name"`
	GroupName     string                `json:"group_name"`
	Description   string                `json:"description"`
	ArchiveStage  string                `json:"archive_stage"`
	ArchiveFolder string                `json:"archive_folder"`
	ArchiveTarget string                `json:"archive_target"`
	ArchiveNote   string                `json:"archive_note"`
	SortOrder     int                   `json:"sort_order"`
	Skills        []store.TaskTypeSkill `json:"skills"`
}

func (p taskTypePayload) input() (store.TaskTypeInput, string) {
	in := store.TaskTypeInput{
		Name:          strings.TrimSpace(p.Name),
		GroupName:     strings.TrimSpace(p.GroupName),
		Description:   strings.TrimSpace(p.Description),
		ArchiveStage:  strings.TrimSpace(p.ArchiveStage),
		ArchiveFolder: strings.TrimSpace(p.ArchiveFolder),
		ArchiveTarget: strings.TrimSpace(p.ArchiveTarget),
		ArchiveNote:   strings.TrimSpace(p.ArchiveNote),
		SortOrder:     p.SortOrder,
	}
	if in.GroupName != "日常任务" && in.GroupName != "项目任务" {
		return in, "组别只能是「日常任务」或「项目任务」"
	}
	if in.Name == "" {
		return in, "类型名称不能为空"
	}
	if in.ArchiveStage == "" || in.ArchiveFolder == "" {
		return in, "归档阶段与归档目录必填"
	}
	if in.ArchiveTarget == "" {
		in.ArchiveTarget = "project"
	}
	if in.ArchiveTarget != "project" && in.ArchiveTarget != "public" {
		return in, "归档目标只能是项目或公共空间"
	}
	if in.SortOrder <= 0 {
		in.SortOrder = 999
	}
	return in, ""
}

func normalizeTypeSkills(in []store.TaskTypeSkill) []store.TaskTypeSkill {
	out := make([]store.TaskTypeSkill, 0, len(in))
	seen := map[string]bool{}
	for _, sk := range in {
		name := strings.TrimSpace(sk.Name)
		if name == "" || seen[name] || len(out) >= 10 {
			continue
		}
		seen[name] = true
		out = append(out, store.TaskTypeSkill{Name: name, Version: sk.Version})
	}
	return out
}

// POST /api/task-types 新建任务类型（sysadmin）。
func (a *API) createTaskType(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requirePerm(w, r, "sysadmin"); !ok {
		return
	}
	var payload taskTypePayload
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "请求体不合法", nil)
		return
	}
	in, msg := payload.input()
	if msg != "" {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", msg, nil)
		return
	}

	item, err := a.store.CreateTaskType(r.Context(), in)
	if err != nil {
		a.fail(w, "新建任务类型失败", err)
		return
	}
	if skills := normalizeTypeSkills(payload.Skills); len(skills) > 0 {
		if err := a.store.SetTaskTypeSkills(r.Context(), item.Code, skills); err != nil {
			a.fail(w, "保存默认技能失败", err)
			return
		}
		if item, err = a.store.GetTaskType(r.Context(), item.Code); err != nil {
			a.fail(w, "读取任务类型失败", err)
			return
		}
	}
	a.writeAudit(r, "task_type.create", "task_type", item.Code, map[string]any{"name": item.Name})
	writeOK(w, item)
}

// PATCH /api/task-types/{code} 编辑任务类型（编码不可改）。
func (a *API) updateTaskType(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requirePerm(w, r, "sysadmin"); !ok {
		return
	}
	code := r.PathValue("code")
	existing, err := a.store.GetTaskType(r.Context(), code)
	if err != nil || existing == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "任务类型不存在", nil)
		return
	}
	var payload taskTypePayload
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "请求体不合法", nil)
		return
	}
	in, msg := payload.input()
	if msg != "" {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", msg, nil)
		return
	}

	item, err := a.store.UpdateTaskType(r.Context(), code, in)
	if err != nil {
		a.fail(w, "保存任务类型失败", err)
		return
	}
	if payload.Skills != nil {
		if err := a.store.SetTaskTypeSkills(r.Context(), code, normalizeTypeSkills(payload.Skills)); err != nil {
			a.fail(w, "保存默认技能失败", err)
			return
		}
		if item, err = a.store.GetTaskType(r.Context(), code); err != nil {
			a.fail(w, "读取任务类型失败", err)
			return
		}
	}
	a.writeAudit(r, "task_type.update", "task_type", code, map[string]any{"name": item.Name})
	writeOK(w, item)
}

// PATCH /api/task-types/{code}/status 启用 / 停用。
func (a *API) setTaskTypeStatus(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requirePerm(w, r, "sysadmin"); !ok {
		return
	}
	var payload struct {
		Enabled bool `json:"enabled"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "请求体不合法", nil)
		return
	}
	code := r.PathValue("code")
	if err := a.store.SetTaskTypeEnabled(r.Context(), code, payload.Enabled); err != nil {
		a.fail(w, "更新任务类型状态失败", err)
		return
	}
	a.writeAudit(r, "task_type.status", "task_type", code, map[string]any{"enabled": payload.Enabled})
	writeOK(w, map[string]any{"ok": true})
}

// DELETE /api/task-types/{code} 删除；有任务引用时只允许停用。
func (a *API) deleteTaskType(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requirePerm(w, r, "sysadmin"); !ok {
		return
	}
	code := r.PathValue("code")
	used, err := a.store.CountTasksByType(r.Context(), code)
	if err != nil {
		a.fail(w, "检查任务引用失败", err)
		return
	}
	if used > 0 {
		writeError(w, http.StatusConflict, "TYPE_IN_USE",
			"已有任务使用该类型，只能停用，不能删除", nil)
		return
	}
	if err := a.store.DeleteTaskType(r.Context(), code); err != nil {
		a.fail(w, "删除任务类型失败", err)
		return
	}
	a.writeAudit(r, "task_type.delete", "task_type", code, nil)
	writeOK(w, map[string]bool{"ok": true})
}
