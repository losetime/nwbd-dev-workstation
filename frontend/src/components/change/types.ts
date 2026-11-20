/** 变更表单模型与默认值。 */
export interface ChangeFormModel {
    type: string;
    source: string;
    content: string;
    reason: string;
    impact: string;
    budget_before: string;
    budget_after: string;
    plan_due_after: string;
    material_path: string;
}

/** 发起变更时可选择的项目。 */
export interface ProjectOption {
    id: string;
    name: string;
    budget: number | null;
    plan_end: string | null;
}

export function emptyChangeForm(budget = "", planEnd = ""): ChangeFormModel {
    return {
        type: "经费变更",
        source: "内部",
        content: "",
        reason: "",
        impact: "",
        budget_before: budget,
        budget_after: "",
        plan_due_after: planEnd,
        material_path: "",
    };
}
