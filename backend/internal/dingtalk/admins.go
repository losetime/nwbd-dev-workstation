package dingtalk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// 钉钉管理员等级：1 主管理员，2 子管理员。
const (
	AdminLevelSuper = 1
	AdminLevelSub   = 2
)

// Admin 是钉钉管理员列表中的一项。
type Admin struct {
	UserID string
	Level  int
}

// ListAdmins 拉取企业管理员列表（主管理员 + 子管理员）。
//
// 接口：POST /topapi/user/listadmin，需要「成员信息读权限」；
// 该接口不受通讯录权限范围限制。
func (c *Client) ListAdmins(ctx context.Context) ([]Admin, error) {
	token, err := c.AccessToken(ctx, false)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.cfg.DingTalkOAPIBase+"/topapi/user/listadmin?access_token="+url.QueryEscape(token),
		bytes.NewReader([]byte("{}")))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("读取钉钉管理员列表失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	var out struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		Result  []struct {
			UserID   string `json:"userid"`
			SysLevel int    `json:"sys_level"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("钉钉管理员列表解析失败: %w", err)
	}
	if out.ErrCode == 43007 {
		return nil, fmt.Errorf("钉钉管理员列表无权限（43007）：请在开发者后台给应用勾选「成员信息读权限」")
	}
	if out.ErrCode != 0 {
		return nil, fmt.Errorf("钉钉管理员列表读取失败: %s（errcode=%d）", out.ErrMsg, out.ErrCode)
	}

	admins := make([]Admin, 0, len(out.Result))
	for _, item := range out.Result {
		if item.UserID == "" || item.SysLevel <= 0 {
			continue
		}
		admins = append(admins, Admin{UserID: item.UserID, Level: item.SysLevel})
	}
	return admins, nil
}
