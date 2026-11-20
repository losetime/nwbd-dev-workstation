<script setup lang="ts">
/** 通用确认框。调用时机：需要用户二次确认的危险/警告操作，替代系统原生 confirm。 */
import { onUnmounted, watch } from "vue";

const props = withDefaults(
    defineProps<{
        open: boolean;
        title?: string;
        message: string;
        confirmText?: string;
        cancelText?: string;
        tone?: "warn" | "danger";
        /** 只保留「确定」一个按钮（点遮罩仍可关闭）。 */
        showCancel?: boolean;
    }>(),
    { showCancel: true },
);

const emit = defineEmits<{ confirm: []; cancel: [] }>();

watch(
    () => props.open,
    (open) => {
        document.body.style.overflow = open ? "hidden" : "";
    },
    { immediate: true },
);

onUnmounted(() => {
    document.body.style.overflow = "";
});
</script>

<template>
    <div
        v-if="open"
        class="fixed inset-0 z-50 flex items-start justify-center bg-ink/35 px-4 py-24"
        @click.self="emit('cancel')"
    >
        <div class="w-full max-w-[460px] border border-line bg-white shadow-[0_24px_60px_rgba(18,19,22,0.18)]">
            <header class="border-b border-line px-5 py-3.5">
                <h3 class="font-display text-[15px] tracking-tight text-ink">{{ title || "确认操作" }}</h3>
            </header>

            <div class="px-5 py-4">
                <p
                    class="whitespace-pre-wrap border px-3 py-2.5 text-[12.5px] leading-6"
                    :class="tone === 'danger' ? 'border-accent/30 bg-accent-soft/60 text-accent' : tone === 'warn' ? 'border-amber-500/40 bg-amber-50 text-amber-800' : 'border-line bg-paper text-ink-soft'"
                >
                    {{ message }}
                </p>
            </div>

            <footer class="flex justify-end gap-2 border-t border-line px-5 py-3">
                <button v-if="showCancel" class="btn-quiet" type="button" @click="emit('cancel')">
                    {{ cancelText || "取消" }}
                </button>
                <button class="btn-solid" type="button" @click="emit('confirm')">{{ confirmText || "确定" }}</button>
            </footer>
        </div>
    </div>
</template>
