"""SQL Server 适配器（pymssql）。"""
from __future__ import annotations

from .base import BaseAdapter

try:
    import pymssql
except ImportError:  # 依赖未安装时保持模块可导入
    pymssql = None


class MSSQLAdapter(BaseAdapter):
    """Microsoft SQL Server。"""

    TEST_SQL = "SELECT @@VERSION"

    def connect(self):
        if pymssql is None:
            raise RuntimeError("未安装 pymssql，请在驱动管理中安装（pip install pymssql）")
        host, port = self._endpoint()
        self._conn = pymssql.connect(
            server=host,
            port=port,
            user=self.conn.get("username", ""),
            password=self.conn.get("password", ""),
            database=self.conn.get("db_name") or "",
            login_timeout=10,
        )
        return self._conn
