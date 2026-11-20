-- 0015 困难归档留痕：记录归档人与归档时间

ALTER TABLE issue ADD COLUMN handled_by VARCHAR(26) REFERENCES app_user (id);
ALTER TABLE issue ADD COLUMN handled_at TIMESTAMPTZ;

-- 历史数据：之前通过页面归档的，从审计日志回填操作人
UPDATE issue i
SET handled_by = a.actor_id,
    handled_at = a.created_at
FROM audit_log a
WHERE a.target_type = 'issue'
  AND a.target_id = i.id
  AND a.action = 'issue.archive'
  AND i.handled_by IS NULL;
