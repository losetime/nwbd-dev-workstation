/** 人员与权限接口封装。 */

import { query, request } from "@/@api/http";

export interface Person {
    id: string;
    name: string;
    userid: string;
    title: string;
    dept: string;
    job_type: string;
    level: string;
    status: string;
    role: string;
    perms: string[];
    custom: boolean;
    avatar: string;
}

export interface PersonFilters {
    keyword?: string;
    job_type?: string;
    status?: string;
}

/** 人员列表。 */
export async function listPeople(filters: PersonFilters = {}): Promise<Person[]> {
    const data = await request<{ items: Person[]; total: number }>(`/api/users${query(filters)}`);
    return data.items;
}

export interface UpdatePersonPayload {
    job_type?: string;
    level?: string;
    status?: string;
    perms?: string[];
}

/** 更新人员（岗位 / 级别 / 状态 / 界面权限）。 */
export function updatePerson(id: string, payload: UpdatePersonPayload): Promise<Person> {
    return request<Person>(`/api/users/${id}`, { method: "PATCH", body: JSON.stringify(payload) });
}

/** 手动触发通讯录同步。 */
export function syncPeople(): Promise<{ total: number; synced: number }> {
    return request<{ total: number; synced: number }>("/api/users/sync", { method: "POST" });
}

export interface ManagerOption {
    id: string;
    name: string;
    job_type: string;
    title: string;
    dept: string;
}

/** 项目经理候选：任何能看项目的人都可以调用（不需要系统管理权限）。 */
export async function listManagers(): Promise<ManagerOption[]> {
    const data = await request<{ items: ManagerOption[]; total: number }>("/api/users/managers");
    return data.items;
}

export const JOB_TYPES = ["部门经理", "项目经理", "开发", "测试", "UI", "其他"];
export const LEVELS = ["初级", "中级", "高级", "专家"];

export const PERM_OPTIONS: { key: string; label: string }[] = [
    { key: "myproj", label: "我的项目" },
    { key: "pending", label: "我的待办" },
    { key: "mytask", label: "我的任务" },
    { key: "resview", label: "资源视图" },
    { key: "myrisk", label: "风险管理" },
    { key: "mychange", label: "变更管理" },
    { key: "help", label: "困难上报" },
    { key: "sysadmin", label: "系统管理" },
];
