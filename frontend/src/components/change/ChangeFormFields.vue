<script setup lang="ts">
/** 变更表单字段（项目详情 / 变更管理页共用）。 */
import type { ChangeFormModel } from "./types";

defineProps<{ model: ChangeFormModel }>();
</script>

<template>
    <div class="grid gap-3 sm:grid-cols-2">
        <label class="flex flex-col gap-1">
            <span class="label !mb-0">变更类型</span>
            <select v-model="model.type" class="field">
                <option value="经费变更">经费变更</option>
                <option value="计划变更">计划变更</option>
                <option value="其他变更">其他变更</option>
            </select>
        </label>
        <label class="flex flex-col gap-1">
            <span class="label !mb-0">变更来源</span>
            <select v-model="model.source" class="field">
                <option value="内部">内部</option>
                <option value="外部">外部</option>
            </select>
        </label>
        <label class="flex flex-col gap-1 sm:col-span-2">
            <span class="label !mb-0">变更内容 <b class="text-accent">*</b></span>
            <textarea v-model="model.content" class="field h-16 resize-none" placeholder="要变更什么" />
        </label>
        <label class="flex flex-col gap-1 sm:col-span-2">
            <span class="label !mb-0">变更原因 <b class="text-accent">*</b></span>
            <textarea v-model="model.reason" class="field h-16 resize-none" placeholder="为什么要变更" />
        </label>
        <label class="flex flex-col gap-1 sm:col-span-2">
            <span class="label !mb-0">影响范围 <b class="text-accent">*</b></span>
            <textarea v-model="model.impact" class="field h-16 resize-none" placeholder="对进度、成本、范围的影响" />
        </label>
        <label class="flex flex-col gap-1">
            <span class="label !mb-0">项目实施预算（万元）</span>
            <input v-model="model.budget_before" class="field" type="number" step="0.01" />
        </label>
        <label class="flex flex-col gap-1">
            <span class="label !mb-0">变更后预算（万元）</span>
            <input v-model="model.budget_after" class="field" type="number" step="0.01" />
        </label>
        <label class="flex flex-col gap-1">
            <span class="label !mb-0">变更后交付日期</span>
            <input v-model="model.plan_due_after" class="field" type="date" />
        </label>
        <label class="flex flex-col gap-1">
            <span class="label !mb-0">变更材料位置</span>
            <input v-model="model.material_path" class="field" placeholder="服务器路径" />
        </label>
    </div>
</template>
