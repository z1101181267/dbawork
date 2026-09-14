"""catalog 模块测试：25 条、key 唯一、必填字段齐全。"""
from drivers import catalog


def test_catalog_has_25_types():
    assert len(catalog.CATALOG) == 25


def test_keys_unique():
    keys = [t["key"] for t in catalog.CATALOG]
    assert len(keys) == len(set(keys)) == 25


def test_required_fields_present():
    for t in catalog.CATALOG:
        for field in ("key", "name_zh", "name_en", "driver_kind", "order"):
            assert t.get(field) not in (None, ""), f"{t.get('key')} 缺少字段 {field}"
        assert t["driver_kind"] in ("python", "jdbc")
        if t["driver_kind"] == "python":
            assert t.get("package_name"), f"{t['key']} 缺少 package_name"


def test_by_key():
    assert catalog.by_key("mysql")["name_zh"] == "MySQL"
    assert catalog.by_key("sqlite")["package_name"] == "sqlite3"
    assert catalog.by_key("不存在的类型") is None
    assert catalog.by_key("") is None


def test_python_jdbc_partition():
    py = catalog.python_types()
    jd = catalog.jdbc_types()
    assert len(py) + len(jd) == 25
    assert len(py) > 0 and len(jd) > 0


def test_import_name_map():
    # 特殊映射：连字符包名
    assert catalog.IMPORT_NAME_MAP["clickhouse-driver"] == "clickhouse_driver"
    # 默认规则
    assert catalog.IMPORT_NAME_MAP["pymysql"] == "pymysql"
    assert catalog.import_name("a-b-c") == "a_b_c"
    assert catalog.import_name("") == ""
