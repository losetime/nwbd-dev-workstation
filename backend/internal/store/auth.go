package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/jackc/pgx/v5"
)

// AuthUser 是登录后的用户，等于人员档案。
type AuthUser = Person

// TokenInfo 是校验通过的令牌信息。
type TokenInfo struct {
	TokenID   string
	User      Person
	ExpiresAt time.Time
}

// HashToken 返回 token 的 SHA-256 十六进制摘要。
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// UpsertUser 按钉钉 userId 写入或更新用户（不含岗位字段，保留给开发登录）。
func (s *Store) UpsertUser(ctx context.Context, u Person) (Person, error) {
	return s.SyncPersonOnLogin(ctx, u)
}

// InsertToken 写入令牌（只存哈希）。
func (s *Store) InsertToken(ctx context.Context, id, userID, label, tokenHash string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO auth_token (id, user_id, label, token_hash, expires_at)
		VALUES ($1, $2, $3, $4, $5)`,
		id, userID, label, tokenHash, expiresAt,
	)
	return err
}

// UserInfo 按主键取用户。
func (s *Store) UserInfo(ctx context.Context, id string) (*Person, error) {
	return s.GetPerson(ctx, id)
}

// FindToken 按哈希取有效令牌与用户。
func (s *Store) FindToken(ctx context.Context, tokenHash string) (*TokenInfo, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT t.id, t.expires_at, `+personColumnListU+`
		FROM auth_token t
		JOIN app_user u ON u.id = t.user_id
		WHERE t.token_hash = $1 AND t.revoked_at IS NULL AND t.expires_at > now()`,
		tokenHash,
	)
	var info TokenInfo
	if err := row.Scan(
		&info.TokenID, &info.ExpiresAt,
		&info.User.ID, &info.User.DingTalkUserID, &info.User.UnionID, &info.User.Name, &info.User.Title,
		&info.User.Dept, &info.User.Role, &info.User.JobType, &info.User.Level, &info.User.EmployeeNo,
		&info.User.Status, &info.User.DeptID, &info.User.Avatar, &info.User.UIPerms,
		&info.User.LastLoginAt, &info.User.CreatedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &info, nil
}

// TouchToken 更新最后使用时间（失败不影响请求）。
func (s *Store) TouchToken(ctx context.Context, tokenID string) {
	_, _ = s.pool.Exec(ctx, "UPDATE auth_token SET last_used_at = now() WHERE id = $1", tokenID)
}

// RevokeToken 吊销令牌。
func (s *Store) RevokeToken(ctx context.Context, tokenID string) error {
	_, err := s.pool.Exec(ctx, "UPDATE auth_token SET revoked_at = now() WHERE id = $1", tokenID)
	return err
}

// SyncDingTalkRoles 用钉钉管理员列表全量刷新本地角色。
//
// adminIDs → admin，subAdminIDs → sub_admin；其余当前是 admin/sub_admin
// 但已不在列表里的账号降回 member。默认值始终是 member。
func (s *Store) SyncDingTalkRoles(ctx context.Context, adminIDs, subAdminIDs []string) (int64, error) {
	if adminIDs == nil {
		adminIDs = []string{}
	}
	if subAdminIDs == nil {
		subAdminIDs = []string{}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var total int64
	steps := []struct {
		sql  string
		args []any
	}{
		{
			sql:  "UPDATE app_user SET role = 'sub_admin' WHERE dingtalk_userid = ANY($1::text[]) AND role <> 'sub_admin'",
			args: []any{subAdminIDs},
		},
		{
			sql:  "UPDATE app_user SET role = 'admin' WHERE dingtalk_userid = ANY($1::text[]) AND role <> 'admin'",
			args: []any{adminIDs},
		},
	}
	for _, step := range steps {
		tag, execErr := tx.Exec(ctx, step.sql, step.args...)
		if execErr != nil {
			return 0, execErr
		}
		total += tag.RowsAffected()
	}

	// 安全阀：本地至少要有一个账号与钉钉管理员列表对得上，才执行降权。
	var mapped int
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FROM app_user
		WHERE dingtalk_userid = ANY($1::text[]) OR dingtalk_userid = ANY($2::text[])`,
		adminIDs, subAdminIDs,
	).Scan(&mapped); err != nil {
		return 0, err
	}
	if mapped > 0 {
		tag, err := tx.Exec(ctx, `
			UPDATE app_user SET role = 'member'
			WHERE role IN ('admin', 'sub_admin')
			  AND NOT (dingtalk_userid = ANY($1::text[]) OR dingtalk_userid = ANY($2::text[]))`,
			adminIDs, subAdminIDs)
		if err != nil {
			return 0, err
		}
		total += tag.RowsAffected()
	}

	// 本地例外：无论钉钉管理员列表怎么变，这些账号始终保持 admin。
	tag, err := tx.Exec(ctx,
		"UPDATE app_user SET role = 'admin' WHERE role_override AND role <> 'admin'")
	if err != nil {
		return 0, err
	}
	total += tag.RowsAffected()

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return total, nil
}

// EnsureDevUser 用于本地开发登录：以固定账号建档，并强制保留 admin。
func (s *Store) EnsureDevUser(ctx context.Context, u Person) (Person, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO app_user (id, dingtalk_userid, name, title, dept, role, role_override,
			job_type, status, last_login_at)
		VALUES ($1, $2, $3, $4, $5, 'admin', TRUE, '部门经理', '在职', now())
		ON CONFLICT (dingtalk_userid) DO UPDATE SET
			name = EXCLUDED.name,
			title = EXCLUDED.title,
			dept = EXCLUDED.dept,
			role = 'admin',
			role_override = TRUE,
			last_login_at = now()
		RETURNING `+personColumnList,
		u.ID, u.DingTalkUserID, u.Name, u.Title, u.Dept,
	)
	return scanPerson(row)
}

// DeviceCode 是 CLI 登录流程中的设备码。
type DeviceCode struct {
	DeviceCode     string
	UserCode       string
	Status         string
	UserID         *string
	TokenPlain     *string
	TokenExpiresAt *time.Time
	ExpiresAt      time.Time
}

// CreateDeviceCode 生成设备码。
func (s *Store) CreateDeviceCode(ctx context.Context, code, userCode string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO device_code (device_code, user_code, expires_at) VALUES ($1, $2, $3)`,
		code, userCode, expiresAt)
	return err
}

// GetDeviceCode 读取设备码。
func (s *Store) GetDeviceCode(ctx context.Context, code string) (*DeviceCode, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT device_code, user_code, status, user_id, token_plain, token_expires_at, expires_at
		FROM device_code WHERE device_code = $1`, code)
	var d DeviceCode
	err := row.Scan(&d.DeviceCode, &d.UserCode, &d.Status, &d.UserID, &d.TokenPlain, &d.TokenExpiresAt, &d.ExpiresAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// ApproveDeviceCode 扫码完成后绑定用户与令牌。
func (s *Store) ApproveDeviceCode(ctx context.Context, code, userID, tokenPlain string, tokenExpiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE device_code
		SET status = 'approved', user_id = $2, token_plain = $3, token_expires_at = $4
		WHERE device_code = $1 AND status = 'pending'`,
		code, userID, tokenPlain, tokenExpiresAt)
	return err
}

// ConsumeDeviceCode 脚本取走 token 后清空明文。
func (s *Store) ConsumeDeviceCode(ctx context.Context, code string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE device_code SET token_plain = NULL, consumed_at = now() WHERE device_code = $1`, code)
	return err
}
