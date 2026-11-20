/** 全局轻提示（Toast）：成功 / 失败 / 普通信息，自动消失，不打断操作。 */
import { ref } from "vue";

export type ToastTone = "success" | "error" | "info";

export interface ToastItem {
    id: number;
    message: string;
    tone: ToastTone;
}

const items = ref<ToastItem[]>([]);
let seq = 0;

function dismiss(id: number): void {
    items.value = items.value.filter((t) => t.id !== id);
}

function push(message: string, tone: ToastTone): void {
    const text = message.trim();
    if (!text) return;
    const id = ++seq;
    items.value = [...items.value, { id, message: text, tone }];
    window.setTimeout(() => dismiss(id), tone === "error" ? 4200 : 2600);
}

export function useToast() {
    return {
        items,
        dismiss,
        success: (message: string) => push(message, "success"),
        error: (message: string) => push(message, "error"),
        info: (message: string) => push(message, "info"),
    };
}
