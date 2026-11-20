-- 0005 任务看板：把任务从「一行表格」升级为可跟踪的工作单元。
-- 参考 nwbd-automation-demo 的任务模型：描述、验收标准、优先级、标签、时间线、执行结果。
-- skill 字段预留，本期不接技能执行。

ALTER TABLE task
    ADD COLUMN description      TEXT,
    ADD COLUMN acceptance       TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN priority         VARCHAR(4) NOT NULL DEFAULT 'P1'
        CHECK (priority IN ('P0', 'P1', 'P2')),
    ADD COLUMN tags             TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN links            JSONB NOT NULL DEFAULT '[]',
    ADD COLUMN result_summary   TEXT,
    ADD COLUMN result_artifacts TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN result_agent     VARCHAR(64),
    ADD COLUMN finished_at      TIMESTAMPTZ,
    ADD COLUMN skill_name       VARCHAR(128),
    ADD COLUMN skill_version    INT;

CREATE INDEX ix_task_priority ON task (priority);

CREATE TABLE task_event (
    id         VARCHAR(26) PRIMARY KEY,
    task_id    VARCHAR(26) NOT NULL REFERENCES task (id) ON DELETE CASCADE,
    type       VARCHAR(24) NOT NULL,
    message    TEXT NOT NULL,
    actor_id   VARCHAR(26),
    actor_name VARCHAR(128),
    level      VARCHAR(8) NOT NULL DEFAULT 'info',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ix_task_event_task ON task_event (task_id, created_at);
