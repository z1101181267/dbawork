"""数据库类型 -> 适配器注册表。"""
from __future__ import annotations

from .mssql_adapter import MSSQLAdapter
from .mysql_adapter import MySQLAdapter
from .oracle_adapter import OracleAdapter
from .postgres_adapter import PostgreSQLAdapter
from .sqlite_adapter import SQLiteAdapter

#: 核心适配器映射
REGISTRY: dict[str, type] = {
    "mysql": MySQLAdapter,
    "oracle": OracleAdapter,
    "postgresql": PostgreSQLAdapter,
    "mssql": MSSQLAdapter,
    "sqlite": SQLiteAdapter,
}

#: 兼容数据库别名 -> 核心类型
ALIASES: dict[str, str] = {
    # 类 MySQL 协议
    "mariadb": "mysql",
    "tidb": "mysql",
    "doris": "mysql",
    "starrocks": "mysql",
    "oceanbase": "mysql",
    "polardb": "mysql",
    "tdsql": "mysql",
    "gbase8a": "mysql",
    # 类 PostgreSQL 协议
    "gaussdb": "postgresql",
    "opengauss": "postgresql",
    "kingbase": "postgresql",
}


def get_adapter(db_type: str):
    """返回 db_type 对应的适配器类；未实现返回 None。"""
    key = (db_type or "").strip().lower()
    key = ALIASES.get(key, key)
    return REGISTRY.get(key)


def unsupported_message(db_type: str) -> str:
    """未实现适配器时的统一错误信息。"""
    return f"{db_type} 适配器尚未实现"
