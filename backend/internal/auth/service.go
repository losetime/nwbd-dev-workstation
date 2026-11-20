// Package auth 提供钉钉扫码登录、设备码与长效令牌校验。
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"nwbd-dev-workstation/internal/config"
	"nwbd-dev-workstation/internal/dingtalk"
	"nwbd-dev-workstation/internal/domain"
	"nwbd-dev-workstation/internal/store"
)

const deviceTTL = 10 * time.Minute

// Service 负责登录与令牌。
type Service struct {
	cfg    config.Config
	store  *store.Store
	client *dingtalk.Client
	log    *slog.Logger
	syncMu sync.Mutex
}

// NewService 构造服务。
func NewService(cfg config.Config, st *store.Store, client *dingtalk.Client, log *slog.Logger) *Service {
	return &Service{cfg: cfg, store: st, client: client, log: log}
}

// DeviceStart 是脚本发起登录时拿到的信息。
type DeviceStart struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// PollResult 是脚本轮询的结果。
type PollResult struct {
	Status    string    `json:"status"`
	Token     string    `json:"token,omitempty"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	User      *UserInfo `json:"user,omitempty"`
}

// UserInfo 是返回给前端/脚本的用户信息。
type UserInfo struct {
	Name string `json:"name"`
	Dept string `json:"dept"`
	Role string `json:"role"`
}

// StartDevice 生成设备码与登录页地址。
func (s *Service) StartDevice(ctx context.Context) (DeviceStart, error) {
	code, err := randomToken(24)
	if err != nil {
		return DeviceStart{}, err
	}
	userCode, err := randomCode(6)
	if err != nil {
		return DeviceStart{}, err
	}
	expires := time.Now().Add(deviceTTL)
	if err := s.store.CreateDeviceCode(ctx, code, userCode, expires); err != nil {
		return DeviceStart{}, err
	}
	return DeviceStart{
		DeviceCode:      code,
		UserCode:        userCode,
		VerificationURI: s.loginURL() + "?device=" + code,
		ExpiresIn:       int(deviceTTL.Seconds()),
		Interval:        2,
	}, nil
}

// PollDevice 供脚本轮询；token 只会返回一次。
func (s *Service) PollDevice(ctx context.Context, deviceCode string) (PollResult, error) {
	record, err := s.store.GetDeviceCode(ctx, deviceCode)
	if err != nil {
		return PollResult{}, err
	}
	if record == nil {
		return PollResult{Status: "unknown"}, nil
	}
	if record.Status == "approved" && record.TokenPlain != nil {
		expires := time.Time{}
		if record.TokenExpiresAt != nil {
			expires = *record.TokenExpiresAt
		}
		result := PollResult{Status: "approved", Token: *record.TokenPlain, ExpiresAt: expires}
		if record.UserID != nil {
			if info, lookupErr := s.store.UserInfo(ctx, *record.UserID); lookupErr == nil && info != nil {
				result.User = &UserInfo{Name: info.Name, Dept: info.Dept, Role: info.Role}
			}
		}
		_ = s.store.ConsumeDeviceCode(ctx, deviceCode)
		return result, nil
	}
	if time.Now().After(record.ExpiresAt) {
		return PollResult{Status: "expired"}, nil
	}
	return PollResult{Status: "pending"}, nil
}

// Exchange 用扫码得到的 authCode 完成登录，并把令牌绑定到设备码。
// 返回用户信息与令牌明文（令牌只用于下发 Cookie / 设备码，服务端只存哈希）。
func (s *Service) Exchange(ctx context.Context, authCode, deviceCode string) (*store.AuthUser, string, error) {
	loginUser, err := s.client.ExchangeUserCode(ctx, authCode)
	if err != nil {
		return nil, "", err
	}

	user, err := s.store.SyncPersonOnLogin(ctx, store.Person{
		ID:             store.NewID(),
		DingTalkUserID: loginUser.UserID,
		UnionID:        loginUser.UnionID,
		Name:           loginUser.Name,
		Title:          loginUser.Title,
		Dept:           loginUser.Dept,
		JobType:        domain.JobTypeForTitle(loginUser.Title),
		Status:         "在职",
	})
	if err != nil {
		return nil, "", err
	}

	// 角色以钉钉管理员列表为准：默认 member，主/子管理员由同步提升。
	// 同步失败不影响登录，沿用库里的旧角色。
	if syncErr := s.SyncRoles(ctx); syncErr != nil {
		s.log.Warn("登录时同步钉钉角色失败，沿用现有角色", "user", user.Name, "error", syncErr)
	} else if refreshed, lookupErr := s.store.UserInfo(ctx, user.ID); lookupErr == nil && refreshed != nil {
		user = *refreshed
	}

	token, expiresAt, tokenHash, err := s.issueToken()
	if err != nil {
		return nil, "", err
	}
	if err := s.store.InsertToken(ctx, store.NewID(), user.ID, "login", tokenHash, expiresAt); err != nil {
		return nil, "", err
	}

	if deviceCode != "" {
		if err := s.store.ApproveDeviceCode(ctx, deviceCode, user.ID, token, expiresAt); err != nil {
			s.log.Warn("绑定设备码失败", "error", err)
		}
	}

	s.log.Info("用户登录成功", "user", user.Name, "user_id", user.ID)
	return &user, token, nil
}

// DevLoginEnabled 返回本地开发登录是否可用。
func (s *Service) DevLoginEnabled() bool { return s.cfg.DevLoginEnabled() }

// IssueForUser 为指定用户签发一个令牌（仅用于本地测试身份切换）。
func (s *Service) IssueForUser(ctx context.Context, userID string) (*store.Person, string, error) {
	user, err := s.store.GetPerson(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	if user == nil {
		return nil, "", errors.New("用户不存在")
	}
	if user.Status != "在职" {
		return nil, "", errors.New("该用户已离职，不能切换")
	}

	token, expiresAt, tokenHash, err := s.issueToken()
	if err != nil {
		return nil, "", err
	}
	if err := s.store.InsertToken(ctx, store.NewID(), user.ID, "dev-as", tokenHash, expiresAt); err != nil {
		return nil, "", err
	}
	s.log.Warn("测试身份切换（仅本地）", "user", user.Name, "user_id", user.ID)
	return user, token, nil
}

// DevLogin 仅本地开发使用：跳过钉钉，为固定账号签发令牌。
func (s *Service) DevLogin(ctx context.Context) (*store.AuthUser, string, error) {
	if !s.cfg.DevLoginEnabled() {
		return nil, "", errors.New("本地开发登录未启用（NWBD_WS_ENVIRONMENT=local 且 NWBD_WS_DEV_LOGIN_USERID 非空）")
	}

	user, err := s.store.EnsureDevUser(ctx, store.AuthUser{
		ID:             store.NewID(),
		DingTalkUserID: s.cfg.DevLoginUserID,
		Name:           s.cfg.DevLoginName,
		Title:          "本地开发",
		Dept:           "本地",
	})
	if err != nil {
		return nil, "", err
	}

	token, expiresAt, tokenHash, err := s.issueToken()
	if err != nil {
		return nil, "", err
	}
	if err := s.store.InsertToken(ctx, store.NewID(), user.ID, "dev", tokenHash, expiresAt); err != nil {
		return nil, "", err
	}

	s.log.Warn("已使用本地开发登录（不经过钉钉）", "user", user.Name, "user_id", user.ID)
	return &user, token, nil
}

// SyncRoles 拉取钉钉管理员列表并全量刷新本地角色。
//
// 规则：默认 member；sys_level=1 → admin，sys_level=2 → sub_admin；
// 被撤销管理员的人降回 member。
func (s *Service) SyncRoles(ctx context.Context) error {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()

	admins, err := s.client.ListAdmins(ctx)
	if err != nil {
		return err
	}
	if len(admins) == 0 {
		// 正常企业至少有主管理员；空列表更可能是接口异常，不能据此把所有人降权。
		return fmt.Errorf("钉钉管理员列表为空，跳过同步")
	}

	levels := make(map[string]int, len(admins))
	for _, admin := range admins {
		if current, ok := levels[admin.UserID]; !ok || admin.Level < current {
			levels[admin.UserID] = admin.Level
		}
	}

	adminIDs := make([]string, 0, len(levels))
	subAdminIDs := make([]string, 0, len(levels))
	for userID, level := range levels {
		if level == dingtalk.AdminLevelSuper {
			adminIDs = append(adminIDs, userID)
			continue
		}
		subAdminIDs = append(subAdminIDs, userID)
	}

	changed, err := s.store.SyncDingTalkRoles(ctx, adminIDs, subAdminIDs)
	if err != nil {
		return err
	}
	s.log.Info("钉钉管理员角色已同步",
		"admins", len(adminIDs), "sub_admins", len(subAdminIDs), "changed", changed)
	return nil
}

// StartRoleSync 启动角色同步：先立即同步一次，之后按 interval 定时刷新。
// 失败只记日志，等待下一轮；不影响服务和登录。
func (s *Service) StartRoleSync(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	go func() {
		if err := s.SyncRoles(ctx); err != nil {
			s.log.Warn("初始钉钉角色同步失败", "error", err)
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.SyncRoles(ctx); err != nil {
					s.log.Warn("定时同步钉钉管理员失败", "error", err)
				}
			}
		}
	}()
}

// Authenticate 校验 Bearer token。
func (s *Service) Authenticate(ctx context.Context, token string) (*store.TokenInfo, error) {
	token = strings.TrimSpace(strings.TrimPrefix(token, "Bearer "))
	if token == "" {
		return nil, errors.New("缺少令牌")
	}
	info, err := s.store.FindToken(ctx, store.HashToken(token))
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, errors.New("令牌无效或已过期")
	}
	s.store.TouchToken(ctx, info.TokenID)
	return info, nil
}

// Logout 吊销当前令牌。
func (s *Service) Logout(ctx context.Context, token string) error {
	token = strings.TrimSpace(strings.TrimPrefix(token, "Bearer "))
	if token == "" {
		return nil
	}
	info, err := s.store.FindToken(ctx, store.HashToken(token))
	if err != nil || info == nil {
		return nil
	}
	return s.store.RevokeToken(ctx, info.TokenID)
}

// TokenTTL 返回令牌有效期。
func (s *Service) TokenTTL() time.Duration {
	days := s.cfg.TokenTTLDays
	if days <= 0 {
		days = 30
	}
	return time.Duration(days) * 24 * time.Hour
}

// LoginURL 返回前端登录页地址。
func (s *Service) LoginURL() string { return s.loginURL() }

func (s *Service) loginURL() string {
	base := s.cfg.PublicBaseURL
	if base == "" {
		base = "http://127.0.0.1:8090"
	}
	return strings.TrimRight(base, "/") + "/login"
}

func (s *Service) issueToken() (string, time.Time, string, error) {
	raw, err := randomToken(32)
	if err != nil {
		return "", time.Time{}, "", err
	}
	token := "nwbdws_" + raw
	return token, time.Now().Add(s.TokenTTL()), store.HashToken(token), nil
}

func randomToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func randomCode(n int) (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, n)
	for i, b := range buf {
		out[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(out), nil
}
