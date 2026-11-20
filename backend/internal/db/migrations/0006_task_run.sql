-- 0006 任务执行与产出：技能执行记录、产出物、工时确认所需字段。

CREATE TABLE task_run (
    id            VARCHAR(26) PRIMARY KEY,
    task_id       VARCHAR(26) NOT NULL REFERENCES task (id) ON DELETE CASCADE,
    agent_id      VARCHAR(26) REFERENCES app_user (id),
    agent_name    VARCHAR(128),
    skill_name    VARCHAR(128),
    skill_version INT,
    status        VARCHAR(16) NOT NULL DEFAULT 'running'
        CHECK (status IN ('running', 'done', 'failed')),
    summary       TEXT,
    started_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at   TIMESTAMPTZ
);

CREATE INDEX ix_task_run_task ON task_run (task_id, started_at DESC);

ALTER TABLE task ADD COLUMN current_run_id VARCHAR(26) REFERENCES task_run (id) ON DELETE SET NULL;

CREATE TABLE task_artifact (
    id           VARCHAR(26) PRIMARY KEY,
    task_id      VARCHAR(26) NOT NULL REFERENCES task (id) ON DELETE CASCADE,
    run_id       VARCHAR(26) REFERENCES task_run (id) ON DELETE SET NULL,
    name         VARCHAR(255) NOT NULL,
    object_key   VARCHAR(512) NOT NULL,
    size_bytes   BIGINT NOT NULL DEFAULT 0,
    content_type VARCHAR(128),
    uploaded_by  VARCHAR(26) REFERENCES app_user (id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ix_task_artifact_task ON task_artifact (task_id, created_at);
