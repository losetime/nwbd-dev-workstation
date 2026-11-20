<script setup lang="ts">
/** 项目详情。调用时机：从项目卡片进入；含 P2 的项目编辑、状态、项目经理、节点计划、任务、成员操作。 */
import { computed, nextTick, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";

import {
    addMembers,
    createRisk,
    createTask,
    deleteProject,
    getProject,
    listCandidates,
    replaceManagers,
    saveMilestonePlan,
    setMemberStatus,
    updateMilestone,
    updateProject,
    updateProjectStatus,
    updateRisk,
    updateRiskFlag,
    type Candidate,
    type Milestone,
    type ProjectDetail,
    type Risk,
} from "@/@api/projects";
import { listManagers, type ManagerOption } from "@/@api/users";
import { listSkills, listTaskTypes, type SkillItem, type TaskTypeOption } from "@/@api/tasks";
import { listContractApprovals, type ContractApproval } from "@/@api/approvals";
import { listProjectChanges, type ChangeRecord } from "@/@api/changes";
import ChangeCreateModal from "@/components/change/ChangeCreateModal.vue";
import { ApiError } from "@/@api/http";
import TaskFormFields from "@/components/task/TaskFormFields.vue";
import { PROJECT_PLAN_CODE, emptyTaskForm, parseAcceptance, type TaskFormModel } from "@/components/task/types";
import ConfirmDialog from "@/components/ui/ConfirmDialog.vue";
import Modal from "@/components/ui/Modal.vue";
import { useAuth } from "@/store/auth";
import { PROJECT_STATUSES, RISK_FLAGS, TYPE_SHORT, fmtDate, fmtWan, riskTone, statusTone } from "@/utils/project";

const route = useRoute();
const router = useRouter();
const auth = useAuth();

const loading = ref(true);
const error = ref("");
const detail = ref<ProjectDetail | null>(null);
const notice = ref("");
const noticeTone = ref<"info" | "warn">("info");

const modal = ref("");
const saving = ref(false);
const modalError = ref("");

const project = computed(() => detail.value?.project ?? null);
const canManage = computed(() => detail.value?.can_manage === true);
const isBoss = computed(() => {
    const u = auth.user.value;
    return u?.job_type === "部门经理";
});

const projectId = computed(() => String(route.params.id));

async function load(): Promise<void> {
    loading.value = true;
    error.value = "";
    try {
        detail.value = await getProject(projectId.value);
        changes.value = await listProjectChanges(projectId.value).catch(() => []);
    } catch (err) {
        error.value = err instanceof Error ? err.message : "加载失败";
    } finally {
        loading.value = false;
    }
    // 从「风险管理」页跳过来时定位到 ⑤ 区块（等 loading 结束、区块渲染完）
    if (route.query.focus === "risk") {
        await nextTick();
        document.getElementById("risk-section")?.scrollIntoView({ behavior: "smooth", block: "start" });
    }
}

function closeModal(): void {
    modal.value = "";
    modalError.value = "";
    saving.value = false;
}

/** 统一执行一次写操作：成功则关弹窗、刷新详情。 */
async function run(action: () => Promise<unknown>, successText: string): Promise<void> {
    saving.value = true;
    modalError.value = "";
    try {
        await action();
        notice.value = successText;
        closeModal();
        await load();
    } catch (err) {
        modalError.value = err instanceof Error ? err.message : "操作失败";
    } finally {
        saving.value = false;
    }
}

// ---------- 编辑信息 ----------
const info = ref({
    name: "",
    customer: "",
    contract_amount: "",
    plan_start: "",
    plan_end: "",
    budget: "",
    labor_budget: "",
    labor_rate: "",
    remark: "",
});
const infoInitial = ref({ ...info.value });

function openInfo(): void {
    const p = project.value;
    if (!p) return;
    info.value = {
        name: p.name,
        customer: p.customer || "",
        contract_amount: p.contract_amount === null ? "" : String(p.contract_amount),
        plan_start: fmtDate(p.plan_start),
        plan_end: fmtDate(p.plan_end),
        budget: p.budget === null ? "" : String(p.budget),
        labor_budget: p.labor_budget === null ? "" : String(p.labor_budget),
        labor_rate: p.labor_rate === null ? "" : String(p.labor_rate),
        remark: p.remark || "",
    };
    infoInitial.value = { ...info.value };
    modal.value = "info";
}

async function saveInfo(): Promise<void> {
    const payload: Record<string, string | number> = {};
    const keys = ["name", "customer", "contract_amount", "plan_start", "plan_end", "budget", "labor_budget", "labor_rate", "remark"] as const;
    for (const key of keys) {
        if (info.value[key] === infoInitial.value[key]) continue;
        const raw = info.value[key].trim();
        if (key === "name" && !raw) {
            modalError.value = "项目名称不能为空";
            return;
        }
        if (key === "name" || key === "customer" || key === "remark") {
            payload[key] = raw;
        } else if (key === "plan_start" || key === "plan_end") {
            if (raw) payload[key] = raw;
        } else if (raw !== "") {
            payload[key] = Number(raw);
        }
    }
    if (!Object.keys(payload).length) {
        modalError.value = "没有修改任何内容";
        return;
    }
    await run(() => updateProject(projectId.value, payload), "项目信息已更新");
}

// ---------- 状态 / 风险标识 ----------
const statusValue = ref("");
const riskValue = ref("");

function openStatus(): void {
    statusValue.value = project.value?.status || "未开始";
    modal.value = "status";
}

function openRisk(): void {
    riskValue.value = project.value?.risk_flag || "";
    modal.value = "risk";
}

// ---------- 更换项目经理 ----------
const managerOptions = ref<ManagerOption[]>([]);
const managerIds = ref<string[]>([]);

async function openManagers(): Promise<void> {
    modal.value = "pm";
    modalError.value = "";
    managerIds.value = (detail.value?.members ?? [])
        .filter((m) => m.project_role === "项目经理" && m.status === "参与中")
        .map((m) => m.user_id);
    try {
        managerOptions.value = await listManagers();
    } catch (err) {
        modalError.value = err instanceof Error ? err.message : "读取项目经理失败";
    }
}

function toggleManager(id: string): void {
    const idx = managerIds.value.indexOf(id);
    if (idx >= 0) managerIds.value.splice(idx, 1);
    else managerIds.value.push(id);
}

// ---------- 节点计划 ----------
interface PlanRow {
    id: string;
    stage: string;
    name: string;
    plan_due: string;
    contract_due: string;
    locked: boolean;
}

const planRows = ref<PlanRow[]>([]);

function openPlan(): void {
    planRows.value = (detail.value?.milestones ?? []).map((m: Milestone) => ({
        id: m.id,
        stage: m.stage,
        name: m.name,
        plan_due: fmtDate(m.plan_due),
        contract_due: fmtDate(m.contract_due),
        locked: m.locked,
    }));
    modal.value = "plan";
}

async function savePlan(): Promise<void> {
    const items = planRows.value
        .filter((row) => !row.locked)
        .map((row) => ({ id: row.id, plan_due: row.plan_due, contract_due: row.contract_due }))
        .filter((row) => row.plan_due || row.contract_due);
    if (!items.length) {
        modalError.value = "还没有填写任何日期";
        return;
    }
    await run(() => saveMilestonePlan(projectId.value, items), "节点计划已保存");
}

// ---------- 里程碑单条 ----------
const msTarget = ref<Milestone | null>(null);
const msForm = ref({ status: "未开始", plan_due: "", contract_due: "", actual_done: "", reviewer: "", review_date: "" });

function openMilestone(m: Milestone): void {
    if (!canManage.value) return;
    msTarget.value = m;
    msForm.value = {
        status: m.status,
        plan_due: fmtDate(m.plan_due),
        contract_due: fmtDate(m.contract_due),
        actual_done: fmtDate(m.actual_done),
        reviewer: m.reviewer || "",
        review_date: fmtDate(m.review_date),
    };
    modal.value = "ms";
}

async function saveMilestone(): Promise<void> {
    if (!msTarget.value) return;
    await run(
        () =>
            updateMilestone(projectId.value, msTarget.value!.id, {
                status: msForm.value.status,
                plan_due: msForm.value.plan_due || undefined,
                contract_due: msForm.value.contract_due || undefined,
                actual_done: msForm.value.actual_done || undefined,
                reviewer: msForm.value.reviewer,
                review_date: msForm.value.review_date || undefined,
            }),
        "里程碑已保存",
    );
}

// ---------- 下发任务 ----------
const taskForm = ref<TaskFormModel>(emptyTaskForm());
const skillOptions = ref<SkillItem[]>([]);
const taskTypes = ref<TaskTypeOption[]>([]);
/** 非部门经理看不到「项目策划」（选了会被后端拒绝）。 */
const visibleTaskTypes = computed(() =>
    isBoss.value ? taskTypes.value : taskTypes.value.filter((t) => t.code !== PROJECT_PLAN_CODE),
);
const contractOptions = ref<ContractApproval[]>([]);
const contractLoading = ref(false);
const activeMembers = computed(() => (detail.value?.members ?? []).filter((m) => m.status === "参与中"));
const stageOptions = computed(() => (detail.value?.milestones ?? []).map((m) => m.stage));

function openTask(): void {
    taskForm.value = emptyTaskForm(stageOptions.value[0] || "");
    modal.value = "task";
    if (!skillOptions.value.length) {
        void listSkills()
            .then((items) => (skillOptions.value = items))
            .catch(() => (skillOptions.value = []));
    }
    if (!taskTypes.value.length) {
        void listTaskTypes()
            .then((items) => (taskTypes.value = items))
            .catch(() => (taskTypes.value = []));
    }
    if (isBoss.value && !contractOptions.value.length) {
        contractLoading.value = true;
        void listContractApprovals()
            .then((items) => (contractOptions.value = items))
            .catch(() => (contractOptions.value = []))
            .finally(() => (contractLoading.value = false));
    }
}

async function saveTask(): Promise<void> {
    if (!taskForm.value.task_type_code) {
        modalError.value = "请选择任务类型";
        return;
    }
    if (!taskForm.value.name.trim() || !taskForm.value.owner_id) {
        modalError.value = "任务名称与负责人必填";
        return;
    }
    const isProjectPlan = taskForm.value.task_type_code === PROJECT_PLAN_CODE;
    if (isProjectPlan) {
        if (!taskForm.value.contract_instance_id && !project.value?.contract_instance_id) {
            modalError.value = "请选择项目代号（合同签订单）";
            return;
        }
        if (!taskForm.value.source) {
            modalError.value = "请选择项目来源";
            return;
        }
    }
    const hours = taskForm.value.estimate_hours === "" ? 0 : Number(taskForm.value.estimate_hours);
    await run(
        () =>
            createTask(projectId.value, {
                name: taskForm.value.name.trim(),
                description: taskForm.value.description.trim(),
                acceptance: parseAcceptance(taskForm.value.acceptance),
                priority: taskForm.value.priority,
                skills: taskForm.value.skills,
                stage: taskForm.value.stage,
                owner_id: taskForm.value.owner_id,
                plan_start: taskForm.value.plan_start || "",
                plan_due: taskForm.value.plan_due || "",
                estimate_hours: Number.isFinite(hours) ? hours : 0,
                task_type_code: taskForm.value.task_type_code,
                approval: isProjectPlan
                    ? {
                          contract_instance_id: taskForm.value.contract_instance_id,
                          source: taskForm.value.source,
                      }
                    : undefined,
            }),
        isProjectPlan ? "任务已下发，钉钉审批已发起" : "任务已下发",
    );
}

/** 任务状态由负责人操作，项目详情只读。 */
/** 打开任务详情（仅项目经理 / 部门经理）。 */
function openTaskDetail(taskId: string): void {
    void router.push(`/tasks/${taskId}`);
}

// ---------- 增加成员 ----------
const candidates = ref<Candidate[]>([]);
const memberIds = ref<string[]>([]);
const memberRole = ref("软件开发");

async function openMembers(): Promise<void> {
    modal.value = "member";
    modalError.value = "";
    memberIds.value = [];
    memberRole.value = "软件开发";
    try {
        candidates.value = await listCandidates(projectId.value);
    } catch (err) {
        modalError.value = err instanceof Error ? err.message : "读取候选人失败";
    }
}

function toggleMember(id: string): void {
    const idx = memberIds.value.indexOf(id);
    if (idx >= 0) memberIds.value.splice(idx, 1);
    else memberIds.value.push(id);
}

async function saveMembers(): Promise<void> {
    if (!memberIds.value.length) {
        modalError.value = "请选择要加入的成员";
        return;
    }
    await run(() => addMembers(projectId.value, memberIds.value, memberRole.value), "成员已加入");
}

/** 成员退出 / 恢复。有未完成任务时弹提示框，只告知、不可退出。 */
const confirmOpen = ref(false);
const confirmMessage = ref("");
const deleteOpen = ref(false);

async function confirmDeleteProject(): Promise<void> {
    deleteOpen.value = false;
    try {
        await deleteProject(projectId.value);
        await router.push("/");
    } catch (err) {
        noticeTone.value = "warn";
        notice.value = err instanceof Error ? err.message : "删除项目失败";
    }
}

async function toggleMemberStatus(userId: string, status: string): Promise<void> {
    notice.value = "";
    noticeTone.value = "info";
    try {
        await setMemberStatus(projectId.value, userId, status);
        notice.value = status === "已退出" ? "成员已退出项目" : "成员已恢复参与";
        await load();
    } catch (err) {
        if (err instanceof ApiError && err.code === "HAS_OPEN_TASKS") {
            confirmMessage.value = err.message;
            confirmOpen.value = true;
            return;
        }
        // 其他业务拦截用警告色提示，不是系统故障
        noticeTone.value = "warn";
        notice.value = err instanceof Error ? err.message : "更新成员失败";
    }
}

/** 提示框：点「确定」只是关闭。 */
function closeConfirm(): void {
    confirmOpen.value = false;
}

// ---------- 汇总 ----------
const taskSummary = computed(() => {
    const tasks = detail.value?.tasks ?? [];
    return {
        total: tasks.length,
        open: tasks.filter((t) => t.status === "未开始" || t.status === "进行中" || t.status === "已延期").length,
        done: tasks.filter((t) => t.status === "已完成").length,
        overdue: tasks.filter((t) => t.overdue).length,
    };
});

const memberSummary = computed(() => {
    const members = detail.value?.members ?? [];
    return { active: members.filter((m) => m.status === "参与中").length, total: members.length };
});

const RISK_TYPES = ["资源", "进度", "经费", "客户", "技术", "其他"];
const RISK_LEVELS = ["高", "中", "低"];
const RISK_STATUSES = ["监控中", "已发生", "已解决"];

const riskForm = ref({
    id: "",
    title: "",
    type: "其他",
    level: "中",
    impact: "",
    action: "",
    owner_id: "",
    target_date: "",
    status: "监控中",
    resolved_date: "",
});
const riskTarget = ref<Risk | null>(null);

function openRiskCreate(): void {
    riskForm.value = {
        id: "",
        title: "",
        type: "其他",
        level: "中",
        impact: "",
        action: "",
        owner_id: "",
        target_date: "",
        status: "监控中",
        resolved_date: "",
    };
    riskTarget.value = null;
    modal.value = "riskitem";
}

function openRiskEdit(r: Risk): void {
    if (!canManage.value) return;
    riskForm.value = {
        id: r.id,
        title: r.title,
        type: r.type,
        level: r.level,
        impact: r.impact,
        action: r.action,
        owner_id: r.owner_id || "",
        target_date: fmtDate(r.target_date),
        status: r.status,
        resolved_date: fmtDate(r.resolved_date),
    };
    riskTarget.value = r;
    modal.value = "riskitem";
}

async function saveRisk(): Promise<void> {
    const f = riskForm.value;
    if (!f.title.trim()) {
        modalError.value = "风险标题必填";
        return;
    }
    if (f.status === "已解决" && !f.resolved_date) {
        modalError.value = "请填写解决日期";
        return;
    }
    const base = {
        title: f.title.trim(),
        type: f.type,
        level: f.level,
        impact: f.impact.trim(),
        action: f.action.trim(),
        owner_id: f.owner_id,
        target_date: f.target_date || undefined,
    };
    if (f.id) {
        await run(
            () =>
                updateRisk(f.id, {
                    ...base,
                    status: f.status,
                    resolved_date: f.status === "已解决" ? f.resolved_date : undefined,
                }),
            "风险已更新",
        );
        return;
    }
    await run(() => createRisk(projectId.value, base), "风险已登记");
}

/** 目标解决日已过且未解决。 */
function riskOverdue(r: Risk): boolean {
    if (r.status === "已解决" || !r.target_date) return false;
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    return new Date(r.target_date) < today;
}

// ---------- 项目变更 ----------
const changes = ref<ChangeRecord[]>([]);
const changeOpen = ref(false);

function riskLevelTone(level: string): string {
    if (level === "高") return "border-accent/40 bg-accent-soft text-accent";
    if (level === "中") return "border-amber-500/40 bg-amber-50 text-amber-700";
    return "border-line bg-paper text-ink-soft";
}

const riskSummary = computed(() => {
    const risks = detail.value?.risks ?? [];
    const open = risks.filter((r) => r.status !== "已解决");
    return { open: open.length, total: risks.length, high: open.filter((r) => r.level === "高").length };
});

const budgetAlert = computed(() => {
    const p = project.value;
    if (!p || p.budget === null || p.cost_actual === null) return "未填，不判定";
    return p.cost_actual > p.budget ? `超支 ${(p.cost_actual - p.budget).toFixed(1)} 万` : "未超支";
});

onMounted(load);
</script>

<template>
    <section class="mx-auto max-w-[1180px]">
        <button class="btn-ghost -ml-2 text-[12px]" type="button" @click="router.back()">← 返回</button>

        <p v-if="error" class="mt-4 border border-accent/30 bg-accent-soft/60 px-4 py-3 text-[12.5px] text-accent">
            {{ error }}
        </p>
        <div v-else-if="loading" class="mt-4 border border-line bg-white px-5 py-16 text-center text-[12.5px] text-ink-faint">
            正在加载…
        </div>

        <template v-else-if="detail && project">
            <!-- 头部 -->
            <header class="mt-3 border border-line bg-white px-6 py-5">
                <div class="flex flex-wrap items-start justify-between gap-4">
                    <div class="min-w-0">
                        <div class="flex flex-wrap items-center gap-2">
                            <h2 class="font-display text-[24px] leading-tight tracking-tight text-ink">{{ project.name }}</h2>
                            <span class="border border-line bg-paper px-2 py-0.5 text-[10.5px] text-ink-soft">
                                {{ TYPE_SHORT[project.type] || project.type }}
                            </span>
                            <span v-if="project.risk_flag" class="border px-2 py-0.5 text-[10.5px]" :class="riskTone(project.risk_flag)">
                                {{ project.risk_flag }}
                            </span>
                            <span class="border px-2 py-0.5 text-[10.5px]" :class="statusTone(project.status)">
                                {{ project.status }}
                            </span>
                            <span
                                v-if="project.contract_status"
                                class="border border-accent/40 bg-accent-soft px-2 py-0.5 text-[10.5px] text-accent"
                            >
                                合同{{ project.contract_status }}
                            </span>
                        </div>
                        <p class="mt-2 text-[12px] text-ink-faint">
                            <span>编号 {{ project.code || "暂无" }}</span>
                            <span v-if="project.customer"> · 客户 {{ project.customer }}</span>
                            <span v-if="project.managers"> · 项目经理 {{ project.managers }}</span>
                            <span v-if="project.plan_start || project.plan_end">
                                · {{ fmtDate(project.plan_start) }} ~ {{ fmtDate(project.plan_end) }}
                            </span>
                        </p>
                    </div>
                    <div class="text-right">
                        <p class="font-display text-[26px] leading-none text-ink">
                            {{ project.progress === null ? "—" : project.progress + "%" }}
                        </p>
                        <p class="mt-1 text-[10.5px] text-ink-faint">任务完成</p>
                    </div>
                </div>

                <div v-if="canManage || isBoss" class="mt-4 flex flex-wrap gap-2 border-t border-line pt-4">
                    <button v-if="isBoss" class="btn-quiet !py-1.5 text-[12px]" type="button" @click="openInfo">编辑信息</button>
                    <button v-if="isBoss" class="btn-quiet !py-1.5 text-[12px]" type="button" @click="openStatus">
                        状态：{{ project.status }}
                    </button>
                    <button v-if="canManage" class="btn-quiet !py-1.5 text-[12px]" type="button" @click="openPlan">填写节点计划</button>
                    <button v-if="canManage" class="btn-solid !py-1.5 text-[12px]" type="button" @click="openTask">+ 下发任务</button>
                    <button v-if="canManage" class="btn-quiet !py-1.5 text-[12px]" type="button" @click="changeOpen = true">发起变更</button>
                    <button v-if="isBoss" class="btn-quiet !py-1.5 text-[12px]" type="button" @click="openMembers">+ 增加成员</button>
                    <button v-if="isBoss" class="btn-quiet !py-1.5 text-[12px]" type="button" @click="openManagers">更换项目经理</button>
                    <button v-if="isBoss" class="btn-quiet !py-1.5 text-[12px]" type="button" @click="openRisk">标记风险</button>
                    <button v-if="isBoss" class="btn-quiet !py-1.5 text-[12px] !text-accent" type="button" @click="deleteOpen = true">
                        删除项目
                    </button>
                </div>
            </header>

            <p
                v-if="notice"
                class="mt-3 border px-4 py-2 text-[12px]"
                :class="noticeTone === 'warn' ? 'border-amber-500/40 bg-amber-50 text-amber-800' : 'border-line bg-white text-ink-soft'"
            >
                {{ notice }}
            </p>

            <!-- ① 阶段与里程碑 -->
            <section class="mt-4 shell-card">
                <header class="flex items-center justify-between border-b border-line px-5 py-3">
                    <h3 class="font-display text-[15px] text-ink">① 阶段与里程碑</h3>
                    <span class="text-[11px] text-ink-faint">
                        共 {{ detail.milestones.length }} 个阶段<template v-if="canManage"> · 点行可编辑</template>
                    </span>
                </header>
                <div v-if="!detail.milestones.length" class="px-5 py-6 text-[12px] text-ink-faint">不设阶段（售后项目）</div>
                <div v-else class="overflow-x-auto">
                    <table class="w-full text-[12px]">
                        <thead class="bg-paper text-left text-[10.5px] tracking-[0.06em] text-ink-faint uppercase">
                            <tr>
                                <th class="px-4 py-2 font-medium">阶段</th>
                                <th class="px-4 py-2 font-medium">里程碑</th>
                                <th class="px-4 py-2 font-medium">计划完成</th>
                                <th class="px-4 py-2 font-medium">合同要求</th>
                                <th class="px-4 py-2 font-medium">实际完成</th>
                                <th class="px-4 py-2 font-medium">状态</th>
                            </tr>
                        </thead>
                        <tbody class="divide-y divide-line">
                            <tr
                                v-for="m in detail.milestones"
                                :key="m.id"
                                :class="[{ 'bg-accent-soft/25': m.plan_overdue || m.contract_overdue }, canManage ? 'cursor-pointer hover:bg-paper' : '']"
                                @click="openMilestone(m)"
                            >
                                <td class="px-4 py-2 text-ink">{{ m.stage }}</td>
                                <td class="px-4 py-2 text-ink-soft">{{ m.name }}</td>
                                <td class="px-4 py-2" :class="m.plan_overdue ? 'text-accent' : 'text-ink-soft'">
                                    {{ fmtDate(m.plan_due) || "—" }}
                                </td>
                                <td class="px-4 py-2" :class="m.contract_overdue ? 'text-accent' : 'text-ink-soft'">
                                    {{ fmtDate(m.contract_due) || "—" }}
                                </td>
                                <td class="px-4 py-2 text-ink-soft">{{ fmtDate(m.actual_done) || "—" }}</td>
                                <td class="px-4 py-2 text-ink-soft">{{ m.status }}</td>
                            </tr>
                        </tbody>
                    </table>
                </div>
            </section>

            <!-- ② 合同与预算 -->
            <section class="mt-4 grid gap-4 lg:grid-cols-2">
                <div class="shell-card">
                    <header class="border-b border-line px-5 py-3">
                        <h3 class="font-display text-[15px] text-ink">② 合同与预算</h3>
                    </header>
                    <dl class="divide-y divide-line text-[12.5px]">
                        <div class="flex justify-between px-5 py-2.5">
                            <dt class="text-ink-faint">合同金额</dt>
                            <dd class="text-ink">{{ fmtWan(project.contract_amount) }}</dd>
                        </div>
                        <div class="flex justify-between px-5 py-2.5">
                            <dt class="text-ink-faint">预算</dt>
                            <dd class="text-ink">{{ fmtWan(project.budget) }}</dd>
                        </div>
                        <div class="flex justify-between px-5 py-2.5">
                            <dt class="text-ink-faint">已发生成本</dt>
                            <dd :class="budgetAlert.startsWith('超支') ? 'text-accent' : 'text-ink'">
                                {{ fmtWan(project.cost_actual) }}
                                <span class="ml-2 text-[11px] text-ink-faint">{{ budgetAlert }}</span>
                            </dd>
                        </div>
                        <div class="flex justify-between px-5 py-2.5">
                            <dt class="text-ink-faint">人力预算 / 费率</dt>
                            <dd class="text-ink">
                                {{ fmtWan(project.labor_budget) }} /
                                {{ project.labor_rate === null ? "未设置" : project.labor_rate + " 元·时" }}
                            </dd>
                        </div>
                    </dl>
                </div>

                
            </section>

            <!-- ③ 项目任务 -->
            <section class="mt-4 shell-card">
                <header class="flex flex-wrap items-center justify-between gap-2 border-b border-line px-5 py-3">
                    <h3 class="font-display text-[15px] text-ink">③ 项目任务</h3>
                    <p class="text-[11px] text-ink-faint">
                        共 {{ taskSummary.total }} 项 · 在办 {{ taskSummary.open }} · 已完成 {{ taskSummary.done }} · 逾期 {{ taskSummary.overdue }}
                    </p>
                </header>
                <div v-if="!detail.tasks.length" class="px-5 py-6 text-[12px] text-ink-faint">还没有任务</div>
                <div v-else class="overflow-x-auto">
                    <table class="w-full text-[12px]">
                        <thead class="bg-paper text-left text-[10.5px] tracking-[0.06em] text-ink-faint uppercase">
                            <tr>
                                <th class="px-4 py-2 font-medium">任务</th>
                                <th class="px-4 py-2 font-medium">阶段</th>
                                <th class="px-4 py-2 font-medium">负责人</th>
                                <th class="px-4 py-2 font-medium">计划</th>
                                <th class="px-4 py-2 font-medium">状态</th>
                                <th class="px-4 py-2 font-medium">预估 / 实际</th>
                            </tr>
                        </thead>
                        <tbody class="divide-y divide-line">
                            <tr
                                v-for="t in detail.tasks"
                                :key="t.id"
                                :class="[{ 'bg-accent-soft/25': t.overdue }, canManage ? 'cursor-pointer hover:bg-paper' : '']"
                                @click="canManage && openTaskDetail(t.id)"
                            >
                                <td class="px-4 py-2 text-ink">{{ t.name }}</td>
                                <td class="px-4 py-2 text-ink-soft">{{ t.stage || "—" }}</td>
                                <td class="px-4 py-2 text-ink-soft">{{ t.owner_name }}</td>
                                <td class="px-4 py-2 text-ink-soft">
                                    {{ fmtDate(t.plan_start) || "—" }} ~ {{ fmtDate(t.plan_due) || "—" }}
                                </td>
                                <td class="px-4 py-2" :class="t.overdue ? 'text-accent' : 'text-ink-soft'">{{ t.status }}</td>
                                <td class="px-4 py-2 text-ink-soft">{{ t.estimate_hours }} / {{ t.actual_hours }} h</td>
                            </tr>
                        </tbody>
                    </table>
                </div>
            </section>

            <!-- ④ 项目成员 -->
            <section class="mt-4 shell-card">
                <header class="flex flex-wrap items-center justify-between gap-2 border-b border-line px-5 py-3">
                    <h3 class="font-display text-[15px] text-ink">④ 项目成员</h3>
                    <p class="text-[11px] text-ink-faint">参与中 {{ memberSummary.active }} 人 · 记录共 {{ memberSummary.total }} 条</p>
                </header>
                <div v-if="!detail.members.length" class="px-5 py-6 text-[12px] text-ink-faint">还没有成员</div>
                <ul v-else class="divide-y divide-line">
                    <li
                        v-for="m in detail.members"
                        :key="m.user_id"
                        class="flex flex-wrap items-center gap-3 px-5 py-2.5 text-[12.5px]"
                        :class="m.status === '已退出' ? 'text-ink-faint' : 'text-ink'"
                    >
                        <span class="w-20 shrink-0">{{ m.name }}</span>
                        <span class="w-24 shrink-0 text-[11.5px] text-ink-soft">{{ m.project_role }}</span>
                        <span class="w-16 shrink-0 text-[11.5px] text-ink-faint">{{ m.job_type }}</span>
                        <span class="text-[11.5px] text-ink-faint">{{ m.status }}</span>
                        <span class="ml-auto flex items-center gap-3">
                            <span class="text-[11px] text-ink-faint">投入 {{ m.allocation }}%</span>
                            <button
                                v-if="canManage"
                                class="btn-ghost !py-0.5 text-[11px]"
                                type="button"
                                @click="toggleMemberStatus(m.user_id, m.status === '参与中' ? '已退出' : '参与中')"
                            >
                                {{ m.status === "参与中" ? "退出项目" : "恢复参与" }}
                            </button>
                        </span>
                    </li>
                </ul>
            </section>

            <!-- ⑤ 风险管理 -->
            <section id="risk-section" class="mt-4 shell-card">
                <header class="flex flex-wrap items-center justify-between gap-2 border-b border-line px-5 py-3">
                    <div class="flex items-center gap-3">
                        <h3 class="font-display text-[15px] text-ink">⑤ 风险管理</h3>
                        <p class="text-[11px] text-ink-faint">
                            未解决 {{ riskSummary.open }} 项 · 共 {{ riskSummary.total }} 项
                            <template v-if="riskSummary.high"> · <span class="text-accent">高等级 {{ riskSummary.high }} 项</span></template>
                        </p>
                    </div>
                    <button v-if="canManage" class="btn-quiet !py-1 text-[11.5px]" type="button" @click="openRiskCreate">
                        + 登记风险
                    </button>
                </header>
                <div v-if="!detail.risks.length" class="px-5 py-6 text-[12px] text-ink-faint">暂无风险</div>
                <ul v-else class="divide-y divide-line">
                    <li
                        v-for="r in detail.risks"
                        :key="r.id"
                        class="px-5 py-3 text-[12.5px]"
                        :class="canManage ? 'cursor-pointer hover:bg-paper' : ''"
                        @click="openRiskEdit(r)"
                    >
                        <div class="flex flex-wrap items-center gap-2">
                            <span class="font-mono text-[10.5px] text-ink-faint">{{ r.code }}</span>
                            <span class="text-ink">{{ r.title }}</span>
                            <span class="border px-1.5 py-0.5 text-[10.5px]" :class="riskLevelTone(r.level)">{{ r.level }}</span>
                            <span class="border border-line bg-white px-1.5 py-0.5 text-[10.5px] text-ink-soft">{{ r.type }}</span>
                            <span class="text-[11px]" :class="r.status === '已解决' ? 'text-ink-faint' : 'text-ink-soft'">{{ r.status }}</span>
                            <span v-if="riskOverdue(r)" class="text-[10.5px] text-accent">已超期</span>
                            <span v-if="r.owner_name" class="ml-auto text-[11px] text-ink-faint">
                                责任人 {{ r.owner_name }}
                                <template v-if="r.target_date"> · 目标 {{ fmtDate(r.target_date) }}</template>
                                <template v-if="r.resolved_date"> · 解决 {{ fmtDate(r.resolved_date) }}</template>
                            </span>
                        </div>
                        <p v-if="r.description" class="mt-1 text-[11.5px] leading-relaxed text-ink-soft">{{ r.description }}</p>
                        <p v-if="r.action" class="mt-1 text-[11.5px] leading-relaxed text-ink-faint">应对：{{ r.action }}</p>
                    </li>
                </ul>
            </section>

            <!-- ⑥ 项目变更 -->
            <section class="mt-4 shell-card">
                <header class="flex flex-wrap items-center justify-between gap-2 border-b border-line px-5 py-3">
                    <div class="flex items-center gap-3">
                        <h3 class="font-display text-[15px] text-ink">⑥ 项目变更</h3>
                        <p class="text-[11px] text-ink-faint">共 {{ changes.length }} 条</p>
                    </div>
                    <button v-if="canManage" class="btn-quiet !py-1 text-[11.5px]" type="button" @click="changeOpen = true">
                        + 发起变更
                    </button>
                </header>
                <div v-if="!changes.length" class="px-5 py-6 text-[12px] text-ink-faint">暂无变更</div>
                <ul v-else class="divide-y divide-line">
                    <li
                        v-for="c in changes"
                        :key="c.id"
                        class="cursor-pointer px-5 py-3 text-[12.5px] hover:bg-paper"
                        @click="router.push('/changes')"
                    >
                        <div class="flex flex-wrap items-center gap-2">
                            <span class="font-mono text-[10.5px] text-ink-faint">{{ c.code }}</span>
                            <span class="border border-line bg-paper px-1.5 py-0.5 text-[10.5px] text-ink-soft">{{ c.type }}</span>
                            <span class="text-ink">{{ c.content }}</span>
                            <span
                                class="border px-1.5 py-0.5 text-[10.5px]"
                                :class="
                                    c.approval_status === '已通过'
                                        ? 'border-emerald-600/30 bg-emerald-50 text-emerald-700'
                                        : c.approval_status === '已否决'
                                          ? 'border-accent/40 bg-accent-soft text-accent'
                                          : c.approval_status === '已撤销'
                                            ? 'border-line bg-paper text-ink-faint'
                                            : 'border-amber-500/40 bg-amber-50 text-amber-700'
                                "
                            >
                                {{ c.approval_status }}
                            </span>
                            <span class="ml-auto text-[11px] text-ink-faint">{{ c.applicant_name }} · {{ fmtDate(c.applied_on) }}</span>
                        </div>
                    </li>
                </ul>
            </section>

            <section v-if="project.weekly_summary" class="mt-4 shell-card px-5 py-4">
                <div class="flex items-center justify-between">
                    <h3 class="font-display text-[15px] text-ink">周度总结</h3>
                    <span class="text-[10.5px] text-ink-faint">{{ fmtDate(project.summary_at) }}</span>
                </div>
                <p class="mt-2 whitespace-pre-wrap text-[12px] leading-6 text-ink-soft">{{ project.weekly_summary }}</p>
            </section>
        </template>

        <!-- ========== 弹窗 ========== -->
        <Modal :open="modal === 'info'" title="编辑项目信息" width="620px" @close="closeModal">
            <div class="grid gap-3 sm:grid-cols-2">
                <label class="flex flex-col gap-1 sm:col-span-2">
                    <span class="label !mb-0">项目名称</span>
                    <input v-model="info.name" class="field" placeholder="项目名称" />
                </label>
                <label class="flex flex-col gap-1">
                    <span class="label !mb-0">客户</span>
                    <input v-model="info.customer" class="field" />
                </label>
                <label class="flex flex-col gap-1">
                    <span class="label !mb-0">合同金额（万元）</span>
                    <input v-model="info.contract_amount" class="field" type="number" step="0.01" />
                </label>
                <label class="flex flex-col gap-1">
                    <span class="label !mb-0">计划开始</span>
                    <input v-model="info.plan_start" class="field" type="date" />
                </label>
                <label class="flex flex-col gap-1">
                    <span class="label !mb-0">计划结束</span>
                    <input v-model="info.plan_end" class="field" type="date" />
                </label>
                <label class="flex flex-col gap-1">
                    <span class="label !mb-0">预算（万元）</span>
                    <input v-model="info.budget" class="field" type="number" step="0.01" />
                </label>
                <label class="flex flex-col gap-1">
                    <span class="label !mb-0">人力预算（万元）</span>
                    <input v-model="info.labor_budget" class="field" type="number" step="0.01" />
                </label>
                <label class="flex flex-col gap-1">
                    <span class="label !mb-0">人力费率（元/工时）</span>
                    <input v-model="info.labor_rate" class="field" type="number" step="0.01" />
                </label>
                <label class="flex flex-col gap-1 sm:col-span-2">
                    <span class="label !mb-0">备注</span>
                    <input v-model="info.remark" class="field" />
                </label>
            </div>
            <p class="mt-3 text-[10.5px] leading-relaxed text-ink-faint">
                项目编号、名称、类型不可改（涉及历史关联）。留空的项目不会改动已有值。
            </p>
            <p v-if="modalError" class="mt-3 border border-accent/30 bg-accent-soft/60 px-3 py-2 text-[12px] text-accent">{{ modalError }}</p>
            <template #footer>
                <button class="btn-quiet" type="button" @click="closeModal">取消</button>
                <button class="btn-solid" type="button" :disabled="saving" @click="saveInfo">{{ saving ? "保存中…" : "保存" }}</button>
            </template>
        </Modal>

        <Modal :open="modal === 'status'" title="修改项目状态" width="420px" @close="closeModal">
            <div class="grid grid-cols-2 gap-2">
                <button
                    v-for="s in PROJECT_STATUSES"
                    :key="s"
                    class="border px-3 py-2 text-[12.5px] transition"
                    :class="statusValue === s ? 'border-ink bg-ink text-white' : 'border-line bg-white text-ink-soft hover:border-ink/30'"
                    type="button"
                    @click="statusValue = s"
                >
                    {{ s }}
                </button>
            </div>
            <p class="mt-3 text-[10.5px] leading-relaxed text-ink-faint">
                状态由人工调整，任何阶段都可改；完成任务或改计划不会自动改状态。
            </p>
            <p v-if="modalError" class="mt-3 border border-accent/30 bg-accent-soft/60 px-3 py-2 text-[12px] text-accent">{{ modalError }}</p>
            <template #footer>
                <button class="btn-quiet" type="button" @click="closeModal">取消</button>
                <button class="btn-solid" type="button" :disabled="saving" @click="run(() => updateProjectStatus(projectId, statusValue), '状态已更新')">
                    {{ saving ? "保存中…" : "保存" }}
                </button>
            </template>
        </Modal>

        <Modal :open="modal === 'risk'" title="标记风险标识" width="420px" @close="closeModal">
            <div class="grid grid-cols-2 gap-2">
                <button
                    v-for="f in RISK_FLAGS"
                    :key="f"
                    class="border px-3 py-2 text-[12.5px] transition"
                    :class="riskValue === f ? 'border-ink bg-ink text-white' : 'border-line bg-white text-ink-soft hover:border-ink/30'"
                    type="button"
                    @click="riskValue = f"
                >
                    {{ f }}
                </button>
                <button
                    class="border px-3 py-2 text-[12.5px] transition"
                    :class="riskValue === '' ? 'border-ink bg-ink text-white' : 'border-line bg-white text-ink-soft hover:border-ink/30'"
                    type="button"
                    @click="riskValue = ''"
                >
                    清除标记
                </button>
            </div>
            <p class="mt-3 text-[10.5px] leading-relaxed text-ink-faint">
                风险标识由部门经理人工判定，独立于风险清单，驾驶舱按它统计。
            </p>
            <p v-if="modalError" class="mt-3 border border-accent/30 bg-accent-soft/60 px-3 py-2 text-[12px] text-accent">{{ modalError }}</p>
            <template #footer>
                <button class="btn-quiet" type="button" @click="closeModal">取消</button>
                <button class="btn-solid" type="button" :disabled="saving" @click="run(() => updateRiskFlag(projectId, riskValue), '风险标识已更新')">
                    {{ saving ? "保存中…" : "保存" }}
                </button>
            </template>
        </Modal>

        <Modal :open="modal === 'pm'" title="更换项目经理" width="520px" @close="closeModal">
            <p class="text-[11px] leading-relaxed text-ink-faint">
                项目经理可设多人、权限相同；移出名单的人会记为「已退出」。
            </p>
            <div class="mt-3 flex flex-wrap gap-2">
                <button
                    v-for="m in managerOptions"
                    :key="m.id"
                    class="border px-2.5 py-1 text-[12px] transition"
                    :class="managerIds.includes(m.id) ? 'border-ink bg-ink text-white' : 'border-line bg-white text-ink-soft hover:border-ink/30'"
                    type="button"
                    @click="toggleManager(m.id)"
                >
                    {{ m.name }}<span class="ml-1 text-[10px] opacity-60">{{ m.job_type }}</span>
                </button>
            </div>
            <p v-if="modalError" class="mt-3 border border-accent/30 bg-accent-soft/60 px-3 py-2 text-[12px] text-accent">{{ modalError }}</p>
            <template #footer>
                <button class="btn-quiet" type="button" @click="closeModal">取消</button>
                <button class="btn-solid" type="button" :disabled="saving" @click="run(() => replaceManagers(projectId, managerIds), '项目经理已更新')">
                    {{ saving ? "保存中…" : "保存" }}
                </button>
            </template>
        </Modal>

        <Modal :open="modal === 'plan'" title="填写节点计划" width="640px" @close="closeModal">
            <p class="text-[11px] leading-relaxed text-ink-faint">
                按阶段一次性填计划完成日和合同要求完成日；已锁定的阶段置灰，改动要走变更单。
            </p>
            <table class="mt-3 w-full text-[12px]">
                <thead class="bg-paper text-left text-[10.5px] text-ink-faint uppercase">
                    <tr>
                        <th class="px-3 py-2 font-medium">阶段</th>
                        <th class="px-3 py-2 font-medium">计划完成</th>
                        <th class="px-3 py-2 font-medium">合同要求完成日</th>
                    </tr>
                </thead>
                <tbody class="divide-y divide-line">
                    <tr v-for="row in planRows" :key="row.id">
                        <td class="px-3 py-1.5 text-ink">
                            {{ row.stage }}
                            <span v-if="row.locked" class="ml-1 text-[10px] text-ink-faint">已锁</span>
                        </td>
                        <td class="px-3 py-1.5">
                            <input v-model="row.plan_due" class="field !py-1 !text-[12px]" type="date" :disabled="row.locked" />
                        </td>
                        <td class="px-3 py-1.5">
                            <input v-model="row.contract_due" class="field !py-1 !text-[12px]" type="date" :disabled="row.locked" />
                        </td>
                    </tr>
                </tbody>
            </table>
            <p v-if="modalError" class="mt-3 border border-accent/30 bg-accent-soft/60 px-3 py-2 text-[12px] text-accent">{{ modalError }}</p>
            <template #footer>
                <button class="btn-quiet" type="button" @click="closeModal">取消</button>
                <button class="btn-solid" type="button" :disabled="saving" @click="savePlan">{{ saving ? "保存中…" : "保存计划" }}</button>
            </template>
        </Modal>

        <Modal :open="modal === 'ms'" title="节点计划" width="560px" @close="closeModal">
            <p class="text-[11px] text-ink-faint">
                阶段「{{ msTarget?.stage }}」· 里程碑「{{ msTarget?.name }}」
            </p>
            <div class="mt-3 grid gap-3 sm:grid-cols-2">
                <label class="flex flex-col gap-1">
                    <span class="label !mb-0">里程碑状态</span>
                    <select v-model="msForm.status" class="field">
                        <option v-for="s in ['未开始', '进行中', '已完成', '已延期']" :key="s" :value="s">{{ s }}</option>
                    </select>
                </label>
                <label class="flex flex-col gap-1">
                    <span class="label !mb-0">实际完成</span>
                    <input v-model="msForm.actual_done" class="field" type="date" />
                </label>
                <label class="flex flex-col gap-1">
                    <span class="label !mb-0">计划完成</span>
                    <input v-model="msForm.plan_due" class="field" type="date" />
                </label>
                <label class="flex flex-col gap-1">
                    <span class="label !mb-0">合同要求完成日</span>
                    <input v-model="msForm.contract_due" class="field" type="date" />
                </label>
                <label class="flex flex-col gap-1">
                    <span class="label !mb-0">评审人</span>
                    <input v-model="msForm.reviewer" class="field" />
                </label>
                <label class="flex flex-col gap-1">
                    <span class="label !mb-0">评审日期</span>
                    <input v-model="msForm.review_date" class="field" type="date" />
                </label>
            </div>
            <p class="mt-3 text-[10.5px] leading-relaxed text-ink-faint">
                把状态改成「进行中」后，项目当前阶段会自动推进到该阶段。
            </p>
            <p v-if="modalError" class="mt-3 border border-accent/30 bg-accent-soft/60 px-3 py-2 text-[12px] text-accent">{{ modalError }}</p>
            <template #footer>
                <button class="btn-quiet" type="button" @click="closeModal">取消</button>
                <button class="btn-solid" type="button" :disabled="saving" @click="saveMilestone">{{ saving ? "保存中…" : "保存" }}</button>
            </template>
        </Modal>

        <Modal :open="modal === 'task'" title="下发任务" width="640px" @close="closeModal">
            <TaskFormFields
                :model="taskForm"
                :stages="stageOptions"
                :members="activeMembers"
                :skills="skillOptions"
                :task-types="visibleTaskTypes"
                :contracts="contractOptions"
                :contract-linked="!!project?.contract_instance_id"
                :contract-code="project?.code || ''"
                :show-task-type="true"
                owner-required
            />
            <p v-if="contractLoading" class="mt-2 text-[10.5px] text-ink-faint">正在读取钉钉合同签订单…</p>
            <p v-if="modalError" class="mt-3 border border-accent/30 bg-accent-soft/60 px-3 py-2 text-[12px] text-accent">{{ modalError }}</p>
            <template #footer>
                <button class="btn-quiet" type="button" @click="closeModal">取消</button>
                <button class="btn-solid" type="button" :disabled="saving" @click="saveTask">{{ saving ? "下发中…" : "下发任务" }}</button>
            </template>
        </Modal>

        <Modal :open="modal === 'member'" title="增加成员" width="600px" @close="closeModal">
            <label class="flex flex-col gap-1">
                <span class="label !mb-0">项目角色</span>
                <select v-model="memberRole" class="field">
                    <option v-for="r in ['软件开发', '测试', 'UI设计']" :key="r" :value="r">{{ r }}</option>
                </select>
            </label>
            <p class="mt-3 text-[10.5px] text-ink-faint">候选人只列出还没加入本项目的在职人员。</p>
            <div class="mt-2 grid max-h-[300px] grid-cols-2 gap-1 overflow-y-auto sm:grid-cols-3">
                <button
                    v-for="c in candidates"
                    :key="c.id"
                    class="flex items-center justify-between border px-2.5 py-1.5 text-left text-[12px]"
                    :class="memberIds.includes(c.id) ? 'border-ink bg-paper text-ink' : 'border-line bg-white text-ink-soft hover:border-ink/30'"
                    type="button"
                    @click="toggleMember(c.id)"
                >
                    <span>{{ c.name }}</span>
                    <span class="text-[10px] text-ink-faint">{{ c.job_type }}</span>
                </button>
            </div>
            <p v-if="!candidates.length" class="mt-2 text-[11.5px] text-ink-faint">没有可加入的人（都已在项目里）</p>
            <p v-if="modalError" class="mt-3 border border-accent/30 bg-accent-soft/60 px-3 py-2 text-[12px] text-accent">{{ modalError }}</p>
            <template #footer>
                <button class="btn-quiet" type="button" @click="closeModal">取消</button>
                <button class="btn-solid" type="button" :disabled="saving" @click="saveMembers">{{ saving ? "保存中…" : "加入项目" }}</button>
            </template>
        </Modal>

        <Modal :open="modal === 'riskitem'" :title="riskForm.id ? '编辑风险' : '登记风险'" width="620px" @close="closeModal">
            <div class="grid gap-3 sm:grid-cols-2">
                <label class="flex flex-col gap-1 sm:col-span-2">
                    <span class="label !mb-0">风险标题 <b class="text-accent">*</b></span>
                    <input v-model="riskForm.title" class="field" placeholder="如 第三方接口交付延期" />
                </label>
                <label class="flex flex-col gap-1">
                    <span class="label !mb-0">类型</span>
                    <select v-model="riskForm.type" class="field">
                        <option v-for="t in RISK_TYPES" :key="t" :value="t">{{ t }}</option>
                    </select>
                </label>
                <label class="flex flex-col gap-1">
                    <span class="label !mb-0">等级</span>
                    <select v-model="riskForm.level" class="field">
                        <option v-for="l in RISK_LEVELS" :key="l" :value="l">{{ l }}</option>
                    </select>
                </label>
                <label class="flex flex-col gap-1 sm:col-span-2">
                    <span class="label !mb-0">影响描述</span>
                    <textarea v-model="riskForm.impact" class="field h-16 resize-none" placeholder="会发生什么后果" />
                </label>
                <label class="flex flex-col gap-1 sm:col-span-2">
                    <span class="label !mb-0">应对措施</span>
                    <textarea v-model="riskForm.action" class="field h-16 resize-none" placeholder="打算怎么处理" />
                </label>
                <label class="flex flex-col gap-1">
                    <span class="label !mb-0">责任人</span>
                    <select v-model="riskForm.owner_id" class="field">
                        <option value="">未指定</option>
                        <option v-for="m in activeMembers" :key="m.user_id" :value="m.user_id">
                            {{ m.name }} · {{ m.project_role }}
                        </option>
                    </select>
                </label>
                <label class="flex flex-col gap-1">
                    <span class="label !mb-0">目标解决日</span>
                    <input v-model="riskForm.target_date" class="field" type="date" />
                </label>
                <template v-if="riskForm.id">
                    <label class="flex flex-col gap-1">
                        <span class="label !mb-0">状态</span>
                        <select v-model="riskForm.status" class="field">
                            <option v-for="s in RISK_STATUSES" :key="s" :value="s">{{ s }}</option>
                        </select>
                    </label>
                    <label v-if="riskForm.status === '已解决'" class="flex flex-col gap-1">
                        <span class="label !mb-0">解决日期 <b class="text-accent">*</b></span>
                        <input v-model="riskForm.resolved_date" class="field" type="date" />
                    </label>
                </template>
            </div>
            <p class="mt-2 text-[10.5px] leading-relaxed text-ink-faint">
                风险只做登记与跟踪，不删除；解决后保留记录。已解决的风险只有部门经理可以重开。
            </p>
            <p v-if="modalError" class="mt-3 border border-accent/30 bg-accent-soft/60 px-3 py-2 text-[12px] text-accent">{{ modalError }}</p>
            <template #footer>
                <button class="btn-quiet" type="button" @click="closeModal">取消</button>
                <button class="btn-solid" type="button" :disabled="saving" @click="saveRisk">
                    {{ saving ? "保存中…" : riskForm.id ? "保存" : "登记" }}
                </button>
            </template>
        </Modal>

        <ChangeCreateModal
            :open="changeOpen"
            :project-id="projectId"
            :initial-budget="project?.budget === null || project?.budget === undefined ? '' : String(project.budget)"
            :initial-plan-end="fmtDate(project?.plan_end)"
            @close="changeOpen = false"
            @created="load()"
        />


        <ConfirmDialog
            :open="confirmOpen"
            title="提示"
            :message="confirmMessage"
            confirm-text="确定"
            :show-cancel="false"
            tone="warn"
            @confirm="closeConfirm"
            @cancel="closeConfirm"
        />

        <ConfirmDialog
            :open="deleteOpen"
            title="删除项目"
            :message="`删除后项目不再出现在列表里，任务、工时、风险等数据会保留但无法访问。确定删除「${project?.name ?? ''}」吗？`"
            confirm-text="删除"
            cancel-text="取消"
            tone="danger"
            @confirm="confirmDeleteProject"
            @cancel="deleteOpen = false"
        />
    </section>
</template>



