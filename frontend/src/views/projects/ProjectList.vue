<script setup lang="ts">
/** 我的项目（项目列表）。调用时机：登录后的首页；部门经理看全部，其他人看自己参与的。 */
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";

import { createProject, listProjects, type Project } from "@/@api/projects";
import { listManagers, type ManagerOption } from "@/@api/users";
import Modal from "@/components/ui/Modal.vue";
import { useAuth } from "@/store/auth";
import {
    PROJECT_STATUSES,
    PROJECT_TYPES,
    RISK_FLAGS,
    STAGES,
    TYPE_SHORT,
    fmtDate,
    fmtWan,
    riskTone,
    stageIndex,
    statusTone,
} from "@/utils/project";

const auth = useAuth();
const router = useRouter();

const loading = ref(true);
const error = ref("");
const projects = ref<Project[]>([]);

const filters = ref({ keyword: "", type: "", status: "", risk_flag: "" });

const isBoss = computed(() => {
    const u = auth.user.value;
    return u?.job_type === "部门经理";
});

/** 加载列表。 */
async function load(): Promise<void> {
    loading.value = true;
    error.value = "";
    try {
        projects.value = await listProjects({
            keyword: filters.value.keyword || undefined,
            type: filters.value.type || undefined,
            status: filters.value.status || undefined,
            risk_flag: filters.value.risk_flag || undefined,
        });
    } catch (err) {
        error.value = err instanceof Error ? err.message : "加载失败";
    } finally {
        loading.value = false;
    }
}

function openDetail(id: string): void {
    void router.push({ name: "project-detail", params: { id } });
}

function stagesOf(p: Project): string[] {
    return STAGES[p.type] ?? [];
}

// ---------- 新建项目 ----------
const showCreate = ref(false);
const creating = ref(false);
const createError = ref("");
const managers = ref<ManagerOption[]>([]);
const managerError = ref("");
const form = ref({
    code: "",
    name: "",
    type: PROJECT_TYPES[0],
    customer: "",
    contract_amount: "" as string | number,
    manager_ids: [] as string[],
});

async function openCreate(): Promise<void> {
    form.value = { code: "", name: "", type: PROJECT_TYPES[0], customer: "", contract_amount: "", manager_ids: [] };
    createError.value = "";
    managerError.value = "";
    showCreate.value = true;
    if (!managers.value.length) {
        try {
            managers.value = await listManagers();
        } catch (err) {
            managerError.value = err instanceof Error ? err.message : "读取项目经理失败";
        }
    }
}

function toggleManager(id: string): void {
    const list = form.value.manager_ids;
    const idx = list.indexOf(id);
    if (idx >= 0) list.splice(idx, 1);
    else list.push(id);
}

async function submitCreate(): Promise<void> {
    createError.value = "";
    if (!form.value.code.trim() || !form.value.name.trim()) {
        createError.value = "项目编号与项目名称必填";
        return;
    }
    if (!form.value.manager_ids.length) {
        createError.value = "至少选择一位项目经理";
        return;
    }
    creating.value = true;
    try {
        const amount = form.value.contract_amount === "" ? null : Number(form.value.contract_amount);
        await createProject({
            code: form.value.code.trim(),
            name: form.value.name.trim(),
            type: form.value.type,
            customer: form.value.customer.trim(),
            contract_amount: Number.isFinite(amount as number) ? (amount as number) : null,
            manager_ids: form.value.manager_ids,
        });
        showCreate.value = false;
        await load();
    } catch (err) {
        createError.value = err instanceof Error ? err.message : "新建失败";
    } finally {
        creating.value = false;
    }
}

onMounted(load);
</script>

<template>
    <section class="mx-auto max-w-[1180px]">
        <header class="flex flex-wrap items-end justify-between gap-4">
            <div>
                <h2 class="font-display text-[26px] leading-tight tracking-tight text-ink">我的项目</h2>
                <p class="mt-1.5 text-[12.5px] text-ink-faint">
                    {{ isBoss ? "部门全部项目" : "你参与的项目" }} · 共 {{ projects.length }} 个
                </p>
            </div>
            <button v-if="isBoss" class="btn-solid" type="button" @click="openCreate">+ 新建项目</button>
        </header>

        <div class="mt-5 flex flex-wrap items-end gap-3 border border-line bg-white px-4 py-3">
            <label class="flex flex-col gap-1">
                <span class="text-[10px] tracking-[0.08em] text-ink-faint uppercase">项目名称</span>
                <input
                    v-model="filters.keyword"
                    class="field !py-1.5 !text-[12.5px] w-[180px]"
                    placeholder="名称 / 编号 / 客户"
                    @keyup.enter="load"
                />
            </label>
            <label class="flex flex-col gap-1">
                <span class="text-[10px] tracking-[0.08em] text-ink-faint uppercase">类型</span>
                <select v-model="filters.type" class="field !py-1.5 !text-[12.5px] w-[132px]" @change="load">
                    <option value="">全部</option>
                    <option v-for="t in PROJECT_TYPES" :key="t" :value="t">{{ t }}</option>
                </select>
            </label>
            <label class="flex flex-col gap-1">
                <span class="text-[10px] tracking-[0.08em] text-ink-faint uppercase">状态</span>
                <select v-model="filters.status" class="field !py-1.5 !text-[12.5px] w-[110px]" @change="load">
                    <option value="">全部</option>
                    <option v-for="s in PROJECT_STATUSES" :key="s" :value="s">{{ s }}</option>
                </select>
            </label>
            <label class="flex flex-col gap-1">
                <span class="text-[10px] tracking-[0.08em] text-ink-faint uppercase">风险标识</span>
                <select v-model="filters.risk_flag" class="field !py-1.5 !text-[12.5px] w-[104px]" @change="load">
                    <option value="">全部</option>
                    <option v-for="r in RISK_FLAGS" :key="r" :value="r">{{ r }}</option>
                </select>
            </label>
            <button class="btn-quiet !py-1.5" type="button" @click="load">查询</button>
        </div>

        <p v-if="error" class="mt-4 border border-accent/30 bg-accent-soft/60 px-4 py-3 text-[12.5px] text-accent">
            {{ error }}
        </p>

        <div v-if="loading" class="mt-6 border border-line bg-white px-5 py-16 text-center text-[12.5px] text-ink-faint">
            正在加载…
        </div>

        <div v-else-if="!projects.length" class="mt-6 border border-line bg-white px-5 py-16 text-center">
            <p class="font-display text-[16px] text-ink">还没有项目</p>
            <p class="mt-2 text-[12px] text-ink-faint">
                {{ isBoss ? "点右上角「新建项目」开始。" : "还没有被加入任何项目。" }}
            </p>
        </div>

        <div v-else class="mt-4 space-y-3">
            <article
                v-for="p in projects"
                :key="p.id"
                class="card-in shell-card cursor-pointer px-5 py-4 transition-shadow hover:shadow-[0_10px_30px_rgba(18,19,22,0.06)]"
                @click="openDetail(p.id)"
            >
                <div class="flex flex-wrap items-start justify-between gap-3">
                    <div class="min-w-0">
                        <div class="flex flex-wrap items-center gap-2">
                            <h3 class="font-display text-[17px] leading-snug text-ink">{{ p.name }}</h3>
                            <span class="border border-line bg-paper px-2 py-0.5 text-[10.5px] text-ink-soft">
                                {{ TYPE_SHORT[p.type] || p.type }}
                            </span>
                            <span v-if="p.risk_flag" class="border px-2 py-0.5 text-[10.5px]" :class="riskTone(p.risk_flag)">
                                {{ p.risk_flag }}
                            </span>
                            <span class="border px-2 py-0.5 text-[10.5px]" :class="statusTone(p.status)">
                                {{ p.status }}
                            </span>
                        </div>
                        <p class="mt-1.5 text-[11.5px] text-ink-faint">
                            {{ p.code || "暂无" }}
                            <span v-if="p.customer"> · {{ p.customer }}</span>
                            <span v-if="p.managers"> · PM {{ p.managers }}</span>
                            <span> · 预算 {{ fmtWan(p.budget) }}</span>
                            <span v-if="p.plan_start || p.plan_end">
                                · {{ fmtDate(p.plan_start) }} ~ {{ fmtDate(p.plan_end) }}
                            </span>
                        </p>
                    </div>
                    <div class="shrink-0 text-right">
                        <p class="font-display text-[22px] leading-none text-ink">
                            {{ p.progress === null ? "—" : p.progress + "%" }}
                        </p>
                        <p class="mt-1 text-[10.5px] text-ink-faint">任务完成</p>
                    </div>
                </div>

                <!-- 阶段条 -->
                <div v-if="stagesOf(p).length" class="mt-4 flex gap-1.5">
                    <div v-for="(s, i) in stagesOf(p)" :key="s" class="min-w-0 flex-1">
                        <div
                            class="h-[3px]"
                            :class="i < stageIndex(p.type, p.current_stage) ? 'bg-ink' : i === stageIndex(p.type, p.current_stage) ? 'bg-accent' : 'bg-line'"
                        ></div>
                        <p
                            class="mt-1.5 truncate text-[10px]"
                            :class="i === stageIndex(p.type, p.current_stage) ? 'text-ink' : 'text-ink-faint'"
                        >
                            {{ s }}
                        </p>
                    </div>
                </div>
                <p v-else class="mt-4 text-[11px] text-ink-faint">不设阶段</p>

                <div class="mt-3.5 flex flex-wrap items-center gap-x-5 gap-y-1 text-[11.5px] text-ink-soft">
                    <span>在办任务 {{ p.task_total - p.task_done }}</span>
                    <span>已完成 {{ p.task_done }}</span>
                    <span>成员 {{ p.member_count }}</span>
                    <span :class="p.open_risks > 0 ? 'text-accent' : ''">未关闭风险 {{ p.open_risks }}</span>
                    <span class="ml-auto text-ink-faint">详情 →</span>
                </div>
            </article>
        </div>

        <Modal :open="showCreate" title="新建项目" width="620px" @close="showCreate = false">
            <div class="grid gap-3 sm:grid-cols-2">
                <label class="flex flex-col gap-1">
                    <span class="label !mb-0">项目编号 <b class="text-accent">*</b></span>
                    <input v-model="form.code" class="field" placeholder="如 XM-2026-001" />
                </label>
                <label class="flex flex-col gap-1">
                    <span class="label !mb-0">项目名称 <b class="text-accent">*</b></span>
                    <input v-model="form.name" class="field" placeholder="合同名称 / 产品名称" />
                </label>
                <label class="flex flex-col gap-1">
                    <span class="label !mb-0">项目类型 <b class="text-accent">*</b></span>
                    <select v-model="form.type" class="field">
                        <option v-for="t in PROJECT_TYPES" :key="t" :value="t">{{ t }}</option>
                    </select>
                </label>
                <label class="flex flex-col gap-1">
                    <span class="label !mb-0">客户</span>
                    <input v-model="form.customer" class="field" placeholder="选填" />
                </label>
                <label class="flex flex-col gap-1">
                    <span class="label !mb-0">合同金额（万元）</span>
                    <input v-model="form.contract_amount" class="field" type="number" step="0.01" placeholder="选填" />
                </label>
            </div>

            <div class="mt-4">
                <p class="label">项目经理 <b class="text-accent">*</b>（可多选，职位含「项目经理」）</p>
                <div class="mt-1 flex flex-wrap gap-2">
                    <button
                        v-for="m in managers"
                        :key="m.id"
                        class="border px-2.5 py-1 text-[12px] transition"
                        :class="form.manager_ids.includes(m.id) ? 'border-ink bg-ink text-white' : 'border-line bg-white text-ink-soft hover:border-ink/30'"
                        type="button"
                        @click="toggleManager(m.id)"
                    >
                        {{ m.name }}
                    </button>
                    <span v-if="!managers.length" class="text-[11.5px] text-ink-faint">
                        {{ managerError || "没有可用的项目经理（请先同步通讯录）" }}
                    </span>
                </div>
            </div>

            <p class="mt-3 text-[11px] leading-relaxed text-ink-faint">
                保存后按项目类型自动生成阶段里程碑；计划、成员、预算后续在详情页补。
            </p>
            <p v-if="createError" class="mt-3 border border-accent/30 bg-accent-soft/60 px-3 py-2 text-[12px] text-accent">
                {{ createError }}
            </p>

            <template #footer>
                <button class="btn-quiet" type="button" @click="showCreate = false">取消</button>
                <button class="btn-solid" type="button" :disabled="creating" @click="submitCreate">
                    {{ creating ? "保存中…" : "保存" }}
                </button>
            </template>
        </Modal>
    </section>
</template>


