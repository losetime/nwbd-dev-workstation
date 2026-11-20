<script setup lang="ts">
/** 变更管理。调用时机：左侧导航进入；查看各项目的钉钉变更审批与变更内容。 */
import { onMounted, ref } from "vue";

import { listChanges, type ChangeRecord } from "@/@api/changes";
import { listProjects } from "@/@api/projects";
import ChangeCreateModal from "@/components/change/ChangeCreateModal.vue";
import ChangeDetailModal from "@/components/change/ChangeDetailModal.vue";
import type { ProjectOption } from "@/components/change/types";
import StatCard from "@/components/ui/StatCard.vue";
import { useToast } from "@/composables/useToast";
import { fmtDate } from "@/utils/project";

const loading = ref(true);
const error = ref("");
const items = ref<ChangeRecord[]>([]);
const stats = ref({ total: 0, pending: 0 });
const status = ref("");
const changeOpen = ref(false);
const projectOptions = ref<ProjectOption[]>([]);
const toast = useToast();
/** 只有部门经理和项目经理能发起变更。 */
const canCreate = ref(false);

async function openCreate(): Promise<void> {
    if (!canCreate.value) return;
    changeOpen.value = true;
    if (projectOptions.value.length) return;
    try {
        const list = await listProjects({ scope: "managed" });
        projectOptions.value = list.map((p) => ({
            id: p.id,
            name: p.name,
            budget: p.budget,
            plan_end: p.plan_end,
        }));
    } catch (err) {
        toast.error(err instanceof Error ? err.message : "读取项目失败");
    }
}

const detailOpen = ref(false);
const detailItem = ref<ChangeRecord | null>(null);

async function load(): Promise<void> {
    loading.value = true;
    error.value = "";
    try {
        const data = await listChanges(status.value);
        items.value = data.items;
        stats.value = data.stats;
        canCreate.value = data.can_create;
    } catch (err) {
        error.value = err instanceof Error ? err.message : "加载失败";
    } finally {
        loading.value = false;
    }
}

function openDetail(c: ChangeRecord): void {
    detailItem.value = c;
    detailOpen.value = true;
}

function statusTone(s: string): string {
    if (s === "已通过") return "border-emerald-600/30 bg-emerald-50 text-emerald-700";
    if (s === "已否决") return "border-accent/40 bg-accent-soft text-accent";
    if (s === "已撤销") return "border-line bg-paper text-ink-faint";
    return "border-amber-500/40 bg-amber-50 text-amber-700";
}

onMounted(() => {
    void load();
});
</script>

<template>
    <section class="mx-auto max-w-[1180px]">
        <header class="flex flex-wrap items-end justify-between gap-3">
            <div>
                <h1 class="font-display text-[26px] leading-tight tracking-tight text-ink">变更管理</h1>
                <p class="mt-1 text-[11.5px] text-ink-faint">变更在钉钉走「05 项目变更申请」，这里查看进度与内容。</p>
            </div>
            <div class="flex items-center gap-2">
                <select v-model="status" class="field !w-[130px] !py-1 !text-[12px]" @change="load">
                    <option value="">全部状态</option>
                    <option value="审批中">审批中</option>
                    <option value="已通过">已通过</option>
                    <option value="已否决">已否决</option>
                    <option value="已撤销">已撤销</option>
                </select>
                <button v-if="canCreate" class="btn-solid !py-1.5 text-[12px]" type="button" @click="openCreate">+ 发起变更</button>
            </div>
        </header>

        <div class="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-3">
            <StatCard label="审批中" :value="stats.pending" :tone="stats.pending ? 'warn' : 'default'" />
            <StatCard label="合计" :value="stats.total" />
        </div>

        <p v-if="error" class="mt-3 border border-accent/30 bg-accent-soft/60 px-4 py-2 text-[12px] text-accent">{{ error }}</p>

        <div class="mt-4 shell-card">
            <div v-if="loading" class="px-5 py-16 text-center text-[12.5px] text-ink-faint">正在加载…</div>
            <div v-else-if="!items.length" class="px-5 py-16 text-center text-[12.5px] text-ink-faint">暂无变更</div>
            <div v-else class="overflow-x-auto">
                <table class="w-full text-[12px]">
                    <thead class="border-b border-line bg-white text-left text-[10.5px] tracking-[0.06em] text-ink-faint uppercase">
                        <tr>
                            <th class="px-4 py-2 font-medium">项目</th>
                            <th class="px-4 py-2 font-medium">变更</th>
                            <th class="px-4 py-2 font-medium">来源</th>
                            <th class="px-4 py-2 font-medium">状态</th>
                            <th class="px-4 py-2 font-medium">申请人</th>
                            <th class="px-4 py-2 font-medium">日期</th>
                        </tr>
                    </thead>
                    <tbody class="divide-y divide-line">
                        <tr v-for="c in items" :key="c.id" class="cursor-pointer bg-white hover:bg-paper" @click="openDetail(c)">
                            <td class="px-4 py-2 text-ink-soft">{{ c.project_name }}</td>
                            <td class="px-4 py-2 text-ink">
                                <span class="mr-1.5 font-mono text-[10.5px] text-ink-faint">{{ c.code }}</span>
                                <span class="border border-line bg-paper px-1.5 py-0.5 text-[10.5px] text-ink-soft">{{ c.type }}</span>
                                <span class="ml-1.5">{{ c.content }}</span>
                            </td>
                            <td class="px-4 py-2 text-ink-soft">{{ c.source }}</td>
                            <td class="px-4 py-2">
                                <span class="border px-1.5 py-0.5 text-[10.5px]" :class="statusTone(c.approval_status)">{{ c.approval_status }}</span>
                            </td>
                            <td class="px-4 py-2 text-ink-soft">{{ c.applicant_name }}</td>
                            <td class="px-4 py-2 text-ink-soft">{{ fmtDate(c.applied_on) || "—" }}</td>
                        </tr>
                    </tbody>
                </table>
            </div>
        </div>

        <ChangeDetailModal
            :open="detailOpen"
            :change="detailItem"
            @close="detailOpen = false"
        />

        <ChangeCreateModal
            :open="changeOpen"
            :project-options="projectOptions"
            @close="changeOpen = false"
            @created="load()"
        />
    </section>
</template>
