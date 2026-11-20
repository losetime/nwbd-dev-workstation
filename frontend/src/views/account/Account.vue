<script setup lang="ts">
/** 用户信息。调用时机：顶部用户菜单进入；展示登录身份与访问令牌。 */
import { computed, ref } from "vue";
import { useRouter } from "vue-router";

import { useToast } from "@/composables/useToast";
import { useAuth } from "@/store/auth";

const router = useRouter();
const auth = useAuth();

const copied = ref(false);
const toast = useToast();

const token = computed(() => auth.token.value);
const user = computed(() => auth.user.value);
const roleLabel = computed(() => {
    switch (user.value?.role) {
        case "admin":
            return "管理员";
        case "sub_admin":
            return "子管理员";
        default:
            return "普通成员";
    }
});

/** 复制令牌。 */
async function copy(): Promise<void> {
    if (!token.value) return;
    try {
        await navigator.clipboard.writeText(token.value);
        copied.value = true;
        window.setTimeout(() => (copied.value = false), 2000);
    } catch {
        toast.error("浏览器不允许写剪贴板，请手动选中复制");
    }
}

async function logout(): Promise<void> {
    await auth.logout();
    await router.push({ name: "login" });
}
</script>

<template>
    <section class="card-in mx-auto max-w-[1080px]">
        <header class="flex flex-wrap items-end justify-between gap-4">
            <div>
                <h2 class="font-display text-[24px] leading-tight tracking-tight text-ink">用户信息</h2>
                <p class="mt-1.5 text-[13px] text-ink-faint">登录身份与脚本访问令牌。</p>
            </div>
            <button class="btn-quiet" type="button" @click="logout">退出登录</button>
        </header>

        <div class="mt-6 shell-card max-w-[620px]">
            <dl class="divide-y divide-line text-[12.5px]">
                <div class="flex gap-3 px-5 py-3">
                    <dt class="w-24 shrink-0 text-ink-faint">姓名</dt>
                    <dd class="min-w-0 flex-1 text-ink">{{ user?.name || "—" }}</dd>
                </div>
                <div class="flex gap-3 px-5 py-3">
                    <dt class="w-24 shrink-0 text-ink-faint">部门</dt>
                    <dd class="min-w-0 flex-1">{{ user?.dept || "—" }}</dd>
                </div>
                <div class="flex gap-3 px-5 py-3">
                    <dt class="w-24 shrink-0 text-ink-faint">职位</dt>
                    <dd class="min-w-0 flex-1">{{ user?.title || "—" }}</dd>
                </div>
                <div class="flex gap-3 px-5 py-3">
                    <dt class="w-24 shrink-0 text-ink-faint">角色</dt>
                    <dd class="min-w-0 flex-1">{{ roleLabel }}</dd>
                </div>
                <div class="flex gap-3 px-5 py-3">
                    <dt class="w-24 shrink-0 text-ink-faint">钉钉 userId</dt>
                    <dd class="min-w-0 flex-1 truncate font-mono text-[12px] text-ink-soft">
                        {{ user?.userid || "—" }}
                    </dd>
                </div>
            </dl>

            <div class="border-t border-line px-5 py-4">
                <div class="flex items-center justify-between gap-3">
                    <p class="text-[11px] tracking-[0.08em] text-ink-faint uppercase">访问令牌</p>
                    <button v-if="token" class="btn-ghost !py-1 text-[11px]" type="button" @click="copy">
                        {{ copied ? "已复制" : "复制" }}
                    </button>
                </div>

                <template v-if="token">
                    <pre class="mt-2 break-all whitespace-pre-wrap bg-paper px-3 py-2 font-mono text-[11.5px] text-ink">{{ token }}</pre>
                    <p class="mt-2 text-[11px] leading-relaxed text-ink-faint">
                        登录时自动生成，有效期 30 天。命令行调用时放请求头：
                        <span class="font-mono">Authorization: Bearer &lt;令牌&gt;</span>
                    </p>
                </template>
                <template v-else>
                    <p class="mt-2 text-[11.5px] leading-relaxed text-ink-faint">
                        本浏览器没有令牌记录（令牌只在登录时下发，服务端只保存摘要）。
                        退出后重新扫码登录，即可在这里看到新令牌。
                    </p>
                </template>
            </div>
        </div>
    </section>
</template>
