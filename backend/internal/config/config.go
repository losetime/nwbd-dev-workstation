// Package config 读取运行期配置。
//
// 所有值来自环境变量（前缀 NWBD_WS_），密钥不落代码、不进镜像。
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config 是服务运行所需的全部配置。
type Config struct {
	AppName     string
	Environment string
	LogLevel    string
	HTTPAddr    string
	CORSOrigins []string

	DatabaseURL string

	DingTalkTimeout      time.Duration
	DingTalkClientID     string
	DingTalkClientSecret string
	DingTalkCorpID       string
	DingTalkAPIBase      string
	DingTalkOAPIBase     string

	// 审批专用应用（04 项目策划流程）：发起、撤销、状态读取、Stream 订阅
	ApprovalClientID      string
	ApprovalClientSecret  string
	ApprovalAgentID       int64
	ApprovalProcessCode   string
	ContractProcessCode   string
	ChangeProcessCode     string
	ApprovalStreamEnabled bool

	// PublicBaseURL 是本系统对外地址，用于拼登录页与跳转。
	PublicBaseURL string
	// LoginRedirect 是钉钉扫码登录的 redirect_uri，必须与钉钉后台登记的一致。
	// 留空则用 PublicBaseURL + /login。
	LoginRedirect string
	// DevLoginUserID 非空且环境为 local 时，允许跳过钉钉直接登录（仅本地开发用）。
	DevLoginUserID string
	DevLoginName   string
	// DevActAs 为 true 且环境为 local 时，允许切换成任意在职人员（测试用）。
	DevActAs             bool
	TokenTTLDays         int
	RoleSyncMinutes      int
	DirectorySyncMinutes int

	// 文件管理（nwbd-file-manage）内部接口：任务产出统一归档到那边
	FileManageBase    string
	FileManageToken   string
	FileManageTimeout time.Duration
	// MaxUploadMB 单文件上限，与服务端文件管理保持一致
	MaxUploadMB int

	// 线上技能库（nwbd-skills）
	SkillsBase    string
	SkillsToken   string
	SkillsTimeout time.Duration
}

// Load 读取并校验配置。缺关键项时返回错误，让容器直接退出而不是带病运行。
func Load() (Config, error) {
	cfg := Config{
		AppName:     env("NWBD_WS_APP_NAME", "nwbd-dev-workstation"),
		Environment: env("NWBD_WS_ENVIRONMENT", "local"),
		LogLevel:    env("NWBD_WS_LOG_LEVEL", "INFO"),
		HTTPAddr:    env("NWBD_WS_HTTP_ADDR", ":8000"),

		DatabaseURL: env("NWBD_WS_DATABASE_URL", "postgresql://dev_workstation:dev_workstation@db:5432/dev_workstation"),

		DingTalkTimeout:      time.Duration(envInt("NWBD_WS_DINGTALK_TIMEOUT_SECONDS", 10)) * time.Second,
		DingTalkClientID:     env("NWBD_WS_DINGTALK_CLIENT_ID", ""),
		DingTalkClientSecret: env("NWBD_WS_DINGTALK_CLIENT_SECRET", ""),
		DingTalkCorpID:       env("NWBD_WS_DINGTALK_CORP_ID", ""),
		DingTalkAPIBase:      strings.TrimRight(env("NWBD_WS_DINGTALK_API_BASE", "https://api.dingtalk.com"), "/"),
		DingTalkOAPIBase:     strings.TrimRight(env("NWBD_WS_DINGTALK_OAPI_BASE", "https://oapi.dingtalk.com"), "/"),

		ApprovalClientID:      env("NWBD_WS_APPROVAL_CLIENT_ID", ""),
		ApprovalClientSecret:  env("NWBD_WS_APPROVAL_CLIENT_SECRET", ""),
		ApprovalAgentID:       int64(envInt("NWBD_WS_APPROVAL_AGENT_ID", 0)),
		ApprovalProcessCode:   env("NWBD_WS_APPROVAL_PROCESS_CODE", "PROC-2E309DC0-76E4-47D4-94C4-F0A2780B4100"),
		ContractProcessCode:   env("NWBD_WS_CONTRACT_PROCESS_CODE", "PROC-ACB82876-19CB-4B99-8FF1-8CAB8EB0223F"),
		ChangeProcessCode:     env("NWBD_WS_CHANGE_PROCESS_CODE", "PROC-CA89833F-7725-4098-811D-9F52F3FDBA4D"),
		ApprovalStreamEnabled: envBool("NWBD_WS_APPROVAL_STREAM_ENABLED", true),

		PublicBaseURL:        strings.TrimRight(env("NWBD_WS_PUBLIC_BASE_URL", "http://127.0.0.1:8090"), "/"),
		LoginRedirect:        env("NWBD_WS_LOGIN_REDIRECT", ""),
		DevLoginUserID:       env("NWBD_WS_DEV_LOGIN_USERID", ""),
		DevLoginName:         env("NWBD_WS_DEV_LOGIN_NAME", "本地开发账号"),
		DevActAs:             envBool("NWBD_WS_DEV_ACT_AS", false),
		TokenTTLDays:         envInt("NWBD_WS_TOKEN_TTL_DAYS", 30),
		RoleSyncMinutes:      envInt("NWBD_WS_ROLE_SYNC_MINUTES", 10),
		DirectorySyncMinutes: envInt("NWBD_WS_DIRECTORY_SYNC_MINUTES", 30),

		FileManageBase:    strings.TrimRight(env("NWBD_WS_FILE_MANAGE_BASE", "http://nwbd-fm-api:8000"), "/"),
		FileManageToken:   env("NWBD_WS_FILE_MANAGE_TOKEN", ""),
		FileManageTimeout: time.Duration(envInt("NWBD_WS_FILE_MANAGE_TIMEOUT_SECONDS", 300)) * time.Second,
		MaxUploadMB:       envInt("NWBD_WS_MAX_UPLOAD_MB", 100),

		SkillsBase:    strings.TrimRight(env("NWBD_WS_SKILLS_BASE", "http://8.160.178.247:18138"), "/"),
		SkillsToken:   env("NWBD_WS_SKILLS_TOKEN", ""),
		SkillsTimeout: time.Duration(envInt("NWBD_WS_SKILLS_TIMEOUT_SECONDS", 25)) * time.Second,
	}

	for _, origin := range strings.Split(env("NWBD_WS_CORS_ORIGINS", ""), ",") {
		if trimmed := strings.TrimSpace(origin); trimmed != "" {
			cfg.CORSOrigins = append(cfg.CORSOrigins, trimmed)
		}
	}

	if cfg.DatabaseURL == "" {
		return cfg, fmt.Errorf("NWBD_WS_DATABASE_URL 不能为空")
	}
	return cfg, nil
}

// HasDingTalkCredentials 判断是否具备调用钉钉接口的最小条件。
func (c Config) HasDingTalkCredentials() bool {
	return c.DingTalkClientID != "" && c.DingTalkClientSecret != ""
}

// HasApprovalCredentials 判断审批专用应用是否可用。
func (c Config) HasApprovalCredentials() bool {
	return c.ApprovalClientID != "" && c.ApprovalClientSecret != ""
}

// DevLoginEnabled 返回是否允许本地开发登录。非 local 环境永远为 false。
func (c Config) DevLoginEnabled() bool {
	return c.Environment == "local" && strings.TrimSpace(c.DevLoginUserID) != ""
}

// DevActAsEnabled 返回是否允许「切换测试身份」。非 local 环境永远为 false。
func (c Config) DevActAsEnabled() bool {
	return c.Environment == "local" && c.DevActAs
}

func env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}

func envBool(key string, fallback bool) bool {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}
