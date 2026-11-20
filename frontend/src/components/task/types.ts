/** 任务表单的共享模型：下发任务与编辑任务共用一套字段。 */
import type { TaskSkill } from "@/@api/tasks";

/** 「项目策划」的字典编码：选它要发起 04 钉钉审批（仅部门经理）。 */
export const PROJECT_PLAN_CODE = "TT01";

export interface TaskFormModel {
    task_type_code: string;
    contract_instance_id: string;
    source: string;
    name: string;
    description: string;
    acceptance: string;
    priority: string;
    skills: TaskSkill[];
    stage: string;
    owner_id: string;
    plan_start: string;
    plan_due: string;
    estimate_hours: string;
}

/** 负责人下拉项（本项目参与中成员）。 */
export interface MemberOption {
    user_id: string;
    name: string;
    project_role: string;
}

/** 空表单；stage 默认取第一个阶段。 */
export function emptyTaskForm(stage = ""): TaskFormModel {
    return {
        task_type_code: "",
        contract_instance_id: "",
        source: "中标合同",
        name: "",
        description: "",
        acceptance: "",
        priority: "P1",
        skills: [],
        stage,
        owner_id: "",
        plan_start: "",
        plan_due: "",
        estimate_hours: "",
    };
}

/** 验收标准文本 → 数组。 */
export function parseAcceptance(text: string): string[] {
    return text
        .split("\n")
        .map((s) => s.trim())
        .filter(Boolean);
}

/** 数组 → 验收标准文本。 */
export function acceptanceText(lines: string[]): string {
    return (lines ?? []).join("\n");
}
