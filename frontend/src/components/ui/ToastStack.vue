<script setup lang="ts">
/** 全局 Toast 容器：挂在 AppLayout，一次就够。 */
import { useToast, type ToastTone } from "@/composables/useToast";

const { items, dismiss } = useToast();

function barTone(tone: ToastTone): string {
    if (tone === "success") return "border-l-emerald-600";
    if (tone === "error") return "border-l-accent";
    return "border-l-ink/30";
}

function dotTone(tone: ToastTone): string {
    if (tone === "success") return "bg-emerald-600";
    if (tone === "error") return "bg-accent";
    return "bg-ink/40";
}
</script>

<template>
    <div class="pointer-events-none fixed bottom-6 right-6 z-[70] flex w-[min(360px,calc(100vw-3rem))] flex-col gap-2">
        <TransitionGroup name="toast" tag="div" class="flex flex-col gap-2">
            <button
                v-for="t in items"
                :key="t.id"
                class="pointer-events-auto flex w-full items-start gap-2.5 border border-l-2 border-line bg-white px-4 py-2.5 text-left text-[12.5px] leading-relaxed shadow-[0_10px_30px_rgba(18,19,22,0.10)]"
                :class="barTone(t.tone)"
                type="button"
                @click="dismiss(t.id)"
            >
                <span class="mt-[6px] h-1.5 w-1.5 shrink-0 rounded-full" :class="dotTone(t.tone)"></span>
                <span class="min-w-0 flex-1 text-ink">{{ t.message }}</span>
            </button>
        </TransitionGroup>
    </div>
</template>

<style scoped>
.toast-enter-active,
.toast-leave-active {
    transition: opacity 0.18s ease, transform 0.18s ease;
}
.toast-enter-from,
.toast-leave-to {
    opacity: 0;
    transform: translateY(6px);
}
</style>
