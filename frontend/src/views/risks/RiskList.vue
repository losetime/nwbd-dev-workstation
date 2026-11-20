<script setup lang="ts">
/** 风险管理。调用时机：左侧导航进入；跨项目查看风险，部门经理看全部，其他人看自己参与的项目。 */
import { onMounted, ref } from "vue";
import { useRouter } from "vue-router";

import { listAllRisks, listProjects, type RiskRow, type RiskStats } from "@/@api/projects";
import RiskCreateModal from "@/components/risk/RiskCreateModal.vue";
import type { RiskProjectOption } from "@/components/risk/types";
import Modal from "@/components/ui/Modal.vue";
import StatCard from "@/components/ui/StatCard.vue";
import { useToast } from "@/composables/useToast";
import { fmtDate } from "@/utils/project";

const router = useRouter();

const loading = ref(true);
const error = ref("");
const items = ref<RiskRow[]>([]);
const stats = ref<RiskStats>({ total: 0, open: 0, high: 0, overdue: 0, resolved: 0 });

const status = ref("未解决");
const level = ref("");
const keyword = ref("");

const detail = ref<RiskRow | null>(null);

/** 只有部门经理和项目经理能登记风险。 */
const canCreate = ref(false);
const createOpen = ref(false);
const projectOptions = ref<RiskProjectOption[]>([]);
const toast = useToast();

async function load(): Promise<void> {
    loading.value = true;
    error.value = "";
    try {
        const data = await listAllRisks({
            status: status.value || undefined,
            level: level.value || undefined,
            keyword: keyword.value.trim() || undefined,
        });
        items.value = data.items;
        stats.value = data.stats;
        canCreate.value = data.can_create;
    } catch (err) {
        error.value = err instanceof Error ? err.message : "加载失败";
    } finally {
        loading.value = false;
    }
}

async function openCreate(): Promise<void> {
    if (!canCreate.value) return;
    createOpen.value = true;
    if (projectOptions.value.length) return;
    try {
        const list = await listProjects({ scope: "managed" });
        projectOptions.value = list.map((p) => ({ id: p.id, name: p.name }));
    } catch (err) {
        toast.error(err instanceof Error ? err.message : "读取项目失败");
    }
}

function openDetail(r: RiskRow): void {
    detail.value = r;
}

function openProject(): void {
    if (!detail.value) return;
    void router.push({ path: `/projects/${detail.value.project_id}`, query: { focus: "risk" } });
}

function levelTone(l: string): string {
    if (l === "高") return "border-accent/40 bg-accent-soft text-accent";
    if (l === "中") return "border-amber-500/40 bg-amber-50 text-amber-700";
    return "border-line bg-paper text-ink-soft";
}

onMounted(() => {
    void load();
});
</script>

<template>
    <section class="mx-auto max-w-[1180px]">
        <header class="flex flex-wrap items-end justify-between gap-3">
            <div>
                <h1 class="font-display text-[26px] leading-tight tracking-tight text-ink">风险管理</h1>
                <p class="mt-1 text-[11.5px] text-ink-faint">跨项目风险总览，点任意一行查看详情。</p>
            </div>
            <div class="flex flex-wrap items-center gap-2">
                <select v-model="status" class="field !w-[120px] !py-1 !text-[12px]" @change="load">
                    <option value="未解决">未解决</option>
                    <option value="">全部</option>
                    <option value="监控中">监控中</option>
                    <option value="已发生">已发生</option>
                    <option value="已解决">已解决</option>
                </select>
                <select v-model="level" class="field !w-[100px] !py-1 !text-[12px]" @change="load">
                    <option value="">全部等级</option>
                    <option value="高">高</option>
                    <option value="中">中</option>
                    <option value="低">低</option>
                </select>
                <input
                    v-model="keyword"
                    class="field !w-[200px] !py-1 !text-[12px]"
                    placeholder="搜索标题 / 编号 / 项目"
                    type="text"
                    @keyup.enter="load"
                />
                <button class="btn-quiet !py-1 text-[12px]" type="button" @click="load">查询</button>
                <button v-if="canCreate" class="btn-solid !py-1.5 text-[12px]" type="button" @click="openCreate">+ 登记风险</button>
            </div>
        </header>

        <!-- 统计卡片（按数据范围整体统计，不受筛选影响） -->
        <div class="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-4">
            <StatCard label="未解决" :value="stats.open" :tone="stats.open ? 'warn' : 'default'" />
            <StatCard label="高等级" :value="stats.high" :tone="stats.high ? 'accent' : 'default'" />
            <StatCard label="已超期" :value="stats.overdue" :tone="stats.overdue ? 'accent' : 'default'" />
            <StatCard label="已解决" :value="stats.resolved" />
        </div>

        <p v-if="error" class="mt-3 border border-accent/30 bg-accent-soft/60 px-4 py-2 text-[12px] text-accent">{{ error }}</p>

        <div class="mt-4 shell-card">
            <div v-if="loading" class="px-5 py-16 text-center text-[12.5px] text-ink-faint">正在加载…</div>
            <div v-else-if="!items.length" class="px-5 py-16 text-center text-[12.5px] text-ink-faint">
                当前筛选下没有风险
            </div>
            <div v-else class="overflow-x-auto">
                <table class="w-full text-[12px]">
                    <thead class="border-b border-line bg-white text-left text-[10.5px] tracking-[0.06em] text-ink-faint uppercase">
                        <tr>
                            <th class="px-4 py-2 font-medium">项目</th>
                            <th class="px-4 py-2 font-medium">标题</th>
                            <th class="px-4 py-2 font-medium">等级</th>
                            <th class="px-4 py-2 font-medium">类型</th>
                            <th class="px-4 py-2 font-medium">状态</th>
                            <th class="px-4 py-2 font-medium">责任人</th>
                            <th class="px-4 py-2 font-medium">目标 / 解决</th>
                        </tr>
                    </thead>
                    <tbody class="divide-y divide-line">
                        <tr
                            v-for="r in items"
                            :key="r.id"
                            class="cursor-pointer bg-white transition-colors hover:bg-paper"
                            @click="openDetail(r)"
                        >
                            <td class="px-4 py-2 text-ink-soft">
                                <span class="block text-[10.5px] text-ink-faint">{{ r.project_code }}</span>
                                {{ r.project_name }}
                            </td>
                            <td class="px-4 py-2 text-ink">{{ r.title }}</td>
                            <td class="px-4 py-2">
                                <span class="border px-1.5 py-0.5 text-[10.5px]" :class="levelTone(r.level)">{{ r.level }}</span>
                            </td>
                            <td class="px-4 py-2 text-ink-soft">{{ r.type }}</td>
                            <td class="px-4 py-2" :class="r.status === '已解决' ? 'text-ink-faint' : 'text-ink-soft'">
                                {{ r.status }}
                                <span v-if="r.overdue" class="ml-1 text-[10.5px] text-accent">已超期</span>
                            </td>
                            <td class="px-4 py-2 text-ink-soft">{{ r.owner_name || "—" }}</td>
                            <td class="px-4 py-2 text-ink-soft">
                                {{ fmtDate(r.target_date) || "—" }}
                                <template v-if="r.resolved_date"> / {{ fmtDate(r.resolved_date) }}</template>
                            </td>
                        </tr>
                    </tbody>
                </table>
            </div>
        </div>

        <p class="mt-2 text-[10.5px] leading-relaxed text-ink-faint">
            点任意一行查看详情；登记与后续处理在项目详情「⑤ 风险管理」或本页「+ 登记风险」。
        </p>

        <!-- 风险详情 -->
        <Modal :open="!!detail" title="风险详情" width="640px" @close="detail = null">
            <template v-if="detail">
                <p class="text-[11px] text-ink-faint">
                    <span class="font-mono">{{ detail.code }}</span>
                </p>
                <h3 class="mt-1 font-display text-[16px] leading-snug text-ink">{{ detail.title }}</h3>

                <dl class="mt-4 grid gap-x-6 gap-y-2.5 text-[12.5px] sm:grid-cols-2">
                    <div class="flex justify-between gap-3">
                        <dt class="shrink-0 text-ink-faint">项目</dt>
                        <dd class="min-w-0 text-right text-ink">{{ detail.project_name }}</dd>
                    </div>
                    <div class="flex justify-between gap-3">
                        <dt class="shrink-0 text-ink-faint">等级</dt>
                        <dd class="text-right">
                            <span class="border px-1.5 py-0.5 text-[10.5px]" :class="levelTone(detail.level)">{{ detail.level }}</span>
                        </dd>
                    </div>
                    <div class="flex justify-between gap-3">
                        <dt class="shrink-0 text-ink-faint">类型</dt>
                        <dd class="text-right text-ink">{{ detail.type || "—" }}</dd>
                    </div>
                    <div class="flex justify-between gap-3">
                        <dt class="shrink-0 text-ink-faint">状态</dt>
                        <dd class="text-right text-ink">
                            {{ detail.status }}
                            <span v-if="detail.overdue" class="ml-1 text-[10.5px] text-accent">已超期</span>
                        </dd>
                    </div>
                    <div class="flex justify-between gap-3">
                        <dt class="shrink-0 text-ink-faint">责任人</dt>
                        <dd class="text-right text-ink">{{ detail.owner_name || "—" }}</dd>
                    </div>
                    <div class="flex justify-between gap-3">
                        <dt class="shrink-0 text-ink-faint">登记日期</dt>
                        <dd class="text-right text-ink">{{ fmtDate(detail.identified_on) || "—" }}</dd>
                    </div>
                    <div class="flex justify-between gap-3">
                        <dt class="shrink-0 text-ink-faint">目标日期</dt>
                        <dd class="text-right text-ink">{{ fmtDate(detail.target_date) || "—" }}</dd>
                    </div>
                    <div class="flex justify-between gap-3">
                        <dt class="shrink-0 text-ink-faint">解决日期</dt>
                        <dd class="text-right text-ink">{{ fmtDate(detail.resolved_date) || "—" }}</dd>
                    </div>
                </dl>

                <div class="mt-4 border-t border-line pt-3">
                    <p class="text-[10.5px] tracking-wide text-ink-faint">影响</p>
                    <p class="mt-1 whitespace-pre-line text-[12.5px] leading-relaxed text-ink-soft">{{ detail.impact || "—" }}</p>
                </div>
                <div class="mt-3 border-t border-line pt-3">
                    <p class="text-[10.5px] tracking-wide text-ink-faint">描述</p>
                    <p class="mt-1 whitespace-pre-line text-[12.5px] leading-relaxed text-ink-soft">{{ detail.description || "—" }}</p>
                </div>
                <div class="mt-3 border-t border-line pt-3">
                    <p class="text-[10.5px] tracking-wide text-ink-faint">应对措施</p>
                    <p class="mt-1 whitespace-pre-line text-[12.5px] leading-relaxed text-ink-soft">{{ detail.action || "—" }}</p>
                </div>
            </template>
            <template #footer>
                <button class="btn-quiet" type="button" @click="detail = null">关闭</button>
                <button class="btn-ghost" type="button" @click="openProject">打开项目</button>
            </template>
        </Modal>

        <RiskCreateModal
            :open="createOpen"
            :project-options="projectOptions"
            @close="createOpen = false"
            @created="load()"
        />
    </section>
</template>
