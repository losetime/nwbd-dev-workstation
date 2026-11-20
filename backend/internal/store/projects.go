package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Project 是项目台账一行（含列表页要用的统计字段）。
type Project struct {
	ID                 string
	Code               string
	Name               string
	Type               string
	Status             string
	Customer           string
	CurrentStage       string
	CurrentMilestone   string
	MilestoneStatus    string
	PlanStart          *time.Time
	PlanEnd            *time.Time
	ActualEnd          *time.Time
	ContractAmount     *float64
	Budget             *float64
	CostActual         *float64
	LaborBudget        *float64
	LaborRate          *float64
	RiskFlag           string
	Remark             string
	WeeklySummary      string
	SummaryAt          *time.Time
	CreatedAt          time.Time
	ContractInstanceID string
	ContractBusinessID string
	ContractStatus     string
	DeletedAt          *time.Time

	TaskTotal   int
	TaskDone    int
	OpenRisks   int
	MemberCount int
	Managers    string
}

const projectColumns = `p.id, p.code, p.name, p.type, p.status, COALESCE(p.customer, ''),
	COALESCE(p.current_stage, ''), COALESCE(p.current_milestone, ''), COALESCE(p.milestone_status, ''),
	p.plan_start, p.plan_end, p.actual_end,
	p.contract_amount, p.budget, p.cost_actual, p.labor_budget, p.labor_rate,
	COALESCE(p.risk_flag, ''), COALESCE(p.remark, ''), COALESCE(p.weekly_summary, ''), p.summary_at,
	p.created_at,
	COALESCE(p.dt_contract_instance_id, ''), COALESCE(p.dt_business_id, ''),
	COALESCE(p.contract_status, ''), p.deleted_at`

const projectStats = `,
	(SELECT count(*) FROM task t WHERE t.project_id = p.id AND t.status <> '已取消'),
	(SELECT count(*) FROM task t WHERE t.project_id = p.id AND t.status = '已完成'),
	(SELECT count(*) FROM risk r WHERE r.project_id = p.id AND r.status <> '已解决'),
	(SELECT count(*) FROM project_member m WHERE m.project_id = p.id AND m.status = '参与中'),
	COALESCE((SELECT string_agg(u.name, '、' ORDER BY u.name)
		FROM project_member m JOIN app_user u ON u.id = m.user_id
		WHERE m.project_id = p.id AND m.status = '参与中' AND m.project_role = '项目经理'), '')`

func scanProject(row pgx.Row, withStats bool) (Project, error) {
	var p Project
	dest := []any{&p.ID, &p.Code, &p.Name, &p.Type, &p.Status, &p.Customer,
		&p.CurrentStage, &p.CurrentMilestone, &p.MilestoneStatus,
		&p.PlanStart, &p.PlanEnd, &p.ActualEnd,
		&p.ContractAmount, &p.Budget, &p.CostActual, &p.LaborBudget, &p.LaborRate,
		&p.RiskFlag, &p.Remark, &p.WeeklySummary, &p.SummaryAt,
		&p.CreatedAt,
		&p.ContractInstanceID, &p.ContractBusinessID, &p.ContractStatus, &p.DeletedAt}
	if withStats {
		dest = append(dest, &p.TaskTotal, &p.TaskDone, &p.OpenRisks, &p.MemberCount, &p.Managers)
	}
	err := row.Scan(dest...)
	return p, err
}

// ProjectFilter 列表筛选。
type ProjectFilter struct {
	Type     string
	Status   string
	RiskFlag string
	Keyword  string
	// ViewerID 非空时只返回该人参与中的项目
	ViewerID string
	// ManagerID 非空时只返回该人担任项目经理的项目
	ManagerID string
}

// ListProjects 返回项目列表（带统计列）。
func (s *Store) ListProjects(ctx context.Context, f ProjectFilter) ([]Project, error) {
	sql := `SELECT ` + projectColumns + projectStats + ` FROM project p WHERE p.deleted_at IS NULL AND p.status <> '已取消'`
	args := []any{}

	add := func(cond string, val any) {
		args = append(args, val)
		sql += " AND " + cond + " $" + itoa(len(args))
	}
	if f.ViewerID != "" {
		add("EXISTS (SELECT 1 FROM project_member m WHERE m.project_id = p.id AND m.user_id =", f.ViewerID)
		sql += " AND m.status = '参与中')"
	}
	if f.ManagerID != "" {
		add("EXISTS (SELECT 1 FROM project_member m WHERE m.project_id = p.id AND m.user_id =", f.ManagerID)
		sql += " AND m.status = '参与中' AND m.project_role = '项目经理')"
	}
	if f.Type != "" {
		add("p.type =", f.Type)
	}
	if f.Status != "" {
		add("p.status =", f.Status)
	}
	if f.RiskFlag != "" {
		add("COALESCE(p.risk_flag, '') =", f.RiskFlag)
	}
	if f.Keyword != "" {
		args = append(args, "%"+f.Keyword+"%")
		n := itoa(len(args))
		sql += " AND (p.name ILIKE $" + n + " OR p.code ILIKE $" + n + " OR COALESCE(p.customer,'') ILIKE $" + n + ")"
	}
	sql += " ORDER BY p.created_at DESC"

	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Project, 0, 32)
	for rows.Next() {
		p, err := scanProject(rows, true)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetProject 取单个项目（含统计列）。
func (s *Store) GetProject(ctx context.Context, id string) (*Project, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+projectColumns+projectStats+` FROM project p WHERE p.id = $1 AND p.deleted_at IS NULL`, id)
	p, err := scanProject(row, true)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ProjectInput 新建项目。
type ProjectInput struct {
	Code               string
	Name               string
	Type               string
	Customer           string
	ContractAmount     *float64
	ManagerIDs         []string
	CreatedBy          string
	ContractInstanceID string
	ContractBusinessID string
}

// CreateProject 在一个事务里写项目、按阶段模板生成里程碑、写入项目经理。
func (s *Store) CreateProject(ctx context.Context, in ProjectInput, stages []string) (*Project, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	id := NewID()
	currentStage, currentMilestone := "", ""
	if len(stages) > 0 {
		currentStage = stages[0]
		currentMilestone = stages[0] + "完成"
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO project (id, code, name, type, status, customer, current_stage, current_milestone,
			milestone_status, contract_amount, created_by, dt_contract_instance_id, dt_business_id)
		VALUES ($1, $2, $3, $4, '未开始', NULLIF($5, ''), NULLIF($6, ''), NULLIF($7, ''), $8, $9, NULLIF($10, ''),
			NULLIF($11, ''), NULLIF($12, ''))`,
		id, in.Code, in.Name, in.Type, in.Customer, currentStage, currentMilestone,
		milestoneStatusFor(len(stages)), in.ContractAmount, in.CreatedBy,
		in.ContractInstanceID, in.ContractBusinessID,
	); err != nil {
		return nil, err
	}

	for i, stage := range stages {
		if _, err := tx.Exec(ctx, `
			INSERT INTO milestone (id, project_id, stage, name, status, sort_order)
			VALUES ($1, $2, $3, $4, '未开始', $5)`,
			NewID(), id, stage, stage+"完成", i,
		); err != nil {
			return nil, err
		}
	}

	for _, uid := range in.ManagerIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO project_member (id, project_id, user_id, project_role, status)
			VALUES ($1, $2, $3, '项目经理', '参与中')
			ON CONFLICT (project_id, user_id) DO NOTHING`,
			NewID(), id, uid,
		); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetProject(ctx, id)
}

func milestoneStatusFor(stageCount int) any {
	if stageCount == 0 {
		return nil
	}
	return "未开始"
}

// Milestone 里程碑。
type Milestone struct {
	ID          string
	ProjectID   string
	Stage       string
	Name        string
	PlanDue     *time.Time
	ContractDue *time.Time
	ActualDone  *time.Time
	Status      string
	Reviewer    string
	ReviewDate  *time.Time
	Version     int
	Locked      bool
	SortOrder   int
}

// ListMilestones 按顺序返回项目的里程碑。
func (s *Store) ListMilestones(ctx context.Context, projectID string) ([]Milestone, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, project_id, stage, name, plan_due, contract_due, actual_done, status,
		       COALESCE(reviewer, ''), review_date, version, locked, sort_order
		FROM milestone WHERE project_id = $1 ORDER BY sort_order`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Milestone, 0, 8)
	for rows.Next() {
		var m Milestone
		if err := rows.Scan(&m.ID, &m.ProjectID, &m.Stage, &m.Name, &m.PlanDue, &m.ContractDue,
			&m.ActualDone, &m.Status, &m.Reviewer, &m.ReviewDate, &m.Version, &m.Locked, &m.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Task 任务。
type Task struct {
	ID            string
	ProjectID     string
	Stage         string
	Name          string
	OwnerID       string
	OwnerName     string
	JobType       string
	PlanStart     *time.Time
	PlanDue       *time.Time
	ActualDone    *time.Time
	Status        string
	EstimateHours float64
	ActualHours   float64
	SubtaskTotal  int
	SubtaskDone   int
}

// ListTasks 返回项目的任务，逾期优先、计划完成日升序。
func (s *Store) ListTasks(ctx context.Context, projectID string) ([]Task, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT t.id, t.project_id, COALESCE(t.stage, ''), t.name, t.owner_id, u.name,
		       COALESCE(t.job_type, ''), t.plan_start, t.plan_due, t.actual_done, t.status,
		       t.estimate_hours, t.actual_hours,
		       (SELECT count(*) FROM task_subtask s WHERE s.task_id = t.id),
		       (SELECT count(*) FROM task_subtask s WHERE s.task_id = t.id AND s.done)
		FROM task t JOIN app_user u ON u.id = t.owner_id
		WHERE t.project_id = $1
		ORDER BY (t.status IN ('未开始','进行中','已延期')) DESC, t.plan_due NULLS LAST, t.name`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Task, 0, 32)
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Stage, &t.Name, &t.OwnerID, &t.OwnerName,
			&t.JobType, &t.PlanStart, &t.PlanDue, &t.ActualDone, &t.Status,
			&t.EstimateHours, &t.ActualHours, &t.SubtaskTotal, &t.SubtaskDone); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetTask 取单个任务。
func (s *Store) GetTask(ctx context.Context, id string) (*Task, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT t.id, t.project_id, COALESCE(t.stage, ''), t.name, t.owner_id, u.name,
		       COALESCE(t.job_type, ''), t.plan_start, t.plan_due, t.actual_done, t.status,
		       t.estimate_hours, t.actual_hours,
		       (SELECT count(*) FROM task_subtask s WHERE s.task_id = t.id),
		       (SELECT count(*) FROM task_subtask s WHERE s.task_id = t.id AND s.done)
		FROM task t JOIN app_user u ON u.id = t.owner_id
		WHERE t.id = $1`, id)
	var t Task
	if err := row.Scan(&t.ID, &t.ProjectID, &t.Stage, &t.Name, &t.OwnerID, &t.OwnerName,
		&t.JobType, &t.PlanStart, &t.PlanDue, &t.ActualDone, &t.Status,
		&t.EstimateHours, &t.ActualHours, &t.SubtaskTotal, &t.SubtaskDone); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

// Member 项目成员。
type Member struct {
	ID          string
	UserID      string
	Name        string
	JobType     string
	Title       string
	ProjectRole string
	Allocation  int
	Status      string
	JoinedAt    *time.Time
	LeftAt      *time.Time
}

// ListMembers 返回项目全部成员（含已退出，前端灰显）。
func (s *Store) ListMembers(ctx context.Context, projectID string) ([]Member, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT m.id, m.user_id, u.name, u.job_type, COALESCE(u.title, ''),
		       m.project_role, m.allocation, m.status, m.joined_at, m.left_at
		FROM project_member m JOIN app_user u ON u.id = m.user_id
		WHERE m.project_id = $1
		ORDER BY (m.status = '参与中') DESC, (m.project_role = '项目经理') DESC, u.name`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Member, 0, 16)
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.ID, &m.UserID, &m.Name, &m.JobType, &m.Title,
			&m.ProjectRole, &m.Allocation, &m.Status, &m.JoinedAt, &m.LeftAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Risk 风险。
type Risk struct {
	ID           string
	Code         string
	ProjectID    string
	Title        string
	Description  string
	Type         string
	Level        string
	Impact       string
	Action       string
	OwnerID      string
	OwnerName    string
	TargetDate   *time.Time
	ResolvedDate *time.Time
	Status       string
	IdentifiedOn *time.Time
}

// ListRisks 返回项目风险，未解决在前。
func (s *Store) ListRisks(ctx context.Context, projectID string) ([]Risk, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT r.id, COALESCE(r.code, ''), r.project_id, r.title, COALESCE(r.description, ''),
		       r.type, r.level, COALESCE(r.impact, ''), COALESCE(r.action, ''),
		       COALESCE(r.owner_id, ''), COALESCE(u.name, ''), r.target_date, r.resolved_date,
		       r.status, r.identified_on
		FROM risk r LEFT JOIN app_user u ON u.id = r.owner_id
		WHERE r.project_id = $1
		ORDER BY (r.status <> '已解决') DESC, CASE r.level WHEN '高' THEN 0 WHEN '中' THEN 1 ELSE 2 END,
		         r.identified_on DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Risk, 0, 16)
	for rows.Next() {
		var r Risk
		if err := rows.Scan(&r.ID, &r.Code, &r.ProjectID, &r.Title, &r.Description,
			&r.Type, &r.Level, &r.Impact, &r.Action, &r.OwnerID, &r.OwnerName,
			&r.TargetDate, &r.ResolvedDate, &r.Status, &r.IdentifiedOn); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// IsProjectMember 判断某人是否在项目中（参与中）。
func (s *Store) IsProjectMember(ctx context.Context, projectID, userID string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM project_member
		WHERE project_id = $1 AND user_id = $2 AND status = '参与中')`, projectID, userID).Scan(&ok)
	return ok, err
}

// IsProjectManager 判断某人是否是项目的项目经理。
func (s *Store) IsProjectManager(ctx context.Context, projectID, userID string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM project_member
		WHERE project_id = $1 AND user_id = $2 AND status = '参与中' AND project_role = '项目经理')`,
		projectID, userID).Scan(&ok)
	return ok, err
}

// ProjectUpdate 可编辑的项目信息（nil = 不修改，与原型「留空不改」一致）。
type ProjectUpdate struct {
	Name           *string
	Customer       *string
	ContractAmount *float64
	PlanStart      *time.Time
	PlanEnd        *time.Time
	Budget         *float64
	LaborBudget    *float64
	LaborRate      *float64
	Remark         *string
}

// UpdateProject 更新项目基本信息。
func (s *Store) UpdateProject(ctx context.Context, id string, in ProjectUpdate) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE project SET
			name = COALESCE($10, name),
			customer = COALESCE($2, customer),
			contract_amount = COALESCE($3, contract_amount),
			plan_start = COALESCE($4, plan_start),
			plan_end = COALESCE($5, plan_end),
			budget = COALESCE($6, budget),
			labor_budget = COALESCE($7, labor_budget),
			labor_rate = COALESCE($8, labor_rate),
			remark = COALESCE($9, remark),
			updated_at = now()
		WHERE id = $1`,
		id, in.Customer, in.ContractAmount, in.PlanStart, in.PlanEnd,
		in.Budget, in.LaborBudget, in.LaborRate, in.Remark, in.Name)
	return err
}

// LinkContract 把合同审批实例关联到已有项目（按编号匹配到的情况）。
func (s *Store) LinkContract(ctx context.Context, projectID, instanceID, businessID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE project SET dt_contract_instance_id = NULLIF($2, ''), dt_business_id = NULLIF($3, ''),
			contract_status = '', updated_at = now()
		WHERE id = $1`, projectID, instanceID, businessID)
	return err
}

// DeleteProject 软删除项目：列表与详情不再可见，数据保留。
func (s *Store) DeleteProject(ctx context.Context, id, actorID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE project SET deleted_at = now(), deleted_by = NULLIF($2, ''), updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL`, id, actorID)
	return err
}

// ProjectByContractInstance 按合同审批实例找项目（含已删除，便于关联）。
func (s *Store) ProjectByContractInstance(ctx context.Context, instanceID string) (*Project, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+projectColumns+projectStats+`
		FROM project p WHERE p.dt_contract_instance_id = $1 AND p.deleted_at IS NULL LIMIT 1`, instanceID)
	p, err := scanProject(row, true)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ProjectByCode 按项目编号找项目。
func (s *Store) ProjectByCode(ctx context.Context, code string) (*Project, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+projectColumns+projectStats+`
		FROM project p WHERE p.code = $1 AND p.deleted_at IS NULL LIMIT 1`, code)
	p, err := scanProject(row, true)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// MarkContractStatus 按合同实例标记项目上的合同状态，返回受影响项目 ID。
func (s *Store) MarkContractStatus(ctx context.Context, instanceID, status string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `
		UPDATE project SET contract_status = $2, updated_at = now()
		WHERE dt_contract_instance_id = $1 AND deleted_at IS NULL
		RETURNING id`, instanceID, status).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return id, err
}

// UpdateProjectStatus 手动改项目状态。
func (s *Store) UpdateProjectStatus(ctx context.Context, id, status string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE project SET status = $2, updated_at = now() WHERE id = $1`, id, status)
	return err
}

// UpdateRiskFlag 部门经理手工标注风险标识。
func (s *Store) UpdateRiskFlag(ctx context.Context, id, flag string) error {
	var value any
	if flag != "" {
		value = flag
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE project SET risk_flag = $2, updated_at = now() WHERE id = $1`, id, value)
	return err
}

// ReplaceManagers 更换项目经理：新名单设为项目经理并恢复参与，移出的置为已退出。
func (s *Store) ReplaceManagers(ctx context.Context, projectID string, userIDs []string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		UPDATE project_member SET status = '已退出', left_at = CURRENT_DATE
		WHERE project_id = $1 AND project_role = '项目经理' AND status = '参与中'
		  AND NOT (user_id = ANY($2::text[]))`, projectID, userIDs); err != nil {
		return err
	}
	for _, uid := range userIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO project_member (id, project_id, user_id, project_role, status)
			VALUES ($1, $2, $3, '项目经理', '参与中')
			ON CONFLICT (project_id, user_id) DO UPDATE SET
				project_role = '项目经理', status = '参与中', left_at = NULL`,
			NewID(), projectID, uid); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// MilestonePlanInput 批量填计划的一行。
type MilestonePlanInput struct {
	ID          string
	PlanDue     *time.Time
	ContractDue *time.Time
}

// UpdateMilestonePlans 批量保存节点计划，已锁定的阶段跳过；返回实际更新条数。
func (s *Store) UpdateMilestonePlans(ctx context.Context, projectID string, plans []MilestonePlanInput) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	updated := 0
	for _, p := range plans {
		if p.PlanDue == nil && p.ContractDue == nil {
			continue
		}
		tag, err := tx.Exec(ctx, `
			UPDATE milestone SET
				plan_due = COALESCE($3, plan_due),
				contract_due = COALESCE($4, contract_due),
				updated_at = now()
			WHERE id = $1 AND project_id = $2 AND NOT locked`,
			p.ID, projectID, p.PlanDue, p.ContractDue)
		if err != nil {
			return 0, err
		}
		updated += int(tag.RowsAffected())
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	if err := s.RecalcProjectStage(ctx, projectID); err != nil {
		return updated, err
	}
	return updated, nil
}

// MilestoneUpdate 单条里程碑编辑。
type MilestoneUpdate struct {
	PlanDue     *time.Time
	ContractDue *time.Time
	Status      *string
	ActualDone  *time.Time
	Reviewer    *string
	ReviewDate  *time.Time
}

// UpdateMilestone 编辑单个里程碑，锁定后拒绝计划类修改。
func (s *Store) UpdateMilestone(ctx context.Context, projectID, id string, in MilestoneUpdate) error {
	var locked bool
	if err := s.pool.QueryRow(ctx, `SELECT locked FROM milestone WHERE id = $1 AND project_id = $2`,
		id, projectID).Scan(&locked); err != nil {
		return err
	}
	if locked {
		return ErrMilestoneLocked
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE milestone SET
			plan_due = COALESCE($3, plan_due),
			contract_due = COALESCE($4, contract_due),
			status = COALESCE($5, status),
			actual_done = COALESCE($6, actual_done),
			reviewer = COALESCE($7, reviewer),
			review_date = COALESCE($8, review_date),
			updated_at = now()
		WHERE id = $1 AND project_id = $2`,
		id, projectID, in.PlanDue, in.ContractDue, in.Status, in.ActualDone, in.Reviewer, in.ReviewDate)
	if err != nil {
		return err
	}
	return s.RecalcProjectStage(ctx, projectID)
}

// ErrMilestoneLocked 表示阶段已锁定，须走变更。
var ErrMilestoneLocked = errors.New("该阶段已锁定，修改请走变更单")

// RecalcProjectStage 按里程碑重算当前阶段 / 当前里程碑 / 里程碑状态。
// 全部完成时停留在最后一个阶段。
func (s *Store) RecalcProjectStage(ctx context.Context, projectID string) error {
	var stage, milestone, status string
	err := s.pool.QueryRow(ctx, `
		SELECT stage, name, status FROM milestone
		WHERE project_id = $1 AND status <> '已完成'
		ORDER BY sort_order LIMIT 1`, projectID).Scan(&stage, &milestone, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		_, err = s.pool.Exec(ctx, `
			UPDATE project p SET
				current_stage = COALESCE(
					(SELECT stage FROM milestone WHERE project_id = $1 ORDER BY sort_order DESC LIMIT 1),
					p.current_stage),
				current_milestone = COALESCE(
					(SELECT name FROM milestone WHERE project_id = $1 ORDER BY sort_order DESC LIMIT 1),
					p.current_milestone),
				milestone_status = '已完成',
				updated_at = now()
			WHERE p.id = $1`, projectID)
		return err
	}
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE project SET current_stage = $2, current_milestone = $3,
			milestone_status = $4, updated_at = now()
		WHERE id = $1`, projectID, stage, milestone, status)
	return err
}

// TaskInput 新建任务。
type TaskInput struct {
	Name               string
	Description        string
	Acceptance         []string
	Priority           string
	Tags               []string
	Skills             []TaskSkill
	Stage              string
	OwnerID            string
	JobType            string
	PlanStart          *time.Time
	PlanDue            *time.Time
	EstimateHours      float64
	CreatedBy          string
	TaskType           string
	TaskTypeCode       string
	ArchiveFolder      string
	ArchiveTarget      string
	ApprovalInstanceID string
	ApprovalStatus     string
}

// CreateTask 下发任务。
func (s *Store) CreateTask(ctx context.Context, projectID string, in TaskInput) (string, error) {
	id := NewID()
	if in.Priority == "" {
		in.Priority = "P1"
	}
	if in.Acceptance == nil {
		in.Acceptance = []string{}
	}
	if in.Tags == nil {
		in.Tags = []string{}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		INSERT INTO task (id, project_id, stage, name, description, acceptance, priority, tags,
			owner_id, job_type, plan_start, plan_due, status, estimate_hours, created_by,
			task_type, task_type_code, archive_folder, archive_target, approval_instance_id, approval_status)
		VALUES ($1, $2, NULLIF($3, ''), $4, NULLIF($5, ''), $6, $7, $8,
			$9, NULLIF($10, ''), $11, $12, '未开始', $13, NULLIF($14, ''),
			COALESCE(NULLIF($15, ''), '普通任务'), NULLIF($16, ''), NULLIF($17, ''), COALESCE(NULLIF($18, ''), 'project'), NULLIF($19, ''), NULLIF($20, ''))`,
		id, projectID, in.Stage, in.Name, in.Description, in.Acceptance, in.Priority, in.Tags,
		in.OwnerID, in.JobType, in.PlanStart, in.PlanDue,
		in.EstimateHours, in.CreatedBy, in.TaskType, in.TaskTypeCode, in.ArchiveFolder, in.ArchiveTarget,
		in.ApprovalInstanceID, in.ApprovalStatus); err != nil {
		return "", err
	}
	for i, sk := range in.Skills {
		source := sk.Source
		if source == "" {
			source = "manual"
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO task_skill (id, task_id, skill_name, skill_version, sort_order, source)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			NewID(), id, sk.Name, sk.Version, i, source); err != nil {
			return "", err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	_ = s.AddTaskEvent(ctx, id, "created", "任务已下发："+in.Name, in.CreatedBy, "", "info", 0)
	return id, nil
}

// UpdateTaskStatus 任务状态流转；完成时补实际完成日。
func (s *Store) UpdateTaskStatus(ctx context.Context, id, status string, done *time.Time) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE task SET status = $2, actual_done = COALESCE($3, actual_done), updated_at = now()
		WHERE id = $1`, id, status, done)
	return err
}

// AddMembers 批量增加项目成员。
func (s *Store) AddMembers(ctx context.Context, projectID string, userIDs []string, role string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, uid := range userIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO project_member (id, project_id, user_id, project_role, status)
			VALUES ($1, $2, $3, $4, '参与中')
			ON CONFLICT (project_id, user_id) DO UPDATE SET
				project_role = EXCLUDED.project_role, status = '参与中', left_at = NULL`,
			NewID(), projectID, uid, role); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// SetMemberStatus 退出 / 恢复参与。
func (s *Store) SetMemberStatus(ctx context.Context, projectID, userID, status string) error {
	var leftAt any
	if status == "已退出" {
		leftAt = time.Now()
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE project_member SET status = $3, left_at = $4
		WHERE project_id = $1 AND user_id = $2`, projectID, userID, status, leftAt)
	return err
}
