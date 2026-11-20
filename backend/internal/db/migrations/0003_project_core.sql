-- 0003 项目主档：项目、里程碑、成员、任务、工时、风险、变更（预留）、困难上报、审计。
-- 原型用名称关联，这里全部改为 ID 外键；名称仅作展示冗余。

CREATE TABLE project (
    id               VARCHAR(26) PRIMARY KEY,
    code             VARCHAR(64) NOT NULL UNIQUE,
    name             VARCHAR(200) NOT NULL UNIQUE,
    type             VARCHAR(32) NOT NULL
        CHECK (type IN ('合同实施项目', '售前项目', '自研项目', '售后项目')),
    status           VARCHAR(16) NOT NULL DEFAULT '未开始'
        CHECK (status IN ('未开始', '进行中', '暂停中', '已结束', '已取消', '售后中')),
    customer         VARCHAR(200),
    current_stage    VARCHAR(64),
    current_milestone VARCHAR(128),
    milestone_status VARCHAR(16),
    plan_start       DATE,
    plan_end         DATE,
    actual_end       DATE,
    contract_amount  NUMERIC(12, 2),
    budget           NUMERIC(12, 2),
    cost_actual      NUMERIC(12, 2),
    labor_budget     NUMERIC(12, 2),
    labor_rate       NUMERIC(12, 2),
    risk_flag        VARCHAR(16)
        CHECK (risk_flag IS NULL OR risk_flag IN ('高风险', '低风险', '正常')),
    remark           TEXT,
    weekly_summary   TEXT,
    summary_at       TIMESTAMPTZ,
    plan_doc_key     VARCHAR(512),
    plan_doc_name    VARCHAR(255),
    budget_doc_key   VARCHAR(512),
    budget_doc_name  VARCHAR(255),
    created_by       VARCHAR(26) REFERENCES app_user (id),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ix_project_type_status ON project (type, status);

CREATE TABLE milestone (
    id            VARCHAR(26) PRIMARY KEY,
    project_id    VARCHAR(26) NOT NULL REFERENCES project (id) ON DELETE CASCADE,
    stage         VARCHAR(64) NOT NULL,
    name          VARCHAR(128) NOT NULL,
    plan_due      DATE,
    contract_due  DATE,
    actual_done   DATE,
    status        VARCHAR(16) NOT NULL DEFAULT '未开始'
        CHECK (status IN ('未开始', '进行中', '已完成', '已延期')),
    reviewer      VARCHAR(64),
    review_date   DATE,
    version       INT NOT NULL DEFAULT 1,
    locked        BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order    INT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, stage)
);

CREATE INDEX ix_milestone_project ON milestone (project_id, sort_order);

CREATE TABLE project_member (
    id           VARCHAR(26) PRIMARY KEY,
    project_id   VARCHAR(26) NOT NULL REFERENCES project (id) ON DELETE CASCADE,
    user_id      VARCHAR(26) NOT NULL REFERENCES app_user (id),
    project_role VARCHAR(16) NOT NULL
        CHECK (project_role IN ('项目经理', '软件开发', '硬件开发', '测试', 'UI设计')),
    allocation   INT NOT NULL DEFAULT 100,
    status       VARCHAR(8) NOT NULL DEFAULT '参与中'
        CHECK (status IN ('参与中', '已退出')),
    joined_at    DATE NOT NULL DEFAULT CURRENT_DATE,
    left_at      DATE,
    UNIQUE (project_id, user_id)
);

CREATE INDEX ix_project_member_user ON project_member (user_id, status);

CREATE TABLE task (
    id             VARCHAR(26) PRIMARY KEY,
    project_id     VARCHAR(26) NOT NULL REFERENCES project (id) ON DELETE CASCADE,
    stage          VARCHAR(64),
    name           VARCHAR(200) NOT NULL,
    owner_id       VARCHAR(26) NOT NULL REFERENCES app_user (id),
    job_type       VARCHAR(16),
    plan_start     DATE,
    plan_due       DATE,
    actual_done    DATE,
    status         VARCHAR(16) NOT NULL DEFAULT '未开始'
        CHECK (status IN ('未开始', '进行中', '已完成', '已延期', '已取消')),
    estimate_hours NUMERIC(8, 2) NOT NULL DEFAULT 0,
    actual_hours   NUMERIC(8, 2) NOT NULL DEFAULT 0,
    created_by     VARCHAR(26) REFERENCES app_user (id),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ix_task_project ON task (project_id);
CREATE INDEX ix_task_owner_status ON task (owner_id, status);

CREATE TABLE timesheet (
    id         VARCHAR(26) PRIMARY KEY,
    work_date  DATE NOT NULL,
    project_id VARCHAR(26) NOT NULL REFERENCES project (id) ON DELETE CASCADE,
    task_id    VARCHAR(26) NOT NULL REFERENCES task (id) ON DELETE CASCADE,
    user_id    VARCHAR(26) NOT NULL REFERENCES app_user (id),
    job_type   VARCHAR(16),
    hours      NUMERIC(5, 2) NOT NULL CHECK (hours > 0),
    rate       NUMERIC(12, 2) NOT NULL DEFAULT 0,
    cost       NUMERIC(14, 2) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (task_id, user_id, work_date)
);

CREATE INDEX ix_timesheet_user_date ON timesheet (user_id, work_date);
CREATE INDEX ix_timesheet_project ON timesheet (project_id, work_date);

CREATE TABLE risk (
    id             VARCHAR(26) PRIMARY KEY,
    code           VARCHAR(32) UNIQUE,
    project_id     VARCHAR(26) NOT NULL REFERENCES project (id) ON DELETE CASCADE,
    title          VARCHAR(200) NOT NULL,
    description    TEXT,
    type           VARCHAR(16) NOT NULL DEFAULT '其他'
        CHECK (type IN ('资源', '进度', '经费', '客户', '技术', '其他')),
    level          VARCHAR(8) NOT NULL DEFAULT '中'
        CHECK (level IN ('高', '中', '低')),
    impact         TEXT,
    action         TEXT,
    owner_id       VARCHAR(26) REFERENCES app_user (id),
    target_date    DATE,
    resolved_date  DATE,
    status         VARCHAR(16) NOT NULL DEFAULT '监控中'
        CHECK (status IN ('监控中', '已解决', '已发生')),
    raised_by      VARCHAR(26) REFERENCES app_user (id),
    updated_by     VARCHAR(26) REFERENCES app_user (id),
    identified_on  DATE NOT NULL DEFAULT CURRENT_DATE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ix_risk_project_status ON risk (project_id, status);

-- 变更：本期不做审批流程，仅预留表结构与钉钉审批实例的关联字段
CREATE TABLE change (
    id                   VARCHAR(26) PRIMARY KEY,
    code                 VARCHAR(32) NOT NULL UNIQUE,
    project_id           VARCHAR(26) NOT NULL REFERENCES project (id) ON DELETE CASCADE,
    type                 VARCHAR(32) NOT NULL
        CHECK (type IN ('里程碑变更', '任务计划变更', '预算变更', '范围变更', '放弃跟进')),
    source               VARCHAR(16) NOT NULL DEFAULT '内部变更'
        CHECK (source IN ('内部变更', '外部变更')),
    before_value         TEXT,
    after_value          TEXT,
    reason               TEXT NOT NULL,
    status               VARCHAR(24) NOT NULL DEFAULT '待审批',
    applicant_id         VARCHAR(26) REFERENCES app_user (id),
    applied_on           DATE NOT NULL DEFAULT CURRENT_DATE,
    dingtalk_process_code VARCHAR(64),
    dingtalk_instance_id  VARCHAR(64),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ix_change_project ON change (project_id, status);

CREATE TABLE issue (
    id           VARCHAR(26) PRIMARY KEY,
    code         VARCHAR(32) NOT NULL UNIQUE,
    project_id   VARCHAR(26) NOT NULL REFERENCES project (id) ON DELETE CASCADE,
    task_id      VARCHAR(26) REFERENCES task (id) ON DELETE SET NULL,
    title        VARCHAR(200) NOT NULL,
    description  TEXT,
    reporter_id  VARCHAR(26) NOT NULL REFERENCES app_user (id),
    job_type     VARCHAR(16),
    severity     VARCHAR(16) NOT NULL DEFAULT '一般'
        CHECK (severity IN ('一般', '较严重', '紧急阻塞')),
    status       VARCHAR(16) NOT NULL DEFAULT '待PM查看'
        CHECK (status IN ('待PM查看', '已登记风险', '已忽略归档')),
    handling     TEXT,
    risk_id      VARCHAR(26) REFERENCES risk (id) ON DELETE SET NULL,
    reported_on  DATE NOT NULL DEFAULT CURRENT_DATE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ix_issue_project_status ON issue (project_id, status);

CREATE TABLE audit_log (
    id          VARCHAR(26) PRIMARY KEY,
    actor_id    VARCHAR(26),
    actor_name  VARCHAR(128),
    action      VARCHAR(64) NOT NULL,
    target_type VARCHAR(32) NOT NULL,
    target_id   VARCHAR(26),
    detail      JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ix_audit_target ON audit_log (target_type, target_id);
