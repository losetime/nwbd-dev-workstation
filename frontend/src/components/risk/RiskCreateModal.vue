<script setup lang="ts">
/** 登记风险弹窗。调用时机：风险管理页；也可在项目详情里固定项目使用。 */
import { computed, ref, watch } from "vue";

import { createRisk, getProject } from "@/@api/projects";
import Modal from "@/components/ui/Modal.vue";

import RiskFormFields from "./RiskFormFields.vue";
import { emptyRiskForm, type RiskFormModel, type RiskMemberOption, type RiskProjectOption } from "./types";

const props = defineProps<{
    open: boolean;
    /** 固定项目（项目详情里用）。 */
    projectId?: string;
    /** 可选项目列表（风险管理页用，未传 projectId 时显示选择）。 */
    projectOptions?: RiskProjectOption[];
}>();

const emit = defineEmits<{ close: []; created: [] }>();

const form = ref<RiskFormModel>(emptyRiskForm());
const selectedId = ref("");
const members = ref<RiskMemberOption[]>([]);
const saving = ref(false);
const error = ref("");
const loadingMembers = ref(false);

const currentId = computed(() => props.projectId || selectedId.value);

async function loadMembers(projectID: string): Promise<void> {
    members.value = [];
    if (!projectID) return;
    loadingMembers.value = true;
    try {
        const detail = await getProject(projectID);
        members.value = detail.members
            .filter((m) => m.status === "参与中")
            .map((m) => ({ user_id: m.user_id, name: m.name, project_role: m.project_role }));
    } catch {
        members.value = [];
    } finally {
        loadingMembers.value = false;
    }
}

watch(
    () => props.open,
    (open) => {
        if (!open) return;
        error.value = "";
        selectedId.value = props.projectId || props.projectOptions?.[0]?.id || "";
        form.value = emptyRiskForm();
        void loadMembers(currentId.value);
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

watch(selectedId, (id, prev) => {
    if (!id || id === prev) return;
    form.value = emptyRiskForm();
    void loadMembers(id);
});

async function submit(): Promise<void> {
    const f = form.value;
    if (!currentId.value) {
        error.value = "请选择项目";
        return;
    }
    if (!f.title.trim()) {
        error.value = "标题必填";
        return;
    }
    saving.value = true;
    error.value = "";
    try {
        await createRisk(currentId.value, {
            title: f.title.trim(),
            type: f.type,
            level: f.level,
            impact: f.impact.trim(),
            action: f.action.trim(),
            owner_id: f.owner_id || undefined,
            target_date: f.target_date || undefined,
        });
        emit("created");
        emit("close");
    } catch (err) {
        error.value = err instanceof Error ? err.message : "登记失败";
    } finally {
        saving.value = false;
    }
}
</script>

<template>
    <Modal :open="open" title="登记风险" width="620px" @close="emit('close')">
        <label v-if="!projectId && projectOptions" class="mb-3 flex flex-col gap-1">
            <span class="label !mb-0">项目 <b class="text-accent">*</b></span>
            <select v-model="selectedId" class="field">
                <option value="">请选择项目</option>
                <option v-for="p in projectOptions" :key="p.id" :value="p.id">{{ p.name }}</option>
            </select>
        </label>

        <RiskFormFields :model="form" :members="members" />
        <p v-if="loadingMembers" class="mt-2 text-[10.5px] text-ink-faint">正在读取项目成员…</p>

        <p class="mt-2 text-[10.5px] leading-relaxed text-ink-faint">
            风险只做登记与跟踪，不删除；解决后保留记录。已解决的风险只有部门经理可以重开。
        </p>
        <p v-if="error" class="mt-3 border border-accent/30 bg-accent-soft/60 px-3 py-2 text-[12px] text-accent">{{ error }}</p>
        <template #footer>
            <button class="btn-quiet" type="button" @click="emit('close')">取消</button>
            <button class="btn-solid" type="button" :disabled="saving" @click="submit">{{ saving ? "提交中…" : "登记" }}</button>
        </template>
    </Modal>
</template>
