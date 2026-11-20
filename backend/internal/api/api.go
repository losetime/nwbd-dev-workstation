// Package api 提供 HTTP 接口。响应结构与 nwbd-file-manage 保持一致，前端封装可以复用。
package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"nwbd-dev-workstation/internal/auth"
	"nwbd-dev-workstation/internal/config"
	"nwbd-dev-workstation/internal/dingtalk"
	"nwbd-dev-workstation/internal/filemanage"
	"nwbd-dev-workstation/internal/org"
	"nwbd-dev-workstation/internal/skills"
	"nwbd-dev-workstation/internal/store"
)

// API 汇集各依赖。
type API struct {
	cfg       config.Config
	store     *store.Store
	auth      *auth.Service
	org       *org.Service
	files     *filemanage.Client
	skills    *skills.Client
	approval  *dingtalk.ApprovalClient
	ding      *dingtalk.Client
	log       *slog.Logger
	startedAt time.Time
}

// New 构造 API。
func New(
	cfg config.Config,
	st *store.Store,
	authService *auth.Service,
	orgService *org.Service,
	files *filemanage.Client,
	skillsClient *skills.Client,
	dingClient *dingtalk.Client,
	approvalClient *dingtalk.ApprovalClient,
	log *slog.Logger,
) *API {
	return &API{
		cfg: cfg, store: st, auth: authService, org: orgService,
		files: files, skills: skillsClient, ding: dingClient,
		approval: approvalClient, log: log, startedAt: time.Now(),
	}
}

// Router 注册全部路由。用标准库 mux，Go 1.22+ 原生支持方法与路径参数。
func (a *API) Router() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", a.health)
	mux.HandleFunc("GET /api/ready", a.ready)
	mux.HandleFunc("GET /api/meta", a.meta)

	mux.HandleFunc("GET /api/auth/config", a.authConfig)
	mux.HandleFunc("POST /api/auth/device", a.startDevice)
	mux.HandleFunc("POST /api/auth/device/poll", a.pollDevice)
	mux.HandleFunc("POST /api/auth/dingtalk/exchange", a.exchangeLogin)
	mux.HandleFunc("POST /api/auth/dev-login", a.devLogin)
	mux.HandleFunc("GET /api/auth/me", a.me)
	mux.HandleFunc("POST /api/auth/logout", a.logout)

	mux.HandleFunc("GET /api/users", a.listUsers)
	mux.HandleFunc("GET /api/users/managers", a.listManagers)
	mux.HandleFunc("PATCH /api/users/{id}", a.updateUser)
	mux.HandleFunc("POST /api/users/sync", a.syncUsers)

	mux.HandleFunc("GET /api/projects", a.listProjects)
	mux.HandleFunc("POST /api/projects", a.createProject)
	mux.HandleFunc("GET /api/projects/{id}", a.getProject)
	mux.HandleFunc("DELETE /api/projects/{id}", a.deleteProject)
	mux.HandleFunc("PATCH /api/projects/{id}", a.updateProject)
	mux.HandleFunc("PATCH /api/projects/{id}/status", a.updateProjectStatus)
	mux.HandleFunc("PATCH /api/projects/{id}/risk-flag", a.updateRiskFlag)
	mux.HandleFunc("GET /api/risks", a.listRisks)
	mux.HandleFunc("GET /api/issues", a.listIssues)
	mux.HandleFunc("POST /api/issues", a.createIssue)
	mux.HandleFunc("POST /api/issues/{id}/archive", a.archiveIssue)
	mux.HandleFunc("POST /api/projects/{id}/risks", a.createRisk)
	mux.HandleFunc("PATCH /api/risks/{id}", a.updateRisk)
	mux.HandleFunc("PUT /api/projects/{id}/managers", a.replaceManagers)
	mux.HandleFunc("POST /api/projects/{id}/milestones/plan", a.saveMilestonePlan)
	mux.HandleFunc("PATCH /api/projects/{id}/milestones/{milestoneID}", a.updateMilestone)
	mux.HandleFunc("POST /api/projects/{id}/tasks", a.createTask)
	mux.HandleFunc("PATCH /api/tasks/{id}/status", a.updateTaskStatus)

	mux.HandleFunc("GET /api/my/tasks", a.listMyTasks)
	mux.HandleFunc("GET /api/my/tasks/{id}", a.getMyTask)
	mux.HandleFunc("PATCH /api/my/tasks/{id}", a.updateMyTask)
	mux.HandleFunc("PATCH /api/my/tasks/{id}/status", a.updateMyTaskStatus)
	mux.HandleFunc("POST /api/my/tasks/{id}/log", a.logMyTask)
	mux.HandleFunc("POST /api/my/tasks/{id}/complete", a.completeMyTask)
	mux.HandleFunc("POST /api/my/tasks/{id}/start", a.startMyTask)
	mux.HandleFunc("POST /api/my/tasks/{id}/artifacts", a.uploadTaskArtifact)
	mux.HandleFunc("GET /api/my/tasks/{id}/artifacts/{artifactId}", a.downloadTaskArtifact)
	mux.HandleFunc("GET /api/my/tasks/{id}/timesheet", a.getTaskTimesheet)
	mux.HandleFunc("POST /api/my/tasks/{id}/timesheet", a.saveTaskTimesheet)

	mux.HandleFunc("GET /api/task-types", a.listTaskTypes)
	mux.HandleFunc("GET /api/skills", a.listSkills)
	mux.HandleFunc("GET /api/skills/{name}", a.getSkill)
	mux.HandleFunc("GET /api/approvals/contracts", a.listContractApprovals)
	mux.HandleFunc("GET /api/tasks/{id}/approval", a.getTaskApproval)
	mux.HandleFunc("GET /api/changes", a.listChanges)
	mux.HandleFunc("GET /api/changes/{id}", a.getChange)
	mux.HandleFunc("GET /api/projects/{id}/changes", a.listProjectChanges)
	mux.HandleFunc("POST /api/projects/{id}/changes", a.createChange)

	mux.HandleFunc("GET /api/dev/people", a.devPeople)
	mux.HandleFunc("POST /api/dev/act-as", a.devActAs)
	mux.HandleFunc("POST /api/projects/{id}/members", a.addMembers)
	mux.HandleFunc("GET /api/projects/{id}/candidates", a.listMemberCandidates)
	mux.HandleFunc("PATCH /api/projects/{id}/members/{userID}", a.setMemberStatus)

	return a.withRecover(a.withCORS(a.withLogging(a.withAuth(mux))))
}

// cookieSecure 决定 Cookie 是否带 Secure：本地 http 访问时不能带，否则浏览器直接丢弃。
func (a *API) cookieSecure() bool {
	return strings.HasPrefix(a.cfg.PublicBaseURL, "https://")
}

// ---------- 中间件 ----------

func (a *API) withCORS(next http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, origin := range a.cfg.CORSOrigins {
		allowed[origin] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && allowed[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *API) withRecover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				a.log.Error("请求处理 panic", "path", r.URL.Path, "panic", recovered, "stack", string(debug.Stack()))
				writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "服务内部错误，请查看日志", nil)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (a *API) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if !strings.HasPrefix(r.URL.Path, "/api/health") {
			a.log.Info("http", "method", r.Method, "path", r.URL.Path, "cost_ms", time.Since(start).Milliseconds())
		}
	})
}

// ---------- 响应封装 ----------

type fieldIssue struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type envelope struct {
	Code    string       `json:"code"`
	Data    any          `json:"data,omitempty"`
	Message string       `json:"message,omitempty"`
	Details []fieldIssue `json:"details,omitempty"`
}

func writeOK(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, envelope{Code: "OK", Data: data})
}

func writeError(w http.ResponseWriter, status int, code, message string, details []fieldIssue) {
	writeJSON(w, status, envelope{Code: code, Message: message, Details: details})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("写响应失败", "error", err)
	}
}

func decodeJSON(r *http.Request, target any) error {
	defer func() { _ = r.Body.Close() }()
	return json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(target)
}

func intParam(r *http.Request, key string, fallback, min, max int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// ---------- 基础接口 ----------

type healthDB struct {
	OK    bool    `json:"ok"`
	Error *string `json:"error"`
}

type healthBody struct {
	OK       bool     `json:"ok"`
	Time     string   `json:"time"`
	Version  string   `json:"version"`
	UptimeMS int64    `json:"uptime_ms"`
	DB       healthDB `json:"db"`
}

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	body := healthBody{
		Time:     time.Now().Format(time.RFC3339),
		Version:  Version,
		UptimeMS: time.Since(a.startedAt).Milliseconds(),
	}

	if err := a.store.Pool().Ping(ctx); err != nil {
		message := err.Error()
		body.DB = healthDB{OK: false, Error: &message}
	} else {
		body.DB = healthDB{OK: true}
	}

	body.OK = body.DB.OK
	if !body.OK {
		writeJSON(w, http.StatusServiceUnavailable, envelope{Code: "OK", Data: body})
		return
	}
	writeOK(w, body)
}

func (a *API) ready(w http.ResponseWriter, r *http.Request) {
	if err := a.store.Pool().Ping(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "数据库不可用", nil)
		return
	}
	writeOK(w, map[string]bool{"ready": true})
}

func (a *API) meta(w http.ResponseWriter, r *http.Request) {
	writeOK(w, map[string]any{
		"app":         a.cfg.AppName,
		"version":     Version,
		"environment": a.cfg.Environment,
		"login_url":   a.auth.LoginURL(),
	})
}

// Version 是当前代码版本，随发布更新。
const Version = "0.1.0"
