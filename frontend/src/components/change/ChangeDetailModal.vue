<script setup lang="ts">
/** 变更详情弹窗。调用时机：变更管理列表、项目详情的变更区块。 */
import { ref, watch } from "vue";
import { useRouter } from "vue-router";

import { getChange, type ChangeApproval, type ChangeRecord } from "@/@api/changes";
import Modal from "@/components/ui/Modal.vue";
import { fmtDate } from "@/utils/project";

const props = withDefaults(
    defineProps<{
        open: boolean;
        change: ChangeRecord | null;
        /** 项目详情里不需要「看项目」入口。 */
        showProjectLink?: boolean;
    }>(),
    { showProjectLink: true },
);

const emit = defineEmits<{ close: [] }>();

const router = useRouter();
const detail = ref<ChangeRecord | null>(null);
const approval = ref<ChangeApproval | null>(null);
const error = ref("");

watch(
    () => props.open,
    (open) => {
        if (!open) return;
        detail.value = props.change;
        approval.value = null;
        error.value = "";
        void load();
    },
    { immediate: true },
);

async function load(): Promise<void> {
    if (!props.change) return;
    try {
        const data = await getChange(props.change.id);
        detail.value = data.change;
        approval.value = data.approval;
    } catch (err) {
        error.value = err instanceof Error ? err.message : "读取审批详情失败";
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
    emit("close");
    void router.push(`/projects/${detail.value.project_id}`);
}
</script>

<template>
    <Modal :open="open" title="变更详情" width="640px" @close="emit('close')">
        <template v-if="detail">
            <div class="flex flex-wrap items-center gap-2">
                <span class="font-mono text-[11px] text-ink-faint">{{ detail.code }}</span>
                <span class="text-[13px] text-ink">{{ detail.project_name }}</span>
                <span class="border px-1.5 py-0.5 text-[10.5px]" :class="statusTone(detail.approval_status)">
                    {{ detail.approval_status }}
                </span>
                <button
                    v-if="showProjectLink"
                    class="ml-auto text-[11.5px] text-ink-soft underline"
                    type="button"
                    @click="openProject"
                >
                    看项目 →
                </button>
            </div>
            <dl class="mt-3 divide-y divide-line text-[12px]">
                <div class="flex justify-between py-1.5">
                    <dt class="text-ink-faint">类型 / 来源</dt>
                    <dd class="text-ink">{{ detail.type }} · {{ detail.source }}</dd>
                </div>
                <div class="flex justify-between py-1.5">
                    <dt class="text-ink-faint">申请人 / 日期</dt>
                    <dd class="text-ink">{{ detail.applicant_name }} · {{ fmtDate(detail.applied_on) }}</dd>
                </div>
                <div v-if="detail.budget_before !== null || detail.budget_after !== null" class="flex justify-between py-1.5">
                    <dt class="text-ink-faint">预算（万元）</dt>
                    <dd class="text-ink">{{ detail.budget_before ?? "—" }} → {{ detail.budget_after ?? "—" }}</dd>
                </div>
                <div v-if="detail.plan_due_after" class="flex justify-between py-1.5">
                    <dt class="text-ink-faint">变更后交付日期</dt>
                    <dd class="text-ink">{{ fmtDate(detail.plan_due_after) }}</dd>
                </div>
                <div v-if="detail.material_path" class="flex justify-between py-1.5">
                    <dt class="text-ink-faint">材料位置</dt>
                    <dd class="max-w-[380px] truncate text-ink">{{ detail.material_path }}</dd>
                </div>
            </dl>
            <div class="mt-3 grid gap-2">
                <div>
                    <p class="text-[10.5px] text-ink-faint">变更内容</p>
                    <p class="mt-1 whitespace-pre-wrap text-[12px] leading-6 text-ink-soft">{{ detail.content }}</p>
                </div>
                <div>
                    <p class="text-[10.5px] text-ink-faint">变更原因</p>
                    <p class="mt-1 whitespace-pre-wrap text-[12px] leading-6 text-ink-soft">{{ detail.reason }}</p>
                </div>
                <div>
                    <p class="text-[10.5px] text-ink-faint">影响范围</p>
                    <p class="mt-1 whitespace-pre-wrap text-[12px] leading-6 text-ink-soft">{{ detail.impact }}</p>
                </div>
            </div>

            <p v-if="error" class="mt-3 border border-accent/30 bg-accent-soft/60 px-3 py-2 text-[12px] text-accent">{{ error }}</p>

            <div v-if="approval" class="mt-4 border-t border-line pt-3">
                <div class="flex items-center justify-between text-[11.5px]">
                    <span class="text-ink-faint">钉钉审批 {{ approval.business_id }}</span>
                    <span class="text-ink-soft">{{ approval.display_status }}</span>
                </div>
                <div class="mt-2 space-y-1.5">
                    <p v-for="(op, idx) in approval.operations.slice(-8)" :key="idx" class="text-[11px] leading-relaxed text-ink-soft">
                        <span class="text-ink-faint">{{ fmtDate(op.date) }}</span>
                        <span class="ml-1.5">{{ op.show_name || op.type }}</span>
                        <span v-if="op.result && op.result !== 'NONE'" class="ml-1">
                            · {{ op.result === "AGREE" ? "同意" : op.result === "REFUSE" ? "拒绝" : op.result }}
                        </span>
                        <span v-if="op.remark" class="ml-1 text-ink-faint">「{{ op.remark }}」</span>
                    </p>
                </div>
            </div>
        </template>
        <template #footer>
            <button class="btn-quiet" type="button" @click="emit('close')">关闭</button>
        </template>
    </Modal>
</template>
