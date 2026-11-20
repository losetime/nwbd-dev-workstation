-- 0014 任务类型改为字典驱动
--
-- 原来 task_type 只能取「普通任务 / 项目策划」，现在名称来自 task_type 字典，
-- 校验改由 task_type_code 完成，去掉旧的枚举约束。

ALTER TABLE task DROP CONSTRAINT IF EXISTS task_type_check;
ALTER TABLE task ALTER COLUMN task_type DROP DEFAULT;
