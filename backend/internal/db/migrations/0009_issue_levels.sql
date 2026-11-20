-- 0009：困难上报与风险口径对齐
-- 级别改为 高/中/低，状态改为 待处理/已转风险/已归档
ALTER TABLE issue DROP CONSTRAINT IF EXISTS issue_severity_check;
ALTER TABLE issue DROP CONSTRAINT IF EXISTS issue_status_check;

UPDATE issue SET severity = CASE severity
    WHEN '一般' THEN '低'
    WHEN '较严重' THEN '中'
    WHEN '紧急阻塞' THEN '高'
    ELSE '中'
END;

UPDATE issue SET status = CASE status
    WHEN '待PM查看' THEN '待处理'
    WHEN '已登记风险' THEN '已转风险'
    WHEN '已忽略归档' THEN '已归档'
    ELSE status
END;

ALTER TABLE issue ALTER COLUMN severity SET DEFAULT '中';
ALTER TABLE issue ALTER COLUMN status SET DEFAULT '待处理';

ALTER TABLE issue ADD CONSTRAINT issue_severity_check CHECK (severity IN ('高', '中', '低'));
ALTER TABLE issue ADD CONSTRAINT issue_status_check CHECK (status IN ('待处理', '已转风险', '已归档'));
