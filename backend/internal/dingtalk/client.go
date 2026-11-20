// Package dingtalk 封装钉钉开放平台调用（登录、通讯录、管理员）。
package dingtalk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"nwbd-dev-workstation/internal/config"
)

// Client 负责取 access_token 与调用钉钉接口。
type Client struct {
	cfg  config.Config
	http *http.Client

	mu      sync.Mutex
	token   string
	expires time.Time
}

// NewClient 构造客户端。
func NewClient(cfg config.Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.DingTalkTimeout},
	}
}

// AccessToken 取应用级 token，进程内缓存并提前 5 分钟过期。
func (c *Client) AccessToken(ctx context.Context, force bool) (string, error) {
	if !c.cfg.HasDingTalkCredentials() {
		return "", fmt.Errorf("未配置钉钉应用凭据")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if !force && c.token != "" && time.Now().Before(c.expires) {
		return c.token, nil
	}

	endpoint := c.cfg.DingTalkOAPIBase + "/gettoken?appkey=" + url.QueryEscape(c.cfg.DingTalkClientID) +
		"&appsecret=" + url.QueryEscape(c.cfg.DingTalkClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("无法访问钉钉开放平台: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}

	var payload struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("钉钉鉴权响应无法解析: %w", err)
	}
	if payload.ErrCode != 0 || payload.AccessToken == "" {
		return "", fmt.Errorf("钉钉鉴权失败: %s", payload.ErrMsg)
	}

	c.token = payload.AccessToken
	ttl := payload.ExpiresIn
	if ttl <= 0 {
		ttl = 7200
	}
	c.expires = time.Now().Add(time.Duration(ttl-300) * time.Second)
	return c.token, nil
}

// summarize 截断响应体，避免把长错误写进日志。
func summarize(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len([]rune(text)) > 300 {
		return string([]rune(text)[:300]) + "…"
	}
	return text
}
