/** 项目相关的展示工具。调用时机：列表卡片与详情页共用，避免各页各写一套。 */

export const STAGES: Record<string, string[]> = {
    合同实施项目: ["项目策划", "需求分析", "系统设计", "开发实现", "测试定版", "交付试用", "合同验收", "整改结项"],
    自研项目: ["项目策划", "需求分析", "系统设计", "开发实现", "测试定版", "交付试用"],
    售前项目: ["需求对接", "评估验证", "持续跟进"],
    售后项目: [],
};

export const PROJECT_TYPES = ["合同实施项目", "售前项目", "自研项目", "售后项目"];
export const PROJECT_STATUSES = ["未开始", "进行中", "暂停中", "已结束", "已取消", "售后中"];
export const RISK_FLAGS = ["高风险", "低风险", "正常"];

export const TYPE_SHORT: Record<string, string> = {
    合同实施项目: "合同",
    售前项目: "售前",
    自研项目: "自研",
    售后项目: "售后",
};

/** 日期只显示 YYYY-MM-DD。 */
export function fmtDate(value?: string | null): string {
    if (!value) return "";
    return value.slice(0, 10);
}

/** 金额显示：未填显示「—」，不拿 0 冒充。 */
export function fmtAmount(value?: number | null): string {
    if (value === null || value === undefined) return "—";
    return `${value}`;
}

/** 万元显示：未填只显示「—」，不拼单位。 */
export function fmtWan(value?: number | null): string {
    if (value === null || value === undefined) return "—";
    return `${value} 万`;
}

/** 状态标签配色。 */
export function statusTone(status: string): string {
    switch (status) {
        case "进行中":
            return "border-ink/20 bg-paper-deep text-ink";
        case "未开始":
            return "border-line bg-white text-ink-soft";
        case "暂停中":
            return "border-amber-500/40 bg-amber-50 text-amber-700";
        case "已结束":
            return "border-line bg-paper text-ink-faint";
        case "售后中":
            return "border-sky-500/40 bg-sky-50 text-sky-700";
        case "已取消":
            return "border-line bg-paper text-ink-faint line-through";
        default:
            return "border-line bg-white text-ink-soft";
    }
}

/** 风险标识配色。 */
export function riskTone(flag: string): string {
    switch (flag) {
        case "高风险":
            return "border-accent/40 bg-accent-soft text-accent";
        case "低风险":
            return "border-amber-500/40 bg-amber-50 text-amber-700";
        case "正常":
            return "border-emerald-500/30 bg-emerald-50 text-emerald-700";
        default:
            return "border-line bg-white text-ink-faint";
    }
}

/** 当前阶段在阶段序列中的下标；售后/外协返回 -1。 */
export function stageIndex(type: string, stage: string): number {
    const list = STAGES[type] ?? [];
    return list.indexOf(stage);
}

/** 相对时间：刚刚 / N 分钟前 / N 小时前 / N 天前。 */
export function relativeTime(value?: string | null): string {
    if (!value) return "";
    const t = new Date(value).getTime();
    if (Number.isNaN(t)) return "";
    const diff = Date.now() - t;
    if (diff < 60_000) return "刚刚";
    const min = Math.floor(diff / 60_000);
    if (min < 60) return `${min} 分钟前`;
    const hour = Math.floor(min / 60);
    if (hour < 24) return `${hour} 小时前`;
    const day = Math.floor(hour / 24);
    return `${day} 天前`;
}
