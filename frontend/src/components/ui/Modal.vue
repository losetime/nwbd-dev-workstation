<script setup lang="ts">
/** 通用弹窗。调用时机：新建项目、编辑人员等需要表单的场景；打开时锁定页面滚动。 */
import { onUnmounted, watch } from "vue";

const props = defineProps<{ open: boolean; title: string; width?: string }>();
defineEmits<{ close: [] }>();

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
        class="fixed inset-0 z-50 overflow-y-auto bg-ink/35 px-4 py-10"
        @click.self="$emit('close')"
    >
        <div class="mx-auto w-full border border-line bg-white shadow-[0_24px_60px_rgba(18,19,22,0.18)]" :style="{ maxWidth: width || '560px' }">
            <header class="flex items-center justify-between border-b border-line px-5 py-3.5">
                <h3 class="font-display text-[15px] tracking-tight text-ink">{{ title }}</h3>
                <button class="btn-ghost !px-2 !py-0.5 text-[16px] leading-none" type="button" @click="$emit('close')">
                    ×
                </button>
            </header>
            <div class="px-5 py-4">
                <slot />
            </div>
            <footer v-if="$slots.footer" class="flex justify-end gap-2 border-t border-line px-5 py-3">
                <slot name="footer" />
            </footer>
        </div>
    </div>
</template>
