"""MySQL 适配器（pymysql）。"""
from __future__ import annotations

from .base import BaseAdapter

try:
    import pymysql
except ImportError:  # 依赖未安装时保持模块可导入
    pymysql = None


class MySQLAdapter(BaseAdapter):
    """MySQL / MariaDB / TiDB / OceanBase 等兼容协议数据库。"""

    TEST_SQL = "SELECT VERSION()"

    def connect(self):
        if pymysql is None:
            raise RuntimeError("未安装 pymysql，请在驱动管理中安装（pip install pymysql）")
        host, port = self._endpoint()
        extra = self._extra()
        # charset 默认 utf8mb4；db_name 为空传 None（不指定默认库）
        charset = extra.get("charset") or "utf8mb4"
        db_name = self.conn.get("db_name") or None
        self._conn = pymysql.connect(
            host=host,
            port=port,
            user=self.conn.get("username", ""),
            password=self.conn.get("password", ""),
            database=db_name,
            charset=charset,
            connect_timeout=10,
        )
        return self._conn
