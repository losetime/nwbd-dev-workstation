# 研发项目管理系统（nwbd-dev-workstation）

研发部（兼项目部）的内部项目管理系统。业务基线是 `02_可点击原型/研发管理系统原型.html`，
界面按诺维自己的视觉（纸墨色、直角、左侧导航）实现。

当前完成 **P0 + P1**：

- 钉钉扫码登录、人员档案（岗位由钉钉职位映射）、界面权限
- 钉钉通讯录**全量同步**（68 人），没登录过的人也能被分配任务
- 项目台账：列表卡片（阶段条、完成率、成员、未关闭风险）、新建项目（自动生成阶段里程碑）
- 项目详情五个区块：阶段与里程碑、合同与预算、项目任务、项目成员、风险管理
- 系统管理：人员岗位 / 级别 / 状态 / 界面权限

**只在本地 Docker 运行**，数据库与 `nwbd-file-manage` 完全隔离（独立库、独立网络、独立端口）。

---

## 1. 目录结构

```
nwbd-dev-workstation/
├─ backend/                  Go 1.24（标准库 HTTP + pgx）
│  ├─ cmd/api/main.go        配置 → 迁移 → 登录 / 同步 → HTTP
│  └─ internal/
│     ├─ domain/             业务枚举、阶段模板、职位→岗位映射、权限默认值
│     ├─ metrics/            全部计算口径（含单测）
│     ├─ auth/               扫码登录、令牌、角色同步
│     ├─ org/                通讯录同步
│     ├─ dingtalk/           access_token、扫码换用户、部门与成员
│     ├─ store/              pgx 手写 SQL
│     ├─ filemanage/         文件管理内部接口（任务产出归档）
│     ├─ db/                 连接池 + 内嵌迁移
│     └─ api/                路由、鉴权、项目与人员接口
├─ frontend/                 Vue 3 + Vite + Tailwind 4
│  └─ src/
│     ├─ views/projects/     项目列表、项目详情
│     ├─ views/admin/        系统管理 · 人员
│     ├─ views/login/        钉钉扫码
│     ├─ utils/project.ts    阶段模板、状态配色、金额格式
│     └─ @api/               projects / users / http
├─ deploy/                   docker-compose、nginx、.env
└─ docs/P0-模型与接口.md      数据模型、枚举、权限、接口、计算口径
```

## 2. 本地启动

```bash
cd deploy
cp .env.example .env      # 填数据库口令、钉钉凭据、文件管理内部令牌
docker compose up -d --build
```

| 入口 | 地址 |
|---|---|
| 前端 | http://127.0.0.1:8090 |
| API | http://127.0.0.1:8092/api/health |
| PostgreSQL | 127.0.0.1:5434（库/用户见 `.env`） |

改代码后只重建对应镜像：

```bash
docker compose build api && docker compose up -d api
docker compose build web && docker compose up -d web
```

## 3. 关键配置

```dotenv
POSTGRES_PASSWORD=...
DD_CLIENT_ID=...             # 复用 nwbd-file-manage 的企业内部应用
DD_CLIENT_SECRET=...
PUBLIC_BASE_URL=http://127.0.0.1:8090
LOGIN_REDIRECT=http://127.0.0.1:8090/login   # 必须与钉钉后台回调完全一致

# 通讯录全量同步间隔（分钟）
DIRECTORY_SYNC_MINUTES=30

# 线上技能库（nwbd-skills）
SKILLS_BASE=http://8.160.178.247:18138

# 文件管理（nwbd-file-manage）内部接口：任务产出统一归档到那边
# api 容器需加入 Docker 网络 nwbd-link-files-dev
FILE_MANAGE_BASE=http://nwbd-fm-api:8000
FILE_MANAGE_TOKEN=...         # 与文件管理 NWBD_FM_INTERNAL_TOKEN 一致
MAX_UPLOAD_MB=100

# 钉钉审批专用应用（04 项目策划流程：发起 / 状态）
APPROVAL_CLIENT_ID=...
APPROVAL_CLIENT_SECRET=...
APPROVAL_AGENT_ID=...
APPROVAL_PROCESS_CODE=PROC-2E309DC0-76E4-47D4-94C4-F0A2780B4100
CONTRACT_PROCESS_CODE=PROC-ACB82876-19CB-4B99-8FF1-8CAB8EB0223F
APPROVAL_STREAM_ENABLED=true

# 本地开发：登录页显示「以本地开发账号登录」按钮，并用页头「测试身份」切换人员
# 上线前必须清空这两项
DEV_LOGIN_USERID=dev_local   # 留空 = 只有钉钉扫码
DEV_ACT_AS=true              # false = 不显示测试身份下拉
```

## 4. 钉钉登录回调（必须两侧对齐）

「应用开发 → 登录与分享 → 回调域名」里登记完整 URL，本地和线上都要有：

```
https://files.nwbd.net/login
http://127.0.0.1:8090/login
```

- `LOGIN_REDIRECT` 必须与其中一条**完全一致**（协议、端口、路径）
- **页面地址必须与 `LOGIN_REDIRECT` 一致**，否则钉钉返回 `11021`（二维码在、手机扫完没反应）
- 改 `.env` 后要重启 api
- 排查不要用自动化接管的 Chrome（`navigator.webdriver=true` 会被风控拒）

## 5. 钉钉后台权限

- 「成员信息读权限」→ 钉钉管理员列表（角色同步）
- 通讯录读权限 → 部门列表、部门成员（**全量同步依赖它**）
- 应用可见范围要覆盖研发部与项目部，否则同步不到人
- 权限范围外的部门拉不到；同步只做新增/更新，不会把不可见的人标记离职

## 6. 岗位与权限

**岗位**由钉钉职位映射（规则见 `docs/P0-模型与接口.md`）：项目经理 / 部门经理 / 开发 / 测试 / UI / 其他。
管理员在「系统管理」里改过的岗位会标记为手工值，通讯录同步不再覆盖。

**界面权限** 8 项：我的项目 / 我的待办 / 我的任务 / 资源视图 / 风险管理 / 变更管理 / 困难上报 / 系统管理。
个人未单独配置时按岗位默认；数据范围按角色（部门经理全部、其他人只看自己参与的项目）。

## 7. 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/auth/me` | 当前用户（含 job_type、perms） |
| GET | `/api/users` | 人员列表 |
| PATCH | `/api/users/{id}` | 改岗位 / 级别 / 状态 / 权限 |
| POST | `/api/users/sync` | 手动同步通讯录 |
| GET | `/api/projects` | 项目列表 |
| POST | `/api/projects` | 新建项目 |
| GET | `/api/projects/{id}` | 项目详情 |
| POST | `/api/projects/{id}/attachments?kind=plan\|budget` | 上传附件 |
| GET | `/api/projects/{id}/attachments/{kind}` | 下载附件 |
| PATCH | `/api/projects/{id}` | 编辑项目信息 |
| PATCH | `/api/projects/{id}/status` | 改项目状态 |
| PATCH | `/api/projects/{id}/risk-flag` | 标记风险标识 |
| PUT | `/api/projects/{id}/managers` | 更换项目经理 |
| POST | `/api/projects/{id}/milestones/plan` | 批量填节点计划 |
| PATCH | `/api/projects/{id}/milestones/{milestoneID}` | 单条里程碑 |
| POST | `/api/projects/{id}/tasks` | 下发任务 |
| PATCH | `/api/tasks/{id}/status` | 任务状态流转 |
| GET | `/api/projects/{id}/candidates` | 可加入的候选人 |
| GET | `/api/my/tasks` | 我的任务看板（统计 / 筛选 / 搜索） |
| GET | `/api/my/tasks/{id}` | 任务详情（含时间线、执行结果） |
| PATCH | `/api/my/tasks/{id}` | 编辑任务内容 |
| PATCH | `/api/my/tasks/{id}/status` | 任务状态流转 |
| POST | `/api/my/tasks/{id}/log` | 记录进展 |
| POST | `/api/my/tasks/{id}/complete` | 提交结果并完成 |
| POST | `/api/my/tasks/{id}/execute` | 按技能触发执行（认领） |
| POST | `/api/my/tasks/{id}/artifacts` | 上传产出（multipart，服务端转发文件管理归档） |
| GET | `/api/my/tasks/{id}/artifacts/{artifactId}` | 下载产出（服务端代理文件管理文件流） |
| GET | `/api/task-types` | 任务类型字典（33 项项目任务） |
| GET | `/api/my/tasks/{id}/timesheet` | 工时确认建议 |
| POST | `/api/my/tasks/{id}/timesheet` | 确认工时 |
| GET | `/api/skills` | 技能库列表（代理 nwbd-skills） |
| POST | `/api/projects/{id}/members` | 增加成员 |
| PATCH | `/api/projects/{id}/members/{userID}` | 退出 / 恢复参与 |

## 8. 排查

| 现象 | 处理 |
|---|---|
| 扫码后报 `redirect_uri参数错误` | `LOGIN_REDIRECT` 不在回调列表里，或协议/端口/路径不一致 |
| 二维码在，扫完没反应（`11021`） | 页面地址与 `LOGIN_REDIRECT` 不一致；或用了自动化浏览器 |
| 人员列表为空 | 看日志「通讯录同步完成」；检查通讯录权限范围 |
| 岗位不对 | 钉钉职位没命中关键词，可在系统管理里手工改（改后不再跟随同步） |
| 产出上传失败 | 看返回提示；`docker logs nwbd-ws-api`，检查 `FILE_MANAGE_TOKEN` 与文件管理是否就绪 |
| 角色一直是 member | `43007` 表示缺「成员信息读权限」 |
| 发版后页面还是旧界面 | `index.html` 已设 `no-cache`，仍需浏览器强刷一次（Ctrl+F5）；JS/CSS 文件名带 hash，可长缓存 |

## 9. Skill 包（nwbd-dev-workstation）

`skills/nwbd-dev-workstation/` 是给 agent 用的技能包，按 `nwbd-file-upload` 的令牌约定实现：

```
skills/nwbd-dev-workstation/
├─ SKILL.md
└─ scripts/
   ├─ workstation.ps1   Windows
   └─ workstation.sh    macOS / Linux（需要 jq）
```

命令：`tasks` / `show` / `start` / `log` / `upload` / `done` / `skills`。
令牌优先级：`-Token` → `NWBD_WS_TOKEN` → 本地配置 `~/.config/nwbd-dev-workstation/config.json`；没有或过期时退出码 2。
默认平台地址 `http://127.0.0.1:8092`，可用 `-BaseUrl` / `NWBD_WS_BASE_URL` 覆盖。

发布到技能库时把这个目录打包即可（当前仓库里只存源码，不做上传）。

## 10. 下一步
- P4：变更单 + 钉钉审批
- P5：风险 / 困难上报 / 我的待办
- P6：资源视图、项目驾驶舱

