-- 0007 执行次数：用「任务第 N 次执行」替代 task_run。
-- 每次开始/重新打开任务，attempt_count +1，时间线事件带上 attempt。

ALTER TABLE task ADD COLUMN attempt_count INT NOT NULL DEFAULT 0;
ALTER TABLE task_event ADD COLUMN attempt INT NOT NULL DEFAULT 0;

ALTER TABLE task_artifact DROP COLUMN IF EXISTS run_id;
ALTER TABLE task DROP COLUMN IF EXISTS current_run_id;
DROP TABLE IF EXISTS task_run;
