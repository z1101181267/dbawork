"""数据库类型目录。

加载 config/db_type_catalog.json（对标 DBCheck 的 25 类 DB_TYPE_CATALOG），
供驱动管理、适配器注册等模块统一使用。文件为只读资源，不在本模块内修改。
"""
from __future__ import annotations

import json
from pathlib import Path
from typing import Optional

# py-backend 根目录（本文件位于 py-backend/drivers/ 下）
ROOT = Path(__file__).resolve().parent.parent
# 目录配置文件
CONFIG_PATH = ROOT / "config" / "db_type_catalog.json"
# DBAWORK 数据目录（data/），驱动 jar 存放于 data/drivers/<db_type>/<version>/
DATA_DIR = ROOT.parent / "data"
DRIVERS_DIR = DATA_DIR / "drivers"

# 标准库白名单（无需 pip 安装的“包名”）
STDLIB_PACKAGES = {"sqlite3"}

# pip 包名 -> import 名的特殊映射；其余默认把 "-" 替换为 "_"
_SPECIAL_IMPORT_NAMES = {"clickhouse-driver": "clickhouse_driver"}


def import_name(package_name: str) -> str:
    """把 pip 包名解析为 import 名（如 clickhouse-driver -> clickhouse_driver）。"""
    if not package_name:
        return ""
    return _SPECIAL_IMPORT_NAMES.get(package_name, package_name.replace("-", "_"))


def _load_catalog() -> list[dict]:
    """读取并校验目录文件。"""
    with CONFIG_PATH.open("r", encoding="utf-8") as f:
        raw = json.load(f)
    types = raw.get("types") or []
    if len(types) != 25:
        raise ValueError(f"db_type_catalog.json 应包含 25 类数据库，实际 {len(types)} 条")
    keys = [t.get("key") for t in types]
    if len(keys) != len(set(keys)):
        raise ValueError("db_type_catalog.json 存在重复的 key")
    return types


#: 全量目录（按 order 升序）
CATALOG: list[dict] = sorted(_load_catalog(), key=lambda t: t.get("order", 0))
#: key -> 目录项
CATALOG_BY_KEY: dict[str, dict] = {t["key"]: t for t in CATALOG}

#: pip 包名 -> import 名映射（覆盖目录中全部 python 驱动）
IMPORT_NAME_MAP: dict[str, str] = {
    t["package_name"]: import_name(t["package_name"])
    for t in CATALOG
    if t.get("package_name")
}


def by_key(key: str) -> Optional[dict]:
    """按 key 查找目录项；不存在返回 None。"""
    return CATALOG_BY_KEY.get((key or "").strip())


def python_types() -> list[dict]:
    """返回使用 Python 驱动的目录项。"""
    return [t for t in CATALOG if t.get("driver_kind") == "python"]


def jdbc_types() -> list[dict]:
    """返回使用 JDBC 驱动的目录项。"""
    return [t for t in CATALOG if t.get("driver_kind") == "jdbc"]
