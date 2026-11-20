<script setup lang="ts">
/** 统计卡片：小标题 + 大数字。调用时机：列表页顶部的关键指标。 */
withDefaults(
    defineProps<{
        label: string;
        value: number | string;
        /** accent 用于需要警示的指标（逾期、高风险）。 */
        tone?: "default" | "accent" | "warn";
    }>(),
    { tone: "default" },
);
</script>

<template>
    <div class="shell-card px-4 py-3">
        <p class="text-[10.5px] tracking-[0.08em] text-ink-faint uppercase">{{ label }}</p>
        <p
            class="mt-1 font-display text-[24px] leading-none"
            :class="tone === 'accent' ? 'text-accent' : tone === 'warn' ? 'text-amber-700' : 'text-ink'"
        >
            {{ value }}
        </p>
    </div>
</template>
