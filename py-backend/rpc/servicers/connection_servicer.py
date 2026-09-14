"""ConnectionService 实现：连接测试（直连 / SSH、WinRM 隧道）。"""
from __future__ import annotations

import json
import os
import sqlite3
import time
from datetime import datetime, timezone
from pathlib import Path

from adapters import registry
from drivers import catalog
from drivers.ssh_tunnel import SSHTunnel
from drivers.winrm_tunnel import WinRMTunnel
from ..gen import dbawork_pb2, dbawork_pb2_grpc

try:
    from cryptography.hazmat.primitives.ciphers.aead import AESGCM
except ImportError:  # cryptography 由 sshtunnel/paramiko 传递提供
    AESGCM = None


# ---------------- 路径工具 ----------------

def _metadata_db_path() -> Path:
    """元数据库路径：env DBAWORK_DB_PATH 优先，默认 ../data/metadata.db。"""
    env = os.environ.get("DBAWORK_DB_PATH")
    return Path(env) if env else catalog.DATA_DIR / "metadata.db"


def _secret_key_path() -> Path:
    """密钥文件路径：env DBAWORK_SECRET_KEY_PATH 优先，默认 ../data/secret.key。"""
    env = os.environ.get("DBAWORK_SECRET_KEY_PATH")
    return Path(env) if env else catalog.DATA_DIR / "secret.key"


def _now_iso() -> str:
    """UTC ISO8601 时间戳。"""
    return datetime.now(timezone.utc).isoformat()


# ---------------- 核心逻辑 ----------------

def _fail(error: str, started: float) -> dbawork_pb2.TestResult:
    return dbawork_pb2.TestResult(
        ok=False,
        error=error,
        latency_ms=int((time.monotonic() - started) * 1000),
        tested_at=_now_iso(),
    )


def _test_connection(req: dbawork_pb2.DataSourceConn) -> dbawork_pb2.TestResult:
    """执行连接测试：必要时建立隧道 -> 适配器连接 -> 停隧道。

    TestConnection 与 TestConnectionById 共用本逻辑。
    """
    started = time.monotonic()
    tunnel = None
    adapter = None
    connect_host, connect_port = req.host, int(req.port or 0)

    tunnel_type = (req.tunnel.tunnel_type if req.HasField("tunnel") else "") or "none"
    if tunnel_type == "ssh":
        try:
            tunnel = SSHTunnel(
                host=req.tunnel.host,
                ssh_port=req.tunnel.port or 22,
                user=req.tunnel.user,
                password=req.tunnel.password,
                private_key=req.tunnel.private_key,
                key_passphrase=req.tunnel.key_passphrase,
                db_host=req.host,
                db_port=req.port,
            )
            connect_host, connect_port = tunnel.start()
        except Exception as e:
            if tunnel is not None:
                tunnel.stop()
            return _fail(f"隧道建立失败: {e}", started)
    elif tunnel_type == "winrm":
        try:
            tunnel = WinRMTunnel(
                host=req.tunnel.host,
                port=req.tunnel.port or 0,
                user=req.tunnel.user,
                password=req.tunnel.password,
                auth_scheme=req.tunnel.auth_scheme or "ntlm",
                transport=req.tunnel.transport or "http",
                use_ssl=req.tunnel.use_ssl,
                db_host=req.host,
                db_port=req.port,
            )
            connect_host, connect_port = tunnel.start()
        except Exception as e:
            if tunnel is not None:
                tunnel.stop()
            return _fail(f"隧道建立失败: {e}", started)
    elif tunnel_type not in ("", "none"):
        return _fail(f"未知的隧道类型: {tunnel_type}", started)

    adapter_cls = registry.get_adapter(req.db_type)
    if adapter_cls is None:
        if tunnel is not None:
            tunnel.stop()
        return _fail(registry.unsupported_message(req.db_type), started)

    try:
        conn_dict = {
            "db_type": req.db_type,
            "host": req.host,
            "port": int(req.port or 0),
            "db_name": req.db_name,
            "username": req.username,
            "password": req.password,
            "extra": dict(req.extra),
            "connect_host": connect_host,
            "connect_port": connect_port,
        }
        adapter = adapter_cls(conn_dict)
        ok, error, version = adapter.test()
    except Exception as e:
        ok, error, version = False, f"连接失败: {type(e).__name__}: {e}", ""
    finally:
        if adapter is not None:
            adapter.close()
        if tunnel is not None:
            tunnel.stop()

    return dbawork_pb2.TestResult(
        ok=ok,
        error=error,
        db_version=version,
        latency_ms=int((time.monotonic() - started) * 1000),
        tested_at=_now_iso(),
    )


def _decrypt(cipher, ct, nonce) -> str:
    """AES-GCM 解密（nonce 单独存储，无 AAD）；空密文返回空串。"""
    if not ct:
        return ""
    if not nonce or len(nonce) != 12:
        raise ValueError("nonce 长度不合法（应为 12 字节）")
    return cipher.decrypt(bytes(nonce), bytes(ct), None).decode("utf-8")


def _test_connection_by_id(ds_id: int) -> dbawork_pb2.TestResult:
    """按数据源 ID 测试连接：读取元数据库 + 解密凭据 + 复用 TestConnection 逻辑。"""
    started = time.monotonic()

    db_path = _metadata_db_path()
    if not db_path.is_file():
        return _fail(f"元数据库不存在: {db_path}（请先启动 Go 网关创建数据源，或设置 DBAWORK_DB_PATH）", started)

    try:
        uri = db_path.resolve().as_uri() + "?mode=ro"
        conn = sqlite3.connect(uri, uri=True)
        conn.row_factory = sqlite3.Row
        try:
            row = conn.execute("SELECT * FROM datasources WHERE id = ?", (ds_id,)).fetchone()
        finally:
            conn.close()
    except sqlite3.Error as e:
        return _fail(f"读取元数据库失败: {e}", started)
    if row is None:
        return _fail(f"数据源不存在: id={ds_id}", started)

    key_path = _secret_key_path()
    if not key_path.is_file():
        return _fail(
            f"密钥文件不存在: {key_path}（请先由 Go 网关注册数据源生成 secret.key，"
            "或设置环境变量 DBAWORK_SECRET_KEY_PATH）",
            started,
        )
    if AESGCM is None:
        return _fail("缺少 cryptography 依赖（由 sshtunnel/paramiko 提供），无法解密凭据", started)
    key = key_path.read_bytes()
    if len(key) != 32:
        return _fail(f"密钥文件长度应为 32 字节，实际 {len(key)} 字节: {key_path}", started)

    # 解密凭据（字段不存在时视为空）
    keys = row.keys()

    def cell(name: str):
        return row[name] if name in keys else None

    try:
        cipher = AESGCM(key)
        password = _decrypt(cipher, cell("password_enc"), cell("password_iv"))
        tunnel_password = _decrypt(cipher, cell("tunnel_password_enc"), cell("tunnel_password_iv"))
        tunnel_key = _decrypt(cipher, cell("tunnel_key_enc"), cell("tunnel_key_iv"))
        tunnel_key_pass = _decrypt(cipher, cell("tunnel_key_pass_enc"), cell("tunnel_key_pass_iv"))
    except Exception as e:
        return _fail(f"凭据解密失败: {e}", started)

    # 扩展参数
    extra: dict[str, str] = {}
    raw_extra = cell("extra_params") or ""
    if raw_extra.strip():
        try:
            parsed = json.loads(raw_extra)
        except json.JSONDecodeError as e:
            return _fail(f"扩展参数不是合法 JSON: {e}", started)
        if isinstance(parsed, dict):
            extra = {str(k): str(v) for k, v in parsed.items() if v is not None}

    req = dbawork_pb2.DataSourceConn(
        id=row["id"],
        db_type=cell("db_type") or "",
        host=cell("host") or "",
        port=int(cell("port") or 0),
        db_name=cell("db_name") or "",
        username=cell("username") or "",
        password=password,
        extra=extra,
        tunnel=dbawork_pb2.ServerTunnel(
            tunnel_type=cell("tunnel_type") or "none",
            host=cell("tunnel_host") or "",
            port=int(cell("tunnel_port") or 0),
            user=cell("tunnel_user") or "",
            password=tunnel_password,
            private_key=tunnel_key,
            key_passphrase=tunnel_key_pass,
            auth_scheme=cell("tunnel_auth_scheme") or "",
            transport=cell("tunnel_transport") or "",
            use_ssl=bool(cell("tunnel_use_ssl")),
        ),
    )
    return _test_connection(req)


# ---------------- Servicer ----------------

class ConnectionServicer(dbawork_pb2_grpc.ConnectionServiceServicer):
    """ConnectionService：连接测试。"""

    def TestConnection(self, request, context):  # noqa: N802 (gRPC 命名)
        try:
            return _test_connection(request)
        except Exception as e:
            return dbawork_pb2.TestResult(
                ok=False, error=f"内部错误: {type(e).__name__}: {e}", tested_at=_now_iso()
            )

    def TestConnectionById(self, request, context):  # noqa: N802
        try:
            return _test_connection_by_id(int(request.id))
        except Exception as e:
            return dbawork_pb2.TestResult(
                ok=False, error=f"内部错误: {type(e).__name__}: {e}", tested_at=_now_iso()
            )
