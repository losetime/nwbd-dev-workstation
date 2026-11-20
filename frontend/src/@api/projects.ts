/** 项目接口封装。 */

import { query, request } from "@/@api/http";

export interface Project {
    id: string;
    code: string;
    name: string;
    type: string;
    status: string;
    customer: string;
    current_stage: string;
    current_milestone: string;
    milestone_status: string;
    plan_start: string | null;
    plan_end: string | null;
    actual_end: string | null;
    contract_amount: number | null;
    budget: number | null;
    cost_actual: number | null;
    labor_budget: number | null;
    labor_rate: number | null;
    risk_flag: string;
    weekly_summary: string;
    summary_at: string | null;
    remark: string;
    task_total: number;
    task_done: number;
    open_risks: number;
    member_count: number;
    managers: string;
    progress: number | null;
    created_at: string;
    contract_instance_id: string;
    contract_business_id: string;
    contract_status: string;
}

export interface Milestone {
    id: string;
    stage: string;
    name: string;
    plan_due: string | null;
    contract_due: string | null;
    actual_done: string | null;
    status: string;
    reviewer: string;
    review_date: string | null;
    version: number;
    locked: boolean;
    plan_overdue: boolean;
    contract_overdue: boolean;
}

export interface Task {
    id: string;
    name: string;
    stage: string;
    owner_id: string;
    owner_name: string;
    job_type: string;
    plan_start: string | null;
    plan_due: string | null;
    actual_done: string | null;
    status: string;
    estimate_hours: number;
    actual_hours: number;
    overdue: boolean;
    delay_days: number;
}

export interface Member {
    user_id: string;
    name: string;
    job_type: string;
    title: string;
    project_role: string;
    allocation: number;
    status: string;
    joined_at: string | null;
    left_at: string | null;
}

export interface Risk {
    id: string;
    code: string;
    title: string;
    description: string;
    type: string;
    level: string;
    impact: string;
    action: string;
    owner_id: string;
    owner_name: string;
    target_date: string | null;
    resolved_date: string | null;
    status: string;
    identified_on: string | null;
}

export interface ProjectDetail {
    project: Project;
    milestones: Milestone[];
    tasks: Task[];
    members: Member[];
    risks: Risk[];
    can_manage: boolean;
}

export interface ProjectFilters {
    type?: string;
    status?: string;
    risk_flag?: string;
    keyword?: string;
    /** managed = 我担任项目经理的项目（部门经理仍返回全部）。 */
    scope?: string;
}

/** 拉取项目列表。 */
export async function listProjects(filters: ProjectFilters = {}): Promise<Project[]> {
    const data = await request<{ items: Project[]; total: number }>(`/api/projects${query(filters)}`);
    return data.items;
}

/** 拉取项目详情。 */
export function getProject(id: string): Promise<ProjectDetail> {
    return request<ProjectDetail>(`/api/projects/${id}`);
}

export interface CreateProjectPayload {
    code: string;
    name: string;
    type: string;
    customer: string;
    contract_amount: number | null;
    manager_ids: string[];
}

/** 新建项目。 */
export function createProject(payload: CreateProjectPayload): Promise<Project> {
    return request<Project>("/api/projects", { method: "POST", body: JSON.stringify(payload) });
}

// ---------- P2：项目操作 ----------

export interface ProjectInfoPayload {
    name?: string;
    customer?: string;
    contract_amount?: number;
    plan_start?: string;
    plan_end?: string;
    budget?: number;
    labor_budget?: number;
    labor_rate?: number;
    remark?: string;
}

/** 编辑项目信息。 */
export function updateProject(id: string, payload: ProjectInfoPayload): Promise<Project> {
    return request<Project>(`/api/projects/${id}`, { method: "PATCH", body: JSON.stringify(payload) });
}

/** 手动改项目状态。 */
export function updateProjectStatus(id: string, status: string): Promise<Project> {
    return request<Project>(`/api/projects/${id}/status`, { method: "PATCH", body: JSON.stringify({ status }) });
}

/** 标记风险标识（部门经理）。 */
export function updateRiskFlag(id: string, risk_flag: string): Promise<Project> {
    return request<Project>(`/api/projects/${id}/risk-flag`, { method: "PATCH", body: JSON.stringify({ risk_flag }) });
}

/** 更换项目经理（部门经理）。 */
export function replaceManagers(id: string, manager_ids: string[]): Promise<Project> {
    return request<Project>(`/api/projects/${id}/managers`, { method: "PUT", body: JSON.stringify({ manager_ids }) });
}

/** 删除项目（软删除，部门经理）。 */
export function deleteProject(id: string): Promise<{ ok: boolean }> {
    return request<{ ok: boolean }>(`/api/projects/${id}`, { method: "DELETE" });
}

export interface PlanItem {
    id: string;
    plan_due: string;
    contract_due: string;
}

/** 批量填写节点计划。 */
export function saveMilestonePlan(id: string, items: PlanItem[]): Promise<{ updated: number }> {
    return request<{ updated: number }>(`/api/projects/${id}/milestones/plan`, {
        method: "POST",
        body: JSON.stringify({ items }),
    });
}

export interface MilestonePayload {
    plan_due?: string;
    contract_due?: string;
    status?: string;
    actual_done?: string;
    reviewer?: string;
    review_date?: string;
}

/** 编辑单个里程碑。 */
export function updateMilestone(id: string, milestoneId: string, payload: MilestonePayload): Promise<Project> {
    return request<Project>(`/api/projects/${id}/milestones/${milestoneId}`, {
        method: "PATCH",
        body: JSON.stringify(payload),
    });
}

export interface TaskPayload {
    name: string;
    description?: string;
    acceptance?: string[];
    priority?: string;
    tags?: string[];
    skills?: { name: string; version: number | null }[];
    stage: string;
    owner_id: string;
    plan_start: string;
    plan_due: string;
    estimate_hours: number;
    task_type_code: string;
    approval?: { contract_instance_id: string; source: string };
}

/** 下发任务。 */
export function createTask(id: string, payload: TaskPayload): Promise<{ id: string }> {
    return request<{ id: string }>(`/api/projects/${id}/tasks`, { method: "POST", body: JSON.stringify(payload) });
}

/** 任务状态流转。 */
export function updateTaskStatus(taskId: string, status: string): Promise<{ ok: boolean }> {
    return request<{ ok: boolean }>(`/api/tasks/${taskId}/status`, { method: "PATCH", body: JSON.stringify({ status }) });
}

// ---------- 风险管理 ----------

export interface RiskPayload {
    title: string;
    description?: string;
    type: string;
    level: string;
    impact?: string;
    action?: string;
    owner_id?: string;
    target_date?: string;
}

export interface RiskUpdatePayload extends Partial<RiskPayload> {
    status?: string;
    resolved_date?: string;
}

/** 登记风险。 */
export function createRisk(projectId: string, payload: RiskPayload): Promise<{ risk: Risk }> {
    return request<{ risk: Risk }>(`/api/projects/${projectId}/risks`, {
        method: "POST",
        body: JSON.stringify(payload),
    });
}

/** 编辑风险 / 改状态。 */
export function updateRisk(riskId: string, payload: RiskUpdatePayload): Promise<{ risk: Risk }> {
    return request<{ risk: Risk }>(`/api/risks/${riskId}`, {
        method: "PATCH",
        body: JSON.stringify(payload),
    });
}

export interface RiskRow extends Risk {
    project_id: string;
    project_name: string;
    project_code: string;
    overdue: boolean;
}

export interface RiskStats {
    total: number;
    open: number;
    high: number;
    overdue: number;
    resolved: number;
}

/** 跨项目风险列表（部门经理看全部，其他人只看自己参与的项目）。 */
export function listAllRisks(filters: { status?: string; level?: string; keyword?: string } = {}): Promise<{
    items: RiskRow[];
    stats: RiskStats;
}> {
    return request<{ items: RiskRow[]; stats: RiskStats }>(`/api/risks${query(filters)}`);
}

export interface Candidate {
    id: string;
    name: string;
    job_type: string;
    dept: string;
    title: string;
}

/** 可加入项目的候选人。 */
export async function listCandidates(id: string): Promise<Candidate[]> {
    const data = await request<{ items: Candidate[]; total: number }>(`/api/projects/${id}/candidates`);
    return data.items;
}

/** 增加项目成员。 */
export function addMembers(id: string, user_ids: string[], project_role: string): Promise<{ ok: boolean }> {
    return request<{ ok: boolean }>(`/api/projects/${id}/members`, {
        method: "POST",
        body: JSON.stringify({ user_ids, project_role }),
    });
}

/** 退出 / 恢复参与。有未完成任务时接口返回 409，前端只做提示。 */
export function setMemberStatus(id: string, userId: string, status: string): Promise<{ ok: boolean }> {
    return request<{ ok: boolean }>(`/api/projects/${id}/members/${userId}`, {
        method: "PATCH",
        body: JSON.stringify({ status }),
    });
}
