<script setup lang="ts">
/** 风险表单字段（项目详情 / 风险管理页共用）。 */
import { RISK_LEVELS, RISK_STATUSES, RISK_TYPES, type RiskFormModel, type RiskMemberOption } from "./types";

withDefaults(
    defineProps<{
        model: RiskFormModel;
        members: RiskMemberOption[];
        /** 编辑已有风险时显示状态与解决日期。 */
        showStatus?: boolean;
    }>(),
    { showStatus: false, members: () => [] },
);
</script>

<template>
    <div class="grid gap-3 sm:grid-cols-2">
        <label class="flex flex-col gap-1 sm:col-span-2">
            <span class="label !mb-0">标题 <b class="text-accent">*</b></span>
            <input v-model="model.title" class="field" placeholder="如 第三方接口交付延期" />
        </label>
        <label class="flex flex-col gap-1">
            <span class="label !mb-0">类型</span>
            <select v-model="model.type" class="field">
                <option v-for="t in RISK_TYPES" :key="t" :value="t">{{ t }}</option>
            </select>
        </label>
        <label class="flex flex-col gap-1">
            <span class="label !mb-0">等级</span>
            <select v-model="model.level" class="field">
                <option v-for="l in RISK_LEVELS" :key="l" :value="l">{{ l }}</option>
            </select>
        </label>
        <label class="flex flex-col gap-1 sm:col-span-2">
            <span class="label !mb-0">影响描述</span>
            <textarea v-model="model.impact" class="field h-16 resize-none" placeholder="会发生什么后果" />
        </label>
        <label class="flex flex-col gap-1 sm:col-span-2">
            <span class="label !mb-0">应对措施</span>
            <textarea v-model="model.action" class="field h-16 resize-none" placeholder="打算怎么处理" />
        </label>
        <label class="flex flex-col gap-1">
            <span class="label !mb-0">责任人</span>
            <select v-model="model.owner_id" class="field">
                <option value="">未指定</option>
                <option v-for="m in members" :key="m.user_id" :value="m.user_id">
                    {{ m.name }} · {{ m.project_role }}
                </option>
            </select>
        </label>
        <label class="flex flex-col gap-1">
            <span class="label !mb-0">目标解决日</span>
            <input v-model="model.target_date" class="field" type="date" />
        </label>
        <template v-if="showStatus">
            <label class="flex flex-col gap-1">
                <span class="label !mb-0">状态</span>
                <select v-model="model.status" class="field">
                    <option v-for="s in RISK_STATUSES" :key="s" :value="s">{{ s }}</option>
                </select>
            </label>
            <label v-if="model.status === '已解决'" class="flex flex-col gap-1">
                <span class="label !mb-0">解决日期 <b class="text-accent">*</b></span>
                <input v-model="model.resolved_date" class="field" type="date" />
            </label>
        </template>
    </div>
</template>
