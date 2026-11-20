package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// TaskTypeProjectPlanCode 是「项目策划」的字典编码：选它要发起 04 钉钉审批。
const TaskTypeProjectPlanCode = "TT01"

// TaskTypeSkill 是任务类型的默认技能。
type TaskTypeSkill struct {
	Name    string `json:"name"`
	Version *int   `json:"version"`
}

// TaskType 是任务类型字典（来自《任务类型归档阶段对照表》）。
type TaskType struct {
	Code          string          `json:"code"`
	Name          string          `json:"name"`
	GroupName     string          `json:"group_name"`
	ArchiveStage  string          `json:"archive_stage"`
	ArchiveFolder string          `json:"archive_folder"`
	ArchiveTarget string          `json:"archive_target"`
	ArchiveNote   string          `json:"archive_note"`
	Description   string          `json:"description"`
	SortOrder     int             `json:"sort_order"`
	Enabled       bool            `json:"enabled"`
	UpdatedAt     time.Time       `json:"updated_at"`
	Skills        []TaskTypeSkill `json:"skills"`
}

const taskTypeColumns = `code, name, group_name, archive_stage, archive_folder, archive_target,
	archive_note, description, sort_order, enabled, updated_at`

func scanTaskType(row pgx.Row) (*TaskType, error) {
	var t TaskType
	err := row.Scan(&t.Code, &t.Name, &t.GroupName, &t.ArchiveStage, &t.ArchiveFolder,
		&t.ArchiveTarget, &t.ArchiveNote, &t.Description, &t.SortOrder, &t.Enabled, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t.Skills = []TaskTypeSkill{}
	return &t, nil
}

// ListTaskTypes 返回任务类型；includeDisabled 为 true 时包含已停用（管理页用）。
func (s *Store) ListTaskTypes(ctx context.Context, includeDisabled bool) ([]TaskType, error) {
	where := ""
	if !includeDisabled {
		where = " WHERE enabled"
	}
	rows, err := s.pool.Query(ctx, `SELECT `+taskTypeColumns+` FROM task_type`+where+` ORDER BY sort_order, code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]TaskType, 0, 40)
	codes := make([]string, 0, 40)
	for rows.Next() {
		t, err := scanTaskType(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
		codes = append(codes, t.Code)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}
	if err := s.attachTaskTypeSkills(ctx, out, codes); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) attachTaskTypeSkills(ctx context.Context, items []TaskType, codes []string) error {
	rows, err := s.pool.Query(ctx, `
		SELECT task_type_code, skill_name, skill_version FROM task_type_skill
		WHERE task_type_code = ANY($1) ORDER BY sort_order, skill_name`, codes)
	if err != nil {
		return err
	}
	defer rows.Close()

	byCode := map[string][]TaskTypeSkill{}
	for rows.Next() {
		var code, name string
		var version *int
		if err := rows.Scan(&code, &name, &version); err != nil {
			return err
		}
		byCode[code] = append(byCode[code], TaskTypeSkill{Name: name, Version: version})
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range items {
		if skills, ok := byCode[items[i].Code]; ok {
			items[i].Skills = skills
		}
	}
	return nil
}

// GetTaskType 按 code 取任务类型（含停用），不存在返回 nil。
func (s *Store) GetTaskType(ctx context.Context, code string) (*TaskType, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+taskTypeColumns+` FROM task_type WHERE code = $1`, code)
	t, err := scanTaskType(row)
	if err != nil || t == nil {
		return t, err
	}
	items := []TaskType{*t}
	if err := s.attachTaskTypeSkills(ctx, items, []string{t.Code}); err != nil {
		return nil, err
	}
	return &items[0], nil
}

// TaskTypeInput 新建 / 编辑任务类型的字段。
type TaskTypeInput struct {
	Name          string
	GroupName     string
	ArchiveStage  string
	ArchiveFolder string
	ArchiveTarget string
	ArchiveNote   string
	Description   string
	SortOrder     int
}

// CreateTaskType 新建任务类型，编码按组别自动生成（TT / DT）。
func (s *Store) CreateTaskType(ctx context.Context, in TaskTypeInput) (*TaskType, error) {
	prefix := "TT"
	if in.GroupName == "日常任务" {
		prefix = "DT"
	}
	var next int
	if err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(max(CAST(substr(code, 3) AS INTEGER)), 0) + 1
		FROM task_type WHERE code ~ ('^' || $1 || '[0-9]+$')`, prefix).Scan(&next); err != nil {
		return nil, err
	}
	code := fmt.Sprintf("%s%02d", prefix, next)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO task_type (code, name, group_name, archive_stage, archive_folder, archive_target,
			archive_note, description, sort_order, enabled)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,TRUE)`,
		code, in.Name, in.GroupName, in.ArchiveStage, in.ArchiveFolder, in.ArchiveTarget,
		in.ArchiveNote, in.Description, in.SortOrder)
	if err != nil {
		return nil, err
	}
	return s.GetTaskType(ctx, code)
}

// UpdateTaskType 编辑任务类型（编码不可改）。
func (s *Store) UpdateTaskType(ctx context.Context, code string, in TaskTypeInput) (*TaskType, error) {
	_, err := s.pool.Exec(ctx, `
		UPDATE task_type SET name = $2, group_name = $3, archive_stage = $4, archive_folder = $5,
			archive_target = $6, archive_note = $7, description = $8, sort_order = $9, updated_at = now()
		WHERE code = $1`,
		code, in.Name, in.GroupName, in.ArchiveStage, in.ArchiveFolder, in.ArchiveTarget,
		in.ArchiveNote, in.Description, in.SortOrder)
	if err != nil {
		return nil, err
	}
	return s.GetTaskType(ctx, code)
}

// SetTaskTypeEnabled 启用 / 停用。
func (s *Store) SetTaskTypeEnabled(ctx context.Context, code string, enabled bool) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE task_type SET enabled = $2, updated_at = now() WHERE code = $1`, code, enabled)
	return err
}

// CountTasksByType 统计引用该类型的任务数。
func (s *Store) CountTasksByType(ctx context.Context, code string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM task WHERE task_type_code = $1`, code).Scan(&n)
	return n, err
}

// DeleteTaskType 删除任务类型（调用方需先确认没有任务引用）。
func (s *Store) DeleteTaskType(ctx context.Context, code string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM task_type WHERE code = $1`, code)
	return err
}

// SetTaskTypeSkills 覆盖任务类型的默认技能。
func (s *Store) SetTaskTypeSkills(ctx context.Context, code string, skills []TaskTypeSkill) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM task_type_skill WHERE task_type_code = $1`, code); err != nil {
		return err
	}
	for i, sk := range skills {
		if _, err := tx.Exec(ctx, `
			INSERT INTO task_type_skill (id, task_type_code, skill_name, skill_version, sort_order)
			VALUES ($1,$2,$3,$4,$5)`, NewID(), code, sk.Name, sk.Version, i); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
