/** 我的任务（看板）接口。 */

import { query, request } from "@/@api/http";

export type TaskStatus = "未开始" | "进行中" | "已完成" | "已延期" | "已取消";

export interface TaskResult {
    summary: string;
    artifacts: string[];
    agent: string;
    finished_at: string | null;
}

export interface TaskSkill {
    name: string;
    version: number | null;
    /** type = 任务类型带的（锁定，不可删）；manual = 任务里自己加的 */
    source?: "type" | "manual";
}

export interface BoardTask {
    id: string;
    project_id: string;
    project_name: string;
    stage: string;
    name: string;
    description: string;
    acceptance: string[];
    priority: "P0" | "P1" | "P2";
    tags: string[];
    links: { label: string; url: string }[];
    owner_id: string;
    owner_name: string;
    job_type: string;
    plan_start: string | null;
    plan_due: string | null;
    actual_done: string | null;
    status: TaskStatus;
    estimate_hours: number;
    actual_hours: number;
    overdue: boolean;
    result: TaskResult | null;
    skill: { name: string; version: number | null } | null;
    skills: TaskSkill[];
    attempt_count: number;
    last_event_at: string | null;
    stalled: boolean;
    updated_at: string;
    created_at: string;
    created_by: string;
    creator_name: string;
    task_type: string;
    task_type_code: string;
    /** 产出归档到文件管理的哪个一级目录（任务创建时的快照）。 */
    archive_folder: string;
    /** 子任务进度（0/0 表示没有子任务）。 */
    subtask_total: number;
    subtask_done: number;
    approval_instance_id: string;
    approval_status: string;
}

export interface TaskStats {
    total: number;
    by_status: Record<string, number>;
    overdue: number;
}

export interface TaskEvent {
    id: string;
    type: string;
    message: string;
    actor: string;
    level: string;
    attempt: number;
    at: string;
}

/** 任务内部的 TODO（研发拆解的细项）。 */
export interface Subtask {
    id: string;
    task_id: string;
    title: string;
    done: boolean;
    sort_order: number;
    created_by: string;
    creator_name: string;
    created_at: string;
    done_by: string;
    done_by_name: string;
    done_at: string | null;
}

export interface TaskDetail {
    task: BoardTask;
    events: TaskEvent[];
    artifacts: TaskArtifact[];
    subtasks: Subtask[];
    can_edit: boolean;
    can_operate: boolean;
    is_owner: boolean;
    creator_name: string;
}

export interface TaskArtifact {
    id: string;
    name: string;
    /** 归档到文件管理的目录。 */
    folder: string;
    size: number;
    content_type: string;
    uploaded_by: string;
    uploader_name: string;
    created_at: string;
}

/** 任务类型字典项（来自《任务类型归档阶段对照表》）。 */
export interface TaskTypeOption {
    code: string;
    name: string;
    group_name: string;
    archive_stage: string;
    archive_folder: string;
    /** project = 落项目目录；public = 落公共空间「研发部」 */
    archive_target: "project" | "public";
    archive_note: string;
    description: string;
    sort_order: number;
    enabled: boolean;
    updated_at: string;
    skills: { name: string; version: number | null }[];
}

/** 任务类型下拉数据；all=true 仅供管理页（含停用）。 */
export async function listTaskTypes(all = false): Promise<TaskTypeOption[]> {
    const data = await request<{ items: TaskTypeOption[]; total: number }>(
        `/api/task-types${all ? "?all=1" : ""}`,
    );
    return data.items;
}

export interface TaskTypePayload {
    name: string;
    group_name: string;
    description: string;
    archive_stage: string;
    archive_folder: string;
    archive_target: "project" | "public";
    archive_note: string;
    sort_order: number;
    skills: { name: string; version: number | null }[];
}

/** 新建任务类型（管理页）。 */
export function createTaskType(payload: TaskTypePayload): Promise<TaskTypeOption> {
    return request<TaskTypeOption>("/api/task-types", { method: "POST", body: JSON.stringify(payload) });
}

/** 编辑任务类型（编码不可改）。 */
export function updateTaskType(code: string, payload: TaskTypePayload): Promise<TaskTypeOption> {
    return request<TaskTypeOption>(`/api/task-types/${code}`, {
        method: "PATCH",
        body: JSON.stringify(payload),
    });
}

/** 启用 / 停用任务类型。 */
export function setTaskTypeStatus(code: string, enabled: boolean): Promise<{ ok: boolean }> {
    return request<{ ok: boolean }>(`/api/task-types/${code}/status`, {
        method: "PATCH",
        body: JSON.stringify({ enabled }),
    });
}

/** 删除任务类型（有任务引用时后端会拒绝）。 */
export function deleteTaskType(code: string): Promise<{ ok: boolean }> {
    return request<{ ok: boolean }>(`/api/task-types/${code}`, { method: "DELETE" });
}


export interface SkillItem {
    name: string;
    display_name: string;
    summary: string;
    current_version: number;
    version_count: number;
}

export interface SkillDetail {
    name: string;
    display_name: string;
    summary: string;
    current_version: number;
    versions: { version: number; status: string; changelog: string }[];
}

export interface TimesheetSuggestion {
    date: string;
    suggested: number;
    already: number;
    default: number;
}

export interface TimesheetInfo {
    task_id: string;
    estimate: number;
    actual: number;
    plan_start: string | null;
    plan_due: string | null;
    rate: number;
    booked: Record<string, number>;
    suggestions: TimesheetSuggestion[];
    reason: string;
}

/** 任务列表 + 统计。 */
export async function listMyTasks(filters: { status?: string; keyword?: string; scope?: string } = {}): Promise<{
    items: BoardTask[];
    stats: TaskStats;
}> {
    return request<{ items: BoardTask[]; stats: TaskStats }>(`/api/my/tasks${query(filters)}`);
}

/** 任务详情（含时间线）。 */
export function getMyTask(id: string): Promise<TaskDetail> {
    return request<TaskDetail>(`/api/my/tasks/${id}`);
}

/** 改状态。 */
export function updateMyTaskStatus(id: string, status: string): Promise<{ ok: boolean }> {
    return request<{ ok: boolean }>(`/api/my/tasks/${id}/status`, {
        method: "PATCH",
        body: JSON.stringify({ status }),
    });
}

/** 记录一条进展。 */
export function logMyTask(id: string, message: string): Promise<{ ok: boolean }> {
    return request<{ ok: boolean }>(`/api/my/tasks/${id}/log`, {
        method: "POST",
        body: JSON.stringify({ message }),
    });
}

/** 提交结果并完成。 */
export function completeMyTask(id: string, summary: string, artifacts: string[]): Promise<{ ok: boolean }> {
    return request<{ ok: boolean }>(`/api/my/tasks/${id}/complete`, {
        method: "POST",
        body: JSON.stringify({ summary, artifacts }),
    });
}

export interface TaskSkill {
    name: string;
    version: number | null;
    /** type = 任务类型带的（锁定，不可删）；manual = 任务里自己加的 */
    source?: "type" | "manual";
}

export interface TaskEditPayload {
    name?: string;
    description?: string;
    acceptance?: string[];
    priority?: string;
    tags?: string[];
    skills?: TaskSkill[];
    stage?: string;
    owner_id?: string;
    plan_start?: string;
    plan_due?: string;
    estimate_hours?: number;
}

/** 编辑任务内容。 */
export function updateMyTask(id: string, payload: TaskEditPayload): Promise<{ ok: boolean }> {
    return request<{ ok: boolean }>(`/api/my/tasks/${id}`, {
        method: "PATCH",
        body: JSON.stringify(payload),
    });
}

export const TASK_TABS: { key: string; label: string }[] = [    { key: "", label: "全部" },
    { key: "未开始", label: "待办" },
    { key: "进行中", label: "进行中" },
    { key: "已延期", label: "已延期" },
    { key: "已完成", label: "已完成" },
];

/** 状态配色。 */export function taskTone(status: string): string {
    switch (status) {
        case "进行中":
            return "border-ink/20 bg-paper-deep text-ink";
        case "已完成":
            return "border-emerald-500/30 bg-emerald-50 text-emerald-700";
        case "已延期":
            return "border-accent/40 bg-accent-soft text-accent";
        case "已取消":
            return "border-line bg-paper text-ink-faint line-through";
        default:
            return "border-line bg-white text-ink-soft";
    }
}

/** 优先级配色。 */
export function priorityTone(priority: string): string {
    switch (priority) {
        case "P0":
            return "border-accent/40 bg-accent-soft text-accent";
        case "P1":
            return "border-amber-500/40 bg-amber-50 text-amber-700";
        default:
            return "border-line bg-white text-ink-faint";
    }
}

// ---------- 技能与执行 ----------

/** 技能库列表（服务端代理）。 */
export async function listSkills(q = ""): Promise<SkillItem[]> {
    const data = await request<{ items: SkillItem[]; total: number }>(`/api/skills${q ? `?q=${encodeURIComponent(q)}` : ""}`);
    return data.items;
}

/** 技能详情（含版本）。 */
export function getSkillDetail(name: string): Promise<SkillDetail> {
    return request<SkillDetail>(`/api/skills/${encodeURIComponent(name)}`);
}

/** 开始执行（已完成的会重新打开，执行次数 +1）。 */
export function startTask(id: string): Promise<{ attempt: number; reopen: boolean }> {
    return request<{ attempt: number; reopen: boolean }>(`/api/my/tasks/${id}/start`, { method: "POST", body: "{}" });
}

/** 上传产出。 */
export async function uploadArtifact(id: string, file: File): Promise<TaskArtifact> {
    const body = new FormData();
    body.append("file", file);
    return request<TaskArtifact>(`/api/my/tasks/${id}/artifacts`, { method: "POST", body });
}

/** 产出下载地址。 */
export function artifactURL(taskId: string, artifactId: string): string {
    return `/api/my/tasks/${taskId}/artifacts/${artifactId}`;
}

/** 新增子任务（仅任务负责人）。 */
export function createSubtask(taskId: string, title: string): Promise<Subtask> {
    return request<Subtask>(`/api/my/tasks/${taskId}/subtasks`, {
        method: "POST",
        body: JSON.stringify({ title }),
    });
}

/** 更新子任务：改标题或勾选（仅任务负责人）。 */
export function updateSubtask(taskId: string, id: string, payload: { title?: string; done?: boolean }): Promise<Subtask> {
    return request<Subtask>(`/api/my/tasks/${taskId}/subtasks/${id}`, {
        method: "PATCH",
        body: JSON.stringify(payload),
    });
}

/** 删除子任务（仅任务负责人）。 */
export function deleteSubtask(taskId: string, id: string): Promise<{ ok: boolean }> {
    return request<{ ok: boolean }>(`/api/my/tasks/${taskId}/subtasks/${id}`, { method: "DELETE" });
}

/** 工时确认建议。 */
export function getTimesheet(id: string): Promise<TimesheetInfo> {
    return request<TimesheetInfo>(`/api/my/tasks/${id}/timesheet`);
}

/** 确认工时。 */
export function saveTimesheet(id: string, items: { date: string; hours: number }[]): Promise<{ total: number; days: number; rate: number }> {
    return request<{ total: number; days: number; rate: number }>(`/api/my/tasks/${id}/timesheet`, {
        method: "POST",
        body: JSON.stringify({ items }),
    });
}

