/** 困难上报接口。 */
import { query, request } from "@/@api/http";

export type IssueSeverity = "高" | "中" | "低";

export interface Issue {
    id: string;
    code: string;
    project_id: string;
    project_name: string;
    task_id: string;
    task_name: string;
    title: string;
    description: string;
    reporter_id: string;
    reporter_name: string;
    severity: IssueSeverity;
    status: string;
    handling: string;
    risk_id: string;
    reported_on: string | null;
}

export interface IssueStats {
    total: number;
    pending: number;
    risked: number;
    archived: number;
}

/** 困难列表：scope=mine 我上报的，scope=todo 待我处理。 */
export function listIssues(scope: "mine" | "todo", status = ""): Promise<{ items: Issue[]; stats: IssueStats }> {
    return request<{ items: Issue[]; stats: IssueStats }>(`/api/issues${query({ scope, status })}`);
}

export interface IssuePayload {
    project_id: string;
    task_id?: string;
    title: string;
    description?: string;
    severity: IssueSeverity;
}

/** 上报困难。 */
export function createIssue(payload: IssuePayload): Promise<{ issue: Issue }> {
    return request<{ issue: Issue }>("/api/issues", { method: "POST", body: JSON.stringify(payload) });
}

/** 归档（必须写处理说明）。 */
export function archiveIssue(issueId: string, handling: string): Promise<{ issue: Issue }> {
    return request<{ issue: Issue }>(`/api/issues/${issueId}/archive`, {
        method: "POST",
        body: JSON.stringify({ handling }),
    });
}
