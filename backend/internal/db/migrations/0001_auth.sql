-- 0001 登录与令牌：钉钉扫码登录产生的用户、长效令牌、CLI 设备码。

CREATE TABLE app_user (
    id              VARCHAR(26) PRIMARY KEY,
    dingtalk_userid VARCHAR(64) NOT NULL UNIQUE,
    unionid         VARCHAR(128),
    name            VARCHAR(128) NOT NULL,
    title           VARCHAR(128),
    dept            VARCHAR(128),
    role            VARCHAR(16) NOT NULL DEFAULT 'member'
                    CHECK (role IN ('member', 'sub_admin', 'admin')),
    -- 本地管理员例外：TRUE 的账号不受钉钉管理员同步降权
    role_override   BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_login_at   TIMESTAMPTZ
);

CREATE INDEX ix_app_user_role ON app_user (role);

CREATE TABLE auth_token (
    id           VARCHAR(26) PRIMARY KEY,
    user_id      VARCHAR(26) NOT NULL REFERENCES app_user (id) ON DELETE CASCADE,
    label        VARCHAR(64) NOT NULL,
    token_hash   CHAR(64) NOT NULL UNIQUE,
    expires_at   TIMESTAMPTZ NOT NULL,
    revoked_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at TIMESTAMPTZ
);

CREATE INDEX ix_auth_token_user ON auth_token (user_id);

-- 设备码：命令行 / 脚本发起登录，浏览器扫码完成后由它把 token 取走。
CREATE TABLE device_code (
    device_code      VARCHAR(64) PRIMARY KEY,
    user_code        VARCHAR(12) NOT NULL,
    status           VARCHAR(16) NOT NULL DEFAULT 'pending',
    user_id          VARCHAR(26) REFERENCES app_user (id) ON DELETE CASCADE,
    token_plain      VARCHAR(128),
    token_expires_at TIMESTAMPTZ,
    expires_at       TIMESTAMPTZ NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    consumed_at      TIMESTAMPTZ
);
