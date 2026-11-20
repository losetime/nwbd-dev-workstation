package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// RiskInput 登记风险的字段。
type RiskInput struct {
	Title       string
	Description string
	Type        string
	Level       string
	Impact      string
	Action      string
	OwnerID     string
	TargetDate  *time.Time
	// RaisedBy 提出人；为空时用当前操作人（困难转风险时传原上报人）。
	RaisedBy string
}

// CreateRisk 登记风险；编号按 RISK-0001 递增。
func (s *Store) CreateRisk(ctx context.Context, projectID string, in RiskInput, actorID string) (string, error) {
	id := NewID()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var seq int
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(CAST(substr(code, 6) AS INTEGER)), 0) + 1
		FROM risk WHERE code ~ '^RISK-[0-9]+$'`).Scan(&seq); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO risk (id, code, project_id, title, description, type, level, impact, action,
			owner_id, target_date, status, raised_by, identified_on)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, $7, NULLIF($8, ''), NULLIF($9, ''),
			NULLIF($10, ''), $11, '监控中', COALESCE(NULLIF($12, ''), NULLIF($13, '')), CURRENT_DATE)`,
		id, fmt.Sprintf("RISK-%04d", seq), projectID, in.Title, in.Description, in.Type, in.Level,
		in.Impact, in.Action, in.OwnerID, in.TargetDate, in.RaisedBy, actorID); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return id, nil
}

// RiskUpdate 编辑风险（nil = 不修改）。
type RiskUpdate struct {
	Title        *string
	Description  *string
	Type         *string
	Level        *string
	Impact       *string
	Action       *string
	OwnerID      *string
	TargetDate   *time.Time
	Status       *string
	ResolvedDate *time.Time
}

// RiskRow 全局风险列表的一行。
type RiskRow struct {
	Risk
	ProjectName string
	ProjectCode string
}

// RiskFilter 全局风险列表筛选。
type RiskFilter struct {
	Status   string
	Level    string
	Keyword  string
	ViewerID string
}

// ListRiskRows 跨项目风险列表：未解决在前，按等级与目标日排序。
func (s *Store) ListRiskRows(ctx context.Context, f RiskFilter) ([]RiskRow, error) {
	sql := `
		SELECT r.id, COALESCE(r.code, ''), r.project_id, r.title, COALESCE(r.description, ''),
		       r.type, r.level, COALESCE(r.impact, ''), COALESCE(r.action, ''),
		       COALESCE(r.owner_id, ''), COALESCE(u.name, ''), r.target_date, r.resolved_date,
		       r.status, r.identified_on, p.name, p.code
		FROM risk r
		JOIN project p ON p.id = r.project_id
		LEFT JOIN app_user u ON u.id = r.owner_id
		WHERE p.status <> '已取消'`
	args := []any{}
	add := func(cond string, val any) {
		args = append(args, val)
		sql += " AND " + cond + " $" + itoa(len(args))
	}
	if f.ViewerID != "" {
		add("EXISTS (SELECT 1 FROM project_member m WHERE m.project_id = p.id AND m.user_id =", f.ViewerID)
		sql += " AND m.status = '参与中')"
	}
	if f.Status == "未解决" {
		sql += " AND r.status <> '已解决'"
	} else if f.Status != "" {
		add("r.status =", f.Status)
	}
	if f.Level != "" {
		add("r.level =", f.Level)
	}
	if f.Keyword != "" {
		args = append(args, "%"+f.Keyword+"%")
		n := itoa(len(args))
		sql += " AND (r.title ILIKE $" + n + " OR COALESCE(r.code,'') ILIKE $" + n +
			" OR p.name ILIKE $" + n + " OR COALESCE(u.name,'') ILIKE $" + n + ")"
	}
	sql += `
		ORDER BY (r.status <> '已解决') DESC,
		         CASE r.level WHEN '高' THEN 0 WHEN '中' THEN 1 ELSE 2 END,
		         r.target_date NULLS LAST, r.identified_on DESC`

	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]RiskRow, 0, 32)
	for rows.Next() {
		var r RiskRow
		if err := rows.Scan(&r.ID, &r.Code, &r.ProjectID, &r.Title, &r.Description,
			&r.Type, &r.Level, &r.Impact, &r.Action,
			&r.OwnerID, &r.OwnerName, &r.TargetDate, &r.ResolvedDate,
			&r.Status, &r.IdentifiedOn, &r.ProjectName, &r.ProjectCode); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// RiskStats 风险总览（不受列表筛选影响，只按数据范围过滤）。
type RiskStats struct {
	Total    int
	Open     int
	High     int
	Overdue  int
	Resolved int
}

// CountRiskStats 统计风险总览。
func (s *Store) CountRiskStats(ctx context.Context, viewerID string) (RiskStats, error) {
	sql := `
		SELECT
			count(*),
			count(*) FILTER (WHERE r.status <> '已解决'),
			count(*) FILTER (WHERE r.status <> '已解决' AND r.level = '高'),
			count(*) FILTER (WHERE r.status <> '已解决' AND r.target_date < CURRENT_DATE),
			count(*) FILTER (WHERE r.status = '已解决')
		FROM risk r
		JOIN project p ON p.id = r.project_id
		WHERE p.status <> '已取消'`
	args := []any{}
	if viewerID != "" {
		args = append(args, viewerID)
		sql += " AND EXISTS (SELECT 1 FROM project_member m WHERE m.project_id = p.id AND m.user_id = $1 AND m.status = '参与中')"
	}
	var st RiskStats
	err := s.pool.QueryRow(ctx, sql, args...).Scan(&st.Total, &st.Open, &st.High, &st.Overdue, &st.Resolved)
	return st, err
}

// UpdateRisk 编辑风险；解决时写解决日期，重开时清空解决日期。
func (s *Store) UpdateRisk(ctx context.Context, id string, in RiskUpdate, actorID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE risk SET
			title = COALESCE($2, title),
			description = COALESCE($3, description),
			type = COALESCE($4, type),
			level = COALESCE($5, level),
			impact = COALESCE($6, impact),
			action = COALESCE($7, action),
			owner_id = COALESCE($8, owner_id),
			target_date = COALESCE($9, target_date),
			status = COALESCE($10, status),
			resolved_date = CASE
				WHEN $10 = '已解决' THEN COALESCE($11, resolved_date, CURRENT_DATE)
				WHEN $10 IS NOT NULL THEN NULL
				ELSE COALESCE($11, resolved_date)
			END,
			updated_by = NULLIF($12, ''),
			updated_at = now()
		WHERE id = $1`,
		id, in.Title, in.Description, in.Type, in.Level, in.Impact, in.Action,
		in.OwnerID, in.TargetDate, in.Status, in.ResolvedDate, actorID)
	return err
}

// GetRisk 取单条风险（含责任人姓名）。
func (s *Store) GetRisk(ctx context.Context, id string) (*Risk, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT r.id, COALESCE(r.code, ''), r.project_id, r.title, COALESCE(r.description, ''),
		       r.type, r.level, COALESCE(r.impact, ''), COALESCE(r.action, ''),
		       COALESCE(r.owner_id, ''), COALESCE(u.name, ''), r.target_date, r.resolved_date,
		       r.status, r.identified_on
		FROM risk r LEFT JOIN app_user u ON u.id = r.owner_id
		WHERE r.id = $1`, id)

	var r Risk
	err := row.Scan(&r.ID, &r.Code, &r.ProjectID, &r.Title, &r.Description,
		&r.Type, &r.Level, &r.Impact, &r.Action, &r.OwnerID, &r.OwnerName,
		&r.TargetDate, &r.ResolvedDate, &r.Status, &r.IdentifiedOn)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}
