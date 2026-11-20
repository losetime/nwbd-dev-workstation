/** 项目变更（05 项目变更申请）接口。 */
import { query, request } from "@/@api/http";

export interface ChangeRecord {
    id: string;
    code: string;
    project_id: string;
    project_name: string;
    type: string;
    source: string;
    content: string;
    impact: string;
    reason: string;
    budget_before: number | null;
    budget_after: number | null;
    plan_due_after: string | null;
    material_path: string;
    status: string;
    approval_status: string;
    applicant_id: string;
    applicant_name: string;
    applied_on: string | null;
    instance_id: string;
    business_id: string;
    before_value: string;
    after_value: string;
}

export interface ChangePayload {
    type: string;
    source: string;
    content: string;
    reason: string;
    impact: string;
    budget_before?: number | null;
    budget_after?: number | null;
    plan_due_after?: string;
    material_path?: string;
}

export interface ChangeApproval {
    instance_id: string;
    title: string;
    status: string;
    result: string;
    business_id: string;
    create_time: string;
    display_status: string;
    operations: { user_id: string; date: string; type: string; result: string; remark: string; show_name: string }[];
}

/** 发起变更。 */
export function createChange(projectId: string, payload: ChangePayload): Promise<{ change: ChangeRecord }> {
    return request<{ change: ChangeRecord }>(`/api/projects/${projectId}/changes`, {
        method: "POST",
        body: JSON.stringify(payload),
    });
}

/** 某项目的变更列表。 */
export function listProjectChanges(projectId: string): Promise<ChangeRecord[]> {
    return request<{ items: ChangeRecord[] }>(`/api/projects/${projectId}/changes`).then((d) => d.items);
}

/** 跨项目变更列表。 */
export function listChanges(status = ""): Promise<{ items: ChangeRecord[]; stats: { total: number; pending: number } }> {
    return request<{ items: ChangeRecord[]; stats: { total: number; pending: number } }>(
        `/api/changes${query({ status })}`,
    );
}

/** 变更详情（含审批轨迹）。 */
export function getChange(id: string): Promise<{ change: ChangeRecord; approval: ChangeApproval | null }> {
    return request<{ change: ChangeRecord; approval: ChangeApproval | null }>(`/api/changes/${id}`);
}
