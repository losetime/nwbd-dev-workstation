-- 0020 任务类型管理：说明、归档目标、默认技能、组别补齐
--
-- 1. task_type 增加任务说明、归档目标（项目 / 公共空间）、更新时间
-- 2. 补 4 个日常任务类型（DT01–DT04），项目任务排序后移
-- 3. 任务类型默认技能（多对多）
-- 4. task_skill 标记来源：type（类型带的，锁定）/ manual（任务里自己加的）
-- 5. task 快照归档目标，产出上传时决定落项目目录还是公共空间

ALTER TABLE task_type
    ADD COLUMN description    VARCHAR(500) NOT NULL DEFAULT '',
    ADD COLUMN archive_target VARCHAR(16) NOT NULL DEFAULT 'project',
    ADD COLUMN updated_at     TIMESTAMPTZ NOT NULL DEFAULT now();

UPDATE task_type SET sort_order = sort_order + 100 WHERE group_name = '项目任务';

INSERT INTO task_type (code, name, group_name, archive_stage, archive_folder, archive_target, archive_note, description, sort_order) VALUES
    ('DT01', '投标评估', '日常任务', '〇 售前与合同签订', '售前与合同签订', 'project', '投标评估材料、能否承接的评估结论', '', 10),
    ('DT02', '售前评估', '日常任务', '〇 售前与合同签订', '售前与合同签订', 'project', '售前方案评估、投入产出评估材料', '', 20),
    ('DT03', '售后评估', '日常任务', '十 交付与售后', '交付与售后', 'project', '售后需求评估、售后方案材料', '', 30),
    ('DT04', '部门其他', '日常任务', '部门公共/其他', '研发部', 'public', '不隶属任何项目阶段，归档到公共空间', '不属于项目阶段的其他工作', 40);

CREATE TABLE task_type_skill (
    id             VARCHAR(26) PRIMARY KEY,
    task_type_code VARCHAR(16) NOT NULL REFERENCES task_type (code) ON DELETE CASCADE,
    skill_name     VARCHAR(128) NOT NULL,
    skill_version  INT,
    sort_order     INT NOT NULL DEFAULT 0,
    UNIQUE (task_type_code, skill_name)
);

ALTER TABLE task_skill ADD COLUMN source VARCHAR(8) NOT NULL DEFAULT 'manual';
ALTER TABLE task ADD COLUMN archive_target VARCHAR(16) NOT NULL DEFAULT 'project';
