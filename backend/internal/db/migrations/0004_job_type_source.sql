-- 0004 岗位来源标记：区分「钉钉职位映射」与「管理员手工调整」。
-- 手工调整过的岗位不会被通讯录同步覆盖。

ALTER TABLE app_user ADD COLUMN job_type_manual BOOLEAN NOT NULL DEFAULT FALSE;
