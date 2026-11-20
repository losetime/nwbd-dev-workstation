/** 登录状态。调用时机：路由守卫检查、布局页显示用户、用户信息页展示令牌。 */

import { ref } from "vue";

import { request } from "@/@api/http";

export interface AuthUser {
    id: string;
    name: string;
    dept: string;
    title: string;
    role: string;
    job_type: string;
    level: string;
    status: string;
    avatar?: string;
    perms: string[];
    unionid?: string;
    userid?: string;
}

const TOKEN_KEY = "nwbd_ws_token";

const user = ref<AuthUser | null>(null);
const token = ref<string>(localStorage.getItem(TOKEN_KEY) ?? "");
const ready = ref(false);

let loading: Promise<void> | null = null;

/** 首次进入时校验登录态，失败即认定为未登录。 */
async function ensure(): Promise<void> {
    if (ready.value) return;
    if (loading) return loading;
    loading = (async () => {
        try {
            user.value = await request<AuthUser>("/api/auth/me");
        } catch {
            user.value = null;
        } finally {
            ready.value = true;
            loading = null;
        }
    })();
    return loading;
}

/** 登录成功后写入会话；令牌只在本次登录时由服务端返回一次。 */
function setSession(current: AuthUser, plainToken?: string): void {
    user.value = current;
    if (plainToken) {
        token.value = plainToken;
        localStorage.setItem(TOKEN_KEY, plainToken);
    }
    ready.value = true;
}

function clear(): void {
    user.value = null;
    token.value = "";
    localStorage.removeItem(TOKEN_KEY);
    ready.value = true;
}

/** 是否有某个界面权限。 */
function can(key: string): boolean {
    return (user.value?.perms ?? []).includes(key);
}

/** 退出登录：先吊销服务端令牌，再清本地状态。 */
async function logout(): Promise<void> {
    try {
        await request("/api/auth/logout", { method: "POST" });
    } catch {
        // 网络失败也继续清本地状态
    }
    clear();
}

/** 强制刷新登录态。 */
async function refresh(): Promise<void> {
    ready.value = false;
    loading = null;
    await ensure();
}

export function useAuth() {
    return { user, token, ready, ensure, refresh, setSession, clear, logout, can };
}
