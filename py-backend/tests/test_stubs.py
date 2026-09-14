"""stub 与 servicer 测试：rpc.gen 可导入、两个 servicer 类存在、10 个 RPC 齐全。

另含 TestConnectionById 的集成校验（使用临时 metadata.db + secret.key，不依赖外部数据库）。
"""
import importlib
import os
import sqlite3

import pytest

REQUIRED_CONNECTION_RPCS = ["TestConnection", "TestConnectionById"]
REQUIRED_DRIVER_RPCS = [
    "ListDbTypes", "DetectAll", "DetectOne", "InstallDriver",
    "UninstallDriver", "UploadJar", "DeleteJar", "ActivateDriver",
]


# ---------------- stub 与 servicer 存在性 ----------------

def test_gen_stub_importable():
    from rpc.gen import dbawork_pb2, dbawork_pb2_grpc  # noqa: F401
    services = dbawork_pb2.DESCRIPTOR.services_by_name
    assert "ConnectionService" in services
    assert "DriverService" in services


def test_all_10_rpcs_exist():
    from rpc.servicers import ConnectionServicer, DriverServicer
    for name in REQUIRED_CONNECTION_RPCS:
        assert hasattr(ConnectionServicer, name), f"ConnectionServicer 缺少 RPC: {name}"
    for name in REQUIRED_DRIVER_RPCS:
        assert hasattr(DriverServicer, name), f"DriverServicer 缺少 RPC: {name}"
    # proto 中合计 10 个 RPC（2 + 8），且可实例化
    assert len(REQUIRED_CONNECTION_RPCS) + len(REQUIRED_DRIVER_RPCS) == 10
    ConnectionServicer()
    DriverServicer()


def test_modules_importable():
    importlib.import_module("rpc.server")
    importlib.import_module("app")


# ---------------- TestConnectionById 集成校验 ----------------

_DS_TABLE_SQL = """
CREATE TABLE datasources (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    db_type             VARCHAR(32),
    host                VARCHAR(255),
    port                INTEGER,
    db_name             VARCHAR(128),
    username            VARCHAR(128),
    password_enc        BLOB,
    password_iv         BLOB,
    extra_params        TEXT DEFAULT '',
    tunnel_type         VARCHAR(8) DEFAULT 'none',
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
    tunnel_transport    VARCHAR(8) DEFAULT '',
    tunnel_use_ssl      INTEGER DEFAULT 0
);
"""


def _make_fixture(tmp_path, monkeypatch):
    """构造 metadata.db + secret.key（AES-GCM 加密 sqlite 数据源凭据）。"""
    pytest.importorskip("cryptography")
    from cryptography.hazmat.primitives.ciphers.aead import AESGCM

    key = os.urandom(32)
    key_path = tmp_path / "secret.key"
    key_path.write_bytes(key)

    db_path = tmp_path / "metadata.db"
    conn = sqlite3.connect(db_path)
    conn.execute(_DS_TABLE_SQL)
    cipher = AESGCM(key)
    nonce = os.urandom(12)
    ct = cipher.encrypt(nonce, b"sqlite-pass", None)
    conn.execute(
        "INSERT INTO datasources (id, db_type, host, port, db_name, username, password_enc, password_iv, extra_params, tunnel_type)"
        " VALUES (1, 'sqlite', '', 0, ':memory:', '', ?, ?, '', 'none')",
        (ct, nonce),
    )
    conn.commit()
    conn.close()

    monkeypatch.setenv("DBAWORK_DB_PATH", str(db_path))
    monkeypatch.setenv("DBAWORK_SECRET_KEY_PATH", str(key_path))
    return db_path, key_path


def test_test_connection_by_id_sqlite(tmp_path, monkeypatch):
    _make_fixture(tmp_path, monkeypatch)
    from rpc.gen import dbawork_pb2
    from rpc.servicers import ConnectionServicer

    result = ConnectionServicer().TestConnectionById(dbawork_pb2.DsIdRequest(id=1), None)
    assert result.ok, result.error
    assert result.db_version.startswith("3.")  # sqlite_version()
    assert result.tested_at
    assert result.latency_ms >= 0


def test_test_connection_by_id_missing_row(tmp_path, monkeypatch):
    _make_fixture(tmp_path, monkeypatch)
    from rpc.gen import dbawork_pb2
    from rpc.servicers import ConnectionServicer

    result = ConnectionServicer().TestConnectionById(dbawork_pb2.DsIdRequest(id=999), None)
    assert not result.ok
    assert "不存在" in result.error


def test_test_connection_by_id_missing_key(tmp_path, monkeypatch):
    _make_fixture(tmp_path, monkeypatch)
    monkeypatch.setenv("DBAWORK_SECRET_KEY_PATH", str(tmp_path / "no-such.key"))
    from rpc.gen import dbawork_pb2
    from rpc.servicers import ConnectionServicer

    result = ConnectionServicer().TestConnectionById(dbawork_pb2.DsIdRequest(id=1), None)
    assert not result.ok
    assert "密钥文件不存在" in result.error
