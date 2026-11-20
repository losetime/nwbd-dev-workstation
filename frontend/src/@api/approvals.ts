/** 钉钉审批相关接口。 */
import { request } from "@/@api/http";

export interface ContractApproval {
    instance_id: string;
    title: string;
    project_code: string;
    contract_name: string;
    customer: string;
    status: string;
    business_id: string;
}

/** 03 销售合同签订单（项目代号下拉数据源）。 */
export function listContractApprovals(): Promise<ContractApproval[]> {
    return request<{ items: ContractApproval[] }>("/api/approvals/contracts").then((d) => d.items);
}

export interface ApprovalTask {
    task_id: string;
    user_id: string;
    status: string;
    result: string;
    activity_id: string;
}

export interface ApprovalOperation {
    user_id: string;
    date: string;
    type: string;
    result: string;
    remark: string;
    show_name: string;
}

export interface TaskApproval {
    instance_id: string;
    title: string;
    status: string;
    raw_status: string;
    result: string;
    business_id: string;
    create_time: string;
    tasks: ApprovalTask[];
    operations: ApprovalOperation[];
}

/** 任务关联审批的实时状态（无审批时返回 null）。 */
export function getTaskApproval(taskId: string): Promise<TaskApproval | null> {
    return request<{ approval: TaskApproval | null }>(`/api/tasks/${taskId}/approval`).then((d) => d.approval);
}
