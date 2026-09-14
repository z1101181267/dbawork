"""SQLite 适配器（标准库 sqlite3，用于本地演示与测试）。"""
from __future__ import annotations

import sqlite3

from .base import BaseAdapter


class SQLiteAdapter(BaseAdapter):
    """SQLite：db_name 为数据库文件路径或 :memory:。"""

    TEST_SQL = "SELECT sqlite_version()"

    def connect(self):
        db = self.conn.get("db_name") or ":memory:"
        self._conn = sqlite3.connect(db)
        return self._conn
