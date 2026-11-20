// Package skills 是线上技能库（nwbd-skills）的只读客户端。
package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client 访问技能库 HTTP API。
type Client struct {
	base  string
	token string
	http  *http.Client
}

// NewClient 构造客户端。
func NewClient(base, token string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return &Client{
		base:  strings.TrimRight(base, "/"),
		token: token,
		http:  &http.Client{Timeout: timeout},
	}
}

// Skill 是技能库列表里的一项。
type Skill struct {
	Name           string `json:"name"`
	DisplayName    string `json:"display_name"`
	Summary        string `json:"summary"`
	Status         string `json:"status"`
	CurrentVersion int    `json:"current_version"`
	VersionCount   int    `json:"version_count"`
	UpdatedAt      string `json:"updated_at"`
}

// Version 是技能的一个版本。
type Version struct {
	Version   int    `json:"version"`
	Status    string `json:"status"`
	Changelog string `json:"changelog"`
	CreatedAt string `json:"created_at"`
}

// Detail 是技能详情（含版本列表）。
type Detail struct {
	Skill
	Versions []Version `json:"versions"`
}

type envelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// List 拉取技能列表（技能库单页最多 100 条）。
func (c *Client) List(ctx context.Context, page, size int) ([]Skill, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 100 {
		size = 100
	}
	endpoint := fmt.Sprintf("%s/api/skills?page=%d&size=%d", c.base, page, size)
	var data struct {
		Items []Skill `json:"items"`
	}
	if err := c.get(ctx, endpoint, &data); err != nil {
		return nil, err
	}
	return data.Items, nil
}

// ListAll 分页拉全量技能（最多 5 页）。
func (c *Client) ListAll(ctx context.Context) ([]Skill, error) {
	all := make([]Skill, 0, 100)
	for page := 1; page <= 5; page++ {
		items, err := c.List(ctx, page, 100)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if len(items) < 100 {
			break
		}
	}
	return all, nil
}

// Get 拉取技能详情。
func (c *Client) Get(ctx context.Context, name string) (*Detail, error) {
	endpoint := c.base + "/api/skills/" + url.PathEscape(name)
	var detail Detail
	if err := c.get(ctx, endpoint, &detail); err != nil {
		return nil, err
	}
	return &detail, nil
}

// Summarize 把技能摘要截断，避免列表太长。
func Summarize(text string, limit int) string {
	runes := []rune(strings.TrimSpace(text))
	if limit <= 0 || len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit]) + "…"
}

func (c *Client) get(ctx context.Context, endpoint string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("无法连接技能库 %s：%w", c.base, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("技能库返回 HTTP %d", resp.StatusCode)
	}

	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("技能库响应无法解析: %w", err)
	}
	if env.Code != 0 {
		return fmt.Errorf("技能库返回错误（code=%d）：%s", env.Code, env.Message)
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return fmt.Errorf("技能库数据无法解析: %w", err)
	}
	return nil
}
