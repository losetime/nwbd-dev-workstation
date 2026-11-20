package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Issue 困难上报。
type Issue struct {
	ID           string
	Code         string
	ProjectID    string
	ProjectName  string
	TaskID       string
	TaskName     string
	Title        string
	Description  string
	ReporterID   string
	ReporterName string
	JobType      string
	Severity     string
	Status       string
	Handling     string
	RiskID       string
	ReportedOn   *time.Time
}

// IssueInput 上报困难。
type IssueInput struct {
	ProjectID   string
	TaskID      string
	Title       string
	Description string
	Severity    string
}

// IssueFilter 列表筛选。
type IssueFilter struct {
	// ReporterID 非空：只看该人上报的
	ReporterID string
	// ManagerID 非空：待该人处理（其作为项目经理的项目）
	ManagerID string
	// ReporterJobType 非空：只统计该岗位上报的（部门经理看待处理的困难只看项目经理上报的）
	ReporterJobType string
	// ExcludeReporterJobTypes 非空：排除这些岗位上报的（项目经理待处理里排除其他项目经理/部门经理上报的）
	ExcludeReporterJobTypes []string
	// All 为 true 时不按人过滤（部门经理）
	All bool
}

const issueColumns = `
	i.id, i.code, i.project_id, p.name, COALESCE(i.task_id, ''), COALESCE(t.name, ''),
	i.title, COALESCE(i.description, ''), i.reporter_id, u.name, COALESCE(i.job_type, ''),
	i.severity, i.status, COALESCE(i.handling, ''), COALESCE(i.risk_id, ''), i.reported_on`

func scanIssue(row pgx.Row) (Issue, error) {
	var it Issue
	err := row.Scan(&it.ID, &it.Code, &it.ProjectID, &it.ProjectName, &it.TaskID, &it.TaskName,
		&it.Title, &it.Description, &it.ReporterID, &it.ReporterName, &it.JobType,
		&it.Severity, &it.Status, &it.Handling, &it.RiskID, &it.ReportedOn)
	return it, err
}

// CreateIssue 上报困难，编号 ISS-0001 递增。
func (s *Store) CreateIssue(ctx context.Context, in IssueInput, reporterID, jobType string) (string, error) {
	id := NewID()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var seq int
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(CAST(substr(code, 5) AS INTEGER)), 0) + 1
		FROM issue WHERE code ~ '^ISS-[0-9]+$'`).Scan(&seq); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO issue (id, code, project_id, task_id, title, description, reporter_id, job_type,
			severity, status, reported_on)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5, NULLIF($6, ''), $7, NULLIF($8, ''), $9, '待处理', CURRENT_DATE)`,
		id, fmt.Sprintf("ISS-%04d", seq), in.ProjectID, in.TaskID, in.Title, in.Description,
		reporterID, jobType, in.Severity); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return id, nil
}

// ListIssues 困难列表（待处理在前）。
func (s *Store) ListIssues(ctx context.Context, f IssueFilter) ([]Issue, error) {
	sql := `SELECT ` + issueColumns + `
		FROM issue i
		JOIN project p ON p.id = i.project_id
		LEFT JOIN task t ON t.id = i.task_id
		JOIN app_user u ON u.id = i.reporter_id
		WHERE 1 = 1`
	args := []any{}
	if !f.All {
		if f.ReporterID != "" {
			args = append(args, f.ReporterID)
			sql += " AND i.reporter_id = $1"
		} else if f.ManagerID != "" {
			args = append(args, f.ManagerID)
			sql += ` AND i.status = '待处理' AND EXISTS (
				SELECT 1 FROM project_member m
				WHERE m.project_id = i.project_id AND m.user_id = $1
				  AND m.status = '参与中' AND m.project_role = '项目经理')`
		} else {
			sql += " AND false"
		}
	}
	if f.ReporterJobType != "" {
		args = append(args, f.ReporterJobType)
		sql += " AND i.job_type = $" + itoa(len(args))
	}
	if len(f.ExcludeReporterJobTypes) > 0 {
		args = append(args, f.ExcludeReporterJobTypes)
		sql += " AND (i.job_type IS NULL OR i.job_type <> ALL($" + itoa(len(args)) + "))"
	}
	sql += ` ORDER BY (i.status = '待处理') DESC, i.reported_on DESC, i.created_at DESC`

	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Issue, 0, 16)
	for rows.Next() {
		it, err := scanIssue(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// GetIssue 取单条困难。
func (s *Store) GetIssue(ctx context.Context, id string) (*Issue, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+issueColumns+`
		FROM issue i
		JOIN project p ON p.id = i.project_id
		LEFT JOIN task t ON t.id = i.task_id
		JOIN app_user u ON u.id = i.reporter_id
		WHERE i.id = $1`, id)
	it, err := scanIssue(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &it, nil
}

// HandleIssue 处理困难：转风险或归档。
func (s *Store) HandleIssue(ctx context.Context, id, status, handling, riskID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE issue SET status = $2, handling = NULLIF($3, ''),
			risk_id = COALESCE(NULLIF($4, ''), risk_id), updated_at = now()
		WHERE id = $1`, id, status, handling, riskID)
	return err
}
