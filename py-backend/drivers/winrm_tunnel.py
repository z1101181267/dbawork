"""WinRM 隧道（Windows 服务器端口转发）。

通过 pywinrm 在 Windows 服务器上执行 `netsh interface portproxy`，把远端数据库
端口转发到 Windows 服务器上的监听端口，客户端直接连接该监听端口。
    start() -> (tunnel_host, listen_port)
    stop()
"""
from __future__ import annotations

import logging
import random
import re

try:
    import winrm
except ImportError:  # 依赖未安装时保持模块可导入
    winrm = None

logger = logging.getLogger(__name__)

#: 传输协议 -> 默认 WinRM 端口
TRANSPORT_DEFAULT_PORTS = {"http": 5985, "https": 5986}
#: 支持的认证方式
AUTH_SCHEMES = ("basic", "ntlm", "kerberos")
#: 远端监听端口选取范围
PORT_RANGE = (31000, 39999)
MAX_RETRIES = 5


class WinRMTunnel:
    """WinRM 端口转发（需远程 Windows 管理员权限且已开启 WinRM）。"""

    def __init__(self, host: str, port: int = 0, user: str = "", password: str = "",
                 auth_scheme: str = "ntlm", transport: str = "http", use_ssl: bool = False,
                 db_host: str = "", db_port: int = 0):
        self.host = (host or "").strip()
        self.transport = (transport or "http").strip().lower()
        self.port = int(port or TRANSPORT_DEFAULT_PORTS.get(self.transport, 5985))
        self.user = (user or "").strip()
        self.password = password or ""
        self.auth_scheme = (auth_scheme or "ntlm").strip().lower()
        self.use_ssl = bool(use_ssl)
        self.db_host = (db_host or "").strip()
        self.db_port = int(db_port or 0)
        self._session = None
        self._listen_port: int | None = None
        self._validate()

    # ---------------- 参数校验 ----------------

    def _validate(self):
        if not self.host:
            raise ValueError("WinRM 主机不能为空")
        if not self.user:
            raise ValueError("WinRM 用户名不能为空")
        if not self.password:
            raise ValueError("WinRM 密码不能为空")
        if self.auth_scheme not in AUTH_SCHEMES:
            raise ValueError(f"不支持的 WinRM 认证方式: {self.auth_scheme}（可选 basic/ntlm/kerberos）")
        if self.transport not in ("http", "https"):
            raise ValueError(f"不支持的 WinRM 传输协议: {self.transport}（可选 http/https）")
        if not self.db_host or self.db_port <= 0:
            raise ValueError("数据库目标地址/端口无效")

    # ---------------- 远程命令工具 ----------------

    @staticmethod
    def _text(res) -> str:
        """兼容 pywinrm 返回 bytes/str 的输出。"""
        out = getattr(res, "std_out", "") or ""
        if isinstance(out, bytes):
            out = out.decode("utf-8", errors="replace")
        err = getattr(res, "std_err", "") or ""
        if isinstance(err, bytes):
            err = err.decode("utf-8", errors="replace")
        return f"{out}\n{err}".strip()

    @staticmethod
    def _tail(text: str, limit: int = 800) -> str:
        text = (text or "").strip()
        return text if len(text) <= limit else "…" + text[-limit:]

    def _run(self, command: str):
        """在远程执行命令。"""
        return self._session.run_cmd(command)

    def _port_is_free(self, port: int) -> bool:
        """通过 netstat -ano 检查远程端口是否空闲。"""
        res = self._run("netstat -ano")
        if getattr(res, "status_code", 1) != 0:
            raise RuntimeError(
                f"netstat -ano 执行失败（exit={getattr(res, 'status_code', '?')}）: {self._tail(self._text(res))}"
            )
        text = self._text(res).upper()
        for line in text.splitlines():
            if "LISTENING" in line and re.search(rf":{port}\s", line):
                return False
        return True

    def _choose_listen_port(self) -> int:
        """随机选取远程空闲端口，重试 <= 5 次。"""
        for _ in range(MAX_RETRIES):
            port = random.randint(*PORT_RANGE)
            if self._port_is_free(port):
                return port
        raise RuntimeError(
            f"无法在远程服务器上找到空闲端口（已重试 {MAX_RETRIES} 次）；"
            "提示：需远程管理员权限，并检查 WinRM 是否开启（winrm quickconfig）"
        )

    # ---------------- 生命周期 ----------------

    def start(self) -> tuple[str, int]:
        """建立转发，返回客户端应连接的 (tunnel_host, listen_port)。"""
        if winrm is None:
            raise RuntimeError("未安装 pywinrm，请先安装依赖（pip install pywinrm）")
        endpoint = f"{self.transport}://{self.host}:{self.port}/wsman"
        self._session = winrm.Session(
            endpoint=endpoint,
            auth=(self.user, self.password),
            transport=self.auth_scheme,
            server_cert_validation="ignore",
        )
        try:
            listen_port = self._choose_listen_port()
        except RuntimeError:
            self.stop()
            raise
        except Exception as e:
            self.stop()
            raise RuntimeError(
                f"WinRM 连接失败（{endpoint}）: {e}；"
                "提示：需远程管理员权限，并检查 WinRM 是否开启（winrm quickconfig）"
            ) from e

        # 建立端口转发
        add_cmd = (
            "netsh interface portproxy add v4tov4 "
            f"listenaddress=0.0.0.0 listenport={listen_port} "
            f"connectaddress={self.db_host} connectport={self.db_port}"
        )
        res = self._run(add_cmd)
        if getattr(res, "status_code", 1) != 0:
            self.stop()
            raise RuntimeError(
                f"WinRM 端口转发建立失败（listenport={listen_port}）: {self._tail(self._text(res))}；"
                "提示：需远程管理员权限，并检查 WinRM 是否开启（winrm quickconfig）"
            )
        self._listen_port = listen_port

        # 防火墙放行（尽力而为，失败仅记录 warning）
        fw_cmd = (
            f'netsh advfirewall firewall add rule name="dbawork-{listen_port}" '
            f"dir=in action=allow protocol=TCP localport={listen_port}"
        )
        try:
            fw_res = self._run(fw_cmd)
            if getattr(fw_res, "status_code", 1) != 0:
                logger.warning("添加防火墙规则失败（忽略）: %s", self._tail(self._text(fw_res)))
        except Exception as e:
            logger.warning("添加防火墙规则失败（忽略）: %s", e)

        return (self.host, listen_port)

    def stop(self):
        """删除 portproxy 与防火墙规则（尽力而为，幂等）。"""
        session, self._session = self._session, None
        listen_port, self._listen_port = self._listen_port, None
        if session is None or listen_port is None:
            return
        self._best_effort(
            session,
            "netsh interface portproxy delete v4tov4 "
            f"listenaddress=0.0.0.0 listenport={listen_port}",
        )
        self._best_effort(
            session,
            f'netsh advfirewall firewall delete rule name="dbawork-{listen_port}"',
        )

    def _best_effort(self, session, command: str):
        try:
            res = session.run_cmd(command)
            if getattr(res, "status_code", 1) != 0:
                logger.warning("远程清理命令失败（忽略）: %s -> %s", command, self._tail(self._text(res)))
        except Exception as e:
            logger.warning("远程清理命令异常（忽略）: %s -> %s", command, e)
