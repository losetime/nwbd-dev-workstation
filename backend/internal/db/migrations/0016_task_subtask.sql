-- 0016 任务子任务：研发把 PM 下发的粗任务拆成 TODO 清单
--
-- 子任务只属于父任务，不进项目任务列表、不记工时、不产出、不走审批。

CREATE TABLE task_subtask (
    id         VARCHAR(26) PRIMARY KEY,
    task_id    VARCHAR(26) NOT NULL REFERENCES task (id) ON DELETE CASCADE,
    title      VARCHAR(200) NOT NULL,
    done       BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order INT NOT NULL DEFAULT 0,
    created_by VARCHAR(26) REFERENCES app_user (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    done_by    VARCHAR(26) REFERENCES app_user (id),
    done_at    TIMESTAMPTZ
);

CREATE INDEX ix_task_subtask_task ON task_subtask (task_id, sort_order, created_at);
