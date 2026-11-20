#!/usr/bin/env pwsh
# 研发项目管理系统 · 任务操作脚本（Windows）
# 用法：workstation.ps1 <命令> [参数...] [-Token ...] [-BaseUrl ...] [-Json]
# 令牌优先级：-Token / 环境变量 NWBD_WS_TOKEN / 本地配置（未过期）
# 没有令牌或令牌过期时退出码为 2，由 agent 提示用户重新复制访问令牌。

param(
    [Parameter(Position = 0)][string]$Command = "tasks",
    [Parameter(Position = 1)][string]$Arg1,
    [Parameter(Position = 2)][string]$Arg2,
    [string]$Token = $env:NWBD_WS_TOKEN,
    [string]$BaseUrl = "",
    [string]$Status = "",
    [string]$Keyword = "",
    [string]$Summary = "",
    [int]$Top = 20,
    [switch]$Json
)

$ErrorActionPreference = "Stop"
$DefaultBaseUrl = "http://127.0.0.1:8092"
$ConfigDir = Join-Path $env:USERPROFILE ".config\nwbd-dev-workstation"
$ConfigFile = Join-Path $ConfigDir "config.json"

function Write-Info($msg) { Write-Host $msg }
function Write-Fail($msg) { Write-Host "错误：$msg" -ForegroundColor Red }

function Format-Date($v) {
    if ($null -eq $v -or $v -eq "") { return "—" }
    if ($v -is [datetime]) { return $v.ToString("yyyy-MM-dd") }
    $s = [string]$v
    if ($s.Length -ge 10) { return $s.Substring(0, 10) }
    return $s
}

function Format-Time($v) {
    if ($null -eq $v -or $v -eq "") { return "" }
    if ($v -is [datetime]) { return $v.ToString("yyyy-MM-dd HH:mm") }
    $s = [string]$v
    if ($s.Length -ge 16) { return $s.Substring(0, 16).Replace("T", " ") }
    return $s
}

function Read-Config {
    if (-not (Test-Path $ConfigFile)) { return @{} }
    try { return (Get-Content $ConfigFile -Raw -Encoding utf8 | ConvertFrom-Json -AsHashtable) } catch { return @{} }
}

function Save-Config([hashtable]$cfg) {
    if (-not (Test-Path $ConfigDir)) { New-Item -ItemType Directory -Path $ConfigDir -Force | Out-Null }
    ($cfg | ConvertTo-Json -Depth 4) | Set-Content $ConfigFile -Encoding utf8
    try { icacls $ConfigFile /inheritance:r /grant:r "$($env:USERNAME):(R,W)" | Out-Null } catch { }
}

$config = Read-Config

# 1) 解析 base url
$base = if ($BaseUrl) { $BaseUrl } elseif ($env:NWBD_WS_BASE_URL) { $env:NWBD_WS_BASE_URL } elseif ($config.base_url) { $config.base_url } else { $DefaultBaseUrl }
$base = $base.TrimEnd("/")

# 2) 解析令牌
$resolved = ""
if ($Token) { $resolved = $Token.Trim() }
elseif ($config.token) { $resolved = $config.token.Trim() }

if (-not $resolved) {
    Write-Fail "需要访问令牌"
    Write-Info "请打开平台（$base），登录后在「用户信息」页复制访问令牌，然后执行："
    Write-Info "  workstation.ps1 $Command -Token <访问令牌>"
    exit 2
}

function Invoke-Api {
    param(
        [string]$Method,
        [string]$Path,
        $Body = $null,
        [switch]$SkipAuth
    )
    $headers = @{}
    if (-not $SkipAuth) { $headers["Authorization"] = "Bearer $resolved" }
    $params = @{
        Method      = $Method
        Uri         = "$base$Path"
        Headers     = $headers
        TimeoutSec  = 60
        ErrorAction = "Stop"
    }
    if ($null -ne $Body) {
        $params["ContentType"] = "application/json; charset=utf-8"
        $params["Body"] = [System.Text.Encoding]::UTF8.GetBytes(($Body | ConvertTo-Json -Depth 6 -Compress))
    }
    try {
        $resp = Invoke-RestMethod @params
    } catch {
        $status = 0
        if ($_.Exception.Response -and $_.Exception.Response.StatusCode) {
            $status = [int]$_.Exception.Response.StatusCode
        }
        if ($status -eq 401) {
            Write-Fail "令牌无效或已过期"
            $config.Remove("token")
            if ($config.Count -gt 0) { Save-Config $config }
            Write-Info "请重新在平台复制访问令牌后执行：workstation.ps1 $Command -Token <访问令牌>"
            exit 2
        }
        $msg = $_.Exception.Message
        try {
            $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
            $bodyText = $reader.ReadToEnd()
            $err = $bodyText | ConvertFrom-Json
            if ($err.message) { $msg = $err.message }
        } catch { }
        Write-Fail $msg
        exit 1
    }
    if ($null -ne $resp.data) { return $resp.data }
    return $resp
}

# 上传产出：multipart 一次提交，由服务端转发到文件管理对应阶段目录。
function Upload-Artifact([string]$TaskId, [string]$FilePath) {
    $file = Get-Item -LiteralPath $FilePath -ErrorAction SilentlyContinue
    if (-not $file) { Write-Fail "文件不存在：$FilePath"; exit 1 }

    $client = [System.Net.Http.HttpClient]::new()
    $client.Timeout = [TimeSpan]::FromSeconds(300)
    $client.DefaultRequestHeaders.Authorization =
        [System.Net.Http.Headers.AuthenticationHeaderValue]::new("Bearer", $resolved)
    $multipart = [System.Net.Http.MultipartFormDataContent]::new()
    $stream = [System.IO.File]::OpenRead($file.FullName)
    $fileContent = [System.Net.Http.StreamContent]::new($stream)
    $fileContent.Headers.ContentType =
        [System.Net.Http.Headers.MediaTypeHeaderValue]::Parse("application/octet-stream")
    $multipart.Add($fileContent, "file", $file.Name)

    try {
        $resp = $client.PostAsync("$base/api/my/tasks/$TaskId/artifacts", $multipart).GetAwaiter().GetResult()
        $text = $resp.Content.ReadAsStringAsync().GetAwaiter().GetResult()
    } finally {
        $stream.Dispose()
        $multipart.Dispose()
        $client.Dispose()
    }

    if ([int]$resp.StatusCode -eq 401) {
        Write-Fail "令牌无效或已过期"
        $config.Remove("token")
        if ($config.Count -gt 0) { Save-Config $config }
        Write-Info "请重新在平台复制访问令牌后执行：workstation.ps1 $Command -Token <访问令牌>"
        exit 2
    }
    if (-not $resp.IsSuccessStatusCode) {
        $msg = $text
        try { $err = $text | ConvertFrom-Json; if ($err.message) { $msg = $err.message } } catch { }
        Write-Fail $msg
        exit 1
    }
    $data = ($text | ConvertFrom-Json).data
    return $data
}

# 令牌校验：成功且用户显式传了 -Token，则保存
$me = Invoke-Api -Method GET -Path "/api/auth/me"
if ($Token) {
    $config["token"] = $resolved
    $config["base_url"] = $base
    Save-Config $config
}

function Show-TaskList($items) {
    if (-not $items -or $items.Count -eq 0) {
        Write-Info "没有任务"
        return
    }
    $items | Select-Object -First $Top | ForEach-Object {
        $due = Format-Date $_.plan_due
        $overdue = if ($_.overdue) { " [已逾期]" } else { "" }
        Write-Info ("{0}  {1,-4}  {2,-4}  {3}{4}" -f $_.id, $_.status, $_.priority, $_.name, $overdue)
        Write-Info ("      项目：{0} · 阶段：{1} · 计划：{2} · 工时：{3}/{4}h" -f $_.project_name, $_.stage, $due, $_.estimate_hours, $_.actual_hours)
    }
}

function Show-Task($task, $events, $artifacts, $run) {
    Write-Info ("任务：{0}" -f $task.name)
    Write-Info ("编号：{0}" -f $task.id)
    Write-Info ("状态：{0} · 优先级：{1}{2}" -f $task.status, $task.priority, $(if ($task.overdue) { " · 已逾期" } else { "" }))
    Write-Info ("项目：{0} · 阶段：{1}" -f $task.project_name, $task.stage)
    Write-Info ("负责人：{0} · 计划：{1} ~ {2} · 工时：{3}/{4}h" -f $task.owner_name, (Format-Date $task.plan_start), (Format-Date $task.plan_due), $task.estimate_hours, $task.actual_hours)
    if ($task.description) { Write-Info ("`n描述：`n{0}" -f $task.description) }
    if ($task.acceptance -and $task.acceptance.Count -gt 0) {
        Write-Info "`n验收标准："
        $task.acceptance | ForEach-Object { Write-Info ("  - {0}" -f $_) }
    }
    $skillNames = @()
    if ($task.skills) { $skillNames = @($task.skills | ForEach-Object { $_.name }) }
    elseif ($task.skill) { $skillNames = @($task.skill.name) }
    if ($skillNames.Count -gt 0) { Write-Info ("`n技能：{0}" -f ($skillNames -join "、")) }
    if ($run) { Write-Info ("执行中：{0} · 开始于 {1}" -f $run.agent_name, $run.started_at) }
    if ($artifacts -and $artifacts.Count -gt 0) {
        Write-Info "`n产出："
        $artifacts | ForEach-Object { Write-Info ("  - {0}（{1} bytes）" -f $_.name, $_.size) }
    }
    if ($events -and $events.Count -gt 0) {
        Write-Info "`n最近进展："
        $events | Select-Object -First 8 | ForEach-Object {
            Write-Info ("  [{0}] {1}" -f (Format-Time $_.at), $_.message)
        }
    }
}

switch ($Command.ToLower()) {
    "tasks" {
        $q = @()
        if ($Status) { $q += "status=$([uri]::EscapeDataString($Status))" }
        if ($Keyword) { $q += "keyword=$([uri]::EscapeDataString($Keyword))" }
        $path = "/api/my/tasks"
        if ($q.Count -gt 0) { $path += "?" + ($q -join "&") }
        $data = Invoke-Api -Method GET -Path $path
        if ($Json) { $data | ConvertTo-Json -Depth 8; break }
        Show-TaskList $data.items
        Write-Info ("`n共 {0} 项，待办 {1}，进行中 {2}，逾期 {3}" -f $data.stats.total, ($data.stats.by_status.'未开始' ?? 0), ($data.stats.by_status.'进行中' ?? 0), $data.stats.overdue)
    }
    "show" {
        if (-not $Arg1) { Write-Fail "用法：workstation.ps1 show <任务ID>"; exit 1 }
        $data = Invoke-Api -Method GET -Path "/api/my/tasks/$Arg1"
        if ($Json) { $data | ConvertTo-Json -Depth 8; break }
        Show-Task $data.task $data.events $data.artifacts $data.current_run
    }
    "start" {
        if (-not $Arg1) { Write-Fail "用法：workstation.ps1 start <任务ID>"; exit 1 }
        $run = Invoke-Api -Method POST -Path "/api/my/tasks/$Arg1/start" -Body @{}
        if ($Json) { $run | ConvertTo-Json -Depth 6; break }
        if ($run.already_running) {
            Write-Info "任务已经在执行中"
        } elseif ($run.reopen) {
            Write-Info ("任务已重新打开，开始第 {0} 次执行" -f $run.attempt)
        } else {
            Write-Info ("已开始第 {0} 次执行" -f $run.attempt)
        }
    }
    "log" {
        if (-not $Arg1 -or -not $Arg2) { Write-Fail "用法：workstation.ps1 log <任务ID> `"进展内容`""; exit 1 }
        Invoke-Api -Method POST -Path "/api/my/tasks/$Arg1/log" -Body @{ message = $Arg2 } | Out-Null
        Write-Info "已记录进展"
    }
    "upload" {
        if (-not $Arg1 -or -not $Arg2) { Write-Fail "用法：workstation.ps1 upload <任务ID> <文件路径>"; exit 1 }
        $art = Upload-Artifact $Arg1 $Arg2
        if ($Json) { $art | ConvertTo-Json -Depth 6; break }
        Write-Info ("产出已上传：{0}（{1} bytes）" -f $art.name, $art.size)
        if ($art.folder) { Write-Info ("已归档到文件管理：{0}" -f $art.folder) }
    }
    "done" {
        if (-not $Arg1) { Write-Fail "用法：workstation.ps1 done <任务ID> [-Summary `"结果摘要`"]"; exit 1 }
        if (-not $Summary) { $Summary = $Arg2 }
        if (-not $Summary) { Write-Fail "请用 -Summary 或第二个参数提供结果摘要"; exit 1 }
        Invoke-Api -Method POST -Path "/api/my/tasks/$Arg1/complete" -Body @{ summary = $Summary; artifacts = @() } | Out-Null
        Write-Info "任务已完成，结果已提交"
    }
    "skills" {
        $data = Invoke-Api -Method GET -Path "/api/skills"
        if ($Json) { $data | ConvertTo-Json -Depth 6; break }
        $data.items | ForEach-Object { Write-Info ("{0} · v{1}" -f $_.name, $_.current_version) }
    }
    default {
        Write-Info "用法：workstation.ps1 <tasks|show|start|log|upload|done|skills> [参数] [-Token ..] [-BaseUrl ..] [-Json]"
        exit 1
    }
}

exit 0

