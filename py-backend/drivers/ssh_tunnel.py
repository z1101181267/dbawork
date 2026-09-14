"""SSH 隧道（Linux 服务器端口转发）。

基于 sshtunnel 库建立本地到远端数据库的端口转发：
    start() -> (host, port)   # 数据库驱动应连接的本地端点
    stop()
"""
from __future__ import annotations

import os
import tempfile
from pathlib import Path

try:
    import sshtunnel
except ImportError:  # 依赖未安装时保持模块可导入
    sshtunnel = None


class SSHTunnel:
    """SSH 端口转发。

    参数：
        host / ssh_port : SSH 服务器地址与端口
        user / password : SSH 用户名 / 密码
        private_key     : 私钥内容（以 -----BEGIN 开头）或私钥文件路径
        key_passphrase  : 私钥口令
        db_host/db_port : 从 SSH 服务器视角看到的数据库地址与端口
    """

    def __init__(self, host: str, ssh_port: int = 22, user: str = "", password: str = "",
                 private_key: str = "", key_passphrase: str = "",
                 db_host: str = "", db_port: int = 0):
        self.host = (host or "").strip()
        self.ssh_port = int(ssh_port or 22)
        self.user = (user or "").strip()
        self.password = password or ""
        self.private_key = (private_key or "").strip()
        self.key_passphrase = key_passphrase or ""
        self.db_host = (db_host or "").strip()
        self.db_port = int(db_port or 0)
        self._server = None
        self._tmp_key_path: str | None = None
        self._validate()

    # ---------------- 参数校验 ----------------

    def _validate(self):
        if not self.host:
            raise ValueError("SSH 主机不能为空")
        if not self.user:
            raise ValueError("SSH 用户名不能为空")
        if not self.password and not self.private_key:
            raise ValueError("SSH 密码与私钥至少提供一项")
        if not self.db_host or self.db_port <= 0:
            raise ValueError("数据库目标地址/端口无效")

    # ---------------- 私钥处理 ----------------

    def _resolve_private_key(self) -> str | None:
        """私钥支持两种输入：内容 -> 写临时文件；或已是文件路径 -> 直接使用。"""
        if not self.private_key:
            return None
        if "-----BEGIN" in self.private_key:
            fd, path = tempfile.mkstemp(prefix="dbawork_ssh_key_", suffix=".pem")
            with os.fdopen(fd, "w", encoding="utf-8") as f:
                content = self.private_key
                if not content.endswith("\n"):
                    content += "\n"
                f.write(content)
            self._tmp_key_path = path
            return path
        p = Path(self.private_key)
        if not p.is_file():
            raise ValueError(f"SSH 私钥文件不存在: {self.private_key}")
        return str(p)

    # ---------------- 生命周期 ----------------

    def start(self) -> tuple[str, int]:
        """建立隧道，返回数据库驱动应连接的本地端点。"""
        if sshtunnel is None:
            raise RuntimeError("未安装 sshtunnel，请先安装依赖（pip install sshtunnel）")
        pkey = self._resolve_private_key()
        kwargs: dict = {
            "ssh_username": self.user,
            "remote_bind_address": (self.db_host, self.db_port),
            "allow_agent": False,
            "host_pkey_direct_keys": False,
        }
        if pkey:
            kwargs["ssh_pkey"] = pkey
            if self.key_passphrase:
                kwargs["ssh_private_key_password"] = self.key_passphrase
        else:
            kwargs["ssh_password"] = self.password
        try:
            self._server = sshtunnel.SSHTunnelForwarder((self.host, self.ssh_port), **kwargs)
            self._server.start()
        except Exception as e:
            self.stop()
            raise RuntimeError(f"SSH 隧道建立失败（{self.host}:{self.ssh_port}）: {e}") from e
        return ("127.0.0.1", int(self._server.local_bind_port))

    def stop(self):
        """停止隧道并清理临时文件（幂等）。"""
        server, self._server = self._server, None
        if server is not None:
            try:
                server.stop()
            except Exception:
                pass
        if self._tmp_key_path:
            try:
                os.unlink(self._tmp_key_path)
            except OSError:
                pass
            self._tmp_key_path = None
