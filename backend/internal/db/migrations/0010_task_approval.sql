-- 0010：任务类型与钉钉审批关联
ALTER TABLE task ADD COLUMN IF NOT EXISTS task_type VARCHAR(16) NOT NULL DEFAULT '普通任务';
ALTER TABLE task ADD COLUMN IF NOT EXISTS approval_instance_id VARCHAR(64);
ALTER TABLE task ADD COLUMN IF NOT EXISTS approval_status VARCHAR(16);
ALTER TABLE task ADD COLUMN IF NOT EXISTS approval_error TEXT;

ALTER TABLE task DROP CONSTRAINT IF EXISTS task_type_check;
ALTER TABLE task ADD CONSTRAINT task_type_check CHECK (task_type IN ('普通任务', '项目策划'));

CREATE INDEX IF NOT EXISTS ix_task_approval ON task (approval_instance_id);
