/** 应用入口。调用时机：浏览器加载 index.html 后执行。 */
import { createApp } from "vue";

import App from "@/App.vue";
import { router } from "@/router";
import "@/style.css";

createApp(App).use(router).mount("#app");
