-- 0002 人员档案：岗位、级别、在职状态、界面权限。
-- 岗位类型由钉钉职位（app_user.title）映射而来，登录与通讯录同步时刷新。

ALTER TABLE app_user
    ADD COLUMN job_type    VARCHAR(16) NOT NULL DEFAULT '其他'
        CHECK (job_type IN ('部门经理', '项目经理', '开发', '测试', 'UI', '其他')),
    ADD COLUMN level       VARCHAR(8) NOT NULL DEFAULT '中级'
        CHECK (level IN ('初级', '中级', '高级', '专家')),
    ADD COLUMN employee_no VARCHAR(64),
    ADD COLUMN status      VARCHAR(8) NOT NULL DEFAULT '在职'
        CHECK (status IN ('在职', '离职')),
    ADD COLUMN dept_id     VARCHAR(64),
    ADD COLUMN avatar      VARCHAR(512),
    -- 界面权限，空数组 = 按岗位默认
    ADD COLUMN ui_perms    TEXT[] NOT NULL DEFAULT '{}';

CREATE INDEX ix_app_user_job_type ON app_user (job_type);
CREATE INDEX ix_app_user_status ON app_user (status);
