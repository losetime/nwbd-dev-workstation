<script setup lang="ts">
/** 任务详情。调用时机：从任务卡片进入；描述 / 验收 / 技能执行 / 产出 / 工时确认 / 时间线。 */
import { computed, onMounted, onUnmounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";

import {
    artifactURL,
    completeMyTask,
    getMyTask,
    getTimesheet,
    listSkills,
    priorityTone,
    saveTimesheet,
    startTask,
    taskTone,
    updateMyTask,
    updateMyTaskStatus,
    uploadArtifact,
    type BoardTask,
    type SkillItem,
    type TaskArtifact,
    type TaskEvent,
    type TimesheetInfo,
} from "@/@api/tasks";
import { getProject } from "@/@api/projects";
import { getTaskApproval, type TaskApproval } from "@/@api/approvals";
import TaskFormFields from "@/components/task/TaskFormFields.vue";
import { acceptanceText, emptyTaskForm, parseAcceptance, type MemberOption, type TaskFormModel } from "@/components/task/types";
import Modal from "@/components/ui/Modal.vue";
import { fmtDate, relativeTime } from "@/utils/project";

const route = useRoute();
const router = useRouter();

const loading = ref(true);
const error = ref("");
const notice = ref("");
const task = ref<BoardTask | null>(null);
const events = ref<TaskEvent[]>([]);
const artifacts = ref<TaskArtifact[]>([]);
const approval = ref<TaskApproval | null>(null);
const canEdit = ref(false);
const canOperate = ref(false);
const isOwner = ref(false);

const modal = ref("");
const saving = ref(false);
const modalError = ref("");

const completeForm = ref({ summary: "" });
const editForm = ref<TaskFormModel>(emptyTaskForm());
const skills = ref<SkillItem[]>([]);
const projectStages = ref<string[]>([]);
const projectMembers = ref<MemberOption[]>([]);

const timesheet = ref<TimesheetInfo | null>(null);
const tsRows = ref<{ date: string; suggested: number; already: number; hours: string }[]>([]);
const uploading = ref(false);

const taskId = computed(() => String(route.params.id));
const tsTotal = computed(() =>
    tsRows.value.reduce((sum, row) => sum + (Number(row.hours) || 0), 0),
);
const isRunning = computed(() => task.value?.status === "进行中");

/** 产出时间线：只取上传产出的事件。 */
const artifactEvents = computed(() => events.value.filter((e) => e.type === "artifact"));

/** 时间显示成 11-20 14:30。 */
function fmtTime(value?: string | null): string {
    if (!value) return "";
    const d = new Date(value);
    if (Number.isNaN(d.getTime())) return "";
    const pad = (n: number) => String(n).padStart(2, "0");
    return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

// 时间线按「第 N 次执行」分组（事件按时间倒序，分组保持倒序）
const eventGroups = computed(() => {
    const groups: { attempt: number; items: TaskEvent[] }[] = [];
    for (const e of events.value) {
        const last = groups[groups.length - 1];
        if (last && last.attempt === e.attempt) last.items.push(e);
        else groups.push({ attempt: e.attempt, items: [e] });
    }
    return groups;
});

async function load(silent = false): Promise<void> {
    if (!silent) loading.value = true;
    error.value = "";
    try {
        const data = await getMyTask(taskId.value);
        task.value = data.task;
        events.value = data.events;
        artifacts.value = data.artifacts;
        canEdit.value = data.can_edit;
        canOperate.value = data.can_operate;
        isOwner.value = data.is_owner;
        if (data.can_operate && !silent) await loadTimesheet();
        if (data.task.task_type === "项目策划" && data.task.approval_instance_id) {
            try {
                approval.value = await getTaskApproval(taskId.value);
            } catch {
                approval.value = null;
            }
        } else {
            approval.value = null;
        }
    } catch (err) {
        if (!silent) error.value = err instanceof Error ? err.message : "加载失败";
    } finally {
        if (!silent) loading.value = false;
    }
}

async function loadTimesheet(): Promise<void> {
    try {
        const info = await getTimesheet(taskId.value);
        timesheet.value = info;
        tsRows.value = info.suggestions.map((s) => ({
            date: s.date,
            suggested: s.suggested,
            already: s.already,
            hours: s.default > 0 ? String(s.default) : "",
        }));
    } catch {
        timesheet.value = null;
    }
}

async function changeStatus(status: string): Promise<void> {
    notice.value = "";
    try {
        await updateMyTaskStatus(taskId.value, status);
        await load();
    } catch (err) {
        notice.value = err instanceof Error ? err.message : "更新状态失败";
    }
}

// 进展只由 agent 通过 skill 记录，页面不提供手动录入

/** 开始执行：第一次开始、延期后继续、已完成后重新打开都走这里。 */
async function startRun(): Promise<void> {
    notice.value = "";
    try {
        const result = await startTask(taskId.value);
        notice.value = result.reopen
            ? `任务已重新打开，开始第 ${result.attempt} 次执行`
            : `已开始第 ${result.attempt} 次执行`;
        await load();
    } catch (err) {
        notice.value = err instanceof Error ? err.message : "开始执行失败";
    }
}

/** 上传产出。 */
async function onUploadArtifact(event: Event): Promise<void> {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    uploading.value = true;
    notice.value = "";
    try {
        await uploadArtifact(taskId.value, file);
        notice.value = `产出已上传：${file.name}`;
        await load();
    } catch (err) {
        notice.value = err instanceof Error ? err.message : "上传失败";
    } finally {
        uploading.value = false;
        input.value = "";
    }
}

function pickArtifact(): void {
    (document.getElementById("artifact-file") as HTMLInputElement | null)?.click();
}

/** 确认工时。 */
async function confirmTimesheet(): Promise<void> {
    const items = tsRows.value
        .map((row) => ({ date: row.date, hours: Number(row.hours) || 0 }))
        .filter((row) => row.hours > 0);
    if (!items.length) {
        notice.value = "没有填写工时";
        return;
    }
    saving.value = true;
    try {
        const result = await saveTimesheet(taskId.value, items);
        notice.value = `已确认 ${result.total} 小时（${result.days} 天）`;
        await load();
    } catch (err) {
        notice.value = err instanceof Error ? err.message : "确认工时失败";
    } finally {
        saving.value = false;
    }
}

function openComplete(): void {
    completeForm.value = { summary: "" };
    modal.value = "complete";
}

async function submitComplete(): Promise<void> {
    if (!completeForm.value.summary.trim()) {
        modalError.value = "写一句结果摘要";
        return;
    }
    saving.value = true;
    modalError.value = "";
    try {
        await completeMyTask(taskId.value, completeForm.value.summary.trim(), []);
        modal.value = "";
        notice.value = "结果已提交，任务完成";
        await load();
    } catch (err) {
        modalError.value = err instanceof Error ? err.message : "提交失败";
    } finally {
        saving.value = false;
    }
}

async function openEdit(): Promise<void> {
    const t = task.value;
    if (!t) return;
    editForm.value = {
        task_type_code: t.task_type_code ?? "",
        contract_instance_id: "",
        source: "中标合同",
        name: t.name,
        description: t.description,
        acceptance: acceptanceText(t.acceptance),
        priority: t.priority,
        skills: t.skills ?? [],
        stage: t.stage,
        owner_id: t.owner_id,
        plan_start: fmtDate(t.plan_start),
        plan_due: fmtDate(t.plan_due),
        estimate_hours: t.estimate_hours ? String(t.estimate_hours) : "",
    };
    modal.value = "edit";
    if (!skills.value.length) {
        try {
            skills.value = await listSkills();
        } catch {
            skills.value = [];
        }
    }
    // 阶段下拉与负责人下拉来自项目详情（参与中成员）
    try {
        const detail = await getProject(t.project_id);
        projectStages.value = detail.milestones.map((m) => m.stage);
        projectMembers.value = detail.members
            .filter((m) => m.status === "参与中")
            .map((m) => ({ user_id: m.user_id, name: m.name, project_role: m.project_role }));
    } catch {
        projectStages.value = [];
        projectMembers.value = [];
    }
}

async function submitEdit(): Promise<void> {
    if (!editForm.value.name.trim()) {
        modalError.value = "任务名称不能为空";
        return;
    }
    saving.value = true;
    modalError.value = "";
    try {
        await updateMyTask(taskId.value, {
            name: editForm.value.name.trim(),
            description: editForm.value.description.trim(),
            acceptance: parseAcceptance(editForm.value.acceptance),
            priority: editForm.value.priority,
            skills: editForm.value.skills,
            stage: editForm.value.stage,
            owner_id: editForm.value.owner_id || undefined,
            plan_start: editForm.value.plan_start || undefined,
            plan_due: editForm.value.plan_due || undefined,
            estimate_hours: editForm.value.estimate_hours === "" ? undefined : Number(editForm.value.estimate_hours),
        });
        modal.value = "";
        notice.value = "任务已更新";
        await load();
    } catch (err) {
        modalError.value = err instanceof Error ? err.message : "保存失败";
    } finally {
        saving.value = false;
    }
}

function formatTime(iso: string): string {
    const d = new Date(iso);
    return `${d.getMonth() + 1}/${d.getDate()} ${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
}

function sizeText(size: number): string {
    return size > 1024 * 1024 ? `${(size / 1024 / 1024).toFixed(1)} MB` : size > 1024 ? `${(size / 1024).toFixed(0)} KB` : `${size} B`;
}

let timer: number | undefined;

onMounted(() => {
    void load();
    // 执行中每 10 秒刷新一次，agent 回写的进展自动出现
    timer = window.setInterval(() => {
        if (!document.hidden && task.value?.status === "进行中") void load(true);
    }, 10000);
});

onUnmounted(() => {
    if (timer) window.clearInterval(timer);
});
</script>

<template>
    <section class="mx-auto max-w-[1080px]">
        <button class="btn-ghost -ml-2 text-[12px]" type="button" @click="router.back()">← 返回</button>

        <p v-if="error" class="mt-4 border border-accent/30 bg-accent-soft/60 px-4 py-3 text-[12.5px] text-accent">
            {{ error }}
        </p>
        <div v-else-if="loading" class="mt-4 border border-line bg-white px-5 py-16 text-center text-[12.5px] text-ink-faint">
            正在加载…
        </div>

        <template v-else-if="task">
            <header class="mt-3 border border-line bg-white px-6 py-5">
                <h2 class="font-display text-[24px] leading-tight tracking-tight text-ink">{{ task.name }}</h2>

                <div class="mt-4 flex flex-wrap gap-2 border-t border-line pt-4">
                    <button
                        v-if="isOwner && task.status !== '进行中'"
                        class="btn-solid !py-1.5 text-[12px]"
                        type="button"
                        @click="startRun"
                    >
                        {{ task.status === "已完成" || task.status === "已取消" ? "重新打开并开始" : "开始任务" }}
                    </button>
                    <button
                        v-if="isOwner && (task.status === '进行中' || task.status === '未开始')"
                        class="btn-quiet !py-1.5 text-[12px]"
                        type="button"
                        @click="changeStatus('已延期')"
                    >
                        标记延期
                    </button>
                    <button v-if="isOwner && task.status !== '已完成' && task.status !== '已取消'" class="btn-quiet !py-1.5 text-[12px]" type="button" @click="openComplete">
                        完成并提交结果
                    </button>
                    <button v-if="canEdit" class="btn-quiet !py-1.5 text-[12px]" type="button" @click="openEdit">编辑</button>
                    <RouterLink class="btn-ghost !py-1.5 text-[12px]" :to="`/projects/${task.project_id}`">看项目 →</RouterLink>
                </div>
            </header>

            <p v-if="notice" class="mt-3 border border-line bg-white px-4 py-2 text-[12px] text-ink-soft">{{ notice }}</p>

            <div class="mt-4 grid gap-4 lg:grid-cols-[1.6fr_1fr]">
                <div class="space-y-4">
                    <section class="shell-card px-5 py-4">
                        <h3 class="font-display text-[15px] text-ink">任务描述</h3>
                        <p v-if="task.description" class="mt-2 whitespace-pre-wrap text-[12.5px] leading-6 text-ink-soft">{{ task.description }}</p>
                        <p v-else class="mt-2 text-[12px] text-ink-faint">未填写描述</p>
                    </section>

                    <section class="shell-card px-5 py-4">
                        <h3 class="font-display text-[15px] text-ink">验收标准</h3>
                        <ul v-if="task.acceptance.length" class="mt-2 list-disc space-y-1 pl-5 text-[12.5px] leading-6 text-ink-soft">
                            <li v-for="item in task.acceptance" :key="item">{{ item }}</li>
                        </ul>
                        <p v-else class="mt-2 text-[12px] text-ink-faint">未设置验收标准</p>
                    </section>

                    <!-- 工时确认 -->
                    <section v-if="canOperate" class="shell-card px-5 py-4">
                        <div class="flex flex-wrap items-center justify-between gap-2">
                            <h3 class="font-display text-[15px] text-ink">工时确认</h3>
                            <span class="text-[10.5px] text-ink-faint">
                                预估 {{ task.estimate_hours }} h · 已填 {{ task.actual_hours }} h
                                <template v-if="timesheet && timesheet.rate"> · 费率 {{ timesheet.rate }} 元/时</template>
                            </span>
                        </div>

                        <p v-if="timesheet?.reason" class="mt-2 border border-line bg-paper px-3 py-2 text-[11.5px] leading-relaxed text-ink-faint">
                            {{ timesheet.reason }}
                        </p>

                        <template v-else-if="tsRows.length">
                            <p class="mt-2 text-[10.5px] leading-relaxed text-ink-faint">
                                系统按任务计划区间与预估工时自动生成每日建议（只摊到工作日）；填 0 跳过该天，确认后写入工时记录。
                            </p>
                            <div class="mt-2 max-h-[280px] space-y-1 overflow-y-auto pr-1">
                                <div v-for="row in tsRows" :key="row.date" class="flex items-center gap-2 text-[12px]">
                                    <span class="w-[104px] shrink-0 text-ink-soft">{{ row.date }}</span>
                                    <span class="w-[64px] shrink-0 text-[10.5px] text-ink-faint">建议 {{ row.suggested }}</span>
                                    <span class="w-[64px] shrink-0 text-[10.5px] text-ink-faint">已报 {{ row.already }}</span>
                                    <input v-model="row.hours" class="field !py-1 !text-[12px] w-[80px]" type="number" step="0.5" min="0" />
                                    <span class="text-[10.5px] text-ink-faint">h</span>
                                </div>
                            </div>
                            <div class="mt-2 flex items-center justify-between border-t border-line pt-2">
                                <span class="text-[12px] text-ink-soft">本次合计 <b class="text-ink">{{ tsTotal }}</b> h</span>
                                <button class="btn-solid !py-1.5 text-[12px]" type="button" :disabled="saving" @click="confirmTimesheet">
                                    {{ saving ? "提交中…" : "确认工时" }}
                                </button>
                            </div>
                        </template>
                    </section>

                    <!-- 产出移到右侧 -->
                    <section class="shell-card px-5 py-4">
                        <div class="flex items-center justify-between">
                            <h3 class="font-display text-[15px] text-ink">执行时间线</h3>
                            <span class="text-[10.5px] text-ink-faint">{{ events.length }} 条</span>
                        </div>

                        <div v-if="events.length" class="mt-3 space-y-4">
                            <div v-for="g in eventGroups" :key="g.attempt" class="border-l border-line pl-4">
                                <p class="mb-2 text-[10.5px] font-medium tracking-[0.08em] text-ink-faint uppercase">
                                    {{ g.attempt > 0 ? `第 ${g.attempt} 次执行` : "任务建立" }}
                                </p>
                                <ul class="space-y-3">
                                    <li v-for="e in g.items" :key="e.id" class="relative">
                                        <span class="absolute -left-[21px] top-1.5 h-2 w-2 rounded-full bg-ink/40"></span>
                                        <p class="whitespace-pre-wrap text-[12.5px] leading-6 text-ink-soft">{{ e.message }}</p>
                                        <p class="mt-0.5 text-[10.5px] text-ink-faint">
                                            {{ formatTime(e.at) }}<template v-if="e.actor"> · {{ e.actor }}</template>
                                        </p>
                                    </li>
                                </ul>
                            </div>
                        </div>
                        <p v-else class="mt-3 text-[12px] text-ink-faint">还没有记录</p>
                    </section>

                    <section v-if="task.result" class="shell-card px-5 py-4">
                        <div class="flex items-center justify-between">
                            <h3 class="font-display text-[15px] text-ink">执行结果</h3>
                            <span class="text-[10.5px] text-ink-faint">
                                {{ task.result.agent }}<template v-if="task.result.finished_at"> · {{ fmtDate(task.result.finished_at) }}</template>
                            </span>
                        </div>
                        <pre class="mt-2 whitespace-pre-wrap bg-paper px-3 py-2 text-[12px] leading-6 text-ink-soft">{{ task.result.summary || "（无摘要）" }}</pre>
                        <div v-if="task.result.artifacts.length" class="mt-2 flex flex-wrap gap-1.5">
                            <span v-for="a in task.result.artifacts" :key="a" class="border border-line bg-white px-1.5 py-0.5 font-mono text-[10.5px] text-ink-soft">
                                {{ a }}
                            </span>
                        </div>
                    </section>
                </div>

                <div class="space-y-4">
                    <section class="shell-card px-5 py-4">
                        <h3 class="font-display text-[15px] text-ink">任务信息</h3>
                        <dl class="mt-2 divide-y divide-line text-[12px]">
                            <div class="flex justify-between py-1.5">
                                <dt class="text-ink-faint">状态</dt>
                                <dd>
                                    <span class="border px-2 py-0.5 text-[10.5px]" :class="taskTone(task.status)">{{ task.status }}</span>
                                    <span v-if="task.stalled" class="ml-1 text-[10.5px] text-amber-700">可能已中断</span>
                                    <span v-if="task.overdue" class="ml-1 text-[10.5px] text-accent">已逾期</span>
                                </dd>
                            </div>
                            <div class="flex justify-between py-1.5">
                                <dt class="text-ink-faint">优先级</dt>
                                <dd class="text-ink">{{ task.priority }}</dd>
                            </div>
                            <div v-if="task.task_type === '项目策划'" class="flex justify-between py-1.5">
                                <dt class="text-ink-faint">任务类型</dt>
                                <dd class="text-ink">项目策划（钉钉审批）</dd>
                            </div>
                            <div class="flex justify-between py-1.5">
                                <dt class="text-ink-faint">执行次数</dt>
                                <dd class="text-ink">{{ task.attempt_count > 0 ? `第 ${task.attempt_count} 次` : "—" }}</dd>
                            </div>
                            <div class="flex justify-between py-1.5">
                                <dt class="text-ink-faint">所属项目</dt>
                                <dd class="max-w-[180px] truncate text-ink">{{ task.project_name }}</dd>
                            </div>
                            <div class="flex justify-between py-1.5">
                                <dt class="text-ink-faint">阶段</dt>
                                <dd class="text-ink">{{ task.stage || "—" }}</dd>
                            </div>
                            <div class="flex justify-between py-1.5">
                                <dt class="text-ink-faint">负责人</dt>
                                <dd class="text-ink">{{ task.owner_name }}</dd>
                            </div>
                            <div class="flex justify-between py-1.5">
                                <dt class="text-ink-faint">下发人</dt>
                                <dd class="text-ink">{{ task.creator_name || "—" }}</dd>
                            </div>
                            <div class="flex justify-between py-1.5">
                                <dt class="text-ink-faint">计划</dt>
                                <dd class="text-ink">{{ fmtDate(task.plan_start) || "—" }} ~ {{ fmtDate(task.plan_due) || "—" }}</dd>
                            </div>
                            <div class="flex justify-between py-1.5">
                                <dt class="text-ink-faint">工时</dt>
                                <dd class="text-ink">{{ task.estimate_hours }} / {{ task.actual_hours }} h</dd>
                            </div>
                            <div v-if="task.status === '进行中'" class="flex justify-between py-1.5">
                                <dt class="text-ink-faint">最后活动</dt>
                                <dd class="text-ink">{{ relativeTime(task.last_event_at || task.updated_at) }}</dd>
                            </div>
                            <div class="flex justify-between py-1.5">
                                <dt class="text-ink-faint">更新于</dt>
                                <dd class="text-ink-soft">{{ formatTime(task.updated_at) }}</dd>
                            </div>
                        </dl>
                    </section>

                    <section v-if="task.task_type === '项目策划'" class="shell-card px-5 py-4">
                        <div class="flex flex-wrap items-center justify-between gap-2">
                            <h3 class="font-display text-[15px] text-ink">钉钉审批</h3>
                            <span
                                class="border px-2 py-0.5 text-[10.5px]"
                                :class="
                                    approval?.status === '已通过'
                                        ? 'border-emerald-600/30 bg-emerald-50 text-emerald-700'
                                        : approval?.status === '已拒绝'
                                          ? 'border-accent/40 bg-accent-soft text-accent'
                                          : approval?.status === '已撤销'
                                            ? 'border-line bg-paper text-ink-faint'
                                            : 'border-amber-500/40 bg-amber-50 text-amber-700'
                                "
                            >
                                {{ approval?.status || task.approval_status || "审批中" }}
                            </span>
                        </div>
                        <p v-if="!task.approval_instance_id" class="mt-2 text-[12px] text-ink-faint">未关联审批实例</p>
                        <template v-else>
                            <dl class="mt-2 divide-y divide-line text-[12px]">
                                <div class="flex justify-between py-1.5">
                                    <dt class="text-ink-faint">审批编号</dt>
                                    <dd class="font-mono text-[11px] text-ink-soft">{{ approval?.business_id || "—" }}</dd>
                                </div>
                                <div class="flex justify-between py-1.5">
                                    <dt class="text-ink-faint">发起时间</dt>
                                    <dd class="text-ink-soft">{{ approval?.create_time ? formatTime(approval.create_time) : "—" }}</dd>
                                </div>
                            </dl>
                            <div v-if="approval?.operations?.length" class="mt-2 space-y-1.5 border-t border-line pt-2">
                                <p
                                    v-for="(op, idx) in approval.operations.slice(-6)"
                                    :key="idx"
                                    class="text-[11px] leading-relaxed text-ink-soft"
                                >
                                    <span class="text-ink-faint">{{ formatTime(op.date) }}</span>
                                    <span class="ml-1.5">{{ op.show_name || op.type }}</span>
                                    <span v-if="op.result" class="ml-1">· {{ op.result === "AGREE" ? "同意" : op.result === "REFUSE" ? "拒绝" : op.result }}</span>
                                    <span v-if="op.remark" class="ml-1 text-ink-faint">「{{ op.remark }}」</span>
                                </p>
                            </div>
                        </template>
                    </section>

                    <section class="shell-card px-5 py-4">
                        <h3 class="font-display text-[15px] text-ink">技能</h3>
                        <template v-if="task.skills?.length">
                            <div class="mt-2 divide-y divide-line">
                                <div v-for="s in task.skills" :key="s.name" class="py-1.5">
                                    <span class="min-w-0 truncate font-mono text-[12px] text-ink">{{ s.name }}</span>
                                </div>
                            </div>
                        </template>
                        <p v-else class="mt-2 text-[12px] text-ink-faint">未绑定技能。任务可以人工执行，也可以在「编辑」里绑定技能。</p>
                    </section>

                    <!-- 产出（放右侧） -->
                    <section class="shell-card px-5 py-4">
                        <div class="flex flex-wrap items-center justify-between gap-2">
                            <h3 class="font-display text-[15px] text-ink">产出</h3>
                            <button v-if="canOperate" class="btn-quiet !py-1 text-[11.5px]" type="button" :disabled="uploading" @click="pickArtifact">
                                {{ uploading ? "上传中…" : "+ 上传产出" }}
                            </button>
                        </div>
                        <input id="artifact-file" type="file" class="hidden" @change="onUploadArtifact" />
                        <ul v-if="artifacts.length" class="mt-2 divide-y divide-line">
                            <li v-for="a in artifacts" :key="a.id" class="py-2 text-[12px]">
                                <div class="flex items-center gap-3">
                                    <span class="min-w-0 flex-1 truncate text-ink">{{ a.name }}</span>
                                    <span class="shrink-0 text-[10.5px] text-ink-faint">{{ sizeText(a.size) }}</span>
                                    <a class="btn-ghost !py-0.5 text-[11px]" :href="artifactURL(task.id, a.id)" target="_blank" rel="noopener">下载</a>
                                </div>
                                <div class="mt-0.5 flex flex-wrap items-center gap-x-2 text-[10.5px] text-ink-faint">
                                    <span>归档到：{{ a.folder || "未归档" }}</span>
                                    <span v-if="a.uploader_name">· {{ a.uploader_name }}</span>
                                    <span>· {{ fmtTime(a.created_at) }}</span>
                                </div>
                            </li>
                        </ul>
                        <p v-else class="mt-2 text-[12px] text-ink-faint">还没有产出。执行过程中或完成后都可以上传。</p>

                        <div v-if="artifactEvents.length" class="mt-3 border-t border-line pt-3">
                            <p class="text-[10.5px] tracking-wide text-ink-faint">产出时间线</p>
                            <ul class="mt-1.5 space-y-1.5">
                                <li v-for="e in artifactEvents" :key="e.id" class="flex items-baseline gap-2 text-[11px]">
                                    <span class="w-[74px] shrink-0 font-mono text-[10px] text-ink-faint">{{ fmtTime(e.at) }}</span>
                                    <span class="min-w-0 flex-1 text-ink-soft">{{ e.message }}</span>
                                    <span class="shrink-0 text-[10px] text-ink-faint">{{ e.actor }}</span>
                                </li>
                            </ul>
                        </div>
                    </section>

                </div>
            </div>
        </template>

        <Modal :open="modal === 'complete'" title="完成并提交结果" width="560px" @close="modal = ''">
            <label class="flex flex-col gap-1">
                <span class="label !mb-0">结果摘要 <b class="text-accent">*</b></span>
                <textarea v-model="completeForm.summary" class="field h-32 resize-none" placeholder="做了什么、结果如何"></textarea>
            </label>
            <p class="mt-2 text-[10.5px] leading-relaxed text-ink-faint">
                产出文件请在详情页「产出」区上传，上传后会一并归档到任务下。
            </p>
            <p v-if="modalError" class="mt-3 border border-accent/30 bg-accent-soft/60 px-3 py-2 text-[12px] text-accent">{{ modalError }}</p>
            <template #footer>
                <button class="btn-quiet" type="button" @click="modal = ''">取消</button>
                <button class="btn-solid" type="button" :disabled="saving" @click="submitComplete">{{ saving ? "提交中…" : "提交并完成" }}</button>
            </template>
        </Modal>

        <Modal :open="modal === 'edit'" title="编辑任务" width="640px" @close="modal = ''">
            <TaskFormFields
                :model="editForm"
                :stages="projectStages"
                :members="projectMembers"
                :skills="skills"
            />
            <p class="mt-2 text-[10.5px] leading-relaxed text-ink-faint">
                改「负责人」即转派：新负责人接手开始/工时/产出，原负责人随之失去该任务的操作入口。
            </p>

            <p v-if="modalError" class="mt-3 border border-accent/30 bg-accent-soft/60 px-3 py-2 text-[12px] text-accent">{{ modalError }}</p>
            <template #footer>
                <button class="btn-quiet" type="button" @click="modal = ''">取消</button>
                <button class="btn-solid" type="button" :disabled="saving" @click="submitEdit">{{ saving ? "保存中…" : "保存" }}</button>
            </template>
        </Modal>
    </section>
</template>

