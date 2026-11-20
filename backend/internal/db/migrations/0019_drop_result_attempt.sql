-- 0019 执行结果改为「重新打开时隐藏」，不再需要记录结果轮次

ALTER TABLE task DROP COLUMN IF EXISTS result_attempt;
