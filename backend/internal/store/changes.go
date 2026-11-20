package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Change 是项目变更申请。
type Change struct {
	ID             string
	Code           string
	ProjectID      string
	ProjectName    string
	Type           string
	Source         string
	Content        string
	Impact         string
	Reason         string
	BudgetBefore   *float64
	BudgetAfter    *float64
	PlanDueAfter   *time.Time
	MaterialPath   string
	Status         string
	ApprovalStatus string
	ApplicantID    string
	ApplicantName  string
	AppliedOn      *time.Time
	DtInstanceID   string
	DtBusinessID   string
	BeforeValue    string
	AfterValue     string
}

// ChangeInput 新建变更申请。
type ChangeInput struct {
	ProjectID     string
	Type          string
	Source        string
	Content       string
	Impact        string
	Reason        string
	BudgetBefore  *float64
	BudgetAfter   *float64
	PlanDueAfter  *time.Time
	MaterialPath  string
	ApplicantID   string
	DtInstanceID  string
	DtBusinessID  string
	DtProcessCode string
	BeforeValue   string
	AfterValue    string
}

const changeColumns = `
	c.id, COALESCE(c.code, ''), c.project_id, p.name, c.type, c.source,
	COALESCE(c.content, ''), COALESCE(c.impact, ''), c.reason,
	c.budget_before, c.budget_after, c.plan_due_after, COALESCE(c.material_path, ''),
	c.status, COALESCE(c.approval_status, ''), COALESCE(c.applicant_id, ''), COALESCE(u.name, ''),
	c.applied_on, COALESCE(c.dingtalk_instance_id, ''), COALESCE(c.dingtalk_business_id, ''),
	COALESCE(c.before_value, ''), COALESCE(c.after_value, '')`

func scanChange(row pgx.Row) (Change, error) {
	var c Change
	err := row.Scan(&c.ID, &c.Code, &c.ProjectID, &c.ProjectName, &c.Type, &c.Source,
		&c.Content, &c.Impact, &c.Reason, &c.BudgetBefore, &c.BudgetAfter, &c.PlanDueAfter,
		&c.MaterialPath, &c.Status, &c.ApprovalStatus, &c.ApplicantID, &c.ApplicantName,
		&c.AppliedOn, &c.DtInstanceID, &c.DtBusinessID, &c.BeforeValue, &c.AfterValue)
	return c, err
}

// CreateChange 新建变更申请，编号 CHG-0001 递增。
func (s *Store) CreateChange(ctx context.Context, in ChangeInput) (string, error) {
	id := NewID()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var seq int
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(CAST(substr(code, 5) AS INTEGER)), 0) + 1
		FROM change WHERE code ~ '^CHG-[0-9]+$'`).Scan(&seq); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO change (id, code, project_id, type, source, reason, status, applicant_id,
			applied_on, dingtalk_process_code, dingtalk_instance_id, dingtalk_business_id,
			content, impact, budget_before, budget_after, plan_due_after, material_path,
			approval_status, before_value, after_value)
		VALUES ($1, $2, $3, $4, $5, $6, '审批中', NULLIF($7, ''), CURRENT_DATE, NULLIF($8, ''),
			NULLIF($9, ''), NULLIF($10, ''), NULLIF($11, ''), NULLIF($12, ''), $13, $14, $15,
			NULLIF($16, ''), '审批中', NULLIF($17, ''), NULLIF($18, ''))`,
		id, fmt.Sprintf("CHG-%04d", seq), in.ProjectID, in.Type, in.Source, in.Reason,
		in.ApplicantID, in.DtProcessCode, in.DtInstanceID, in.DtBusinessID,
		in.Content, in.Impact, in.BudgetBefore, in.BudgetAfter, in.PlanDueAfter,
		in.MaterialPath, in.BeforeValue, in.AfterValue); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return id, nil
}

// GetChange 取单条变更。
func (s *Store) GetChange(ctx context.Context, id string) (*Change, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+changeColumns+`
		FROM change c JOIN project p ON p.id = c.project_id
		LEFT JOIN app_user u ON u.id = c.applicant_id
		WHERE c.id = $1`, id)
	c, err := scanChange(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ChangeByInstance 按钉钉审批实例找变更。
func (s *Store) ChangeByInstance(ctx context.Context, instanceID string) (*Change, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+changeColumns+`
		FROM change c JOIN project p ON p.id = c.project_id
		LEFT JOIN app_user u ON u.id = c.applicant_id
		WHERE c.dingtalk_instance_id = $1`, instanceID)
	c, err := scanChange(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ChangeFilter 变更列表筛选。
type ChangeFilter struct {
	ProjectID string
	Status    string
	ViewerID  string // 非空只返回该人参与中的项目
}

// ListChanges 变更列表（新的在前）。
func (s *Store) ListChanges(ctx context.Context, f ChangeFilter) ([]Change, error) {
	sql := `SELECT ` + changeColumns + `
		FROM change c JOIN project p ON p.id = c.project_id
		LEFT JOIN app_user u ON u.id = c.applicant_id
		WHERE p.deleted_at IS NULL`
	args := []any{}
	add := func(cond string, val any) {
		args = append(args, val)
		sql += " AND " + cond + " $" + itoa(len(args))
	}
	if f.ProjectID != "" {
		add("c.project_id =", f.ProjectID)
	}
	if f.Status != "" {
		add("c.approval_status =", f.Status)
	}
	if f.ViewerID != "" {
		add("EXISTS (SELECT 1 FROM project_member m WHERE m.project_id = p.id AND m.user_id =", f.ViewerID)
		sql += " AND m.status = '参与中')"
	}
	sql += " ORDER BY c.applied_on DESC, c.created_at DESC"

	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Change, 0, 16)
	for rows.Next() {
		c, err := scanChange(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SetChangeApproval 回写审批状态与编号。
func (s *Store) SetChangeApproval(ctx context.Context, id, approvalStatus, status, businessID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE change SET approval_status = $2, status = $3,
			dingtalk_business_id = COALESCE(NULLIF($4, ''), dingtalk_business_id),
			updated_at = now()
		WHERE id = $1`, id, approvalStatus, status, businessID)
	return err
}

// ApplyChangeToProject 审批通过后把变更写回项目（预算 / 计划交付日期）。
func (s *Store) ApplyChangeToProject(ctx context.Context, changeID string) error {
	c, err := s.GetChange(ctx, changeID)
	if err != nil || c == nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE project SET
			budget = COALESCE($2, budget),
			plan_end = COALESCE($3, plan_end),
			updated_at = now()
		WHERE id = $1`, c.ProjectID, c.BudgetAfter, c.PlanDueAfter)
	return err
}
