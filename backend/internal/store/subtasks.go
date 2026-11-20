package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Subtask 是任务内部的 TODO：研发用来把粗任务拆细。
type Subtask struct {
	ID          string     `json:"id"`
	TaskID      string     `json:"task_id"`
	Title       string     `json:"title"`
	Done        bool       `json:"done"`
	SortOrder   int        `json:"sort_order"`
	CreatedBy   string     `json:"created_by"`
	CreatorName string     `json:"creator_name"`
	CreatedAt   time.Time  `json:"created_at"`
	DoneBy      string     `json:"done_by"`
	DoneByName  string     `json:"done_by_name"`
	DoneAt      *time.Time `json:"done_at"`
}

const subtaskColumns = `s.id, s.task_id, s.title, s.done, s.sort_order,
	COALESCE(s.created_by, ''), COALESCE(cu.name, ''), s.created_at,
	COALESCE(s.done_by, ''), COALESCE(du.name, ''), s.done_at`

func scanSubtask(row pgx.Row) (*Subtask, error) {
	var s Subtask
	err := row.Scan(&s.ID, &s.TaskID, &s.Title, &s.Done, &s.SortOrder,
		&s.CreatedBy, &s.CreatorName, &s.CreatedAt, &s.DoneBy, &s.DoneByName, &s.DoneAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// ListSubtasks 任务的子任务，按排序号、创建时间。
func (s *Store) ListSubtasks(ctx context.Context, taskID string) ([]Subtask, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+subtaskColumns+`
		FROM task_subtask s
		LEFT JOIN app_user cu ON cu.id = s.created_by
		LEFT JOIN app_user du ON du.id = s.done_by
		WHERE s.task_id = $1
		ORDER BY s.sort_order, s.created_at`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Subtask, 0, 8)
	for rows.Next() {
		item, err := scanSubtask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

// CreateSubtask 新增子任务，排在最后。
func (s *Store) CreateSubtask(ctx context.Context, taskID, title, userID string) (*Subtask, error) {
	var sortOrder int
	if err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(max(sort_order), 0) + 10 FROM task_subtask WHERE task_id = $1`,
		taskID).Scan(&sortOrder); err != nil {
		return nil, err
	}

	var id string
	if err := s.pool.QueryRow(ctx, `
		INSERT INTO task_subtask (id, task_id, title, sort_order, created_by)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''))
		RETURNING id`, NewID(), taskID, title, sortOrder, userID).Scan(&id); err != nil {
		return nil, err
	}
	return s.GetSubtask(ctx, taskID, id)
}

// GetSubtask 取单个子任务。
func (s *Store) GetSubtask(ctx context.Context, taskID, id string) (*Subtask, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+subtaskColumns+`
		FROM task_subtask s
		LEFT JOIN app_user cu ON cu.id = s.created_by
		LEFT JOIN app_user du ON du.id = s.done_by
		WHERE s.task_id = $1 AND s.id = $2`, taskID, id)
	return scanSubtask(row)
}

// UpdateSubtask 改标题 / 勾选；done 为 nil 时不动。
func (s *Store) UpdateSubtask(ctx context.Context, taskID, id, title string, done *bool, userID string) (*Subtask, error) {
	_, err := s.pool.Exec(ctx, `
		UPDATE task_subtask SET
			title = COALESCE(NULLIF($3, ''), title),
			done = COALESCE($4, done),
			done_by = CASE WHEN $4 IS TRUE THEN COALESCE(NULLIF($5, ''), done_by)
			               WHEN $4 IS FALSE THEN NULL
			               ELSE done_by END,
			done_at = CASE WHEN $4 IS TRUE THEN now()
			               WHEN $4 IS FALSE THEN NULL
			               ELSE done_at END
		WHERE task_id = $1 AND id = $2`, taskID, id, title, done, userID)
	if err != nil {
		return nil, err
	}
	return s.GetSubtask(ctx, taskID, id)
}

// DeleteSubtask 删除子任务。
func (s *Store) DeleteSubtask(ctx context.Context, taskID, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM task_subtask WHERE task_id = $1 AND id = $2`, taskID, id)
	return err
}
