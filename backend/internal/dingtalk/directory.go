package dingtalk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// DirectoryUser 是通讯录里的一个人。
type DirectoryUser struct {
	UserID  string
	UnionID string
	Name    string
	Title   string
	Avatar  string
	Dept    string
	DeptID  string
}

// Directory 拉取全公司通讯录（部门树 + 各部门成员），按 userId 去重。
//
// 受限说明：只返回应用「通讯录权限范围」内的部门与人员；范围外的部门拉不到。
func (c *Client) Directory(ctx context.Context) ([]DirectoryUser, error) {
	token, err := c.AccessToken(ctx, false)
	if err != nil {
		return nil, err
	}

	deptIDs, err := c.allDeptIDs(ctx, token)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool, 128)
	users := make([]DirectoryUser, 0, 128)
	for _, deptID := range deptIDs {
		name, _ := c.departmentName(ctx, deptID, token)
		list, err := c.deptUsers(ctx, deptID, token)
		if err != nil {
			return nil, err
		}
		for _, u := range list {
			if u.UserID == "" || seen[u.UserID] {
				continue
			}
			seen[u.UserID] = true
			u.Dept = name
			u.DeptID = fmt.Sprintf("%d", deptID)
			users = append(users, u)
		}
		// 通讯录接口有频率限制，逐部门拉取之间稍作停顿
		time.Sleep(60 * time.Millisecond)
	}
	return users, nil
}

// allDeptIDs 从根部门开始广度遍历，返回全部部门 ID（含根部门 1）。
func (c *Client) allDeptIDs(ctx context.Context, token string) ([]int64, error) {
	visited := map[int64]bool{}
	queue := []int64{1}
	out := make([]int64, 0, 32)

	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if visited[id] {
			continue
		}
		visited[id] = true
		out = append(out, id)

		subs, err := c.subDeptIDs(ctx, id, token)
		if err != nil {
			return nil, err
		}
		queue = append(queue, subs...)
	}
	return out, nil
}

func (c *Client) subDeptIDs(ctx context.Context, deptID int64, token string) ([]int64, error) {
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		Result  struct {
			DeptIDList []int64 `json:"dept_id_list"`
		} `json:"result"`
	}
	if err := c.postOAPI(ctx, "/topapi/v2/department/listsubid", token,
		map[string]any{"dept_id": deptID}, &result); err != nil {
		return nil, err
	}
	if result.ErrCode != 0 {
		return nil, fmt.Errorf("读取子部门失败: %s（errcode=%d）", result.ErrMsg, result.ErrCode)
	}
	return result.Result.DeptIDList, nil
}

func (c *Client) departmentName(ctx context.Context, deptID int64, token string) (string, error) {
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		Result  struct {
			Name string `json:"name"`
		} `json:"result"`
	}
	if err := c.postOAPI(ctx, "/topapi/v2/department/get", token,
		map[string]any{"dept_id": deptID, "language": "zh_CN"}, &result); err != nil {
		return "", err
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("读取部门失败: %s（errcode=%d）", result.ErrMsg, result.ErrCode)
	}
	return result.Result.Name, nil
}

// deptUsers 分页拉取一个部门的成员。
func (c *Client) deptUsers(ctx context.Context, deptID int64, token string) ([]DirectoryUser, error) {
	out := make([]DirectoryUser, 0, 100)
	cursor := int64(0)
	for page := 0; page < 50; page++ {
		var result struct {
			ErrCode int    `json:"errcode"`
			ErrMsg  string `json:"errmsg"`
			Result  struct {
				HasMore    bool  `json:"has_more"`
				NextCursor int64 `json:"next_cursor"`
				List       []struct {
					UserID  string `json:"userid"`
					UnionID string `json:"unionid"`
					Name    string `json:"name"`
					Title   string `json:"title"`
					Avatar  string `json:"avatar"`
				} `json:"list"`
			} `json:"result"`
		}
		body := map[string]any{
			"dept_id":  deptID,
			"cursor":   cursor,
			"size":     100,
			"language": "zh_CN",
		}
		if err := c.postOAPI(ctx, "/topapi/v2/user/list", token, body, &result); err != nil {
			return nil, err
		}
		if result.ErrCode != 0 {
			return nil, fmt.Errorf("读取部门成员失败: %s（errcode=%d）", result.ErrMsg, result.ErrCode)
		}
		for _, u := range result.Result.List {
			out = append(out, DirectoryUser{
				UserID: u.UserID, UnionID: u.UnionID, Name: u.Name, Title: u.Title, Avatar: u.Avatar,
			})
		}
		if !result.Result.HasMore {
			break
		}
		cursor = result.Result.NextCursor
	}
	return out, nil
}

// postOAPI 调用旧版 OpenAPI（access_token 走 query）。
func (c *Client) postOAPI(ctx context.Context, path, token string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	endpoint := c.cfg.DingTalkOAPIBase + path + "?access_token=" + url.QueryEscape(token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("调用钉钉接口失败: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("钉钉接口 HTTP %d: %s", resp.StatusCode, summarize(raw))
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("解析钉钉响应失败: %w", err)
	}
	return nil
}
