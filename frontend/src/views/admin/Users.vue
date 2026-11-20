<script setup lang="ts">
/** 系统管理 → 人员。调用时机：部门经理进入；岗位映射自钉钉职位，这里可人工校正。 */
import { computed, onMounted, ref } from "vue";

import {
    JOB_TYPES,
    LEVELS,
    PERM_OPTIONS,
    listPeople,
    syncPeople,
    updatePerson,
    type Person,
} from "@/@api/users";
import Modal from "@/components/ui/Modal.vue";
import { useToast } from "@/composables/useToast";

const toast = useToast();
const loading = ref(true);
const error = ref("");
const people = ref<Person[]>([]);
const filters = ref({ keyword: "", job_type: "" });
const syncing = ref(false);

const stats = computed(() => {
    const byJob: Record<string, number> = {};
    people.value.forEach((p) => {
        byJob[p.job_type] = (byJob[p.job_type] ?? 0) + 1;
    });
    return { total: people.value.length, byJob };
});

async function load(): Promise<void> {
    loading.value = true;
    error.value = "";
    try {
        people.value = await listPeople({
            keyword: filters.value.keyword || undefined,
            job_type: filters.value.job_type || undefined,
        });
    } catch (err) {
        error.value = err instanceof Error ? err.message : "加载失败";
    } finally {
        loading.value = false;
    }
}

async function doSync(): Promise<void> {
    syncing.value = true;
    try {
        const result = await syncPeople();
        toast.success(`通讯录同步完成：${result.synced}/${result.total}`);
        await load();
    } catch (err) {
        toast.error(err instanceof Error ? err.message : "同步失败");
    } finally {
        syncing.value = false;
    }
}

// ---------- 编辑 ----------
const editing = ref<Person | null>(null);
const saving = ref(false);
const editError = ref("");
const form = ref({ job_type: "开发", level: "中级", status: "在职", perms: [] as string[] });

function openEdit(p: Person): void {
    editing.value = p;
    editError.value = "";
    form.value = {
        job_type: p.job_type,
        level: p.level,
        status: p.status,
        perms: [...p.perms],
    };
}

function togglePerm(key: string): void {
    const list = form.value.perms;
    const idx = list.indexOf(key);
    if (idx >= 0) list.splice(idx, 1);
    else list.push(key);
}

function resetPerms(): void {
    form.value.perms = [];
}

async function save(): Promise<void> {
    if (!editing.value) return;
    saving.value = true;
    editError.value = "";
    try {
        await updatePerson(editing.value.id, {
            job_type: form.value.job_type,
            level: form.value.level,
            status: form.value.status,
            perms: form.value.perms,
        });
        editing.value = null;
        await load();
    } catch (err) {
        editError.value = err instanceof Error ? err.message : "保存失败";
    } finally {
        saving.value = false;
    }
}

function jobBadge(job: string): string {
    switch (job) {
        case "部门经理":
            return "border-accent/40 bg-accent-soft text-accent";
        case "项目经理":
            return "border-sky-500/30 bg-sky-50 text-sky-700";
        case "开发":
            return "border-line bg-paper text-ink-soft";
        case "测试":
            return "border-emerald-500/30 bg-emerald-50 text-emerald-700";
        case "UI":
            return "border-violet-500/30 bg-violet-50 text-violet-700";
        default:
            return "border-line bg-white text-ink-faint";
    }
}

onMounted(load);
</script>

<template>
    <section class="mx-auto max-w-[1180px]">
        <header class="flex flex-wrap items-end justify-between gap-4">
            <div>
                <h2 class="font-display text-[26px] leading-tight tracking-tight text-ink">人员管理</h2>
                <p class="mt-1.5 text-[12.5px] text-ink-faint">
                    在职 {{ stats.total }} 人 · 岗位由钉钉职位映射，可人工校正
                </p>
            </div>
            <button class="btn-quiet" type="button" :disabled="syncing" @click="doSync">
                {{ syncing ? "同步中…" : "同步通讯录" }}
            </button>
        </header>

        <div class="mt-4 flex flex-wrap gap-2">
            <span v-for="(count, job) in stats.byJob" :key="job" class="border px-2.5 py-1 text-[11.5px]" :class="jobBadge(String(job))">
                {{ job }} {{ count }}
            </span>
        </div>

        <div class="mt-4 flex flex-wrap items-end gap-3 border border-line bg-white px-4 py-3">
            <label class="flex flex-col gap-1">
                <span class="text-[10px] tracking-[0.08em] text-ink-faint uppercase">姓名 / 职位</span>
                <input v-model="filters.keyword" class="field !py-1.5 !text-[12.5px] w-[180px]" @keyup.enter="load" />
            </label>
            <label class="flex flex-col gap-1">
                <span class="text-[10px] tracking-[0.08em] text-ink-faint uppercase">岗位</span>
                <select v-model="filters.job_type" class="field !py-1.5 !text-[12.5px] w-[120px]" @change="load">
                    <option value="">全部</option>
                    <option v-for="j in JOB_TYPES" :key="j" :value="j">{{ j }}</option>
                </select>
            </label>
            <button class="btn-quiet !py-1.5" type="button" @click="load">查询</button>
        </div>

        <p v-if="error" class="mt-3 border border-accent/30 bg-accent-soft/60 px-4 py-2 text-[12.5px] text-accent">{{ error }}</p>

        <div class="mt-4 shell-card overflow-x-auto">
            <table class="w-full text-[12.5px]">
                <thead class="bg-paper text-left text-[10.5px] tracking-[0.06em] text-ink-faint uppercase">
                    <tr>
                        <th class="px-4 py-2.5 font-medium">姓名</th>
                        <th class="px-4 py-2.5 font-medium">钉钉职位</th>
                        <th class="px-4 py-2.5 font-medium">部门</th>
                        <th class="px-4 py-2.5 font-medium">岗位</th>
                        <th class="px-4 py-2.5 font-medium">级别</th>
                        <th class="px-4 py-2.5 font-medium">状态</th>
                        <th class="px-4 py-2.5 font-medium">可访问界面</th>
                        <th class="px-4 py-2.5 font-medium"></th>
                    </tr>
                </thead>
                <tbody class="divide-y divide-line">
                    <tr v-if="loading">
                        <td colspan="8" class="px-4 py-10 text-center text-ink-faint">正在加载…</td>
                    </tr>
                    <tr v-else-if="!people.length">
                        <td colspan="8" class="px-4 py-10 text-center text-ink-faint">没有匹配的人员</td>
                    </tr>
                    <tr v-for="p in people" v-else :key="p.id" class="hover:bg-paper/60">
                        <td class="px-4 py-2.5">
                            <span class="text-ink">{{ p.name }}</span>
                            <span v-if="p.role === 'admin' || p.role === 'sub_admin'" class="ml-1.5 text-[10px] text-ink-faint">钉钉管理员</span>
                        </td>
                        <td class="px-4 py-2.5 text-ink-soft">{{ p.title || "—" }}</td>
                        <td class="px-4 py-2.5 text-ink-soft">{{ p.dept || "—" }}</td>
                        <td class="px-4 py-2.5">
                            <span class="border px-1.5 py-0.5 text-[11px]" :class="jobBadge(p.job_type)">{{ p.job_type }}</span>
                        </td>
                        <td class="px-4 py-2.5 text-ink-soft">{{ p.level }}</td>
                        <td class="px-4 py-2.5" :class="p.status === '离职' ? 'text-ink-faint' : 'text-ink-soft'">{{ p.status }}</td>
                        <td class="px-4 py-2.5">
                            <span class="text-[11.5px] text-ink-soft">
                                {{ p.custom ? "自定义 " + p.perms.length + " 个" : "岗位默认 " + p.perms.length + " 个" }}
                            </span>
                        </td>
                        <td class="px-4 py-2.5 text-right">
                            <button class="btn-ghost !py-1 text-[11.5px]" type="button" @click="openEdit(p)">编辑</button>
                        </td>
                    </tr>
                </tbody>
            </table>
        </div>

        <Modal :open="!!editing" :title="`编辑人员 · ${editing?.name ?? ''}`" width="560px" @close="editing = null">
            <div class="grid gap-3 sm:grid-cols-3">
                <label class="flex flex-col gap-1">
                    <span class="label !mb-0">岗位</span>
                    <select v-model="form.job_type" class="field">
                        <option v-for="j in JOB_TYPES" :key="j" :value="j">{{ j }}</option>
                    </select>
                </label>
                <label class="flex flex-col gap-1">
                    <span class="label !mb-0">级别</span>
                    <select v-model="form.level" class="field">
                        <option v-for="l in LEVELS" :key="l" :value="l">{{ l }}</option>
                    </select>
                </label>
                <label class="flex flex-col gap-1">
                    <span class="label !mb-0">状态</span>
                    <select v-model="form.status" class="field">
                        <option value="在职">在职</option>
                        <option value="离职">离职</option>
                    </select>
                </label>
            </div>

            <div class="mt-4">
                <div class="flex items-center justify-between">
                    <p class="label !mb-0">界面权限</p>
                    <button class="btn-ghost !py-0.5 text-[11px]" type="button" @click="resetPerms">恢复岗位默认</button>
                </div>
                <div class="mt-2 grid grid-cols-2 gap-1.5 sm:grid-cols-3">
                    <label
                        v-for="opt in PERM_OPTIONS"
                        :key="opt.key"
                        class="flex cursor-pointer items-center gap-2 border border-line px-2.5 py-1.5 text-[12px]"
                        :class="form.perms.includes(opt.key) ? 'bg-paper text-ink' : 'bg-white text-ink-soft'"
                    >
                        <input
                            type="checkbox"
                            :checked="form.perms.includes(opt.key)"
                            @change="togglePerm(opt.key)"
                        />
                        {{ opt.label }}
                    </label>
                </div>
                <p class="mt-2 text-[10.5px] leading-relaxed text-ink-faint">
                    全部取消勾选 = 按岗位默认。岗位「其他」没有任何默认页面，登录后会看到提示页。
                </p>
            </div>

            <p v-if="editError" class="mt-3 border border-accent/30 bg-accent-soft/60 px-3 py-2 text-[12px] text-accent">
                {{ editError }}
            </p>

            <template #footer>
                <button class="btn-quiet" type="button" @click="editing = null">取消</button>
                <button class="btn-solid" type="button" :disabled="saving" @click="save">{{ saving ? "保存中…" : "保存" }}</button>
            </template>
        </Modal>
    </section>
</template>
