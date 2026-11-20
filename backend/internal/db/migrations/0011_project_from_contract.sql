-- 0011：合同审批自动建项目、项目软删除、名称可重复
ALTER TABLE project ADD COLUMN IF NOT EXISTS dt_contract_instance_id VARCHAR(64);
ALTER TABLE project ADD COLUMN IF NOT EXISTS dt_business_id VARCHAR(64);
ALTER TABLE project ADD COLUMN IF NOT EXISTS contract_status VARCHAR(16) NOT NULL DEFAULT '';
ALTER TABLE project ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE project ADD COLUMN IF NOT EXISTS deleted_by VARCHAR(26) REFERENCES app_user (id);

-- 合同名称可能重复，项目编号唯一即可（允许为空：合同单没填代号）
ALTER TABLE project DROP CONSTRAINT IF EXISTS project_name_key;
ALTER TABLE project DROP CONSTRAINT IF EXISTS project_code_key;
CREATE UNIQUE INDEX IF NOT EXISTS ux_project_code ON project (code) WHERE code <> '';

CREATE INDEX IF NOT EXISTS ix_project_dt_contract ON project (dt_contract_instance_id);
CREATE INDEX IF NOT EXISTS ix_project_deleted ON project (deleted_at);
