"""PostgreSQL 适配器（psycopg2）。"""
from __future__ import annotations

from .base import BaseAdapter

try:
    import psycopg2
except ImportError:  # 依赖未安装时保持模块可导入
    psycopg2 = None


class PostgreSQLAdapter(BaseAdapter):
    """PostgreSQL / openGauss / GaussDB / Kingbase 等兼容协议数据库。"""

    TEST_SQL = "SELECT version()"

    def connect(self):
        if psycopg2 is None:
            raise RuntimeError("未安装 psycopg2，请在驱动管理中安装（pip install psycopg2-binary）")
        host, port = self._endpoint()
        extra = self._extra()
        kwargs: dict = {
            "host": host,
            "port": port,
            "user": self.conn.get("username", ""),
            "password": self.conn.get("password", ""),
            "dbname": self.conn.get("db_name") or None,
            "connect_timeout": 10,
        }
        # sslmode 仅在有配置时传入
        if extra.get("sslmode"):
            kwargs["sslmode"] = extra["sslmode"]
        self._conn = psycopg2.connect(**kwargs)
        return self._conn
