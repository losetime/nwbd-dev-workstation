-- 0017 记录执行结果属于第几次执行
--
-- 任务重新打开后，result_summary 仍保留上一次的内容；
-- 前端据此标注「上次执行结果（第 N 次）」，避免被当成当前结果。

ALTER TABLE task ADD COLUMN result_attempt INT;

UPDATE task
SET result_attempt = attempt_count
WHERE result_summary IS NOT NULL AND result_attempt IS NULL;
