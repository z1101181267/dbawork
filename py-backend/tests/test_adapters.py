"""适配器测试：sqlite 真连 :memory:；mysql 用 unittest.mock 验证 DSN 参数与 test SQL。"""
import sqlite3
from unittest.mock import MagicMock

import pytest

from adapters.mysql_adapter import MySQLAdapter
from adapters.sqlite_adapter import SQLiteAdapter


def _conn(**over) -> dict:
    """构造 conn 字典（与 connection_servicer 组装的结构一致）。"""
    base = {
        "db_type": "sqlite",
        "host": "",
        "port": 0,
        "db_name": ":memory:",
        "username": "",
        "password": "",
        "extra": {},
        "connect_host": "",
        "connect_port": 0,
    }
    base.update(over)
    return base


# ---------------- sqlite：真实连接 ----------------

def test_sqlite_adapter_real_connect():
    adapter = SQLiteAdapter(_conn())
    ok, err, version = adapter.test()
    assert ok, err
    assert version == sqlite3.sqlite_version


def test_sqlite_adapter_query():
    adapter = SQLiteAdapter(_conn())
    conn = adapter.connect()
    try:
        cur = conn.execute("SELECT 1 + 1")
        assert cur.fetchone()[0] == 2
    finally:
        adapter.close()


# ---------------- mysql：mock 验证 ----------------

def test_mysql_adapter_params_and_test_sql(monkeypatch):
    from adapters import mysql_adapter as mod

    if mod.pymysql is None:
        pytest.skip("pymysql 未安装，跳过 mock 用例")

    fake_conn = MagicMock()
    fake_cur = MagicMock()
    fake_cur.fetchone.return_value = ("8.0.36",)
    fake_conn.cursor.return_value = fake_cur
    fake_connect = MagicMock(return_value=fake_conn)
    monkeypatch.setattr(mod.pymysql, "connect", fake_connect)

    adapter = MySQLAdapter(_conn(
        db_type="mysql", host="10.0.0.8", port=3306, db_name="demo",
        username="root", password="p@ss", extra={"charset": "gbk"},
        connect_host="127.0.0.1", connect_port=33306,
    ))
    ok, err, version = adapter.test()

    assert ok, err
    assert version == "8.0.36"
    kw = fake_connect.call_args.kwargs
    # 隧道端点优先
    assert kw["host"] == "127.0.0.1"
    assert kw["port"] == 33306
    assert kw["user"] == "root"
    assert kw["password"] == "p@ss"
    assert kw["database"] == "demo"
    assert kw["charset"] == "gbk"
    assert kw["connect_timeout"] == 10
    fake_cur.execute.assert_called_once_with("SELECT VERSION()")
    fake_cur.close.assert_called_once()
    fake_conn.close.assert_called_once()


def test_mysql_endpoint_fallback_and_defaults(monkeypatch):
    from adapters import mysql_adapter as mod

    if mod.pymysql is None:
        pytest.skip("pymysql 未安装，跳过 mock 用例")

    fake_conn = MagicMock()
    fake_conn.cursor.return_value.fetchone.return_value = ("5.7.44",)
    fake_connect = MagicMock(return_value=fake_conn)
    monkeypatch.setattr(mod.pymysql, "connect", fake_connect)

    # 无隧道端点 -> 回退原始 host/port；db_name 为空 -> None；charset 默认 utf8mb4
    adapter = MySQLAdapter(_conn(db_type="mysql", host="db.internal", port=3307, db_name=""))
    ok, err, version = adapter.test()
    assert ok, err
    assert version == "5.7.44"
    kw = fake_connect.call_args.kwargs
    assert kw["host"] == "db.internal"
    assert kw["port"] == 3307
    assert kw["database"] is None
    assert kw["charset"] == "utf8mb4"


def test_mysql_connect_failure_message(monkeypatch):
    from adapters import mysql_adapter as mod

    if mod.pymysql is None:
        pytest.skip("pymysql 未安装，跳过 mock 用例")

    def boom(**kwargs):
        raise RuntimeError("boom")

    monkeypatch.setattr(mod.pymysql, "connect", boom)
    adapter = MySQLAdapter(_conn(db_type="mysql", host="h", port=3306))
    ok, err, version = adapter.test()
    assert ok is False
    assert "连接失败" in err
    assert "boom" in err
    assert version == ""
