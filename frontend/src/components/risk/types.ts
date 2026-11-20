/** 风险表单的共享模型：项目详情与风险管理页共用。 */
export const RISK_TYPES = ["资源", "进度", "经费", "客户", "技术", "其他"];
export const RISK_LEVELS = ["高", "中", "低"];
export const RISK_STATUSES = ["监控中", "已发生", "已解决"];

export interface RiskFormModel {
    id: string;
    title: string;
    type: string;
    level: string;
    impact: string;
    action: string;
    owner_id: string;
    target_date: string;
    status: string;
    resolved_date: string;
}

/** 责任人下拉项（项目参与中成员）。 */
export interface RiskMemberOption {
    user_id: string;
    name: string;
    project_role: string;
}

/** 发起登记时可选择的项目。 */
export interface RiskProjectOption {
    id: string;
    name: string;
}

export function emptyRiskForm(ownerId = ""): RiskFormModel {
    return {
        id: "",
        title: "",
        type: "其他",
        level: "中",
        impact: "",
        action: "",
        owner_id: ownerId,
        target_date: "",
        status: "监控中",
        resolved_date: "",
    };
}
