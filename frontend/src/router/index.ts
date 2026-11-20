/** 路由表。调用时机：应用启动时注册；除登录页外都要求已登录。 */

import { createRouter, createWebHistory } from "vue-router";

import { useAuth } from "@/store/auth";

const ProjectList = () => import("@/views/projects/ProjectList.vue");
const ProjectDetail = () => import("@/views/projects/ProjectDetail.vue");
const TaskBoard = () => import("@/views/tasks/TaskBoard.vue");
const TaskDetail = () => import("@/views/tasks/TaskDetail.vue");
const RiskList = () => import("@/views/risks/RiskList.vue");
const HelpList = () => import("@/views/help/HelpList.vue");
const ChangeList = () => import("@/views/changes/ChangeList.vue");
const AdminHome = () => import("@/views/admin/AdminHome.vue");
const Users = () => import("@/views/admin/Users.vue");
const TaskTypes = () => import("@/views/admin/TaskTypes.vue");
const Account = () => import("@/views/account/Account.vue");
const Login = () => import("@/views/login/Login.vue");

export const router = createRouter({
    history: createWebHistory(),
    routes: [
        { path: "/", name: "projects", component: ProjectList, meta: { title: "我的项目", perm: "myproj" } },
        { path: "/projects/:id", name: "project-detail", component: ProjectDetail, meta: { title: "项目详情", perm: "myproj" } },
        { path: "/tasks", name: "tasks", component: TaskBoard, meta: { title: "我的任务", perm: "mytask" } },
        { path: "/tasks/:id", name: "task-detail", component: TaskDetail, meta: { title: "任务详情", perm: "mytask" } },
        { path: "/risks", name: "risks", component: RiskList, meta: { title: "风险管理", perm: "myrisk" } },
        { path: "/help", name: "help", component: HelpList, meta: { title: "我要帮助", perm: "help" } },
        { path: "/changes", name: "changes", component: ChangeList, meta: { title: "变更管理", perm: "mychange" } },
        {
            path: "/admin",
            component: AdminHome,
            redirect: "/admin/users",
            meta: { title: "系统管理", perm: "sysadmin" },
            children: [
                { path: "users", name: "users", component: Users, meta: { title: "人员管理", perm: "sysadmin" } },
                { path: "task-types", name: "task-types", component: TaskTypes, meta: { title: "智能体管理", perm: "sysadmin" } },
            ],
        },
        { path: "/account", name: "account", component: Account, meta: { title: "用户信息" } },
        { path: "/login", name: "login", component: Login, meta: { title: "登录", bare: true, public: true } },
        { path: "/:pathMatch(.*)*", redirect: "/" },
    ],
});

router.beforeEach(async (to) => {
    if (to.meta.public) return true;
    const auth = useAuth();
    await auth.ensure();
    if (!auth.user.value) {
        return { name: "login", query: to.fullPath === "/" ? {} : { redirect: to.fullPath } };
    }
    const perm = to.meta.perm as string | undefined;
    if (perm && !auth.can(perm)) {
        return { name: "projects" };
    }
    return true;
});

router.afterEach((to) => {
    const title = (to.meta.title as string | undefined) ?? "工作台";
    document.title = `诺维研发管理 · ${title}`;
});
