"""Oracle 适配器（python-oracledb，兼容旧驱动 cx_Oracle）。

python-oracledb 是 Oracle 官方新一代驱动（cx_Oracle 的继任者），
默认 thin 模式为纯 Python 实现，无需安装 Oracle 客户端库。
"""
from __future__ import annotations

from .base import BaseAdapter

try:
    import oracledb as _driver
except ImportError:  # 兼容旧驱动
    try:
        import cx_Oracle as _driver  # type: ignore[no-redef]
    except ImportError:
        _driver = None


class OracleAdapter(BaseAdapter):
    """Oracle 数据库（service_name / SID 连接）。"""

    TEST_SQL = "SELECT banner FROM v$version WHERE rownum=1"

    def connect(self):
        if _driver is None:
            raise RuntimeError("未安装 Oracle 驱动，请在驱动管理中安装（pip install oracledb）")
        host, port = self._endpoint()
        extra = self._extra()
        # service_name 优先：extra.service_name -> db_name
        service_name = extra.get("service_name") or self.conn.get("db_name") or ""
        if not service_name:
            raise RuntimeError("Oracle 连接缺少 service_name（请在扩展参数或库名中填写）")
        dsn = _driver.makedsn(host, port, service_name=service_name)
        self._conn = _driver.connect(
            user=self.conn.get("username", ""),
            password=self.conn.get("password", ""),
            dsn=dsn,
        )
        return self._conn
