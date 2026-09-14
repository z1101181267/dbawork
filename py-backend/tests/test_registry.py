"""registry 测试：核心映射 + 别名映射 + 未实现报错。"""
from adapters import registry


def test_core_mapping():
    for key in ("mysql", "sqlite", "oracle", "postgresql", "mssql"):
        assert registry.get_adapter(key) is registry.REGISTRY[key]


def test_alias_mysql_family():
    for alias in ("mariadb", "tidb", "doris", "starrocks", "oceanbase", "polardb", "tdsql", "gbase8a"):
        assert registry.get_adapter(alias) is registry.get_adapter("mysql"), f"别名错误: {alias}"


def test_alias_postgres_family():
    for alias in ("gaussdb", "opengauss", "kingbase"):
        assert registry.get_adapter(alias) is registry.get_adapter("postgresql"), f"别名错误: {alias}"


def test_unimplemented_returns_none():
    assert registry.get_adapter("db2") is None
    assert registry.get_adapter("hive") is None
    assert registry.get_adapter("") is None


def test_unsupported_message():
    assert registry.unsupported_message("db2") == "db2 适配器尚未实现"
