<script setup lang="ts">
/** 任务表单字段（下发任务 / 编辑任务共用同一套字段与布局）。 */
import { computed, watch } from "vue";

import type { ContractApproval } from "@/@api/approvals";
import type { SkillItem, TaskTypeOption } from "@/@api/tasks";
import SkillMultiSelect from "@/components/ui/SkillMultiSelect.vue";

import { PROJECT_PLAN_CODE, type MemberOption, type TaskFormModel } from "./types";

const props = withDefaults(
    defineProps<{
        model: TaskFormModel;
        stages: string[];
        members: MemberOption[];
        skills: SkillItem[];
        /** 任务类型字典（日常 + 项目任务）。 */
        taskTypes?: TaskTypeOption[];
        /** 编辑任务时，任务类型带的技能名（锁定，不可删）。 */
        lockedSkills?: string[];
        /** 下发任务时负责人必填，显示红星。 */
        ownerRequired?: boolean;
        /** 是否显示「任务类型」（仅新建任务时显示）。 */
        showTaskType?: boolean;
        /** 03 合同签订单（项目代号下拉）。 */
        contracts?: ContractApproval[];
        /** 项目已关联合同签订单时，不再显示下拉。 */
        contractLinked?: boolean;
        /** 已关联的项目代号（可能为空）。 */
        contractCode?: string;
    }>(),
    {
        taskTypes: () => [],
        lockedSkills: () => [],
        ownerRequired: false,
        showTaskType: false,
        contracts: () => [],
        contractLinked: false,
        contractCode: "",
    },
);

/** 按组别分组，下拉用 optgroup 展示。 */
const groups = computed(() => {
    const out: { name: string; items: TaskTypeOption[] }[] = [];
    for (const item of props.taskTypes) {
        let group = out.find((g) => g.name === item.group_name);
        if (!group) {
            group = { name: item.group_name, items: [] };
            out.push(group);
        }
        group.items.push(item);
    }
    return out;
});

const selected = computed(() => props.taskTypes.find((t) => t.code === props.model.task_type_code) || null);
const isProjectPlan = computed(() => selected.value?.name === "项目策划" || props.model.task_type_code === PROJECT_PLAN_CODE);

/** 类型带的技能锁定；编辑时由外部传入的也锁定。 */
const lockedNames = computed(() => {
    const names = new Set(props.lockedSkills);
    if (props.showTaskType && selected.value) {
        for (const s of selected.value.skills) names.add(s.name);
    }
    return [...names];
});

// 选「项目策划」时任务名称同步；切换类型时把默认技能带出来（类型带的锁死）
watch(
    () => props.model.task_type_code,
    (next, prev) => {
        if (next === PROJECT_PLAN_CODE) {
            props.model.name = "项目策划";
        } else if (prev === PROJECT_PLAN_CODE && props.model.name === "项目策划") {
            props.model.name = "";
        }
        if (props.showTaskType && next) {
            const type = props.taskTypes.find((t) => t.code === next);
            if (type) {
                props.model.skills = type.skills.map((s) => ({
                    name: s.name,
                    version: s.version,
                    source: "type" as const,
                }));
            }
        }
    },
);
</script>

<template>
    <div class="grid gap-3 sm:grid-cols-2">
        <label v-if="showTaskType" class="flex flex-col gap-1 sm:col-span-2">
            <span class="label !mb-0">任务类型 <b class="text-accent">*</b></span>
            <select v-model="model.task_type_code" class="field">
                <option value="">请选择任务类型</option>
                <optgroup v-for="g in groups" :key="g.name" :label="g.name">
                    <option v-for="t in g.items" :key="t.code" :value="t.code">{{ t.name }}</option>
                </optgroup>
            </select>
            <span v-if="selected" class="text-[10.5px] text-ink-faint">
                {{ selected.archive_stage }} · 产出归档到{{ selected.archive_target === "public" ? "公共空间" : "" }}「{{ selected.archive_folder }}」<template v-if="selected.archive_note">（{{ selected.archive_note }}）</template>
            </span>
        </label>
        <template v-if="isProjectPlan">
            <div v-if="contractLinked" class="flex flex-col gap-1 sm:col-span-2">
                <span class="label !mb-0">项目代号</span>
                <p class="border border-line bg-paper px-3.5 py-2.5 text-[12.5px] text-ink-soft">
                    已关联合同签订单：<span class="font-mono text-ink">{{ contractCode || "暂无" }}</span>
                </p>
            </div>
            <label v-else class="flex flex-col gap-1 sm:col-span-2">
                <span class="label !mb-0">项目代号 <b class="text-accent">*</b></span>
                <select v-model="model.contract_instance_id" class="field">
                    <option value="">请选择（来自 03 销售合同签订单）</option>
                    <option v-for="c in contracts" :key="c.instance_id" :value="c.instance_id">
                        {{ c.project_code }} · {{ c.contract_name || c.title }}
                        <template v-if="c.status !== 'COMPLETED'">（进行中）</template>
                    </option>
                </select>
                <span class="text-[10.5px] text-ink-faint">合同单没填项目代号的不会出现在这里，请先补合同单。</span>
            </label>
            <label class="flex flex-col gap-1 sm:col-span-2">
                <span class="label !mb-0">项目来源 <b class="text-accent">*</b></span>
                <select v-model="model.source" class="field">
                    <option value="中标合同">中标合同</option>
                    <option value="公司内部立项">公司内部立项</option>
                    <option value="顾客直接委托">顾客直接委托</option>
                    <option value="其他">其他</option>
                </select>
            </label>
        </template>
        <label class="flex flex-col gap-1 sm:col-span-2">
            <span class="label !mb-0">任务名称 <b v-if="ownerRequired" class="text-accent">*</b></span>
            <input v-model="model.name" class="field" placeholder="如 后端接口联调" type="text" />
        </label>
        <label class="flex flex-col gap-1 sm:col-span-2">
            <span class="label !mb-0">任务描述</span>
            <textarea v-model="model.description" class="field h-20 resize-none" placeholder="背景、输入材料、期望产出" />
        </label>
        <label class="flex flex-col gap-1 sm:col-span-2">
            <span class="label !mb-0">验收标准（每行一条）</span>
            <textarea v-model="model.acceptance" class="field h-20 resize-none" placeholder="接口文档已更新&#10;联调通过 3 个场景" />
        </label>
        <label class="flex flex-col gap-1">
            <span class="label !mb-0">优先级</span>
            <select v-model="model.priority" class="field">
                <option value="P0">P0</option>
                <option value="P1">P1</option>
                <option value="P2">P2</option>
            </select>
        </label>
        <label class="flex flex-col gap-1">
            <span class="label !mb-0">所属阶段</span>
            <select v-model="model.stage" class="field">
                <option value="">不指定</option>
                <option v-for="s in stages" :key="s" :value="s">{{ s }}</option>
            </select>
        </label>
        <label class="flex flex-col gap-1">
            <span class="label !mb-0">负责人 <b v-if="ownerRequired" class="text-accent">*</b></span>
            <select v-model="model.owner_id" class="field">
                <option value="">请选择（仅本项目参与中成员）</option>
                <option v-for="m in members" :key="m.user_id" :value="m.user_id">
                    {{ m.name }} · {{ m.project_role }}
                </option>
            </select>
        </label>
        <label class="flex flex-col gap-1">
            <span class="label !mb-0">计划开始</span>
            <input v-model="model.plan_start" class="field" type="date" />
        </label>
        <label class="flex flex-col gap-1">
            <span class="label !mb-0">计划完成</span>
            <input v-model="model.plan_due" class="field" type="date" />
        </label>
        <label class="flex flex-col gap-1">
            <span class="label !mb-0">预估工时（小时）</span>
            <input v-model="model.estimate_hours" class="field" type="number" step="0.5" />
        </label>
        <div class="flex flex-col gap-1 sm:col-span-2">
            <span class="label !mb-0">绑定技能</span>
            <SkillMultiSelect v-model="model.skills" :options="skills" :locked="lockedNames" placeholder="不绑定（可多选）" />
        </div>
    </div>
</template>
