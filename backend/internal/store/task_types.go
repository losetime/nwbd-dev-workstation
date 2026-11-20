package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// TaskTypeProjectPlanCode 是「项目策划」的字典编码：选它要发起 04 钉钉审批。
const TaskTypeProjectPlanCode = "TT01"

// TaskType 是任务类型字典（来自《任务类型归档阶段对照表》的项目任务）。
type TaskType struct {
	Code          string `json:"code"`
	Name          string `json:"name"`
	GroupName     string `json:"group_name"`
	ArchiveStage  string `json:"archive_stage"`
	ArchiveFolder string `json:"archive_folder"`
	ArchiveNote   string `json:"archive_note"`
	SortOrder     int    `json:"sort_order"`
}

// ListTaskTypes 返回启用的任务类型，按字典顺序。
func (s *Store) ListTaskTypes(ctx context.Context) ([]TaskType, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT code, name, group_name, archive_stage, archive_folder, archive_note, sort_order
		FROM task_type WHERE enabled ORDER BY sort_order`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]TaskType, 0, 40)
	for rows.Next() {
		var t TaskType
		if err := rows.Scan(&t.Code, &t.Name, &t.GroupName, &t.ArchiveStage,
			&t.ArchiveFolder, &t.ArchiveNote, &t.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetTaskType 按 code 取启用的任务类型。
func (s *Store) GetTaskType(ctx context.Context, code string) (*TaskType, error) {
	var t TaskType
	err := s.pool.QueryRow(ctx, `
		SELECT code, name, group_name, archive_stage, archive_folder, archive_note, sort_order
		FROM task_type WHERE code = $1 AND enabled`, code).
		Scan(&t.Code, &t.Name, &t.GroupName, &t.ArchiveStage, &t.ArchiveFolder, &t.ArchiveNote, &t.SortOrder)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}
