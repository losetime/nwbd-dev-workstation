<script setup lang="ts">
/** 系统管理：人员管理与智能体管理（任务类型）两个页签。 */
import { RouterLink, RouterView, useRoute } from "vue-router";

const route = useRoute();

const tabs = [
    { label: "人员管理", to: "/admin/users" },
    { label: "智能体管理", to: "/admin/task-types" },
];

function active(to: string): boolean {
    return route.path.startsWith(to);
}
</script>

<template>
    <div class="mx-auto max-w-[1180px]">
        <div class="flex items-center gap-1 border-b border-line">
            <RouterLink
                v-for="t in tabs"
                :key="t.to"
                :to="t.to"
                class="px-3 py-2 text-[12.5px] transition-colors"
                :class="active(t.to) ? 'border-b-2 border-accent text-ink' : 'text-ink-soft hover:text-ink'"
            >
                {{ t.label }}
            </RouterLink>
        </div>
        <RouterView />
    </div>
</template>
