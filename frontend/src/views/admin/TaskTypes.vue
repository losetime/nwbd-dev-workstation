<script setup lang="ts">
/** 系统管理 · 任务类型。调用时机：维护任务类型、归档阶段/目录、默认技能与启停。 */
import { computed, onMounted, ref } from "vue";

import {
    createTaskType,
    deleteTaskType,
    listSkills,
    listTaskTypes,
    setTaskTypeStatus,
    updateTaskType,
    type SkillItem,
    type TaskTypeOption,
    type TaskTypePayload,
} from "@/@api/tasks";
import ConfirmDialog from "@/components/ui/ConfirmDialog.vue";
import Modal from "@/components/ui/Modal.vue";
import SkillMultiSelect from "@/components/ui/SkillMultiSelect.vue";
import { useToast } from "@/composables/useToast";

const toast = useToast();

const STAGES = [
    "〇 售前与合同签订",
    "一 立项与策划",
    "二 需求分析",
    "三 设计开发",
    "四 试制",
    "五 试验",
    "六 鉴定与定型",
    "七 采购与外包",
    "八 生产",
    "九 检验与放行",
    "十 交付与售后",
    "十一 软件专项（贯穿全周期）",
    "十二 项目收尾与归档",
    "部门公共/其他",
];

const loading = ref(true);
const error = ref("");
const items = ref<TaskTypeOption[]>([]);
const skills = ref<SkillItem[]>([]);
const query = ref("");

const filtered = computed(() => {
    const q = query.value.trim().toLowerCase();
    if (!q) return items.value;
    return items.value.filter((t) =>
        [t.code, t.name, t.group_name, t.archive_stage, t.archive_folder].some((v) =>
            v.toLowerCase().includes(q),
        ),
    );
});

const formOpen = ref(false);
const editing = ref<TaskTypeOption | null>(null);
const saving = ref(false);
const formError = ref("");
const form = ref<TaskTypePayload>(emptyForm());

const deleteOpen = ref(false);
const deleteTarget = ref<TaskTypeOption | null>(null);

function emptyForm(): TaskTypePayload {
    return {
        name: "",
        group_name: "项目任务",
        description: "",
        archive_stage: "二 需求分析",
        archive_folder: "",
        archive_target: "project",
        archive_note: "",
        sort_order: 999,
        skills: [],
    };
}

async function load(): Promise<void> {
    loading.value = true;
    error.value = "";
    try {
        items.value = await listTaskTypes(true);
    } catch (err) {
        error.value = err instanceof Error ? err.message : "加载失败";
    } finally {
        loading.value = false;
    }
}

function openCreate(): void {
    editing.value = null;
    form.value = emptyForm();
    formError.value = "";
    formOpen.value = true;
}

function openEdit(t: TaskTypeOption): void {
    editing.value = t;
    form.value = {
        name: t.name,
        group_name: t.group_name,
        description: t.description,
        archive_stage: t.archive_stage,
        archive_folder: t.archive_folder,
        archive_target: t.archive_target,
        archive_note: t.archive_note,
        sort_order: t.sort_order,
        skills: t.skills.map((s) => ({ name: s.name, version: s.version })),
    };
    formError.value = "";
    formOpen.value = true;
}

async function submit(): Promise<void> {
    const f = form.value;
    if (!f.name.trim()) {
        formError.value = "类型名称必填";
        return;
    }
    if (!f.archive_stage || !f.archive_folder.trim()) {
        formError.value = "归档阶段与归档目录必填";
        return;
    }
    saving.value = true;
    formError.value = "";
    try {
        if (editing.value) {
            await updateTaskType(editing.value.code, f);
            toast.success("任务类型已保存");
        } else {
            await createTaskType(f);
            toast.success("任务类型已创建");
        }
        formOpen.value = false;
        await load();
    } catch (err) {
        formError.value = err instanceof Error ? err.message : "保存失败";
    } finally {
        saving.value = false;
    }
}

async function toggle(t: TaskTypeOption): Promise<void> {
    try {
        await setTaskTypeStatus(t.code, !t.enabled);
        toast.success(t.enabled ? `已停用「${t.name}」` : `已启用「${t.name}」`);
        await load();
    } catch (err) {
        toast.error(err instanceof Error ? err.message : "更新状态失败");
    }
}

function askDelete(t: TaskTypeOption): void {
    deleteTarget.value = t;
    deleteOpen.value = true;
}

async function confirmDelete(): Promise<void> {
    const t = deleteTarget.value;
    if (!t) return;
    try {
        await deleteTaskType(t.code);
        toast.success(`已删除「${t.name}」`);
        await load();
    } catch (err) {
        toast.error(err instanceof Error ? err.message : "删除失败");
    } finally {
        deleteOpen.value = false;
        deleteTarget.value = null;
    }
}

function fmtDateTime(value?: string | null): string {
    if (!value) return "—";
    const d = new Date(value);
    if (Number.isNaN(d.getTime())) return "—";
    const pad = (n: number) => String(n).padStart(2, "0");
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

onMounted(() => {
    void load();
    void listSkills()
        .then((list) => (skills.value = list))
        .catch(() => (skills.value = []));
});
</script>

<template>
    <section class="mx-auto max-w-[1180px]">
        <header class="flex flex-wrap items-end justify-between gap-3">
            <div>
                <h1 class="font-display text-[26px] leading-tight tracking-tight text-ink">任务类型</h1>
                <p class="mt-1 text-[11.5px] text-ink-faint">
                    维护任务类型、归档阶段/目录与默认技能；日常任务与项目任务都从这里下发。
                </p>
            </div>
            <div class="flex flex-wrap items-center gap-2">
                <input
                    v-model="query"
                    class="field !w-[220px] !py-1 !text-[12px]"
                    placeholder="搜索编号 / 名称 / 分类"
                    type="text"
                />
                <button class="btn-solid !py-1.5 text-[12px]" type="button" @click="openCreate">+ 新增类型</button>
            </div>
        </header>

        <p v-if="error" class="mt-3 border border-accent/30 bg-accent-soft/60 px-4 py-2 text-[12px] text-accent">{{ error }}</p>

        <div class="mt-4 shell-card">
            <div v-if="loading" class="px-5 py-16 text-center text-[12.5px] text-ink-faint">正在加载…</div>
            <div v-else-if="!filtered.length" class="px-5 py-16 text-center text-[12.5px] text-ink-faint">没有匹配的任务类型</div>
            <div v-else class="overflow-x-auto">
                <table class="w-full text-[12px]">
                    <thead class="border-b border-line bg-white text-left text-[10.5px] tracking-[0.06em] text-ink-faint uppercase">
                        <tr>
                            <th class="px-4 py-2 font-medium">编号</th>
                            <th class="px-4 py-2 font-medium">组别</th>
                            <th class="px-4 py-2 font-medium">类型名称</th>
                            <th class="px-4 py-2 font-medium">任务说明</th>
                            <th class="px-4 py-2 font-medium">归档阶段 / 目录</th>
                            <th class="px-4 py-2 font-medium">默认技能</th>
                            <th class="px-4 py-2 font-medium">状态</th>
                            <th class="px-4 py-2 font-medium">更新时间</th>
                            <th class="px-4 py-2 font-medium"></th>
                        </tr>
                    </thead>
                    <tbody class="divide-y divide-line">
                        <tr v-for="t in filtered" :key="t.code" class="bg-white hover:bg-paper">
                            <td class="px-4 py-2 font-mono text-[11px] text-ink-faint">{{ t.code }}</td>
                            <td class="px-4 py-2">
                                <span class="border border-line bg-paper px-1.5 py-0.5 text-[10.5px] text-ink-soft">{{ t.group_name }}</span>
                            </td>
                            <td class="px-4 py-2 text-ink">{{ t.name }}</td>
                            <td class="max-w-[240px] px-4 py-2 text-ink-soft">
                                <span class="line-clamp-2">{{ t.description || "—" }}</span>
                            </td>
                            <td class="px-4 py-2 text-ink-soft">
                                <span class="block text-[11px]">{{ t.archive_stage }}</span>
                                <span class="block text-[10.5px] text-ink-faint">
                                    {{ t.archive_target === "public" ? `公共空间 / ${t.archive_folder}` : t.archive_folder }}
                                </span>
                            </td>
                            <td class="px-4 py-2">
                                <div class="flex flex-wrap gap-1">
                                    <span
                                        v-for="s in t.skills"
                                        :key="s.name"
                                        class="border border-line bg-paper px-1.5 py-0.5 font-mono text-[10px] text-ink-soft"
                                    >{{ s.name }}</span>
                                    <span v-if="!t.skills.length" class="text-[11px] text-ink-faint">—</span>
                                </div>
                            </td>
                            <td class="px-4 py-2">
                                <span
                                    class="border px-1.5 py-0.5 text-[10.5px]"
                                    :class="t.enabled ? 'border-emerald-600/30 bg-emerald-50 text-emerald-700' : 'border-line bg-paper text-ink-faint'"
                                >
                                    {{ t.enabled ? "启用" : "停用" }}
                                </span>
                            </td>
                            <td class="px-4 py-2 text-[11px] text-ink-faint">{{ fmtDateTime(t.updated_at) }}</td>
                            <td class="whitespace-nowrap px-4 py-2 text-right">
                                <button class="btn-ghost !py-0.5 text-[11px]" type="button" @click="openEdit(t)">编辑</button>
                                <button class="btn-ghost !py-0.5 text-[11px]" type="button" @click="toggle(t)">
                                    {{ t.enabled ? "停用" : "启用" }}
                                </button>
                                <button class="btn-ghost !py-0.5 text-[11px] !text-accent" type="button" @click="askDelete(t)">删除</button>
                            </td>
                        </tr>
                    </tbody>
                </table>
            </div>
        </div>

        <Modal :open="formOpen" :title="editing ? `编辑 ${editing.code}` : '新增任务类型'" width="680px" @close="formOpen = false">
            <div class="grid gap-3 sm:grid-cols-2">
                <label class="flex flex-col gap-1">
                    <span class="label !mb-0">组别</span>
                    <select v-model="form.group_name" class="field">
                        <option value="日常任务">日常任务</option>
                        <option value="项目任务">项目任务</option>
                    </select>
                </label>
                <label class="flex flex-col gap-1">
                    <span class="label !mb-0">类型名称 <b class="text-accent">*</b></span>
                    <input v-model="form.name" class="field" placeholder="如 需求规格说明书文档" />
                </label>
                <label class="flex flex-col gap-1 sm:col-span-2">
                    <span class="label !mb-0">任务说明</span>
                    <textarea v-model="form.description" class="field h-16 resize-none" placeholder="这类任务要做什么" />
                </label>
                <label class="flex flex-col gap-1">
                    <span class="label !mb-0">归档阶段 <b class="text-accent">*</b></span>
                    <select v-model="form.archive_stage" class="field">
                        <option v-for="s in STAGES" :key="s" :value="s">{{ s }}</option>
                    </select>
                </label>
                <label class="flex flex-col gap-1">
                    <span class="label !mb-0">归档目标</span>
                    <select v-model="form.archive_target" class="field">
                        <option value="project">项目目录</option>
                        <option value="public">公共空间</option>
                    </select>
                </label>
                <label class="flex flex-col gap-1">
                    <span class="label !mb-0">归档目录 <b class="text-accent">*</b></span>
                    <input
                        v-model="form.archive_folder"
                        class="field"
                        :placeholder="form.archive_target === 'public' ? '如 研发部' : '如 需求分析'"
                    />
                </label>
                <label class="flex flex-col gap-1">
                    <span class="label !mb-0">排序</span>
                    <input v-model.number="form.sort_order" class="field" type="number" />
                </label>
                <label class="flex flex-col gap-1 sm:col-span-2">
                    <span class="label !mb-0">归档说明</span>
                    <input v-model="form.archive_note" class="field" placeholder="这类产出通常包含什么" />
                </label>
                <div class="flex flex-col gap-1 sm:col-span-2">
                    <span class="label !mb-0">默认技能（下发时自动带上，任务里不可删除）</span>
                    <SkillMultiSelect v-model="form.skills" :options="skills" placeholder="不绑定（可多选）" />
                </div>
            </div>
            <p class="mt-2 text-[10.5px] leading-relaxed text-ink-faint">
                编号自动生成且不可修改；已下发任务的类型名称与归档目录不受后续修改影响。
            </p>
            <p v-if="formError" class="mt-3 border border-accent/30 bg-accent-soft/60 px-3 py-2 text-[12px] text-accent">{{ formError }}</p>
            <template #footer>
                <button class="btn-quiet" type="button" @click="formOpen = false">取消</button>
                <button class="btn-solid" type="button" :disabled="saving" @click="submit">{{ saving ? "保存中…" : "保存" }}</button>
            </template>
        </Modal>

        <ConfirmDialog
            :open="deleteOpen"
            title="删除任务类型"
            :message="`确定删除「${deleteTarget?.name || ''}」吗？已有任务使用该类型时只能停用。`"
            confirm-text="删除"
            tone="danger"
            @confirm="confirmDelete"
            @cancel="deleteOpen = false"
        />
    </section>
</template>
