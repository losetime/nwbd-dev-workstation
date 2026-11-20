// Package org 负责组织与人员：钉钉通讯录同步、岗位映射、权限计算。
package org

import (
	"context"
	"log/slog"
	"time"

	"nwbd-dev-workstation/internal/dingtalk"
	"nwbd-dev-workstation/internal/domain"
	"nwbd-dev-workstation/internal/store"
)

// Service 组织与人员服务。
type Service struct {
	store  *store.Store
	client *dingtalk.Client
	log    *slog.Logger
}

// NewService 构造服务。
func NewService(st *store.Store, client *dingtalk.Client, log *slog.Logger) *Service {
	return &Service{store: st, client: client, log: log}
}

// SyncResult 是一次通讯录同步的结果。
type SyncResult struct {
	Total  int `json:"total"`
	Synced int `json:"synced"`
}

// Sync 拉取全公司通讯录并写入人员底表。
//
// 只做「新增 + 更新」：钉钉里删掉或不可见的人不会被自动标记离职，
// 避免通讯录权限范围配置不完整时误伤。
func (s *Service) Sync(ctx context.Context) (SyncResult, error) {
	users, err := s.client.Directory(ctx)
	if err != nil {
		return SyncResult{}, err
	}

	result := SyncResult{Total: len(users)}
	for _, u := range users {
		person := store.Person{
			ID:             store.NewID(),
			DingTalkUserID: u.UserID,
			UnionID:        u.UnionID,
			Name:           u.Name,
			Title:          u.Title,
			Dept:           u.Dept,
			DeptID:         u.DeptID,
			Avatar:         u.Avatar,
			JobType:        domain.JobTypeForTitle(u.Title),
			Status:         "在职",
		}
		if _, err := s.store.UpsertPerson(ctx, person); err != nil {
			s.log.Warn("写入人员失败", "user", u.Name, "error", err)
			continue
		}
		result.Synced++
	}

	// 至少要有部门经理，否则系统没人能管
	s.log.Info("通讯录同步完成", "total", result.Total, "synced", result.Synced)
	return result, nil
}

// StartSync 启动定时同步：启动即同步一次，之后按 interval 刷新。
func (s *Service) StartSync(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	go func() {
		if _, err := s.Sync(ctx); err != nil {
			s.log.Warn("初始通讯录同步失败", "error", err)
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := s.Sync(ctx); err != nil {
					s.log.Warn("定时通讯录同步失败", "error", err)
				}
			}
		}
	}()
}
