package dingtalk

import (
	"context"
	"fmt"
)

// UserDeptID 取用户当前所在部门（钉钉 v2 用户详情），用于发起审批的 deptId。
func (c *Client) UserDeptID(ctx context.Context, userID string) (int64, error) {
	if userID == "" {
		return 0, fmt.Errorf("缺少 userId")
	}
	token, err := c.AccessToken(ctx, false)
	if err != nil {
		return 0, err
	}
	var out struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		Result  struct {
			DeptIDList []int64 `json:"dept_id_list"`
			DeptID     int64   `json:"dept_id"`
		} `json:"result"`
	}
	if err := c.postOAPI(ctx, "/topapi/v2/user/get", token, map[string]any{"userid": userID}, &out); err != nil {
		return 0, err
	}
	if out.ErrCode != 0 {
		return 0, fmt.Errorf("查询用户部门失败: %s", out.ErrMsg)
	}
	if len(out.Result.DeptIDList) > 0 {
		return out.Result.DeptIDList[0], nil
	}
	return out.Result.DeptID, nil
}
