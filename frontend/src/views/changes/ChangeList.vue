<script setup lang="ts">
/** 变更管理。调用时机：左侧导航进入；查看各项目的钉钉变更审批与变更内容。 */
import { onMounted, ref } from "vue";
import { useRouter } from "vue-router";

import { getChange, listChanges, type ChangeApproval, type ChangeRecord } from "@/@api/changes";
import { listProjects } from "@/@api/projects";
import ChangeCreateModal from "@/components/change/ChangeCreateModal.vue";
import type { ProjectOption } from "@/components/change/types";
import Modal from "@/components/ui/Modal.vue";
import StatCard from "@/components/ui/StatCard.vue";
import { fmtDate } from "@/utils/project";

const router = useRouter();
const loading = ref(true);
const error = ref("");
const items = ref<ChangeRecord[]>([]);
const stats = ref({ total: 0, pending: 0 });
const status = ref("");
const changeOpen = ref(false);
const projectOptions = ref<ProjectOption[]>([]);
const createError = ref("");

async function openCreate(): Promise<void> {
    createError.value = "";
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
        createError.value = err instanceof Error ? err.message : "读取项目失败";
    }
}

const detailOpen = ref(false);
const detail = ref<ChangeRecord | null>(null);
const detailApproval = ref<ChangeApproval | null>(null);
const detailError = ref("");

async function load(): Promise<void> {
    loading.value = true;
    error.value = "";
    try {
        const data = await listChanges(status.value);
        items.value = data.items;
        stats.value = data.stats;
    } catch (err) {
        error.value = err instanceof Error ? err.message : "加载失败";
    } finally {
        loading.value = false;
    }
}

async function openDetail(c: ChangeRecord): Promise<void> {
    detail.value = c;
    detailApproval.value = null;
    detailError.value = "";
    detailOpen.value = true;
    try {
        const data = await getChange(c.id);
        detail.value = data.change;
        detailApproval.value = data.approval;
    } catch (err) {
        detailError.value = err instanceof Error ? err.message : "读取审批详情失败";
    }
}

function statusTone(s: string): string {
    if (s === "已通过") return "border-emerald-600/30 bg-emerald-50 text-emerald-700";
    if (s === "已否决") return "border-accent/40 bg-accent-soft text-accent";
    if (s === "已撤销") return "border-line bg-paper text-ink-faint";
    return "border-amber-500/40 bg-amber-50 text-amber-700";
}

function openProject(): void {
    if (!detail.value) return;
    void router.push(`/projects/${detail.value.project_id}`);
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
                <button class="btn-solid !py-1.5 text-[12px]" type="button" @click="openCreate">+ 发起变更</button>
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

        <Modal :open="detailOpen" title="变更详情" width="640px" @close="detailOpen = false">
            <template v-if="detail">
                <div class="flex flex-wrap items-center gap-2">
                    <span class="font-mono text-[11px] text-ink-faint">{{ detail.code }}</span>
                    <span class="text-[13px] text-ink">{{ detail.project_name }}</span>
                    <span class="border px-1.5 py-0.5 text-[10.5px]" :class="statusTone(detail.approval_status)">
                        {{ detail.approval_status }}
                    </span>
                    <button class="ml-auto text-[11.5px] text-ink-soft underline" type="button" @click="openProject">看项目 →</button>
                </div>
                <dl class="mt-3 divide-y divide-line text-[12px]">
                    <div class="flex justify-between py-1.5"><dt class="text-ink-faint">类型 / 来源</dt><dd class="text-ink">{{ detail.type }} · {{ detail.source }}</dd></div>
                    <div class="flex justify-between py-1.5"><dt class="text-ink-faint">申请人 / 日期</dt><dd class="text-ink">{{ detail.applicant_name }} · {{ fmtDate(detail.applied_on) }}</dd></div>
                    <div v-if="detail.budget_before !== null || detail.budget_after !== null" class="flex justify-between py-1.5">
                        <dt class="text-ink-faint">预算（万元）</dt>
                        <dd class="text-ink">{{ detail.budget_before ?? "—" }} → {{ detail.budget_after ?? "—" }}</dd>
                    </div>
                    <div v-if="detail.plan_due_after" class="flex justify-between py-1.5">
                        <dt class="text-ink-faint">变更后交付日期</dt><dd class="text-ink">{{ fmtDate(detail.plan_due_after) }}</dd>
                    </div>
                    <div v-if="detail.material_path" class="flex justify-between py-1.5">
                        <dt class="text-ink-faint">材料位置</dt><dd class="max-w-[380px] truncate text-ink">{{ detail.material_path }}</dd>
                    </div>
                </dl>
                <div class="mt-3 grid gap-2">
                    <div><p class="text-[10.5px] text-ink-faint">变更内容</p><p class="mt-1 whitespace-pre-wrap text-[12px] leading-6 text-ink-soft">{{ detail.content }}</p></div>
                    <div><p class="text-[10.5px] text-ink-faint">变更原因</p><p class="mt-1 whitespace-pre-wrap text-[12px] leading-6 text-ink-soft">{{ detail.reason }}</p></div>
                    <div><p class="text-[10.5px] text-ink-faint">影响范围</p><p class="mt-1 whitespace-pre-wrap text-[12px] leading-6 text-ink-soft">{{ detail.impact }}</p></div>
                </div>

                <p v-if="detailError" class="mt-3 border border-accent/30 bg-accent-soft/60 px-3 py-2 text-[12px] text-accent">{{ detailError }}</p>

                <div v-if="detailApproval" class="mt-4 border-t border-line pt-3">
                    <div class="flex items-center justify-between text-[11.5px]">
                        <span class="text-ink-faint">钉钉审批 {{ detailApproval.business_id }}</span>
                        <span class="text-ink-soft">{{ detailApproval.display_status }}</span>
                    </div>
                    <div class="mt-2 space-y-1.5">
                        <p v-for="(op, idx) in detailApproval.operations.slice(-8)" :key="idx" class="text-[11px] leading-relaxed text-ink-soft">
                            <span class="text-ink-faint">{{ fmtDate(op.date) }}</span>
                            <span class="ml-1.5">{{ op.show_name || op.type }}</span>
                            <span v-if="op.result && op.result !== 'NONE'" class="ml-1">· {{ op.result === "AGREE" ? "同意" : op.result === "REFUSE" ? "拒绝" : op.result }}</span>
                            <span v-if="op.remark" class="ml-1 text-ink-faint">「{{ op.remark }}」</span>
                        </p>
                    </div>
                </div>
            </template>
            <template #footer>
                <button class="btn-quiet" type="button" @click="detailOpen = false">关闭</button>
            </template>
        </Modal>

        <ChangeCreateModal
            :open="changeOpen"
            :project-options="projectOptions"
            @close="changeOpen = false"
            @created="load()"
        />
    </section>
</template>
