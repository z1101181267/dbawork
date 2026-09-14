"""pip 驱动管理：检测 / 安装 / 卸载。

detect 返回结构与 proto DriverStatus 对齐：
    {db_type, driver_kind, package_name, installed, installed_version, note}
"""
from __future__ import annotations

import importlib.metadata
import importlib.util
import sqlite3
import subprocess
import sys
from pathlib import Path

from . import catalog


# ---------------- 内部工具 ----------------

def _status(db_type: str, kind: str, package: str, installed: bool,
            version: str, note: str) -> dict:
    """构造统一的状态字典。"""
    return {
        "db_type": db_type,
        "driver_kind": kind,
        "package_name": package or "",
        "installed": bool(installed),
        "installed_version": version or "",
        "note": note or "",
    }


def _brief(status: dict) -> str:
    """状态简述，用于安装/卸载结果消息。"""
    ver = status.get("installed_version") or "-"
    flag = "是" if status.get("installed") else "否"
    return f"installed={flag}, version={ver}"


# 发行名回退：import/配置名 -> 其他可能的 pip 发行名
_DIST_NAME_FALLBACKS = {"psycopg2": ["psycopg2-binary"]}


def _tail(text: str, limit: int = 1500) -> str:
    """截取输出尾部，避免超长消息。"""
    text = (text or "").strip()
    if len(text) <= limit:
        return text
    return "…" + text[-limit:]


def _package_version(dist_name: str) -> str:
    """取已安装发行版版本号；异常返回空串。

    部分包 pip 发行名与 import 名不同（如 psycopg2-binary），做少量回退尝试。
    """
    candidates = [dist_name] + _DIST_NAME_FALLBACKS.get(dist_name, [])
    for name in candidates:
        try:
            return importlib.metadata.version(name)
        except Exception:
            continue
    return ""


def _detect_python(db_type: str, package_name: str) -> dict:
    """检测 python 驱动包。"""
    if package_name in catalog.STDLIB_PACKAGES:
        # 标准库白名单：恒为已安装
        return _status(db_type, "python", package_name, True, sqlite3.sqlite_version, "标准库模块")
    imp = catalog.import_name(package_name)
    installed = False
    try:
        installed = importlib.util.find_spec(imp) is not None
    except (ImportError, ModuleNotFoundError, ValueError):
        installed = False
    version = _package_version(package_name) if installed else ""
    return _status(db_type, "python", package_name, installed, version, "")


def _detect_jdbc(db_type: str) -> dict:
    """检测 jdbc 驱动：data/drivers/<db_type>/ 下是否存在 .jar。"""
    jar_dir = Path(catalog.DRIVERS_DIR) / db_type
    jars: list[Path] = []
    if jar_dir.is_dir():
        jars = sorted(jar_dir.rglob("*.jar"))
    count = len(jars)
    note = f"共 {count} 个 jar 文件" if count else "未上传 jar 文件"
    return _status(db_type, "jdbc", "", count > 0, "", note)


# ---------------- 对外接口 ----------------

def detect_one(db_type: str) -> dict:
    """检测单个数据库类型的驱动状态。"""
    entry = catalog.by_key(db_type)
    if entry is None:
        return _status(db_type, "", "", False, "", "未知的数据库类型")
    if entry.get("driver_kind") == "jdbc" or entry.get("is_jdbc"):
        return _detect_jdbc(entry["key"])
    return _detect_python(entry["key"], entry.get("package_name", ""))


def detect_all() -> list[dict]:
    """检测目录中全部 25 类数据库的驱动状态。"""
    return [detect_one(t["key"]) for t in catalog.CATALOG]


def install(db_type: str, package_name: str = "", version: str = "") -> tuple[bool, str]:
    """安装 python 驱动；返回 (ok, message)，message 附带最新检测结果。"""
    entry = catalog.by_key(db_type)
    if entry is None:
        return False, f"未知的数据库类型: {db_type}"
    if entry.get("driver_kind") != "python":
        return False, f"{db_type} 为 JDBC 类型，请上传 jar 文件而非 pip 安装"
    pkg = (package_name or entry.get("package_name") or "").strip()
    if not pkg:
        return False, f"{db_type} 未配置 pip 包名"
    if pkg in catalog.STDLIB_PACKAGES:
        return True, "标准库模块，无需安装"

    spec = f"{pkg}=={version.strip()}" if (version or "").strip() else pkg
    cmd = [sys.executable, "-m", "pip", "install", spec,
           "--disable-pip-version-check", "-q"]
    try:
        proc = subprocess.run(cmd, capture_output=True, text=True, timeout=300)
    except subprocess.TimeoutExpired:
        return False, f"安装 {spec} 超时（300 秒），请检查网络或手动安装"
    output = ((proc.stdout or "") + (proc.stderr or "")).strip()

    status = detect_one(db_type)
    if proc.returncode == 0:
        msg = f"安装成功: {spec}；最新检测: {_brief(status)}"
        if output:
            msg += f"；pip 输出尾部: {_tail(output)}"
        return True, msg
    tail = _tail(output) or "无输出"
    return False, f"安装失败（exit={proc.returncode}）: {spec}；输出尾部: {tail}"


def uninstall(db_type: str) -> tuple[bool, str]:
    """卸载 python 驱动；返回 (ok, message)，message 附带最新检测结果。"""
    entry = catalog.by_key(db_type)
    if entry is None:
        return False, f"未知的数据库类型: {db_type}"
    if entry.get("driver_kind") != "python":
        return False, f"{db_type} 为 JDBC 类型，请通过删除 jar 文件卸载"
    pkg = (entry.get("package_name") or "").strip()
    if not pkg:
        return False, f"{db_type} 未配置 pip 包名"
    if pkg in catalog.STDLIB_PACKAGES:
        return False, f"标准库模块（{pkg}）不可卸载"

    cmd = [sys.executable, "-m", "pip", "uninstall", "-y", pkg]
    try:
        proc = subprocess.run(cmd, capture_output=True, text=True, timeout=300)
    except subprocess.TimeoutExpired:
        return False, f"卸载 {pkg} 超时（300 秒）"
    output = ((proc.stdout or "") + (proc.stderr or "")).strip()

    status = detect_one(db_type)
    if proc.returncode == 0:
        msg = f"卸载完成: {pkg}；最新检测: {_brief(status)}"
        if output:
            msg += f"；pip 输出尾部: {_tail(output)}"
        return True, msg
    tail = _tail(output) or "无输出"
    return False, f"卸载失败（exit={proc.returncode}）: {pkg}；输出尾部: {tail}"
