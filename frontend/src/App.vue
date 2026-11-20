<script setup lang="ts">
/** 根组件。调用时机：应用挂载时；登录页用整屏布局，其余页面套 AppLayout。 */
import { computed } from "vue";
import { RouterView, useRoute } from "vue-router";

import AppLayout from "@/layouts/AppLayout.vue";

const route = useRoute();
const bare = computed(() => route.meta.bare === true);
</script>

<template>
    <RouterView v-if="bare" />
    <AppLayout v-else />
</template>
