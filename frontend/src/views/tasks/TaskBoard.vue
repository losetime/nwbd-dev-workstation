<script setup lang="ts">
/** 我的任务（看板）。调用时机：侧栏「我的任务」；任务以卡片呈现，点开进详情。 */
import { computed, onMounted, onUnmounted, ref } from "vue";
import { useRouter } from "vue-router";

import {
    TASK_TABS,
    listMyTasks,
    priorityTone,
    taskTone,
    type BoardTask,
    type TaskStats,
} from "@/@api/tasks";
import StatCard from "@/components/ui/StatCard.vue";
import { useAuth } from "@/store/auth";
import { fmtDate, relativeTime } from "@/utils/project";

const router = useRouter();
const auth = useAuth();

const loading = ref(true);
const error = ref("");
const items = ref<BoardTask[]>([]);
const stats = ref<TaskStats>({ total: 0, by_status: {}, overdue: 0 });
const status = ref("");
const keyword = ref("");
const scopeAll = ref(false);

const isBoss = computed(() => {
    const u = auth.user.value;
    return u?.job_type === "部门经理";
});

const tabs = computed(() =>
    TASK_TABS.map((tab) => ({
        ...tab,
        count: tab.key === "" ? stats.value.total : stats.value.by_status[tab.key] ?? 0,
    })),
);

async function load(silent = false): Promise<void> {
    if (!silent) loading.value = true;
    error.value = "";
    try {
        const data = await listMyTasks({
            status: status.value || undefined,
            keyword: keyword.value.trim() || undefined,
            scope: scopeAll.value ? "all" : undefined,
        });
        items.value = data.items;
        stats.value = data.stats;
    } catch (err) {
        if (!silent) error.value = err instanceof Error ? err.message : "加载失败";
    } finally {
        if (!silent) loading.value = false;
    }
}

function open(id: string): void {
    void router.push({ name: "task-detail", params: { id } });
}

let timer: number | undefined;

onMounted(() => {
    void load();
    // 后台每 20 秒刷新一次，agent 回写的进展最多 20 秒内出现
    timer = window.setInterval(() => {
        if (!document.hidden) void load(true);
    }, 10000);
});

onUnmounted(() => {
    if (timer) window.clearInterval(timer);
});
</script>

<template>
    <section class="mx-auto max-w-[1180px]">
        <header class="flex flex-wrap items-end justify-between gap-4">
            <div>
                <h2 class="font-display text-[26px] leading-tight tracking-tight text-ink">我的任务</h2>
                <p class="mt-1.5 text-[12.5px] text-ink-faint">
                    {{ scopeAll ? "全部任务" : "我负责的任务" }} · 共 {{ stats.total }} 项
                    <span v-if="stats.overdue" class="text-accent"> · 逾期 {{ stats.overdue }}</span>
                </p>
            </div>
            <label v-if="isBoss" class="flex items-center gap-2 text-[12px] text-ink-soft">
                <input v-model="scopeAll" type="checkbox" @change="load()" />
                看全部任务
            </label>
        </header>

        <!-- 统计 -->
        <div class="mt-5 grid grid-cols-2 gap-3 sm:grid-cols-4">
            <StatCard label="全部" :value="stats.total" />
            <StatCard label="待办" :value="stats.by_status['未开始'] ?? 0" />
            <StatCard label="进行中" :value="stats.by_status['进行中'] ?? 0" />
            <StatCard label="已逾期" :value="stats.overdue" :tone="stats.overdue ? 'accent' : 'default'" />
        </div>

        <!-- 筛选 -->
        <div class="mt-4 flex flex-wrap items-center gap-3 border border-line bg-white px-4 py-3">
            <div class="flex flex-wrap gap-1">
                <button
                    v-for="tab in tabs"
                    :key="tab.key"
                    class="border px-3 py-1.5 text-[12px] transition"
                    :class="status === tab.key ? 'border-ink bg-ink text-white' : 'border-line bg-white text-ink-soft hover:border-ink/30'"
                    type="button"
                    @click="status = tab.key; load()"
                >
                    {{ tab.label }}<span v-if="tab.count" class="ml-1 opacity-60">{{ tab.count }}</span>
                </button>
            </div>
            <input
                v-model="keyword"
                class="field ml-auto !py-1.5 !text-[12.5px] w-[220px]"
                placeholder="搜索标题 / 项目 / 标签"
                @keyup.enter="load()"
            />
        </div>

        <p v-if="error" class="mt-4 border border-accent/30 bg-accent-soft/60 px-4 py-3 text-[12.5px] text-accent">
            {{ error }}
        </p>

        <div v-if="loading" class="mt-6 border border-line bg-white px-5 py-16 text-center text-[12.5px] text-ink-faint">
            正在加载…
        </div>
        <div v-else-if="!items.length" class="mt-6 border border-line bg-white px-5 py-16 text-center">
            <p class="font-display text-[16px] text-ink">没有任务</p>
            <p class="mt-2 text-[12px] text-ink-faint">
                {{ status || keyword ? "换个筛选条件试试。" : "项目经理下发任务后会出现在这里。" }}
            </p>
        </div>

        <div v-else class="mt-4 grid gap-3 lg:grid-cols-2">
            <article
                v-for="t in items"
                :key="t.id"
                class="card-in shell-card cursor-pointer px-5 py-4 transition-shadow hover:shadow-[0_10px_30px_rgba(18,19,22,0.06)]"
                @click="open(t.id)"
            >
                <div class="flex flex-wrap items-center gap-2">
                    <span class="border px-2 py-0.5 text-[10.5px]" :class="taskTone(t.status)">{{ t.status }}</span>
                    <span class="border px-2 py-0.5 text-[10.5px]" :class="priorityTone(t.priority)">{{ t.priority }}</span>
                    <span class="text-[11px] text-ink-faint">{{ t.project_name }}</span>
                    <span v-if="t.stage" class="text-[11px] text-ink-faint">· {{ t.stage }}</span>
                    <span v-if="t.overdue" class="ml-auto text-[11px] text-accent">已逾期</span>
                    <span v-else-if="t.stalled" class="ml-auto border border-amber-500/40 bg-amber-50 px-1.5 py-0.5 text-[10px] text-amber-700">
                        可能已中断
                    </span>
                </div>

                <h3 class="mt-2 font-display text-[16px] leading-snug text-ink">{{ t.name }}</h3>
                <p v-if="t.description" class="mt-1.5 line-clamp-2 text-[12px] leading-relaxed text-ink-soft">
                    {{ t.description }}
                </p>

                <div class="mt-2.5 flex flex-wrap gap-1.5">
                    <span v-for="tag in t.tags.slice(0, 4)" :key="tag" class="border border-line bg-paper px-1.5 py-0.5 text-[10.5px] text-ink-soft">
                        #{{ tag }}
                    </span>
                    <span
                        v-if="t.result && t.status === '已完成'"
                        class="border border-emerald-500/30 bg-emerald-50 px-1.5 py-0.5 text-[10.5px] text-emerald-700"
                    >
                        已提交结果
                    </span>
                </div>

                <div class="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-[11.5px] text-ink-soft">
                    <span>计划 {{ fmtDate(t.plan_due) || "—" }}</span>
                    <span>工时 {{ t.estimate_hours }} / {{ t.actual_hours }} h</span>
                    <span v-if="t.subtask_total">子任务 {{ t.subtask_done }}/{{ t.subtask_total }}</span>
                    <span v-if="t.status === '进行中'" class="text-ink-faint">
                        最后活动 {{ relativeTime(t.last_event_at || t.updated_at) }}
                    </span>
                    <span class="ml-auto text-ink-faint">详情 →</span>
                </div>
            </article>
        </div>
    </section>
</template>



