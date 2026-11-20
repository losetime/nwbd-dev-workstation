<script setup lang="ts">
/** 我要帮助（困难上报）。调用时机：左侧导航进入；成员上报困难，项目经理/部门经理转风险或归档。 */
import { computed, onMounted, ref } from "vue";

import {
    archiveIssue,
    createIssue,
    listIssues,
    type Issue,
    type IssueScope,
    type IssueSeverity,
    type IssueStats,
} from "@/@api/issues";
import { getProject, listProjects, type Project } from "@/@api/projects";
import Modal from "@/components/ui/Modal.vue";
import { useToast } from "@/composables/useToast";
import { useAuth } from "@/store/auth";
import { fmtDate } from "@/utils/project";

const auth = useAuth();
const isBoss = computed(() => auth.user.value?.job_type === "部门经理");
const isPm = computed(() => auth.user.value?.job_type === "项目经理");
/** 项目经理 / 部门经理可以处理困难；部门经理不上报，所以没有「我上报的」。 */
const hasTodo = computed(() => isBoss.value || isPm.value);
/** 部门经理暂时也保留「我上报的」页签（后续再定）。 */
const hasMine = computed(() => true);

const toast = useToast();
const scope = ref<IssueScope>(hasTodo.value ? "todo" : "mine");
const loading = ref(true);
const error = ref("");
const items = ref<Issue[]>([]);
const stats = ref<IssueStats>({ total: 0, pending: 0, risked: 0, archived: 0 });

const modal = ref("");
const saving = ref(false);
const modalError = ref("");

const projects = ref<Project[]>([]);
const projectTasks = ref<{ id: string; name: string }[]>([]);
const reportForm = ref({
    project_id: "",
    task_id: "",
    title: "",
    description: "",
    severity: "中" as IssueSeverity,
});

const target = ref<Issue | null>(null);
const archiveForm = ref({ handling: "" });
/** 列表行点击后的详情。 */
const detail = ref<Issue | null>(null);

const emptyText = computed(() => {
    switch (scope.value) {
        case "todo":
            return "没有待你处理的困难";
        case "handled":
            return "你还没有处理过困难";
        case "archived":
            return "还没有已归档的困难（只限当前参与中的项目）";
        default:
            return "你还没有上报过困难";
    }
});

async function load(): Promise<void> {
    loading.value = true;
    error.value = "";
    try {
        const data = await listIssues(scope.value);
        items.value = data.items;
        stats.value = data.stats;
    } catch (err) {
        error.value = err instanceof Error ? err.message : "加载失败";
    } finally {
        loading.value = false;
    }
}

function switchScope(next: IssueScope): void {
    scope.value = next;
    void load();
}

function closeModal(): void {
    modal.value = "";
    modalError.value = "";
    saving.value = false;
}

async function openReport(): Promise<void> {
    reportForm.value = { project_id: "", task_id: "", title: "", description: "", severity: "中" };
    projectTasks.value = [];
    modal.value = "report";
    modalError.value = "";
    if (!projects.value.length) {
        try {
            projects.value = await listProjects();
        } catch (err) {
            modalError.value = err instanceof Error ? err.message : "读取项目失败";
        }
    }
}

async function onProjectChange(): Promise<void> {
    reportForm.value.task_id = "";
    projectTasks.value = [];
    if (!reportForm.value.project_id) return;
    try {
        const detail = await getProject(reportForm.value.project_id);
        projectTasks.value = detail.tasks.map((t) => ({ id: t.id, name: t.name }));
    } catch {
        projectTasks.value = [];
    }
}

async function submitReport(): Promise<void> {
    const f = reportForm.value;
    if (!f.project_id || !f.title.trim()) {
        modalError.value = "项目与标题必填";
        return;
    }
    saving.value = true;
    modalError.value = "";
    try {
        await createIssue({
            project_id: f.project_id,
            task_id: f.task_id || undefined,
            title: f.title.trim(),
            description: f.description.trim(),
            severity: f.severity,
        });
        toast.success("困难已上报，等项目经理处理");
        scope.value = "mine";
        closeModal();
        await load();
    } catch (err) {
        modalError.value = err instanceof Error ? err.message : "上报失败";
    } finally {
        saving.value = false;
    }
}

function openDetail(it: Issue): void {
    detail.value = it;
}

function openArchive(it: Issue): void {
    target.value = it;
    archiveForm.value = { handling: "" };
    modal.value = "archive";
    modalError.value = "";
}

async function submitArchive(): Promise<void> {
    if (!target.value) return;
    if (!archiveForm.value.handling.trim()) {
        modalError.value = "请填写处理说明";
        return;
    }
    saving.value = true;
    modalError.value = "";
    try {
        await archiveIssue(target.value.id, archiveForm.value.handling.trim());
        toast.success("已归档");
        closeModal();
        await load();
    } catch (err) {
        modalError.value = err instanceof Error ? err.message : "归档失败";
    } finally {
        saving.value = false;
    }
}

/** 时间显示成 11-20 14:30。 */
function fmtTime(value?: string | null): string {
    if (!value) return "";
    const d = new Date(value);
    if (Number.isNaN(d.getTime())) return "";
    const pad = (n: number) => String(n).padStart(2, "0");
    return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function severityTone(level: string): string {
    if (level === "高") return "border-accent/40 bg-accent-soft text-accent";
    if (level === "中") return "border-amber-500/40 bg-amber-50 text-amber-700";
    return "border-line bg-paper text-ink-soft";
}

onMounted(() => {
    // 只有处理入口的人默认看「待我处理」，只有上报入口的人默认看「我上报的」
    if (hasTodo.value) scope.value = "todo";
    else scope.value = "mine";
    void load();
});
</script>

<template>
    <section class="mx-auto max-w-[1180px]">
        <header class="flex flex-wrap items-end justify-between gap-3">
            <div>
                <h1 class="font-display text-[26px] leading-tight tracking-tight text-ink">我要帮助</h1>
                <p class="mt-1 text-[11.5px] text-ink-faint">
                    卡住就上报；项目经理看过后给出处理说明并归档。
                </p>
            </div>
            <div class="flex flex-wrap items-center gap-2">
                <button class="btn-solid !py-1.5 text-[12px]" type="button" @click="openReport">
                    + 上报困难
                </button>
            </div>
        </header>

        <div class="mt-4 flex items-center gap-1 border-b border-line">
            <button
                v-if="hasTodo"
                class="px-3 py-2 text-[12.5px] transition-colors"
                :class="scope === 'todo' ? 'border-b-2 border-accent text-ink' : 'text-ink-soft hover:text-ink'"
                type="button"
                @click="switchScope('todo')"
            >
                待我处理
                <span v-if="scope === 'todo' && stats.pending" class="ml-1 text-[10.5px] text-accent">{{ stats.pending }}</span>
            </button>
            <button
                v-if="hasMine"
                class="px-3 py-2 text-[12.5px] transition-colors"
                :class="scope === 'mine' ? 'border-b-2 border-accent text-ink' : 'text-ink-soft hover:text-ink'"
                type="button"
                @click="switchScope('mine')"
            >
                我上报的
            </button>
            <button
                class="px-3 py-2 text-[12.5px] transition-colors"
                :class="scope === 'handled' ? 'border-b-2 border-accent text-ink' : 'text-ink-soft hover:text-ink'"
                type="button"
                @click="switchScope('handled')"
            >
                我处理的
            </button>
            <button
                class="px-3 py-2 text-[12.5px] transition-colors"
                :class="scope === 'archived' ? 'border-b-2 border-accent text-ink' : 'text-ink-soft hover:text-ink'"
                type="button"
                @click="switchScope('archived')"
            >
                已归档
                <span class="ml-1 text-[10px] text-ink-faint" title="只限当前参与中的项目">ⓘ</span>
            </button>
        </div>

        <p v-if="error" class="mt-3 border border-accent/30 bg-accent-soft/60 px-4 py-2 text-[12px] text-accent">{{ error }}</p>

        <div class="mt-4 shell-card">
            <div v-if="loading" class="px-5 py-16 text-center text-[12.5px] text-ink-faint">正在加载…</div>
            <div v-else-if="!items.length" class="px-5 py-16 text-center text-[12.5px] text-ink-faint">
                {{ emptyText }}
            </div>
            <div v-else class="overflow-x-auto">
                <table class="w-full text-[12px]">
                    <thead class="border-b border-line bg-white text-left text-[10.5px] tracking-[0.06em] text-ink-faint uppercase">
                        <tr>
                            <th class="px-4 py-2 font-medium">项目</th>
                            <th class="px-4 py-2 font-medium">标题</th>
                            <th class="px-4 py-2 font-medium">级别</th>
                            <th class="px-4 py-2 font-medium">状态</th>
                            <th class="px-4 py-2 font-medium">上报人</th>
                            <th class="px-4 py-2 font-medium">上报日期</th>
                            <th v-if="scope === 'todo'" class="px-4 py-2 font-medium"></th>
                        </tr>
                    </thead>
                    <tbody class="divide-y divide-line">
                        <tr
                            v-for="it in items"
                            :key="it.id"
                            class="cursor-pointer bg-white transition-colors hover:bg-paper"
                            @click="openDetail(it)"
                        >
                            <td class="px-4 py-2 text-ink-soft">{{ it.project_name }}</td>
                            <td class="px-4 py-2 text-ink">{{ it.title }}</td>
                            <td class="px-4 py-2">
                                <span class="border px-1.5 py-0.5 text-[10.5px]" :class="severityTone(it.severity)">{{ it.severity }}</span>
                            </td>
                            <td class="px-4 py-2" :class="it.status === '待处理' ? 'text-ink-soft' : 'text-ink-faint'">{{ it.status }}</td>
                            <td class="px-4 py-2 text-ink-soft">{{ it.reporter_name }}</td>
                            <td class="px-4 py-2 text-ink-soft">{{ fmtDate(it.reported_on) || "—" }}</td>
                            <td v-if="scope === 'todo'" class="whitespace-nowrap px-4 py-2 text-right">
                                <template v-if="it.status === '待处理'">
                                    <button class="btn-ghost !py-0.5 text-[11px]" type="button" @click.stop="openArchive(it)">归档</button>
                                </template>
                                <span v-else class="text-[10.5px] text-ink-faint">—</span>
                            </td>
                        </tr>
                    </tbody>
                </table>
            </div>
        </div>

        <!-- 困难详情 -->
        <Modal :open="!!detail" title="困难详情" width="600px" @close="detail = null">
            <template v-if="detail">
                <p class="font-mono text-[11px] text-ink-faint">{{ detail.code }}</p>
                <h3 class="mt-1 font-display text-[16px] leading-snug text-ink">{{ detail.title }}</h3>

                <dl class="mt-4 grid gap-x-6 gap-y-2.5 text-[12.5px] sm:grid-cols-2">
                    <div class="flex justify-between gap-3">
                        <dt class="shrink-0 text-ink-faint">项目</dt>
                        <dd class="min-w-0 text-right text-ink">{{ detail.project_name }}</dd>
                    </div>
                    <div class="flex justify-between gap-3">
                        <dt class="shrink-0 text-ink-faint">关联任务</dt>
                        <dd class="min-w-0 text-right text-ink">{{ detail.task_name || "—" }}</dd>
                    </div>
                    <div class="flex justify-between gap-3">
                        <dt class="shrink-0 text-ink-faint">级别</dt>
                        <dd class="text-right">
                            <span class="border px-1.5 py-0.5 text-[10.5px]" :class="severityTone(detail.severity)">{{ detail.severity }}</span>
                        </dd>
                    </div>
                    <div class="flex justify-between gap-3">
                        <dt class="shrink-0 text-ink-faint">状态</dt>
                        <dd class="text-right text-ink">{{ detail.status }}</dd>
                    </div>
                    <div class="flex justify-between gap-3">
                        <dt class="shrink-0 text-ink-faint">上报人</dt>
                        <dd class="text-right text-ink">{{ detail.reporter_name || "—" }}</dd>
                    </div>
                    <div class="flex justify-between gap-3">
                        <dt class="shrink-0 text-ink-faint">上报日期</dt>
                        <dd class="text-right text-ink">{{ fmtDate(detail.reported_on) || "—" }}</dd>
                    </div>
                    <div v-if="detail.handled_at" class="flex justify-between gap-3">
                        <dt class="shrink-0 text-ink-faint">归档人</dt>
                        <dd class="text-right text-ink">{{ detail.handled_by_name || "—" }}</dd>
                    </div>
                    <div v-if="detail.handled_at" class="flex justify-between gap-3">
                        <dt class="shrink-0 text-ink-faint">归档时间</dt>
                        <dd class="text-right text-ink">{{ fmtTime(detail.handled_at) }}</dd>
                    </div>
                </dl>

                <div class="mt-4 border-t border-line pt-3">
                    <p class="text-[10.5px] tracking-wide text-ink-faint">详细情况</p>
                    <p class="mt-1 whitespace-pre-line text-[12.5px] leading-relaxed text-ink-soft">{{ detail.description || "—" }}</p>
                </div>
                <div v-if="detail.handling" class="mt-3 border-t border-line pt-3">
                    <p class="text-[10.5px] tracking-wide text-ink-faint">处理说明</p>
                    <p class="mt-1 whitespace-pre-line text-[12.5px] leading-relaxed text-ink-soft">{{ detail.handling }}</p>
                </div>
            </template>
            <template #footer>
                <button class="btn-quiet" type="button" @click="detail = null">关闭</button>
            </template>
        </Modal>

        <!-- 上报困难 -->
        <Modal :open="modal === 'report'" title="上报困难" width="600px" @close="closeModal">
            <div class="grid gap-3 sm:grid-cols-2">
                <label class="flex flex-col gap-1">
                    <span class="label !mb-0">项目 <b class="text-accent">*</b></span>
                    <select v-model="reportForm.project_id" class="field" @change="onProjectChange">
                        <option value="">请选择（我参与中的项目）</option>
                        <option v-for="p in projects" :key="p.id" :value="p.id">{{ p.name }}</option>
                    </select>
                </label>
                <label class="flex flex-col gap-1">
                    <span class="label !mb-0">关联任务</span>
                    <select v-model="reportForm.task_id" class="field">
                        <option value="">不关联（项目级困难）</option>
                        <option v-for="t in projectTasks" :key="t.id" :value="t.id">{{ t.name }}</option>
                    </select>
                </label>
                <label class="flex flex-col gap-1 sm:col-span-2">
                    <span class="label !mb-0">标题 <b class="text-accent">*</b></span>
                    <input v-model="reportForm.title" class="field" placeholder="如 第三方接口联调卡住，缺少对方测试环境" />
                </label>
                <label class="flex flex-col gap-1">
                    <span class="label !mb-0">级别</span>
                    <select v-model="reportForm.severity" class="field">
                        <option value="高">高</option>
                        <option value="中">中</option>
                        <option value="低">低</option>
                    </select>
                </label>
                <label class="flex flex-col gap-1 sm:col-span-2">
                    <span class="label !mb-0">详细情况</span>
                    <textarea v-model="reportForm.description" class="field h-20 resize-none" placeholder="卡在哪、需要什么支持、已经试过什么" />
                </label>
            </div>
            <p v-if="modalError" class="mt-3 border border-accent/30 bg-accent-soft/60 px-3 py-2 text-[12px] text-accent">{{ modalError }}</p>
            <template #footer>
                <button class="btn-quiet" type="button" @click="closeModal">取消</button>
                <button class="btn-solid" type="button" :disabled="saving" @click="submitReport">{{ saving ? "提交中…" : "上报" }}</button>
            </template>
        </Modal>

        <!-- 归档 -->
        <Modal :open="modal === 'archive'" title="归档困难" width="560px" @close="closeModal">
            <p class="text-[11px] text-ink-faint">{{ target?.code }} · {{ target?.title }}</p>
            <label class="mt-3 flex flex-col gap-1">
                <span class="label !mb-0">处理说明 <b class="text-accent">*</b></span>
                <textarea v-model="archiveForm.handling" class="field h-24 resize-none" placeholder="怎么解决的 / 为什么不构成风险" />
            </label>
            <p v-if="modalError" class="mt-3 border border-accent/30 bg-accent-soft/60 px-3 py-2 text-[12px] text-accent">{{ modalError }}</p>
            <template #footer>
                <button class="btn-quiet" type="button" @click="closeModal">取消</button>
                <button class="btn-solid" type="button" :disabled="saving" @click="submitArchive">{{ saving ? "处理中…" : "归档" }}</button>
            </template>
        </Modal>
    </section>
</template>
