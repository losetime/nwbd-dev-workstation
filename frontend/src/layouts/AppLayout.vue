<script setup lang="ts">
/** 应用布局。调用时机：除登录页外的所有页面；左侧导航按界面权限过滤。 */
import { computed, onMounted, ref } from "vue";
import { RouterLink, RouterView, useRoute, useRouter } from "vue-router";

import { actAs, getAuthConfig, listDevPeople, type DevPerson } from "@/@api/dev";import { useAuth } from "@/store/auth";

const auth = useAuth();
const route = useRoute();
const router = useRouter();

const menuOpen = ref(false);
const devActAs = ref(false);
const devPeople = ref<DevPerson[]>([]);
const switching = ref(false);

const initials = computed(() => (auth.user.value?.name ?? "?").slice(0, 1));
const pageTitle = computed(() => (route.meta.title as string | undefined) ?? "工作台");
const jobLabel = computed(() => auth.user.value?.job_type || "成员");

const navItems = computed(() => {
    const items = [
        { key: "myproj", label: "我的项目", to: "/" },
        { key: "mytask", label: "我的任务", to: "/tasks" },
        { key: "myrisk", label: "风险管理", to: "/risks" },
        { key: "mychange", label: "变更管理", to: "/changes" },
        { key: "help", label: "我要帮助", to: "/help" },
    ];
    if (auth.can("sysadmin")) {
        items.push({ key: "sysadmin", label: "系统管理", to: "/admin/users" });
    }
    return items.filter((item) => auth.can(item.key));
});

function isActive(to: string): boolean {
    if (to === "/") return route.path === "/" || route.path.startsWith("/projects");
    return route.path.startsWith(to);
}

async function logout(): Promise<void> {
    menuOpen.value = false;
    await auth.logout();
    await router.push({ name: "login" });
}

function goAccount(): void {
    menuOpen.value = false;
    void router.push({ name: "account" });
}

onMounted(() => {
    void auth.ensure();
    void loadDevOptions();
});

/** 读取测试开关与可切换人员（默认关闭）。 */
async function loadDevOptions(): Promise<void> {
    try {
        const cfg = await getAuthConfig();
        devActAs.value = Boolean(cfg.devActAs);
        if (devActAs.value) {
            devPeople.value = await listDevPeople();
        }
    } catch {
        devActAs.value = false;
    }
}

/** 切换测试身份：换令牌后整页重载，确保路由与权限重新计算。 */
async function onSwitchRole(event: Event): Promise<void> {
    const select = event.target as HTMLSelectElement;
    const id = select.value;
    if (!id || switching.value) return;
    switching.value = true;
    try {
        const result = await actAs(id);
        // 把新令牌写进本地，用户信息页可以直接复制给 agent 用
        auth.setSession(result.user, result.token);
        window.location.assign("/");
    } catch {
        switching.value = false;
        select.value = "";
    }
}
</script>

<template>
    <div class="min-h-screen bg-paper">
        <aside class="fixed inset-y-0 left-0 z-30 hidden w-[224px] flex-col border-r border-line bg-white lg:flex">
            <RouterLink to="/" class="flex items-center gap-2.5 border-b border-line px-5 py-5">
                <span class="flex h-8 w-8 items-center justify-center bg-ink font-display text-[15px] text-white">
                    N
                </span>
                <span class="font-display text-[14px] leading-tight tracking-tight text-ink">
                    诺维研发<br />项目管理
                </span>
            </RouterLink>

            <nav class="flex-1 py-4">
                <p class="px-5 pb-2 text-[10.5px] tracking-[0.14em] text-ink-faint uppercase">Workspace</p>
                <RouterLink
                    v-for="item in navItems"
                    :key="item.key"
                    :to="item.to"
                    class="nav-item"
                    :class="{ 'nav-item-active': isActive(item.to) }"
                >
                    {{ item.label }}
                </RouterLink>

                <p class="mt-6 px-5 pb-2 text-[10.5px] tracking-[0.14em] text-ink-faint uppercase">Coming</p>
                <p class="px-5 py-2 text-[11.5px] leading-relaxed text-ink-faint">
                    待办 / 任务 / 工时<br />后续接入。
                </p>
            </nav>

            <div class="border-t border-line px-5 py-3 text-[10.5px] text-ink-faint">
                nwbd-dev-workstation · local
            </div>
        </aside>

        <div class="lg:pl-[224px]">
            <header class="sticky top-0 z-20 border-b border-line bg-paper/85 backdrop-blur-xl">
                <div class="flex h-[64px] items-center gap-3 px-5 lg:px-8">
                    <RouterLink to="/" class="flex shrink-0 items-center gap-2 lg:hidden">
                        <span class="flex h-7 w-7 items-center justify-center bg-ink font-display text-[13px] text-white">
                            N
                        </span>
                    </RouterLink>

                    <h1 class="min-w-0 truncate font-display text-[16px] tracking-tight text-ink">{{ pageTitle }}</h1>

                    <div class="relative ml-auto shrink-0">
                        <select
                            v-if="devActAs"
                            class="field mr-2 !w-[150px] !py-1 !text-[11.5px]"
                            :disabled="switching"
                            @change="onSwitchRole"
                        >
                            <option value="">测试身份…</option>
                            <option v-for="p in devPeople" :key="p.id" :value="p.id">
                                {{ p.name }} · {{ p.job_type }}
                            </option>
                        </select>
                    </div>

                    <div class="relative shrink-0">
                        <RouterLink
                            v-if="!auth.user.value"
                            to="/login"
                            class="flex items-center gap-1.5 border border-line bg-white/70 px-3 py-1.5 text-[12px] text-ink-soft transition hover:border-ink/25 hover:text-ink"
                        >
                            登录
                        </RouterLink>

                        <template v-else>
                            <button
                                class="flex items-center gap-2 border border-line bg-white/70 py-1 pl-1 pr-2 transition hover:border-ink/25 sm:pr-3"
                                type="button"
                                @click="menuOpen = !menuOpen"
                            >
                                <span class="flex h-7 w-7 items-center justify-center bg-ink text-[11px] font-medium text-white">
                                    {{ initials }}
                                </span>
                                <span class="hidden text-left text-[12px] leading-tight text-ink-soft sm:block">
                                    <span class="block text-ink">{{ auth.user.value?.name }}</span>
                                    <span class="block text-[10.5px] text-ink-faint">{{ jobLabel }}</span>
                                </span>
                            </button>

                            <div
                                v-if="menuOpen"
                                class="absolute right-0 top-11 w-44 border border-line bg-white py-1 shadow-[0_12px_32px_rgba(18,19,22,0.10)]"
                            >
                                <div class="px-3 py-2 text-[11px] text-ink-faint">
                                    {{ auth.user.value?.dept || "未同步部门" }}
                                </div>
                                <button class="menu-item" type="button" @click="goAccount">用户信息</button>
                                <button class="menu-item text-accent" type="button" @click="logout">退出登录</button>
                            </div>
                        </template>
                    </div>
                </div>
            </header>

            <main class="px-5 py-7 lg:px-8 lg:py-9">
                <RouterView />
            </main>
        </div>

        <div v-if="menuOpen" class="fixed inset-0 z-10" @click="menuOpen = false"></div>
    </div>
</template>
