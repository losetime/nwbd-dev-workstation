-- 0018 历史「已重新打开」的任务，结果轮次不可靠，置空
--
-- 0017 回填时用 attempt_count，但任务被重新打开过的，attempt_count 已经往前走，
-- 无法确定结果属于哪一次；这里把 finished_at 为空的记录清掉，前端只显示「上次执行结果」。

UPDATE task SET result_attempt = NULL WHERE finished_at IS NULL;
