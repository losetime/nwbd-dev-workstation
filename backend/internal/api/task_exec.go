package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"nwbd-dev-workstation/internal/filemanage"
	"nwbd-dev-workstation/internal/metrics"
	"nwbd-dev-workstation/internal/skills"
	"nwbd-dev-workstation/internal/store"
)

// GET /api/skills?q= 技能库列表（服务端代理，前端不直连技能库）。
func (a *API) listSkills(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requirePerm(w, r, "mytask"); !ok {
		return
	}
	items, err := a.skills.ListAll(r.Context())
	if err != nil {
		a.fail(w, "读取技能库失败", err)
		return
	}
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if q != "" {
			haystack := strings.ToLower(item.Name + " " + item.DisplayName + " " + item.Summary)
			if !strings.Contains(haystack, q) {
				continue
			}
		}
		out = append(out, map[string]any{
			"name":            item.Name,
			"display_name":    item.DisplayName,
			"summary":         skills.Summarize(item.Summary, 160),
			"status":          item.Status,
			"current_version": item.CurrentVersion,
			"version_count":   item.VersionCount,
			"updated_at":      item.UpdatedAt,
		})
	}
	writeOK(w, map[string]any{"items": out, "total": len(out)})
}

// GET /api/skills/{name} 技能详情（含版本）。
func (a *API) getSkill(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requirePerm(w, r, "mytask"); !ok {
		return
	}
	detail, err := a.skills.Get(r.Context(), r.PathValue("name"))
	if err != nil {
		a.fail(w, "读取技能详情失败", err)
		return
	}
	versions := make([]map[string]any, 0, len(detail.Versions))
	for _, v := range detail.Versions {
		versions = append(versions, map[string]any{
			"version": v.Version, "status": v.Status, "changelog": v.Changelog, "created_at": v.CreatedAt,
		})
	}
	writeOK(w, map[string]any{
		"name":            detail.Name,
		"display_name":    detail.DisplayName,
		"summary":         skills.Summarize(detail.Summary, 400),
		"current_version": detail.CurrentVersion,
		"versions":        versions,
	})
}

// POST /api/my/tasks/{id}/start 开始一次执行（已完成的会重新打开，次数 +1）。
func (a *API) startMyTask(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "mytask")
	if !ok {
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
			writeError(w, http.StatusForbidden, "FORBIDDEN", "只有任务负责人可以开始执行", nil)
			return
		}
	}

	attempt, reopen, err := a.store.StartAttempt(r.Context(), task.ID)
	if err != nil {
		a.fail(w, "开始执行失败", err)
		return
	}
	if attempt == task.AttemptCount && !reopen && task.Status == "进行中" {
		writeOK(w, map[string]any{"attempt": attempt, "reopen": false, "already_running": true})
		return
	}

	message := fmt.Sprintf("开始第 %d 次执行", attempt)
	if reopen {
		message = fmt.Sprintf("任务被重新打开，开始第 %d 次执行", attempt)
	}
	eventType := "attempt"
	if reopen {
		eventType = "reopen"
	}
	a.store.AddTaskEvent(r.Context(), task.ID, eventType, message, user.ID, user.Name, "info", attempt)
	writeOK(w, map[string]any{"attempt": attempt, "reopen": reopen})
}

// GET /api/task-types 任务类型字典（分组下拉用）；?all=1 供管理页含停用。
func (a *API) listTaskTypes(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requirePerm(w, r, "mytask"); !ok {
		return
	}
	includeDisabled := false
	if r.URL.Query().Get("all") == "1" {
		if _, ok := a.requirePerm(w, r, "sysadmin"); !ok {
			return
		}
		includeDisabled = true
	}
	items, err := a.store.ListTaskTypes(r.Context(), includeDisabled)
	if err != nil {
		a.fail(w, "读取任务类型失败", err)
		return
	}
	writeOK(w, map[string]any{"items": items, "total": len(items)})
}

// POST /api/my/tasks/{id}/artifacts 上传产出：转发到文件管理对应阶段目录。
func (a *API) uploadTaskArtifact(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "mytask")
	if !ok {
		return
	}
	task, err := a.store.GetBoardTask(r.Context(), r.PathValue("id"))
	if err != nil || task == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "任务不存在", nil)
		return
	}
	allowed, err := a.taskAccess(r, user, task)
	if err != nil {
		a.fail(w, "校验任务权限失败", err)
		return
	}
	if !allowed {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "只有任务负责人可以上传产出", nil)
		return
	}
	if !a.files.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "FILE_MANAGE_DISABLED",
			"未配置文件管理内部令牌，暂时不能归档产出", nil)
		return
	}

	// 归档目标：项目目录（按合同实例）或公共空间（固定 space:public）
	archiveTarget := strings.TrimSpace(task.ArchiveTarget)
	if archiveTarget == "" {
		archiveTarget = "project"
	}
	instanceID := "space:public"
	if archiveTarget != "public" {
		project, err := a.store.GetProject(r.Context(), task.ProjectID)
		if err != nil || project == nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "项目不存在", nil)
			return
		}
		if strings.TrimSpace(project.ContractInstanceID) == "" {
			writeError(w, http.StatusConflict, "CONTRACT_REQUIRED", "本项目未关联销售合同，不能归档产出", nil)
			return
		}
		instanceID = project.ContractInstanceID
	}

	folder := strings.TrimSpace(task.ArchiveFolder)
	if folder == "" && task.TaskTypeCode != "" {
		if tt, typeErr := a.store.GetTaskType(r.Context(), task.TaskTypeCode); typeErr == nil && tt != nil {
			folder = tt.ArchiveFolder
		}
	}
	if folder == "" {
		writeError(w, http.StatusConflict, "ARCHIVE_FOLDER_MISSING",
			"该任务没有归档目录，请重新选择任务类型", nil)
		return
	}

	maxBytes := int64(a.cfg.MaxUploadMB) << 20
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes+(1<<20))
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "UPLOAD_TOO_LARGE",
				fmt.Sprintf("文件过大，上限 %d MB", a.cfg.MaxUploadMB), nil)
			return
		}
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "表单解析失败", nil)
		return
	}
	if r.MultipartForm != nil {
		defer func() { _ = r.MultipartForm.RemoveAll() }()
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "缺少文件字段 file", nil)
		return
	}
	defer file.Close()
	if header.Size > maxBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "UPLOAD_TOO_LARGE",
			fmt.Sprintf("文件过大，上限 %d MB", a.cfg.MaxUploadMB), nil)
		return
	}

	fmProject, err := a.files.ProjectByInstance(r.Context(), instanceID)
	if err != nil {
		a.failArchive(w, "查询文件管理项目失败", err)
		return
	}

	result, err := a.files.Upload(r.Context(), fmProject.ID, folder,
		filepath.Base(header.Filename), user.Name, file)
	if err != nil {
		a.failArchive(w, "上传到文件管理失败", err)
		return
	}

	contentType := strings.TrimSpace(header.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	art, err := a.store.AddArtifact(r.Context(), task.ID, result.ID, result.Name, result.Folder, contentType, result.Size, user.ID)
	if err != nil {
		// 文件已经进了文件管理、本地入库失败：回滚，避免孤儿文件
		if delErr := a.files.Delete(r.Context(), result.ID); delErr != nil {
			a.log.Error("回滚文件管理文件失败，需人工处理", "file_id", result.ID, "error", delErr)
		}
		a.fail(w, "保存产出信息失败", err)
		return
	}

	a.store.AddTaskEvent(r.Context(), task.ID, "artifact",
		"上传产出："+art.Name+" → "+art.Folder, user.ID, user.Name, "info", task.AttemptCount)
	writeOK(w, artifactsView([]store.TaskArtifact{*art})[0])
}

// GET /api/my/tasks/{id}/artifacts/{artifactId} 下载产出（服务端代理文件管理）。
func (a *API) downloadTaskArtifact(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "mytask")
	if !ok {
		return
	}
	task, err := a.store.GetBoardTask(r.Context(), r.PathValue("id"))
	if err != nil || task == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "任务不存在", nil)
		return
	}
	allowed, err := a.taskAccess(r, user, task)
	if err != nil {
		a.fail(w, "校验任务权限失败", err)
		return
	}
	if !allowed {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "没有该任务的访问权限", nil)
		return
	}
	art, err := a.store.GetArtifact(r.Context(), r.PathValue("artifactId"))
	if err != nil || art == nil || art.TaskID != task.ID {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "产出不存在", nil)
		return
	}
	if art.FMFileID == "" {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "该产出没有归档文件", nil)
		return
	}

	body, contentType, size, err := a.files.Download(r.Context(), art.FMFileID)
	if err != nil {
		a.failArchive(w, "下载文件失败", err)
		return
	}
	defer body.Close()

	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename*=UTF-8''%s", url.PathEscape(art.Name)))
	if size > 0 {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", size))
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if _, err := io.Copy(w, body); err != nil {
		a.log.Warn("产出下载输出中断", "artifact", art.ID, "error", err)
	}
}

// failArchive 把文件管理的错误翻译成前端能直接展示的提示。
func (a *API) failArchive(w http.ResponseWriter, action string, err error) {
	if errors.Is(err, filemanage.ErrDisabled) {
		writeError(w, http.StatusServiceUnavailable, "FILE_MANAGE_DISABLED",
			"未配置文件管理内部令牌，暂时不能归档产出", nil)
		return
	}
	var apiErr *filemanage.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Code {
		case "PROJECT_NOT_READY":
			message := "文件管理项目目录尚未就绪，请稍后重试"
			if apiErr.ProjectStatus == "failed" {
				message = "文件管理项目目录创建失败，请联系管理员"
			}
			writeError(w, http.StatusConflict, "PROJECT_NOT_READY", message, nil)
			return
		case "NOT_FOUND":
			writeError(w, http.StatusNotFound, "FILE_MANAGE_PROJECT_NOT_FOUND",
				"文件管理中没有该合同对应的项目，可能合同审批尚未同步", nil)
			return
		case "INVALID_FOLDER":
			writeError(w, http.StatusUnprocessableEntity, "INVALID_FOLDER",
				"归档目录不存在，请联系管理员核对目录模板", nil)
			return
		case "FILE_TOO_LARGE":
			writeError(w, http.StatusRequestEntityTooLarge, "UPLOAD_TOO_LARGE",
				fmt.Sprintf("文件过大，上限 %d MB", a.cfg.MaxUploadMB), nil)
			return
		case "FORBIDDEN":
			writeError(w, http.StatusBadGateway, "FILE_MANAGE_TOKEN_INVALID",
				"文件管理内部令牌配置错误，请联系管理员", nil)
			return
		}
	}
	a.log.Error(action, "error", err)
	writeError(w, http.StatusBadGateway, "FILE_MANAGE_FAILED", action+"："+err.Error(), nil)
}

// GET /api/my/tasks/{id}/timesheet 工时确认建议。
func (a *API) getTaskTimesheet(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "mytask")
	if !ok {
		return
	}
	task, err := a.store.GetBoardTask(r.Context(), r.PathValue("id"))
	if err != nil || task == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "任务不存在", nil)
		return
	}

	booked, err := a.store.BookedHours(r.Context(), task.ID, user.ID)
	if err != nil {
		a.fail(w, "读取已报工时失败", err)
		return
	}
	rate, err := a.store.ProjectLaborRate(r.Context(), task.ProjectID)
	if err != nil {
		a.fail(w, "读取人力费率失败", err)
		return
	}

	var suggestions []metrics.DayPlan
	reason := ""
	switch {
	case task.EstimateHours <= 0:
		reason = "任务没有预估工时，无法生成建议；请项目经理补齐后重试"
	case task.PlanStart == nil || task.PlanDue == nil:
		reason = "任务缺少计划起止日期，无法生成建议"
	case task.PlanStart.After(*task.PlanDue):
		reason = "任务计划起止不合法"
	default:
		suggestions = metrics.SuggestedHours(task.EstimateHours, *task.PlanStart, *task.PlanDue, booked)
		if len(suggestions) == 0 {
			reason = "计划区间内没有工作日"
		}
	}

	writeOK(w, map[string]any{
		"task_id":     task.ID,
		"estimate":    task.EstimateHours,
		"actual":      task.ActualHours,
		"plan_start":  task.PlanStart,
		"plan_due":    task.PlanDue,
		"rate":        rate,
		"booked":      booked,
		"suggestions": suggestions,
		"reason":      reason,
	})
}

// POST /api/my/tasks/{id}/timesheet 确认工时。
func (a *API) saveTaskTimesheet(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requirePerm(w, r, "mytask")
	if !ok {
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
			writeError(w, http.StatusForbidden, "FORBIDDEN", "只能确认自己任务的工时", nil)
			return
		}
	}

	var payload struct {
		Items []struct {
			Date  string  `json:"date"`
			Hours float64 `json:"hours"`
		} `json:"items"`
	}
	if err := decodeJSON(r, &payload); err != nil || len(payload.Items) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "缺少工时行", nil)
		return
	}

	items := make([]store.TimesheetItem, 0, len(payload.Items))
	for _, item := range payload.Items {
		d, err := time.Parse("2006-01-02", strings.TrimSpace(item.Date))
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "INVALID_DATE", "日期格式应为 YYYY-MM-DD", nil)
			return
		}
		if item.Hours < 0 {
			writeError(w, http.StatusUnprocessableEntity, "INVALID_HOURS", "工时不能为负", nil)
			return
		}
		if item.Hours == 0 {
			continue
		}
		if task.PlanStart != nil && task.PlanDue != nil {
			if d.Before(*task.PlanStart) || d.After(*task.PlanDue) {
				writeError(w, http.StatusUnprocessableEntity, "OUT_OF_RANGE",
					"确认日期必须落在任务计划区间内："+item.Date, nil)
				return
			}
		}
		items = append(items, store.TimesheetItem{Date: d, Hours: item.Hours})
	}
	if len(items) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "没有可提交的工时", nil)
		return
	}

	rate, err := a.store.ProjectLaborRate(r.Context(), task.ProjectID)
	if err != nil {
		a.fail(w, "读取人力费率失败", err)
		return
	}
	total, err := a.store.AddTimesheets(r.Context(), task.ID, task.ProjectID, user.ID, user.JobType, rate, items)
	if err != nil {
		a.fail(w, "写入工时失败", err)
		return
	}
	a.store.AddTaskEvent(r.Context(), task.ID, "timesheet",
		fmt.Sprintf("确认工时 %.1f 小时（%d 天）", total, len(items)), user.ID, user.Name, "info", task.AttemptCount)
	writeOK(w, map[string]any{"total": total, "days": len(items), "rate": rate})
}

// taskAccess 校验当前用户能否操作该任务（负责人 / 本项目 PM / 部门经理）。
func (a *API) taskAccess(r *http.Request, user *store.Person, task *store.BoardTask) (bool, error) {
	if isBoss(user) || task.OwnerID == user.ID {
		return true, nil
	}
	return a.store.IsProjectManager(r.Context(), task.ProjectID, user.ID)
}

// artifactsView 输出产出物（文件在文件管理，返回归档目录与上传人）。
func artifactsView(items []store.TaskArtifact) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for i := range items {
		a := &items[i]
		out = append(out, map[string]any{
			"id":            a.ID,
			"name":          a.Name,
			"folder":        a.Folder,
			"size":          a.SizeBytes,
			"content_type":  a.ContentType,
			"uploaded_by":   a.UploadedBy,
			"uploader_name": a.UploaderName,
			"created_at":    a.CreatedAt,
		})
	}
	return out
}
