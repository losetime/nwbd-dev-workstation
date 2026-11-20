package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"nwbd-dev-workstation/internal/store"
)

type authContextKey struct{}

// tokenCookieName 是网页端保存登录令牌的 Cookie 名。
const tokenCookieName = "nwbd_ws_token"

// requestToken 取请求里的令牌：优先 Authorization 头，其次 Cookie。
func requestToken(r *http.Request) string {
	if header := strings.TrimSpace(r.Header.Get("Authorization")); header != "" {
		return header
	}
	if cookie, err := r.Cookie(tokenCookieName); err == nil && cookie.Value != "" {
		return "Bearer " + cookie.Value
	}
	return ""
}

// clearTokenCookie 让浏览器删除登录 Cookie。
func clearTokenCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     tokenCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// withAuth 解析 Bearer token / Cookie，并把用户放进 context。
// 除登录相关接口外，所有 /api 接口都要求登录。
func (a *API) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if header := requestToken(r); header != "" {
			if info, err := a.auth.Authenticate(r.Context(), header); err == nil && info != nil {
				ctx := context.WithValue(r.Context(), authContextKey{}, info.User)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		if isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录（缺少或已失效的令牌）", nil)
	})
}

// isPublicPath 返回无需登录即可访问的接口。
func isPublicPath(path string) bool {
	switch path {
	case "/api/health", "/api/ready",
		"/api/auth/config", "/api/auth/device", "/api/auth/device/poll", "/api/auth/dingtalk/exchange",
		"/api/auth/dev-login":
		return true
	}
	return false
}

// currentUser 取当前登录用户，未登录返回 nil。
func currentUser(r *http.Request) *store.AuthUser {
	user, _ := r.Context().Value(authContextKey{}).(store.AuthUser)
	if user.ID == "" {
		return nil
	}
	return &user
}

// requireUser 强制登录；未登录时写 401 并返回 false。
func (a *API) requireUser(w http.ResponseWriter, r *http.Request) (*store.AuthUser, bool) {
	user := currentUser(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录（缺少或已失效的令牌）", nil)
		return nil, false
	}
	return user, true
}
