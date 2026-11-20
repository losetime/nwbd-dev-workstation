-- 0012：项目变更管理（05 项目变更申请）
ALTER TABLE change ADD COLUMN IF NOT EXISTS content TEXT;
ALTER TABLE change ADD COLUMN IF NOT EXISTS impact TEXT;
ALTER TABLE change ADD COLUMN IF NOT EXISTS budget_before NUMERIC(12, 2);
ALTER TABLE change ADD COLUMN IF NOT EXISTS budget_after NUMERIC(12, 2);
ALTER TABLE change ADD COLUMN IF NOT EXISTS plan_due_after DATE;
ALTER TABLE change ADD COLUMN IF NOT EXISTS material_path VARCHAR(512);
ALTER TABLE change ADD COLUMN IF NOT EXISTS approval_status VARCHAR(16);
ALTER TABLE change ADD COLUMN IF NOT EXISTS dingtalk_business_id VARCHAR(64);

-- 现有 change.type 约束改为 05 表单口径
ALTER TABLE change DROP CONSTRAINT IF EXISTS change_type_check;
ALTER TABLE change ADD CONSTRAINT change_type_check
    CHECK (type IN ('经费变更', '计划变更', '其他变更'));

ALTER TABLE change DROP CONSTRAINT IF EXISTS change_source_check;
ALTER TABLE change ADD CONSTRAINT change_source_check
    CHECK (source IN ('内部', '外部'));

CREATE INDEX IF NOT EXISTS ix_change_project ON change (project_id, applied_on DESC);
CREATE INDEX IF NOT EXISTS ix_change_instance ON change (dingtalk_instance_id);
