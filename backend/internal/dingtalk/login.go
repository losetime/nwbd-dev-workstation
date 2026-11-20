package dingtalk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// LoginUser 是扫码登录后得到的用户信息。
type LoginUser struct {
	UserID  string
	UnionID string
	Name    string
	Title   string
	Dept    string
	Active  bool
}

// ExchangeUserCode 用扫码得到的 authCode 换取用户信息。
//
// 链路：authCode → userAccessToken → /contact/users/me（unionId、昵称）→ unionId → userId → 用户详情。
func (c *Client) ExchangeUserCode(ctx context.Context, authCode string) (*LoginUser, error) {
	if authCode == "" {
		return nil, fmt.Errorf("缺少授权码")
	}
	settings := c.cfg

	payload, _ := json.Marshal(map[string]string{
		"clientId":     settings.DingTalkClientID,
		"clientSecret": settings.DingTalkClientSecret,
		"code":         authCode,
		"grantType":    "authorization_code",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		settings.DingTalkAPIBase+"/v1.0/oauth2/userAccessToken", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("换取用户凭证失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("换取用户凭证失败: HTTP %d %s", resp.StatusCode, summarize(body))
	}

	var tokenResp struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil || tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("换取用户凭证失败: %s", summarize(body))
	}

	// 用户级 token 取 unionId 与昵称
	meReq, err := http.NewRequestWithContext(ctx, http.MethodGet,
		settings.DingTalkAPIBase+"/v1.0/contact/users/me", nil)
	if err != nil {
		return nil, err
	}
	meReq.Header.Set("x-acs-dingtalk-access-token", tokenResp.AccessToken)
	meResp, err := c.http.Do(meReq)
	if err != nil {
		return nil, fmt.Errorf("获取用户信息失败: %w", err)
	}
	defer meResp.Body.Close()
	meBody, _ := io.ReadAll(io.LimitReader(meResp.Body, 1<<20))
	if meResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("获取用户信息失败: HTTP %d %s", meResp.StatusCode, summarize(meBody))
	}

	var me struct {
		Nick    string `json:"nick"`
		UnionID string `json:"unionId"`
	}
	if err := json.Unmarshal(meBody, &me); err != nil {
		return nil, fmt.Errorf("解析用户信息失败: %w", err)
	}
	if me.UnionID == "" {
		return nil, fmt.Errorf("未获取到 unionId，请确认应用已开通通讯录读权限")
	}

	userID, err := c.UserIDByUnionID(ctx, me.UnionID)
	if err != nil {
		// 非本公司成员（或不在应用可见范围）时这一步必然失败，直接拒绝登录
		return nil, fmt.Errorf("该钉钉账号不属于本公司或不在应用可见范围内，无法登录")
	}

	user := &LoginUser{UserID: userID, UnionID: me.UnionID, Name: me.Nick}
	if detail, detailErr := c.UserDetail(ctx, userID); detailErr == nil {
		if detail.Name != "" {
			user.Name = detail.Name
		}
		user.Title = detail.Title
		user.Dept = detail.Dept
		if !detail.Active {
			return nil, fmt.Errorf("该钉钉账号已停用，无法登录")
		}
	}
	return user, nil
}

// UserIDByUnionID 用 unionId 换企业内 userId。
func (c *Client) UserIDByUnionID(ctx context.Context, unionID string) (string, error) {
	token, err := c.AccessToken(ctx, false)
	if err != nil {
		return "", err
	}
	return c.userIDByUnionIDWithToken(ctx, unionID, token)
}

func (c *Client) userIDByUnionIDWithToken(ctx context.Context, unionID, token string) (string, error) {
	payload, _ := json.Marshal(map[string]string{"unionid": unionID})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.cfg.DingTalkOAPIBase+"/topapi/user/getbyunionid?access_token="+token, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("unionId 换 userId 失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	var out struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		Result  struct {
			UserID string `json:"userid"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("解析 unionId 结果失败: %w", err)
	}
	if out.ErrCode != 0 || out.Result.UserID == "" {
		return "", fmt.Errorf("unionId 换 userId 失败: %s", out.ErrMsg)
	}
	return out.Result.UserID, nil
}

// UserDetail 读取用户姓名/职位/部门（失败时调用方自行降级）。
func (c *Client) UserDetail(ctx context.Context, userID string) (*LoginUser, error) {
	token, err := c.AccessToken(ctx, false)
	if err != nil {
		return nil, err
	}
	payload, _ := json.Marshal(map[string]string{"userid": userID, "language": "zh_CN"})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.cfg.DingTalkOAPIBase+"/topapi/v2/user/get?access_token="+token, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	var out struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		Result  struct {
			UserID  string `json:"userid"`
			Name    string `json:"name"`
			Title   string `json:"title"`
			UnionID string `json:"unionid"`
			Active  bool   `json:"active"`
			DeptIDs []int  `json:"dept_id_list"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	if out.ErrCode != 0 {
		return nil, fmt.Errorf("读取用户详情失败: %s", out.ErrMsg)
	}
	return &LoginUser{
		UserID:  out.Result.UserID,
		UnionID: out.Result.UnionID,
		Name:    out.Result.Name,
		Title:   out.Result.Title,
		Active:  out.Result.Active,
		Dept:    c.deptName(ctx, out.Result.DeptIDs, token),
	}, nil
}

// deptName 取用户首个部门名称，失败时返回空串（不影响登录）。
func (c *Client) deptName(ctx context.Context, deptIDs []int, token string) string {
	if len(deptIDs) == 0 {
		return ""
	}
	payload, _ := json.Marshal(map[string]any{"dept_id": deptIDs[0], "language": "zh_CN"})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.cfg.DingTalkOAPIBase+"/topapi/v2/department/get?access_token="+token, bytes.NewReader(payload))
	if err != nil {
		return ""
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	var out struct {
		ErrCode int `json:"errcode"`
		Result  struct {
			Name string `json:"name"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.ErrCode != 0 {
		return ""
	}
	return out.Result.Name
}
