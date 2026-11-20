package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// TaskSkill 是任务绑定的技能（多个）。
type TaskSkill struct {
	Name    string `json:"name"`
	Version *int   `json:"version"`
	// Source: type = 任务类型带的（锁定，不可删）；manual = 任务里自己加的
	Source string `json:"source,omitempty"`
}

// BoardTask 是「我的任务」看板的一条任务。
type BoardTask struct {
	ID                 string
	ProjectID          string
	ProjectName        string
	Stage              string
	Name               string
	Description        string
	Acceptance         []string
	Priority           string
	Tags               []string
	Links              []TaskLink
	OwnerID            string
	OwnerName          string
	JobType            string
	PlanStart          *time.Time
	PlanDue            *time.Time
	ActualDone         *time.Time
	Status             string
	EstimateHours      float64
	ActualHours        float64
	ResultSummary      string
	Artifacts          []string
	ResultAgent        string
	FinishedAt         *time.Time
	SkillName          string
	SkillVersion       *int
	Skills             []TaskSkill
	AttemptCount       int
	LastEventAt        *time.Time
	UpdatedAt          time.Time
	CreatedAt          time.Time
	CreatedBy          string
	CreatorName        string
	TaskType           string
	TaskTypeCode       string
	ArchiveFolder      string
	ArchiveTarget      string
	ApprovalInstanceID string
	ApprovalStatus     string
	// SubtaskTotal / SubtaskDone 子任务进度（0/0 表示没有子任务）
	SubtaskTotal int
	SubtaskDone  int
}

// TaskLink 是任务上的外部链接。
type TaskLink struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

const boardTaskColumns = `t.id, t.project_id, p.name, COALESCE(t.stage, ''), t.name,
	COALESCE(t.description, ''), t.acceptance, t.priority, t.tags, t.links,
	t.owner_id, u.name, COALESCE(t.job_type, ''), t.plan_start, t.plan_due, t.actual_done,
	t.status, t.estimate_hours, t.actual_hours,
	COALESCE(t.result_summary, ''), t.result_artifacts, COALESCE(t.result_agent, ''), t.finished_at,
	COALESCE(t.skill_name, ''), t.skill_version, t.attempt_count,
	(SELECT max(e.created_at) FROM task_event e WHERE e.task_id = t.id),
	t.updated_at, t.created_at, COALESCE(t.created_by, ''), COALESCE(cu.name, ''),
	COALESCE(t.task_type, '普通任务'), COALESCE(t.task_type_code, ''), COALESCE(t.archive_folder, ''),
	COALESCE(t.archive_target, 'project'),
	COALESCE(t.approval_instance_id, ''), COALESCE(t.approval_status, ''),
	(SELECT count(*) FROM task_subtask s WHERE s.task_id = t.id),
	(SELECT count(*) FROM task_subtask s WHERE s.task_id = t.id AND s.done)`

func scanBoardTask(row pgx.Row) (BoardTask, error) {
	var t BoardTask
	var links []byte
	err := row.Scan(&t.ID, &t.ProjectID, &t.ProjectName, &t.Stage, &t.Name,
		&t.Description, &t.Acceptance, &t.Priority, &t.Tags, &links,
		&t.OwnerID, &t.OwnerName, &t.JobType, &t.PlanStart, &t.PlanDue, &t.ActualDone,
		&t.Status, &t.EstimateHours, &t.ActualHours,
		&t.ResultSummary, &t.Artifacts, &t.ResultAgent, &t.FinishedAt,
		&t.SkillName, &t.SkillVersion, &t.AttemptCount, &t.LastEventAt, &t.UpdatedAt, &t.CreatedAt,
		&t.CreatedBy, &t.CreatorName, &t.TaskType, &t.TaskTypeCode, &t.ArchiveFolder, &t.ArchiveTarget,
		&t.ApprovalInstanceID, &t.ApprovalStatus, &t.SubtaskTotal, &t.SubtaskDone)
	if err != nil {
		return t, err
	}
	if len(links) > 0 {
		_ = json.Unmarshal(links, &t.Links)
	}
	return t, nil
}

// BoardFilter 我的任务筛选。
type BoardFilter struct {
	OwnerID string
	Status  string
	Keyword string
	// All 为 true 时不按负责人过滤（部门经理看全部）
	All bool
	// ProjectIDs 非空时只看这些项目下的任务
	ProjectIDs []string
}

// TaskStats 看板统计。
type TaskStats struct {
	Total    int            `json:"total"`
	ByStatus map[string]int `json:"by_status"`
	Overdue  int            `json:"overdue"`
}

// ListBoardTasks 返回任务列表与统计。
func (s *Store) ListBoardTasks(ctx context.Context, f BoardFilter) ([]BoardTask, TaskStats, error) {
	where := "WHERE 1=1"
	args := []any{}

	if len(f.ProjectIDs) > 0 {
		args = append(args, f.ProjectIDs)
		where += " AND t.project_id = ANY($" + itoa(len(args)) + "::text[])"
	} else if !f.All {
		args = append(args, f.OwnerID)
		where += " AND t.owner_id = $" + itoa(len(args))
	}
	if f.Status != "" {
		args = append(args, f.Status)
		where += " AND t.status = $" + itoa(len(args))
	}
	if f.Keyword != "" {
		args = append(args, "%"+f.Keyword+"%")
		n := itoa(len(args))
		where += " AND (t.name ILIKE $" + n + " OR COALESCE(t.description, '') ILIKE $" + n +
			" OR p.name ILIKE $" + n + " OR array_to_string(t.tags, ' ') ILIKE $" + n + ")"
	}

	sql := `SELECT ` + boardTaskColumns + `
		FROM task t JOIN project p ON p.id = t.project_id JOIN app_user u ON u.id = t.owner_id
		LEFT JOIN app_user cu ON cu.id = t.created_by
		` + where + `
		ORDER BY (t.status IN ('已完成', '已取消')) , CASE t.status WHEN '已延期' THEN 0 WHEN '进行中' THEN 1 WHEN '未开始' THEN 2 ELSE 3 END,
		         t.plan_due NULLS LAST, t.created_at DESC`

	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, TaskStats{}, err
	}
	defer rows.Close()

	items := make([]BoardTask, 0, 32)
	for rows.Next() {
		t, err := scanBoardTask(rows)
		if err != nil {
			return nil, TaskStats{}, err
		}
		items = append(items, t)
	}
	if err := rows.Err(); err != nil {
		return nil, TaskStats{}, err
	}
	if err := s.loadTaskSkills(ctx, items); err != nil {
		return nil, TaskStats{}, err
	}

	stats := TaskStats{ByStatus: map[string]int{}}
	now := time.Now()
	for i := range items {
		stats.Total++
		stats.ByStatus[items[i].Status]++
		if isOpenStatus(items[i].Status) && items[i].PlanDue != nil && items[i].PlanDue.Before(now) {
			stats.Overdue++
		}
	}
	return items, stats, nil
}

func isOpenStatus(status string) bool {
	return status == "未开始" || status == "进行中" || status == "已延期"
}

// OpenTaskNames 某人在某项目下未完成的任务名（用于阻止退出项目）。
func (s *Store) OpenTaskNames(ctx context.Context, projectID, userID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT name FROM task
		WHERE project_id = $1 AND owner_id = $2 AND status IN ('未开始', '进行中', '已延期')
		ORDER BY plan_due NULLS LAST, name`, projectID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]string, 0, 4)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

// loadTaskSkills 批量填充任务绑定技能（一个任务可绑多个）。
func (s *Store) loadTaskSkills(ctx context.Context, items []BoardTask) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]string, 0, len(items))
	for i := range items {
		ids = append(ids, items[i].ID)
		items[i].Skills = []TaskSkill{}
	}
	rows, err := s.pool.Query(ctx, `
		SELECT task_id, skill_name, skill_version, source FROM task_skill
		WHERE task_id = ANY($1)
		ORDER BY sort_order, created_at`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()

	byTask := map[string][]TaskSkill{}
	for rows.Next() {
		var taskID, name, source string
		var version *int
		if err := rows.Scan(&taskID, &name, &version, &source); err != nil {
			return err
		}
		if source == "" {
			source = "manual"
		}
		byTask[taskID] = append(byTask[taskID], TaskSkill{Name: name, Version: version, Source: source})
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range items {
		if list, ok := byTask[items[i].ID]; ok {
			items[i].Skills = list
		}
	}
	return nil
}

// SetTaskApproval 更新任务关联审批的状态与错误信息。
func (s *Store) SetTaskApproval(ctx context.Context, id, status, lastErr string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE task SET approval_status = NULLIF($2, ''), approval_error = NULLIF($3, ''), updated_at = now()
		WHERE id = $1`, id, status, lastErr)
	return err
}

// TaskByApprovalInstance 按审批实例找任务（Stream 事件回写用）。
func (s *Store) TaskByApprovalInstance(ctx context.Context, instanceID string) (*BoardTask, error) {
	var id string
	err := s.pool.QueryRow(ctx, `SELECT id FROM task WHERE approval_instance_id = $1`, instanceID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.GetBoardTask(ctx, id)
}

// GetBoardTask 取单条任务（含项目/负责人信息）。
func (s *Store) GetBoardTask(ctx context.Context, id string) (*BoardTask, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+boardTaskColumns+`
		FROM task t JOIN project p ON p.id = t.project_id JOIN app_user u ON u.id = t.owner_id
		LEFT JOIN app_user cu ON cu.id = t.created_by
		WHERE t.id = $1`, id)
	t, err := scanBoardTask(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t.Skills = []TaskSkill{}
	single := []BoardTask{t}
	if err := s.loadTaskSkills(ctx, single); err != nil {
		return nil, err
	}
	t.Skills = single[0].Skills
	return &t, nil
}

// TaskEvent 是任务时间线上的一条。
type TaskEvent struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	ActorName string    `json:"actor"`
	Level     string    `json:"level"`
	Attempt   int       `json:"attempt"`
	CreatedAt time.Time `json:"at"`
}

// ListTaskEvents 返回任务时间线（倒序）。
func (s *Store) ListTaskEvents(ctx context.Context, taskID string) ([]TaskEvent, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, type, message, COALESCE(actor_name, ''), level, attempt, created_at
		FROM task_event WHERE task_id = $1 ORDER BY created_at DESC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]TaskEvent, 0, 16)
	for rows.Next() {
		var e TaskEvent
		if err := rows.Scan(&e.ID, &e.Type, &e.Message, &e.ActorName, &e.Level, &e.Attempt, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// AddTaskEvent 追加一条时间线记录（attempt 为所属执行次数）。
func (s *Store) AddTaskEvent(ctx context.Context, taskID, eventType, message, actorID, actorName, level string, attempt int) error {
	if level == "" {
		level = "info"
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO task_event (id, task_id, type, message, actor_id, actor_name, level, attempt)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''), $7, $8)`,
		NewID(), taskID, eventType, message, actorID, actorName, level, attempt)
	return err
}

// StartAttempt 开始一次执行：第 N 次、是否属于重新打开。
func (s *Store) StartAttempt(ctx context.Context, taskID string) (int, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status string
	var attempt int
	if err := tx.QueryRow(ctx, `
		SELECT status, attempt_count FROM task WHERE id = $1 FOR UPDATE`, taskID).Scan(&status, &attempt); err != nil {
		return 0, false, err
	}
	if status == "进行中" {
		return attempt, false, nil
	}

	reopen := status == "已完成" || status == "已取消"
	attempt++
	if _, err := tx.Exec(ctx, `
		UPDATE task SET status = '进行中', attempt_count = $2, finished_at = NULL, updated_at = now()
		WHERE id = $1`, taskID, attempt); err != nil {
		return 0, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, false, err
	}
	return attempt, reopen, nil
}

// TaskEditInput 编辑任务内容（仅下发人）。
type TaskEditInput struct {
	Name          *string
	Description   *string
	Acceptance    *[]string
	Priority      *string
	Tags          *[]string
	Links         *[]TaskLink
	Skills        *[]TaskSkill
	PlanStart     *time.Time
	PlanDue       *time.Time
	EstimateHours *float64
	Stage         *string
	// OwnerID / OwnerJobType 同时给出时表示转派（改负责人）。
	OwnerID      *string
	OwnerJobType *string
}

// UpdateTaskContent 更新任务内容字段；技能整体替换，转派同时更新负责人岗位。
func (s *Store) UpdateTaskContent(ctx context.Context, id string, in TaskEditInput) error {
	var links []byte
	if in.Links != nil {
		encoded, err := json.Marshal(*in.Links)
		if err != nil {
			return err
		}
		links = encoded
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		UPDATE task SET
			name = COALESCE($2, name),
			description = COALESCE($3, description),
			acceptance = COALESCE($4, acceptance),
			priority = COALESCE($5, priority),
			tags = COALESCE($6, tags),
			links = COALESCE($7, links),
			plan_start = COALESCE($8, plan_start),
			plan_due = COALESCE($9, plan_due),
			estimate_hours = COALESCE($10, estimate_hours),
			stage = COALESCE($11, stage),
			owner_id = COALESCE($12, owner_id),
			job_type = COALESCE($13, job_type),
			updated_at = now()
		WHERE id = $1`,
		id, in.Name, in.Description, in.Acceptance, in.Priority, in.Tags, links,
		in.PlanStart, in.PlanDue, in.EstimateHours, in.Stage, in.OwnerID, in.OwnerJobType); err != nil {
		return err
	}
	if in.Skills != nil {
		// 类型带的技能锁定，只替换任务里自己加的
		typeNames := map[string]bool{}
		rows, err := tx.Query(ctx,
			`SELECT skill_name FROM task_skill WHERE task_id = $1 AND source = 'type'`, id)
		if err != nil {
			return err
		}
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				rows.Close()
				return err
			}
			typeNames[name] = true
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `DELETE FROM task_skill WHERE task_id = $1 AND source <> 'type'`, id); err != nil {
			return err
		}
		for i, sk := range *in.Skills {
			if typeNames[sk.Name] {
				continue
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO task_skill (id, task_id, skill_name, skill_version, sort_order, source)
				VALUES ($1, $2, $3, $4, $5, 'manual')`,
				NewID(), id, sk.Name, sk.Version, i); err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}

// CompleteTask 回写执行结果并结束任务。
func (s *Store) CompleteTask(ctx context.Context, id, status, summary string, artifacts []string, actor string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE task SET status = $2,
			result_summary = NULLIF($3, ''),
			result_artifacts = COALESCE($4, result_artifacts),
			result_agent = NULLIF($5, ''),
			finished_at = now(),
			actual_done = COALESCE(actual_done, CURRENT_DATE),
			updated_at = now()
		WHERE id = $1`,
		id, status, summary, artifacts, actor)
	return err
}

// TaskArtifact 是任务产出物，文件本体在 nwbd-file-manage。
type TaskArtifact struct {
	ID           string    `json:"id"`
	TaskID       string    `json:"task_id"`
	FMFileID     string    `json:"-"`
	Name         string    `json:"name"`
	Folder       string    `json:"folder"`
	SizeBytes    int64     `json:"size"`
	ContentType  string    `json:"content_type"`
	UploadedBy   string    `json:"uploaded_by"`
	UploaderName string    `json:"uploader_name"`
	CreatedAt    time.Time `json:"created_at"`
}

const artifactColumns = `a.id, a.task_id, a.fm_file_id, a.name, a.folder, a.size_bytes,
	COALESCE(a.content_type, ''), COALESCE(a.uploaded_by, ''), COALESCE(u.name, ''), a.created_at`

func scanArtifact(row pgx.Row) (*TaskArtifact, error) {
	var a TaskArtifact
	err := row.Scan(&a.ID, &a.TaskID, &a.FMFileID, &a.Name, &a.Folder, &a.SizeBytes,
		&a.ContentType, &a.UploadedBy, &a.UploaderName, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// AddArtifact 保存产出物元数据（文件已在文件管理上传成功）。
func (s *Store) AddArtifact(ctx context.Context, taskID, fmFileID, name, folder, contentType string, size int64, userID string) (*TaskArtifact, error) {
	art := TaskArtifact{
		ID: NewID(), TaskID: taskID, FMFileID: fmFileID, Name: name, Folder: folder,
		SizeBytes: size, ContentType: contentType, CreatedAt: time.Now(),
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO task_artifact (id, task_id, fm_file_id, name, folder, size_bytes, content_type, uploaded_by)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), NULLIF($8, ''))`,
		art.ID, taskID, fmFileID, name, folder, size, contentType, userID)
	if err != nil {
		return nil, err
	}
	if userID != "" {
		_ = s.pool.QueryRow(ctx, "SELECT name FROM app_user WHERE id = $1", userID).Scan(&art.UploaderName)
	}
	return &art, nil
}

// ListArtifacts 任务的产出物（新到旧）。
func (s *Store) ListArtifacts(ctx context.Context, taskID string) ([]TaskArtifact, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+artifactColumns+`
		FROM task_artifact a LEFT JOIN app_user u ON u.id = a.uploaded_by
		WHERE a.task_id = $1 ORDER BY a.created_at DESC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]TaskArtifact, 0, 8)
	for rows.Next() {
		a, err := scanArtifact(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

// GetArtifact 取单个产出物。
func (s *Store) GetArtifact(ctx context.Context, id string) (*TaskArtifact, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+artifactColumns+`
		FROM task_artifact a LEFT JOIN app_user u ON u.id = a.uploaded_by
		WHERE a.id = $1`, id)
	return scanArtifact(row)
}

// BookedHours 某人在某任务上已报的每日工时。
func (s *Store) BookedHours(ctx context.Context, taskID, userID string) (map[string]float64, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT to_char(work_date, 'YYYY-MM-DD'), hours
		FROM timesheet WHERE task_id = $1 AND user_id = $2`, taskID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]float64{}
	for rows.Next() {
		var day string
		var hours float64
		if err := rows.Scan(&day, &hours); err != nil {
			return nil, err
		}
		out[day] = hours
	}
	return out, rows.Err()
}

// ProjectLaborRate 取项目人力费率（未设置返回 0）。
func (s *Store) ProjectLaborRate(ctx context.Context, projectID string) (float64, error) {
	var rate *float64
	if err := s.pool.QueryRow(ctx, `SELECT labor_rate FROM project WHERE id = $1`, projectID).Scan(&rate); err != nil {
		return 0, err
	}
	if rate == nil {
		return 0, nil
	}
	return *rate, nil
}

// TimesheetItem 一条待写入的工时。
type TimesheetItem struct {
	Date  time.Time
	Hours float64
}

// AddTimesheets 逐日写入工时（同日累加），并累加任务实际工时。
func (s *Store) AddTimesheets(ctx context.Context, taskID, projectID, userID, jobType string, rate float64, items []TimesheetItem) (float64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	total := 0.0
	for _, item := range items {
		if item.Hours <= 0 {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO timesheet (id, work_date, project_id, task_id, user_id, job_type, hours, rate, cost)
			VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), $7, $8, $9)
			ON CONFLICT (task_id, user_id, work_date) DO UPDATE SET
				hours = timesheet.hours + EXCLUDED.hours,
				rate = EXCLUDED.rate,
				cost = (timesheet.hours + EXCLUDED.hours) * EXCLUDED.rate`,
			NewID(), item.Date, projectID, taskID, userID, jobType, item.Hours, rate, item.Hours*rate); err != nil {
			return 0, err
		}
		total += item.Hours
	}
	if total > 0 {
		if _, err := tx.Exec(ctx, `
			UPDATE task SET actual_hours = actual_hours + $2, updated_at = now() WHERE id = $1`,
			taskID, total); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return total, nil
}

// IsTaskOwner 判断某人是否是任务负责人。
func (s *Store) IsTaskOwner(ctx context.Context, taskID, userID string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM task WHERE id = $1 AND owner_id = $2)`,
		taskID, userID).Scan(&ok)
	return ok, err
}

// ProjectIDsOfManager 返回某人担任项目经理的项目（含参与中的项目）。
func (s *Store) ProjectIDsOfManager(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT project_id FROM project_member
		WHERE user_id = $1 AND status = '参与中' AND project_role = '项目经理'`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]string, 0, 8)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
