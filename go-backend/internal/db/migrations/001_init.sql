-- DBAWORK 初始表结构（幂等）
-- 时间戳统一使用 RFC3339 UTC 文本（strftime '%Y-%m-%dT%H:%M:%fZ'）。

CREATE TABLE IF NOT EXISTS groups (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        VARCHAR(128) NOT NULL,
    parent_id   INTEGER DEFAULT 0,
    sort_order  INTEGER DEFAULT 0,
    description VARCHAR(256) DEFAULT '',
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE IF NOT EXISTS datasources (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    name                VARCHAR(128) NOT NULL,
    group_id            INTEGER DEFAULT 0,
    db_type             VARCHAR(32)  NOT NULL,
    host                VARCHAR(255) NOT NULL,
    port                INTEGER      NOT NULL,
    db_name             VARCHAR(128) DEFAULT '',
    username            VARCHAR(128) NOT NULL,
    password_enc        BLOB,
    password_iv         BLOB,
    extra_params        TEXT DEFAULT '',
    tunnel_type         VARCHAR(8)  DEFAULT 'none',
    tunnel_host         VARCHAR(255) DEFAULT '',
    tunnel_port         INTEGER DEFAULT 0,
    tunnel_user         VARCHAR(128) DEFAULT '',
    tunnel_password_enc BLOB,
    tunnel_password_iv  BLOB,
    tunnel_key_enc      BLOB,
    tunnel_key_iv       BLOB,
    tunnel_key_pass_enc BLOB,
    tunnel_key_pass_iv  BLOB,
    tunnel_auth_scheme  VARCHAR(16) DEFAULT '',
    tunnel_transport    VARCHAR(8)  DEFAULT '',
    tunnel_use_ssl      INTEGER DEFAULT 0,
    status              TINYINT DEFAULT 0,
    last_test_at        TEXT,
    last_error          TEXT DEFAULT '',
    created_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX IF NOT EXISTS idx_ds_group  ON datasources(group_id);
CREATE INDEX IF NOT EXISTS idx_ds_dbtype ON datasources(db_type);
CREATE INDEX IF NOT EXISTS idx_ds_status ON datasources(status);

CREATE TABLE IF NOT EXISTS drivers (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    db_type           VARCHAR(32)  NOT NULL,
    driver_kind       VARCHAR(8)   NOT NULL,
    package_name      VARCHAR(128) DEFAULT '',
    version           VARCHAR(32)  DEFAULT '',
    driver_class      VARCHAR(255) DEFAULT '',
    jar_filename      VARCHAR(255) DEFAULT '',
    jar_path          TEXT DEFAULT '',
    file_size         INTEGER DEFAULT 0,
    is_active         INTEGER DEFAULT 0,
    installed         INTEGER DEFAULT 0,
    installed_version VARCHAR(64) DEFAULT '',
    note              TEXT DEFAULT '',
    uploaded_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    UNIQUE(db_type, driver_kind, version, package_name)
);
CREATE INDEX IF NOT EXISTS idx_drivers_dbtype ON drivers(db_type);

CREATE TABLE IF NOT EXISTS db_type_hidden (
    db_type   VARCHAR(32) PRIMARY KEY NOT NULL,
    hidden_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE IF NOT EXISTS db_type_custom (
    db_type           VARCHAR(32) PRIMARY KEY NOT NULL,
    name_zh           VARCHAR(64)  NOT NULL,
    name_en           VARCHAR(64)  NOT NULL,
    driver_class_hint VARCHAR(255) DEFAULT '',
    is_jdbc           INTEGER DEFAULT 1,
    created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
