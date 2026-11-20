package api

import (
	"net/http"
	"strings"

	"nwbd-dev-workstation/internal/domain"
	"nwbd-dev-workstation/internal/store"
)

// authConfig 返回前端二维码登录需要的公开参数。
func (a *API) authConfig(w http.ResponseWriter, r *http.Request) {
	redirect := a.cfg.LoginRedirect
	if redirect == "" {
		redirect = a.auth.LoginURL()
	}
	writeOK(w, map[string]any{
		"clientId":    a.cfg.DingTalkClientID,
		"redirectUri": redirect,
		"enabled":     a.cfg.HasDingTalkCredentials(),
		"devLogin":    a.auth.DevLoginEnabled(),
		"devActAs":    a.devEnabled(),
	})
}

// devLogin 是本地开发用的登录入口：不经过钉钉，直接签发令牌。
func (a *API) devLogin(w http.ResponseWriter, r *http.Request) {
	if !a.auth.DevLoginEnabled() {
		writeError(w, http.StatusForbidden, "DEV_LOGIN_DISABLED", "本地开发登录未启用", nil)
		return
	}
	user, token, err := a.auth.DevLogin(r.Context())
	if err != nil {
		a.fail(w, "本地开发登录失败", err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     tokenCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(a.auth.TokenTTL().Seconds()),
		HttpOnly: true,
		Secure:   a.cookieSecure(),
		SameSite: http.SameSiteLaxMode,
	})
	writeOK(w, map[string]any{"ok": true, "user": userView(user), "token": token})
}

// startDevice 供脚本发起登录。
func (a *API) startDevice(w http.ResponseWriter, r *http.Request) {
	start, err := a.auth.StartDevice(r.Context())
	if err != nil {
		a.fail(w, "生成登录设备码失败", err)
		return
	}
	writeOK(w, start)
}

// pollDevice 供脚本轮询登录结果。
func (a *API) pollDevice(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		DeviceCode string `json:"device_code"`
	}
	if err := decodeJSON(r, &payload); err != nil || strings.TrimSpace(payload.DeviceCode) == "" {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "缺少 device_code", nil)
		return
	}
	result, err := a.auth.PollDevice(r.Context(), payload.DeviceCode)
	if err != nil {
		a.fail(w, "查询登录状态失败", err)
		return
	}
	writeOK(w, result)
}

// exchangeLogin 接收前端扫码拿到的 authCode，换取用户并签发令牌。
func (a *API) exchangeLogin(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		AuthCode   string `json:"auth_code"`
		DeviceCode string `json:"device_code"`
	}
	if err := decodeJSON(r, &payload); err != nil || strings.TrimSpace(payload.AuthCode) == "" {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "缺少 auth_code", nil)
		return
	}
	user, token, err := a.auth.Exchange(r.Context(), payload.AuthCode, payload.DeviceCode)
	if err != nil {
		a.fail(w, "钉钉登录失败", err)
		return
	}

	// 网页端用 Cookie 保持登录，脚本用设备码拿令牌
	http.SetCookie(w, &http.Cookie{
		Name:     tokenCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(a.auth.TokenTTL().Seconds()),
		HttpOnly: true,
		Secure:   a.cookieSecure(),
		SameSite: http.SameSiteLaxMode,
	})
	writeOK(w, map[string]any{"ok": true, "user": userView(user), "token": token})
}

// me 返回当前登录用户。
func (a *API) me(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	writeOK(w, userView(user))
}

// logout 吊销当前令牌并清除浏览器 Cookie。
func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	if err := a.auth.Logout(r.Context(), requestToken(r)); err != nil {
		a.fail(w, "退出登录失败", err)
		return
	}
	clearTokenCookie(w, a.cookieSecure())
	writeOK(w, map[string]bool{"ok": true})
}

// userView 是对外暴露的用户字段（前端 store/auth.ts 的契约）。
func userView(user *store.Person) map[string]any {
	return map[string]any{
		"id":       user.ID,
		"name":     user.Name,
		"dept":     user.Dept,
		"title":    user.Title,
		"role":     user.Role,
		"job_type": user.JobType,
		"level":    user.Level,
		"status":   user.Status,
		"avatar":   user.Avatar,
		"perms":    domain.EffectivePerms(user.JobType, user.UIPerms),
		"unionid":  user.UnionID,
		"userid":   user.DingTalkUserID,
	}
}
