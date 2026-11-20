<script setup lang="ts">
/** 技能多选下拉。调用时机：下发 / 编辑任务时绑定多个技能；技能统一跟随技能库当前发布版本。 */
import { computed, onBeforeUnmount, onMounted, ref } from "vue";

import type { SkillItem, TaskSkill } from "@/@api/tasks";

const props = withDefaults(
    defineProps<{
        modelValue: TaskSkill[];
        options: SkillItem[];
        placeholder?: string;
        /** 任务类型带的技能名：锁定，不能取消。 */
        locked?: string[];
    }>(),
    { locked: () => [] },
);

const emit = defineEmits<{ "update:modelValue": [TaskSkill[]] }>();

const open = ref(false);
const query = ref("");
const root = ref<HTMLElement | null>(null);

const filtered = computed(() => {
    const q = query.value.trim().toLowerCase();
    if (!q) return props.options;
    return props.options.filter((s) => s.name.toLowerCase().includes(q));
});

function isSelected(name: string): boolean {
    return props.modelValue.some((s) => s.name === name);
}

function isLocked(name: string): boolean {
    return props.locked.includes(name);
}

function toggle(name: string): void {
    if (isLocked(name)) return;
    if (isSelected(name)) {
        emit(
            "update:modelValue",
            props.modelValue.filter((s) => s.name !== name),
        );
        return;
    }
    emit("update:modelValue", [...props.modelValue, { name, version: null }]);
}

function close(): void {
    open.value = false;
    query.value = "";
}

function onDocClick(e: MouseEvent): void {
    if (!open.value) return;
    if (root.value && !root.value.contains(e.target as Node)) close();
}

onMounted(() => document.addEventListener("click", onDocClick));
onBeforeUnmount(() => document.removeEventListener("click", onDocClick));
</script>

<template>
    <div ref="root" class="relative">
        <button
            type="button"
            class="field flex min-h-[36px] flex-wrap items-center gap-1 text-left"
            @click="open = !open"
        >
            <span v-if="!modelValue.length" class="text-ink-faint">{{ placeholder || "不绑定" }}</span>
            <span
                v-for="s in modelValue"
                :key="s.name"
                class="border border-line bg-paper px-1.5 py-0.5 font-mono text-[10.5px] text-ink-soft"
                :title="isLocked(s.name) ? '任务类型固定技能，不能删除' : ''"
            >
                {{ s.name }}<span v-if="isLocked(s.name)" class="ml-1 text-ink-faint">·固定</span>
            </span>
            <span class="ml-auto text-[10px] text-ink-faint">▾</span>
        </button>

        <div v-if="open" class="absolute z-30 mt-1 w-full border border-line bg-white shadow-[0_12px_30px_rgba(18,19,22,0.12)]">
            <input
                v-model="query"
                class="w-full border-b border-line px-2.5 py-1.5 text-[12px] outline-none"
                placeholder="搜索技能库…"
                type="text"
            />
            <div class="max-h-[220px] overflow-y-auto">
                <button
                    v-for="s in filtered"
                    :key="s.name"
                    class="flex w-full items-center gap-2 px-2.5 py-1.5 text-left text-[12px] hover:bg-paper"
                    :class="[isSelected(s.name) ? 'text-ink' : 'text-ink-soft', isLocked(s.name) ? 'cursor-default opacity-70' : '']"
                    type="button"
                    @click="toggle(s.name)"
                >
                    <span class="w-3 text-[11px] text-ink">{{ isLocked(s.name) ? "🔒" : isSelected(s.name) ? "✓" : "" }}</span>
                    <span class="min-w-0 flex-1 truncate font-mono">{{ s.name }}</span>
                </button>
                <p v-if="!filtered.length" class="px-2.5 py-2 text-[11.5px] text-ink-faint">没有匹配的技能</p>
            </div>
        </div>
    </div>
</template>
