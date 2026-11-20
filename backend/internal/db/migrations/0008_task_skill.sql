-- 0008：任务改为可绑定多个技能
-- 一个任务可绑多个技能，每个技能可指定版本（NULL = 跟随最新）。
CREATE TABLE IF NOT EXISTS task_skill (
    id            VARCHAR(26) PRIMARY KEY,
    task_id       VARCHAR(26) NOT NULL REFERENCES task (id) ON DELETE CASCADE,
    skill_name    VARCHAR(120) NOT NULL,
    skill_version INTEGER,
    sort_order    INTEGER NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (task_id, skill_name)
);

CREATE INDEX IF NOT EXISTS idx_task_skill_task ON task_skill (task_id);

-- 老数据回填：task.skill_name 有值的写成一条
INSERT INTO task_skill (id, task_id, skill_name, skill_version, sort_order)
SELECT substr(md5(t.id || t.skill_name), 1, 26), t.id, t.skill_name, t.skill_version, 0
FROM task t
WHERE COALESCE(t.skill_name, '') <> ''
ON CONFLICT DO NOTHING;
