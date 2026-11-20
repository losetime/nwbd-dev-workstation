package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Person 是人员档案（登录用户 + 业务岗位）。
type Person struct {
	ID             string
	DingTalkUserID string
	UnionID        string
	Name           string
	Title          string
	Dept           string
	Role           string
	JobType        string
	Level          string
	EmployeeNo     string
	Status         string
	DeptID         string
	Avatar         string
	UIPerms        []string
	LastLoginAt    *time.Time
	CreatedAt      time.Time
}

const personColumnList = `id, dingtalk_userid, COALESCE(unionid, ''), name, COALESCE(title, ''),
	COALESCE(dept, ''), role, job_type, level, COALESCE(employee_no, ''),
	status, COALESCE(dept_id, ''), COALESCE(avatar, ''), ui_perms, last_login_at, created_at`

// personColumnListU 是同一份列清单，带 u 别名（用于 JOIN）。
const personColumnListU = `u.id, u.dingtalk_userid, COALESCE(u.unionid, ''), u.name, COALESCE(u.title, ''),
	COALESCE(u.dept, ''), u.role, u.job_type, u.level, COALESCE(u.employee_no, ''),
	u.status, COALESCE(u.dept_id, ''), COALESCE(u.avatar, ''), u.ui_perms, u.last_login_at, u.created_at`

func scanPerson(row pgx.Row) (Person, error) {
	var p Person
	err := row.Scan(&p.ID, &p.DingTalkUserID, &p.UnionID, &p.Name, &p.Title, &p.Dept,
		&p.Role, &p.JobType, &p.Level, &p.EmployeeNo, &p.Status, &p.DeptID, &p.Avatar,
		&p.UIPerms, &p.LastLoginAt, &p.CreatedAt)
	return p, err
}

// UpsertPerson 通讯录同步用：按钉钉 userId 写入或更新，保留本地的级别与界面权限。
func (s *Store) UpsertPerson(ctx context.Context, p Person) (Person, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO app_user (id, dingtalk_userid, unionid, name, title, dept, job_type, status, dept_id, avatar)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (dingtalk_userid) DO UPDATE SET
			unionid = COALESCE(EXCLUDED.unionid, app_user.unionid),
			name = EXCLUDED.name,
			-- 管理员手工调整过岗位的人，同步时不覆盖
			job_type = CASE WHEN app_user.job_type_manual THEN app_user.job_type ELSE EXCLUDED.job_type END,
			title = EXCLUDED.title,
			dept = EXCLUDED.dept,
			status = EXCLUDED.status,
			dept_id = COALESCE(EXCLUDED.dept_id, app_user.dept_id),
			avatar = COALESCE(EXCLUDED.avatar, app_user.avatar)
		RETURNING `+personColumnList,
		p.ID, p.DingTalkUserID, p.UnionID, p.Name, p.Title, p.Dept, p.JobType, p.Status, p.DeptID, p.Avatar,
	)
	return scanPerson(row)
}

// SyncPersonOnLogin 登录时刷新：姓名/职位/部门/岗位，并记录最后登录时间。
func (s *Store) SyncPersonOnLogin(ctx context.Context, p Person) (Person, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO app_user (id, dingtalk_userid, unionid, name, title, dept, job_type, last_login_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, now())
		ON CONFLICT (dingtalk_userid) DO UPDATE SET
			unionid = COALESCE(EXCLUDED.unionid, app_user.unionid),
			name = EXCLUDED.name,
			-- 管理员手工调整过岗位的人，登录时不覆盖
			job_type = CASE WHEN app_user.job_type_manual THEN app_user.job_type ELSE EXCLUDED.job_type END,
			title = EXCLUDED.title,
			dept = EXCLUDED.dept,
			last_login_at = now()
		RETURNING `+personColumnList,
		p.ID, p.DingTalkUserID, p.UnionID, p.Name, p.Title, p.Dept, p.JobType,
	)
	return scanPerson(row)
}

// GetPersonByDingID 按钉钉 userId 找本地人员。
func (s *Store) GetPersonByDingID(ctx context.Context, dingUserID string) (*Person, error) {
	var id string
	err := s.pool.QueryRow(ctx, `SELECT id FROM app_user WHERE dingtalk_userid = $1`, dingUserID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.GetPerson(ctx, id)
}

// GetPerson 按主键取人员。
func (s *Store) GetPerson(ctx context.Context, id string) (*Person, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+personColumnList+` FROM app_user WHERE id = $1`, id)
	p, err := scanPerson(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// PersonFilter 人员查询条件。
type PersonFilter struct {
	Keyword string
	JobType string
	Status  string
}

// ListPeople 返回人员列表，只显示在职（除非指定状态）。
func (s *Store) ListPeople(ctx context.Context, f PersonFilter) ([]Person, error) {
	sql := `SELECT ` + personColumnList + ` FROM app_user WHERE 1=1`
	args := []any{}
	if f.Status != "" {
		args = append(args, f.Status)
		sql += " AND status = $1"
	} else {
		sql += " AND status = '在职'"
	}
	if f.JobType != "" {
		args = append(args, f.JobType)
		sql += " AND job_type = $" + itoa(len(args))
	}
	if f.Keyword != "" {
		args = append(args, "%"+f.Keyword+"%")
		sql += " AND (name ILIKE $" + itoa(len(args)) + " OR COALESCE(title,'') ILIKE $" + itoa(len(args)) + ")"
	}
	sql += " ORDER BY CASE job_type WHEN '部门经理' THEN 0 WHEN '项目经理' THEN 1 WHEN '开发' THEN 2 WHEN '测试' THEN 3 WHEN 'UI' THEN 4 ELSE 5 END, name"

	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Person, 0, 64)
	for rows.Next() {
		p, err := scanPerson(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// UpdatePersonInput 系统管理可改的字段。
type UpdatePersonInput struct {
	JobType *string
	Level   *string
	Status  *string
	UIPerms *[]string
}

// UpdatePerson 只更新管理员允许修改的字段。
func (s *Store) UpdatePerson(ctx context.Context, id string, in UpdatePersonInput) (*Person, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE app_user SET
			job_type = COALESCE($2, job_type),
			job_type_manual = CASE WHEN $2::text IS NULL THEN job_type_manual ELSE TRUE END,
			level = COALESCE($3, level),
			status = COALESCE($4, status),
			ui_perms = COALESCE($5, ui_perms)
		WHERE id = $1
		RETURNING `+personColumnList,
		id, in.JobType, in.Level, in.Status, in.UIPerms,
	)
	p, err := scanPerson(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ListPickableManagers 返回可被选为项目经理的人（在职的项目经理 / 部门经理）。
func (s *Store) ListPickableManagers(ctx context.Context) ([]Person, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+personColumnList+` FROM app_user
		WHERE status = '在职' AND job_type IN ('项目经理', '部门经理')
		ORDER BY CASE job_type WHEN '部门经理' THEN 0 ELSE 1 END, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Person, 0, 16)
	for rows.Next() {
		p, err := scanPerson(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListMemberCandidates 返回可加入项目的在职人员（排除已在项目里的）。
func (s *Store) ListMemberCandidates(ctx context.Context, projectID string) ([]Person, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+personColumnList+` FROM app_user u
		WHERE u.status = '在职'
		  AND NOT EXISTS (SELECT 1 FROM project_member m WHERE m.project_id = $1 AND m.user_id = u.id)
		ORDER BY CASE u.job_type WHEN '开发' THEN 0 WHEN '测试' THEN 1 WHEN 'UI' THEN 2 ELSE 3 END, u.name`,
		projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Person, 0, 64)
	for rows.Next() {
		p, err := scanPerson(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// CountPeople 按岗位统计在职人数（资源视图用）。
func (s *Store) CountPeople(ctx context.Context) (map[string]int, error) {
	rows, err := s.pool.Query(ctx, `SELECT job_type, count(*) FROM app_user WHERE status = '在职' GROUP BY job_type`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var job string
		var n int
		if err := rows.Scan(&job, &n); err != nil {
			return nil, err
		}
		out[job] = n
	}
	return out, rows.Err()
}
