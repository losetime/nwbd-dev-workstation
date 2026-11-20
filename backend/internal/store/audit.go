package store

import (
	"context"
	"encoding/json"
)

// AddAudit 写一条操作日志。
func (s *Store) AddAudit(ctx context.Context, actorID, actorName, action, targetType, targetID string, detail map[string]any) error {
	var body []byte
	if detail != nil {
		encoded, err := json.Marshal(detail)
		if err != nil {
			return err
		}
		body = encoded
	}
	var target any
	if targetID != "" {
		target = targetID
	}
	var actor any
	if actorID != "" {
		actor = actorID
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO audit_log (id, actor_id, actor_name, action, target_type, target_id, detail)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		NewID(), actor, actorName, action, targetType, target, body)
	return err
}
