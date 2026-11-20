<script setup lang="ts">
/** 登录页。调用时机：未登录访问任意页面；整屏布局，内嵌钉钉二维码，不跳转。 */
import { nextTick, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";

import { request } from "@/@api/http";
import FlipWords from "@/components/inspira/FlipWords.vue";
import { useAuth } from "@/store/auth";

const route = useRoute();
const router = useRouter();
const auth = useAuth();

const deviceCode = ref("");
const status = ref<"checking" | "qr" | "success" | "error" | "logged">("checking");
const message = ref("");
const userName = ref("");
const devLoginAvailable = ref(false);

interface AuthConfig {
    clientId: string;
    redirectUri: string;
    enabled: boolean;
    devLogin: boolean;
}

interface ExchangeResult {
    ok: boolean;
    user: { name: string };
    token?: string;
}

declare global {
    interface Window {
        DTFrameLogin?: (
            frameParams: Record<string, unknown>,
            loginParams: Record<string, unknown>,
            success: (result: { authCode?: string }) => void,
            fail: (error: string) => void,
        ) => void;
    }
}

/** 动态加载钉钉登录 SDK。 */
function loadSdk(): Promise<void> {
    return new Promise((resolve, reject) => {
        if (window.DTFrameLogin) {
            resolve();
            return;
        }
        const script = document.createElement("script");
        script.src = "https://g.alicdn.com/dingding/h5-dingtalk-login/0.21.0/ddlogin.js";
        script.onload = () => resolve();
        script.onerror = () => reject(new Error("钉钉登录组件加载失败，请检查网络"));
        document.head.appendChild(script);
    });
}

/** 扫码成功后把 authCode 交给后端，写入会话并直接进入首页。 */
async function exchange(authCode: string): Promise<void> {
    status.value = "checking";
    try {
        const result = await request<ExchangeResult>("/api/auth/dingtalk/exchange", {
            method: "POST",
            body: JSON.stringify({ auth_code: authCode, device_code: deviceCode.value }),
        });
        userName.value = result.user?.name ?? "";

        // 用刚下发的 Cookie 重新拉一次完整用户信息，并把令牌留在本浏览器
        await auth.refresh();
        if (auth.user.value) auth.setSession(auth.user.value, result.token);

        if (deviceCode.value) {
            status.value = "success";
            message.value = "登录成功，可以关闭此页面回到终端。";
            return;
        }
        // 网页登录：直接回首页（或登录前想去的页面），不提供任何手动按钮
        const redirect = typeof route.query.redirect === "string" ? route.query.redirect : "/";
        await router.replace(redirect);
    } catch (err) {
        status.value = "error";
        message.value = err instanceof Error ? err.message : "登录失败";
    }
}

/** 重新加载页面。 */
function reloadPage(): void {
    window.location.reload();
}

/** 本地开发登录：跳过钉钉，直接以固定账号进入（服务端只在本机 local 环境开启时才可用）。 */
async function devLogin(): Promise<void> {
    status.value = "checking";
    try {
        const result = await request<ExchangeResult>("/api/auth/dev-login", { method: "POST" });
        userName.value = result.user?.name ?? "";
        await auth.refresh();
        if (auth.user.value) auth.setSession(auth.user.value, result.token);
        const redirect = typeof route.query.redirect === "string" ? route.query.redirect : "/";
        await router.replace(redirect);
    } catch (err) {
        status.value = "error";
        message.value = err instanceof Error ? err.message : "本地开发登录失败";
    }
}

/** 开始二维码登录。 */
async function startQr(): Promise<void> {
    try {
        const config = await request<AuthConfig>("/api/auth/config");
        devLoginAvailable.value = Boolean(config.devLogin);
        if (!config.enabled) {
            status.value = "error";
            message.value = "服务未配置钉钉应用凭据，无法登录";
            return;
        }
        await loadSdk();
        status.value = "qr";
        await nextTick();
        window.DTFrameLogin?.(
            { id: "dingtalk_qr", width: 300, height: 320 },
            {
                redirect_uri: encodeURIComponent(config.redirectUri),
                client_id: config.clientId,
                scope: "openid",
                response_type: "code",
                state: deviceCode.value || "web",
                prompt: "consent",
            },
            (result) => {
                if (result?.authCode) void exchange(result.authCode);
                else {
                    status.value = "error";
                    message.value = "未拿到授权码，请重试";
                }
            },
            (error) => {
                status.value = "error";
                message.value = `钉钉登录组件报错：${error}`;
            },
        );
    } catch (err) {
        status.value = "error";
        message.value = err instanceof Error ? err.message : "登录页初始化失败";
    }
}

onMounted(async () => {
    deviceCode.value = String(route.query.device ?? "");

    // 已登录（Cookie 有效）：脚本场景提示关闭；网页场景直接进首页
    await auth.ensure();
    if (auth.user.value) {
        userName.value = auth.user.value.name;
        if (deviceCode.value) {
            status.value = "success";
            message.value = "已经登录过了，可以关闭此页面回到终端。";
            return;
        }
        const redirect = typeof route.query.redirect === "string" ? route.query.redirect : "/";
        await router.replace(redirect);
        return;
    }
    void startQr();
});
</script>

<template>
    <div class="flex min-h-screen bg-paper">
        <!-- 左侧陈述 -->
        <section class="relative hidden w-[52%] flex-col justify-between overflow-hidden bg-ink p-12 lg:flex">
            <div class="grain"></div>

            <div class="flex items-center gap-2.5">
                <span class="flex h-8 w-8 items-center justify-center bg-white font-display text-[15px] text-ink">N</span>
                <span class="font-display text-[15px] text-white/90">诺维开发工作台</span>
            </div>

            <div class="relative z-10 max-w-md">
                <p class="font-display text-[40px] leading-[1.25] tracking-tight text-white">
                    扫一下，<br />
                    账号、部门、角色就<FlipWords :words="['自动对齐', '一次到位', '不再重复填']" class="text-[#e9b98a]" />，<br />
                    进来就能干活。
                </p>
                <p class="mt-6 text-[13px] leading-7 text-white/45">
                    钉钉扫码登录后，身份信息从通讯录同步；管理员权限跟随钉钉主/子管理员自动更新。
                </p>
            </div>

            <div class="relative z-10 flex items-center gap-6 text-[11px] text-white/30">
                <span>nwbd-dev-workstation</span>
                <span>·</span>
                <span>内部使用</span>
            </div>
        </section>

        <!-- 右侧登录区 -->
        <section class="flex flex-1 items-center justify-center px-6 py-16">
            <div class="w-full max-w-[380px]">
                <p class="text-[11px] font-medium tracking-[0.18em] text-ink-faint uppercase">Sign in</p>
                <h1 class="mt-3 font-display text-[28px] leading-tight text-ink">登录</h1>
                <p class="mt-2 text-[12px] text-ink-faint">使用钉钉扫码，仅限本公司成员。</p>

                <!-- 已登录（脚本场景） -->
                <div v-if="status === 'logged'" class="mt-8">
                    <p class="font-display text-[16px] text-ink">已登录：{{ userName }}</p>
                    <p class="mt-1.5 text-[11.5px] leading-relaxed text-ink-faint">可以关闭此页面回到终端。</p>
                </div>

                <!-- 二维码 -->
                <div v-else-if="status === 'qr'" class="mt-8">
                    <div id="dingtalk_qr" class="flex min-h-[320px] items-center justify-center border border-line bg-white" />
                    <p class="mt-3 text-center text-[11.5px] text-ink-faint">打开钉钉 App，扫描二维码确认登录</p>
                </div>

                <!-- 成功 -->
                <div v-else-if="status === 'success'" class="mt-8 border border-line bg-white px-5 py-8 text-center">
                    <p class="font-display text-[17px] text-ink">登录成功{{ userName ? `，${userName}` : "" }}</p>
                    <p class="mt-2 text-[12px] text-ink-soft">{{ message }}</p>
                </div>

                <!-- 失败 -->
                <div v-else-if="status === 'error'" class="mt-8">
                    <p class="border border-accent/25 bg-accent-soft/50 px-3 py-2 text-[11.5px] leading-relaxed text-accent">
                        {{ message }}
                    </p>
                    <button class="btn-solid mt-4 w-full py-2.5" type="button" @click="reloadPage">重新加载</button>
                </div>

                <div
                    v-else
                    class="mt-8 flex min-h-[320px] items-center justify-center border border-line bg-white text-[12px] text-ink-faint"
                >
                    正在检查登录状态…
                </div>

                <!-- 本地开发登录：仅在 NWBD_WS_ENVIRONMENT=local 且配置了开发账号时出现 -->
                <div v-if="devLoginAvailable" class="mt-6 border-t border-line pt-4">
                    <p class="text-[10.5px] tracking-[0.1em] text-ink-faint uppercase">Local Only</p>
                    <button class="btn-quiet mt-2 w-full py-2.5" type="button" @click="devLogin">
                        以本地开发账号登录
                    </button>
                    <p class="mt-2 text-[11px] leading-relaxed text-ink-faint">
                        不经过钉钉，仅供本机开发调试。配置项：NWBD_WS_DEV_LOGIN_USERID。
                    </p>
                </div>
            </div>
        </section>
    </div>
</template>

<style scoped>
.grain {
    position: absolute;
    inset: 0;
    background-image: radial-gradient(circle at 18% 22%, rgba(201, 80, 44, 0.28), transparent 42%),
        radial-gradient(circle at 78% 72%, rgba(255, 255, 255, 0.08), transparent 46%);
}
</style>
