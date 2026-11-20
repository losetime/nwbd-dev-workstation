/** 接口请求封装。调用时机：所有业务模块通过 @api 下的方法间接调用，业务代码不直接 fetch。 */

export interface FieldIssue {
    field: string;
    message: string;
}

export class ApiError extends Error {
    code: string;
    details: FieldIssue[];

    constructor(code: string, message: string, details: FieldIssue[] = []) {
        super(message);
        this.code = code;
        this.details = details;
    }
}

interface Envelope<T> {
    code: string;
    data?: T;
    message?: string;
    details?: FieldIssue[];
}

/**
 * 发起请求并解包统一响应结构。
 * 调用时机：各业务接口方法内部调用。
 */
export async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
    const headers = new Headers(init.headers);
    if (!(init.body instanceof FormData)) {
        headers.set("Content-Type", "application/json");
    }

    let response: Response;
    try {
        response = await fetch(path, { ...init, headers, credentials: "same-origin" });
    } catch {
        throw new ApiError("NETWORK_ERROR", "无法连接服务，请确认容器是否在运行");
    }

    const text = await response.text();
    let payload: Envelope<T> | null = null;
    if (text) {
        try {
            payload = JSON.parse(text) as Envelope<T>;
        } catch {
            throw new ApiError("BAD_RESPONSE", "服务返回了无法解析的内容");
        }
    }

    if (!response.ok || !payload || payload.code !== "OK") {
        const code = payload?.code ?? `HTTP_${response.status}`;
        const message = payload?.message ?? `请求失败（HTTP ${response.status}）`;
        throw new ApiError(code, message, payload?.details ?? []);
    }

    return payload.data as T;
}

/** 拼查询串，空值自动丢弃。 */
export function query<T extends object>(params: T): string {
    const search = new URLSearchParams();
    Object.entries(params as Record<string, unknown>).forEach(([key, value]) => {
        if (value === undefined || value === null || value === "") return;
        search.set(key, String(value));
    });
    const text = search.toString();
    return text ? `?${text}` : "";
}
