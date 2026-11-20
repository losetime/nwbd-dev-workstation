<script setup lang="ts">
/** 发起变更弹窗。调用时机：项目详情 / 变更管理页；内部包 Modal + 变更表单。 */
import { computed, ref, watch } from "vue";

import { createChange } from "@/@api/changes";
import Modal from "@/components/ui/Modal.vue";
import { fmtDate } from "@/utils/project";

import ChangeFormFields from "./ChangeFormFields.vue";
import { emptyChangeForm, type ChangeFormModel, type ProjectOption } from "./types";

const props = defineProps<{
    open: boolean;
    /** 固定项目（项目详情里用）。 */
    projectId?: string;
    /** 可选项目列表（变更管理页里用，未传 projectId 时显示选择）。 */
    projectOptions?: ProjectOption[];
    initialBudget?: string;
    initialPlanEnd?: string;
}>();

const emit = defineEmits<{ close: []; created: [] }>();

const form = ref<ChangeFormModel>(emptyChangeForm());
const selectedId = ref("");
const saving = ref(false);
const error = ref("");

const currentId = computed(() => props.projectId || selectedId.value);

function prefill(projectID: string): void {
    const option = props.projectOptions?.find((p) => p.id === projectID);
    form.value = emptyChangeForm(
        option ? (option.budget === null ? "" : String(option.budget)) : (props.initialBudget ?? ""),
        option ? fmtDate(option.plan_end) : (props.initialPlanEnd ?? ""),
    );
}

watch(
    () => props.open,
    (open) => {
        if (!open) return;
        error.value = "";
        selectedId.value = props.projectId || props.projectOptions?.[0]?.id || "";
        prefill(currentId.value);
    },
    { immediate: true },
);

// 项目列表可能比弹窗晚到：到了之后默认选中第一个
watch(
    () => props.projectOptions,
    (list) => {
        if (props.projectId || selectedId.value || !list?.length) return;
        selectedId.value = list[0].id;
    },
);

watch(selectedId, (id) => {
    if (id) prefill(id);
});

async function submit(): Promise<void> {
    const f = form.value;
    if (!currentId.value) {
        error.value = "请选择项目";
        return;
    }
    if (!f.content.trim() || !f.reason.trim() || !f.impact.trim()) {
        error.value = "变更内容、原因、影响范围必填";
        return;
    }
    saving.value = true;
    error.value = "";
    try {
        await createChange(currentId.value, {
            type: f.type,
            source: f.source,
            content: f.content.trim(),
            reason: f.reason.trim(),
            impact: f.impact.trim(),
            budget_before: f.budget_before === "" ? undefined : Number(f.budget_before),
            budget_after: f.budget_after === "" ? undefined : Number(f.budget_after),
            plan_due_after: f.plan_due_after || undefined,
            material_path: f.material_path.trim(),
        });
        emit("created");
        emit("close");
    } catch (err) {
        error.value = err instanceof Error ? err.message : "发起变更失败";
    } finally {
        saving.value = false;
    }
}
</script>

<template>
    <Modal :open="open" title="发起变更" width="640px" @close="emit('close')">
        <label v-if="!projectId && projectOptions" class="mb-3 flex flex-col gap-1">
            <span class="label !mb-0">项目 <b class="text-accent">*</b></span>
            <select v-model="selectedId" class="field">
                <option value="">请选择项目</option>
                <option v-for="p in projectOptions" :key="p.id" :value="p.id">{{ p.name }}</option>
            </select>
        </label>

        <ChangeFormFields :model="form" />

        <p class="mt-2 text-[10.5px] leading-relaxed text-ink-faint">
            提交后会在钉钉发起「05 项目变更申请」，审批通过后系统自动把「变更后预算 / 交付日期」写回项目。
        </p>
        <p v-if="error" class="mt-3 border border-accent/30 bg-accent-soft/60 px-3 py-2 text-[12px] text-accent">{{ error }}</p>
        <template #footer>
            <button class="btn-quiet" type="button" @click="emit('close')">取消</button>
            <button class="btn-solid" type="button" :disabled="saving" @click="submit">{{ saving ? "提交中…" : "提交变更" }}</button>
        </template>
    </Modal>
</template>

