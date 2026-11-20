#!/usr/bin/env bash
# 研发项目管理系统 · 任务操作脚本（macOS / Linux）
# 用法：workstation.sh <命令> [参数...] [--token ...] [--base-url ...] [--json]
# 令牌优先级：--token / 环境变量 NWBD_WS_TOKEN / 本地配置（未过期）
# 没有令牌或令牌过期时退出码为 2，由 agent 提示用户重新复制访问令牌。

set -euo pipefail

DEFAULT_BASE="http://127.0.0.1:8092"
CONFIG="$HOME/.config/nwbd-dev-workstation/config.json"
TOKEN="${NWBD_WS_TOKEN:-}"
BASE_URL="${NWBD_WS_BASE_URL:-}"
CLI_TOKEN=0
STATUS=""
KEYWORD=""
SUMMARY=""
JSON_OUT=0
TOP=20

cmd="${1:-tasks}"
[ $# -gt 0 ] && shift || true

positional=()
while [ $# -gt 0 ]; do
  case "$1" in
    --token) TOKEN="$2"; CLI_TOKEN=1; shift 2 ;;
    --base-url) BASE_URL="$2"; shift 2 ;;
    --status) STATUS="$2"; shift 2 ;;
    --keyword) KEYWORD="$2"; shift 2 ;;
    --summary) SUMMARY="$2"; shift 2 ;;
    --top) TOP="$2"; shift 2 ;;
    --json) JSON_OUT=1; shift ;;
    *) positional+=("$1"); shift ;;
  esac
done

ARG1="${positional[0]:-}"
ARG2="${positional[1]:-}"

if ! command -v jq >/dev/null 2>&1; then
  echo "错误：需要 jq，请先安装（macOS: brew install jq；Debian/Ubuntu: apt install jq）" >&2
  exit 1
fi

mkdir -p "$(dirname "$CONFIG")"

if [ -z "$BASE_URL" ] && [ -f "$CONFIG" ]; then
  BASE_URL="$(jq -r '.base_url // empty' "$CONFIG" 2>/dev/null || true)"
fi
[ -z "$BASE_URL" ] && BASE_URL="$DEFAULT_BASE"
BASE_URL="${BASE_URL%/}"

if [ -z "$TOKEN" ] && [ -f "$CONFIG" ]; then
  TOKEN="$(jq -r '.token // empty' "$CONFIG" 2>/dev/null || true)"
fi

if [ -z "$TOKEN" ]; then
  echo "错误：需要访问令牌" >&2
  echo "请打开平台（$BASE_URL），登录后在「用户信息」页复制访问令牌，然后执行：" >&2
  echo "  workstation.sh $cmd --token <访问令牌>" >&2
  exit 2
fi

save_token() {
  local tmp
  tmp="$(mktemp)"
  if [ -f "$CONFIG" ]; then jq --arg t "$TOKEN" --arg b "$BASE_URL" '.token=$t | .base_url=$b' "$CONFIG" >"$tmp"; else
    jq -n --arg t "$TOKEN" --arg b "$BASE_URL" '{token:$t, base_url:$b}' >"$tmp"
  fi
  mv "$tmp" "$CONFIG"
  chmod 600 "$CONFIG" 2>/dev/null || true
}

api() {
  local method="$1" path="$2" body="${3:-}"
  local url="$BASE_URL$path"
  local resp code
  local args=(-sS -X "$method" -H "Authorization: Bearer $TOKEN" -H "Accept: application/json" -w '\n%{http_code}')
  if [ -n "$body" ]; then
    args+=(-H "Content-Type: application/json" --data-binary "$body")
  fi
  resp="$(curl "${args[@]}" "$url")"
  code="$(printf '%s' "$resp" | tail -n1)"
  resp="$(printf '%s' "$resp" | sed '$d')"

  if [ "$code" = "401" ]; then
    echo "错误：令牌无效或已过期" >&2
    if [ -f "$CONFIG" ]; then
      local tmp; tmp="$(mktemp)"
      jq 'del(.token)' "$CONFIG" >"$tmp" && mv "$tmp" "$CONFIG"
    fi
    echo "请重新在平台复制访问令牌后执行：workstation.sh $cmd --token <访问令牌>" >&2
    exit 2
  fi
  if [ "$code" -ge 400 ]; then
    local msg
    msg="$(printf '%s' "$resp" | jq -r '.message // empty' 2>/dev/null || true)"
    echo "错误：${msg:-HTTP $code}" >&2
    exit 1
  fi
  printf '%s' "$resp"
}

# 校验令牌
api GET /api/auth/me >/dev/null
if [ "$CLI_TOKEN" = "1" ]; then
  save_token
fi

case "$cmd" in
  tasks)
    path="/api/my/tasks"
    q=""
    [ -n "$STATUS" ] && q="status=$(jq -rn --arg v "$STATUS" '$v|@uri')"
    [ -n "$KEYWORD" ] && q="${q:+$q&}keyword=$(jq -rn --arg v "$KEYWORD" '$v|@uri')"
    [ -n "$q" ] && path="$path?$q"
    data="$(api GET "$path")"
    if [ "$JSON_OUT" = "1" ]; then
      printf '%s' "$data" | jq .
      exit 0
    fi
    count="$(printf '%s' "$data" | jq '.data.items | length')"
    if [ "$count" = "0" ]; then echo "没有任务"; exit 0; fi
    printf '%s' "$data" | jq -r --argjson top "$TOP" '
      .data.items[:$top][] |
      "\(.id)  \(.status)  \(.priority)  \(.name)\(if .overdue then " [已逾期]" else "" end)\n      项目：\(.project_name) · 阶段：\(.stage // "—") · 计划：\(.plan_due // "—" | .[0:10]) · 工时：\(.estimate_hours)/\(.actual_hours)h"
    '
    printf '%s' "$data" | jq -r '.data.stats | "\n共 \(.total) 项，待办 \(.by_status["未开始"] // 0)，进行中 \(.by_status["进行中"] // 0)，逾期 \(.overdue)"'
    ;;
  show)
    [ -z "$ARG1" ] && { echo "用法：workstation.sh show <任务ID>" >&2; exit 1; }
    data="$(api GET "/api/my/tasks/$ARG1")"
    if [ "$JSON_OUT" = "1" ]; then printf '%s' "$data" | jq .; exit 0; fi
    printf '%s' "$data" | jq -r '
      .data |
      "任务：\(.task.name)\n编号：\(.task.id)\n状态：\(.task.status) · 优先级：\(.task.priority)\(if .task.overdue then " · 已逾期" else "" end)\n项目：\(.task.project_name) · 阶段：\(.task.stage // "—")\n负责人：\(.task.owner_name) · 计划：\(.task.plan_start // "—" | .[0:10]) ~ \(.task.plan_due // "—" | .[0:10]) · 工时：\(.task.estimate_hours)/\(.task.actual_hours)h",
      (if .task.description then "\n描述：\n\(.task.description)" else "" end),
      (if (.task.acceptance | length) > 0 then "\n验收标准：\n" + ([.task.acceptance[] | "  - \(.)"] | join("\n")) else "" end),
      (if ((.task.skills // []) | length) > 0 then "\n技能：" + ([.task.skills[].name] | join("、")) elif .task.skill then "\n技能：\(.task.skill.name)" else "" end),
      (if .current_run then "\n执行中：\(.current_run.agent_name) · 开始于 \(.current_run.started_at[0:16])" else "" end),
      (if (.artifacts | length) > 0 then "\n产出：\n" + ([.artifacts[] | "  - \(.name)（\(.size) bytes）"] | join("\n")) else "" end),
      (if (.events | length) > 0 then "\n最近进展：\n" + ([.events[:8][] | "  [\(.at[0:16] | gsub("T"; " "))] \(.message)"] | join("\n")) else "" end)
    '
    ;;
  start)
    [ -z "$ARG1" ] && { echo "用法：workstation.sh start <任务ID>" >&2; exit 1; }
    run="$(api POST "/api/my/tasks/$ARG1/start" '{}')"
    if [ "$JSON_OUT" = "1" ]; then printf '%s' "$run" | jq .; exit 0; fi
    printf '%s' "$run" | jq -r '.data | if .already_running then "任务已经在执行中" elif .reopen then "任务已重新打开，开始第 \(.attempt) 次执行" else "已开始第 \(.attempt) 次执行" end'
    ;;
  log)
    [ -z "$ARG1" ] || [ -z "$ARG2" ] && { echo "用法：workstation.sh log <任务ID> \"进展内容\"" >&2; exit 1; }
    body="$(jq -n --arg m "$ARG2" '{message:$m}')"
    api POST "/api/my/tasks/$ARG1/log" "$body" >/dev/null
    echo "已记录进展"
    ;;
  upload)
    [ -z "$ARG1" ] || [ -z "$ARG2" ] && { echo "用法：workstation.sh upload <任务ID> <文件路径>" >&2; exit 1; }
    [ -f "$ARG2" ] || { echo "文件不存在：$ARG2" >&2; exit 1; }
    resp="$(curl -sS -X POST -H "Authorization: Bearer $TOKEN" -H "Accept: application/json" \
      -F "file=@$ARG2" -w '\n%{http_code}' "$BASE_URL/api/my/tasks/$ARG1/artifacts")"
    code="$(printf '%s' "$resp" | tail -n1)"
    resp="$(printf '%s' "$resp" | sed '$d')"
    if [ "$code" = "401" ]; then
      echo "错误：令牌无效或已过期" >&2
      exit 2
    fi
    if [ "$code" -ge 400 ]; then
      msg="$(printf '%s' "$resp" | jq -r '.message // empty' 2>/dev/null || true)"
      echo "错误：${msg:-HTTP $code}" >&2
      exit 1
    fi
    art="$resp"
    if [ "$JSON_OUT" = "1" ]; then printf '%s' "$art" | jq .; exit 0; fi
    printf '%s' "$art" | jq -r '.data | "产出已上传：\(.name)（\(.size) bytes）"'
    printf '%s' "$art" | jq -r '.data.folder // empty | "已归档到文件管理：\(.)"'
    ;;
  done)
    [ -z "$ARG1" ] && { echo "用法：workstation.sh done <任务ID> [--summary \"结果摘要\"]" >&2; exit 1; }
    [ -z "$SUMMARY" ] && SUMMARY="$ARG2"
    [ -z "$SUMMARY" ] && { echo "请用 --summary 或第二个参数提供结果摘要" >&2; exit 1; }
    api POST "/api/my/tasks/$ARG1/complete" "$(jq -n --arg s "$SUMMARY" '{summary:$s, artifacts:[]}')" >/dev/null
    echo "任务已完成，结果已提交"
    ;;
  skills)
    data="$(api GET /api/skills)"
    if [ "$JSON_OUT" = "1" ]; then printf '%s' "$data" | jq .; exit 0; fi
    printf '%s' "$data" | jq -r '.data.items[] | "\(.name) · v\(.current_version)"'
    ;;
  *)
    echo "用法：workstation.sh <tasks|show|start|log|upload|done|skills> [参数] [--token ..] [--base-url ..] [--json]" >&2
    exit 1
    ;;
esac

exit 0

