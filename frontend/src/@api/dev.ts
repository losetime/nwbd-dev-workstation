/** 本地测试身份切换（仅在 NWBD_WS_DEV_ACT_AS=1 且 local 环境可用）。 */

import { request } from "@/@api/http";
import type { AuthUser } from "@/store/auth";

export interface DevPerson {
    id: string;
    name: string;
    job_type: string;
    dept: string;
    title: string;
}

export interface AuthConfig {
    enabled: boolean;
    devLogin: boolean;
    devActAs: boolean;
}

/** 公开登录配置（同时告诉我们测试开关是否打开）。 */
export function getAuthConfig(): Promise<AuthConfig> {
    return request<AuthConfig>("/api/auth/config");
}

/** 可切换的在职人员。 */
export async function listDevPeople(): Promise<DevPerson[]> {
    const data = await request<{ items: DevPerson[]; total: number }>("/api/dev/people");
    return data.items;
}

/** 切换成指定用户。 */
export function actAs(userId: string): Promise<{ ok: boolean; user: AuthUser; token: string }> {
    return request<{ ok: boolean; user: AuthUser; token: string }>("/api/dev/act-as", {
        method: "POST",
        body: JSON.stringify({ user_id: userId }),
    });
}
