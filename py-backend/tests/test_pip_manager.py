"""pip_manager 测试：假包未安装、sqlite3 白名单、pymysql 已安装。"""
import sqlite3

from drivers import catalog, pip_manager


def test_detect_fake_package_not_installed(monkeypatch):
    """不存在的包应检测为未安装。"""
    fake = {"key": "fake_db_x", "driver_kind": "python", "package_name": "dbawork-not-exist-pkg-xyz"}
    monkeypatch.setattr(catalog, "by_key", lambda k: fake if k == "fake_db_x" else None)
    status = pip_manager.detect_one("fake_db_x")
    assert status["installed"] is False
    assert status["db_type"] == "fake_db_x"
    assert status["driver_kind"] == "python"
    assert status["installed_version"] == ""


def test_sqlite_stdlib_whitelist():
    """sqlite3 为标准库白名单：恒已安装，版本取 sqlite3.sqlite_version。"""
    status = pip_manager.detect_one("sqlite")
    assert status["installed"] is True
    assert status["installed_version"] == sqlite3.sqlite_version
    assert status["driver_kind"] == "python"


def test_pymysql_installed_in_venv():
    """核心依赖 pymysql 在 venv 内应检测为已安装。"""
    status = pip_manager.detect_one("mysql")
    assert status["installed"] is True
    assert status["installed_version"], "应能取到 pymysql 版本号"


def test_detect_all_covers_catalog():
    all_status = pip_manager.detect_all()
    assert len(all_status) == 25
    for s in all_status:
        for field in ("db_type", "driver_kind", "package_name", "installed", "installed_version", "note"):
            assert field in s


def test_jdbc_detect_uses_drivers_dir(tmp_path, monkeypatch):
    """jdbc 类型：按 data/drivers/<db_type>/ 下的 jar 数量判定。"""
    monkeypatch.setattr(catalog, "DRIVERS_DIR", tmp_path)
    status = pip_manager.detect_one("db2")
    assert status["installed"] is False
    assert status["note"] == "未上传 jar 文件"

    jar_dir = tmp_path / "db2" / "1.0"
    jar_dir.mkdir(parents=True)
    (jar_dir / "db2jcc.jar").write_bytes(b"jar")
    status2 = pip_manager.detect_one("db2")
    assert status2["installed"] is True
    assert "1" in status2["note"]


def test_unknown_type():
    status = pip_manager.detect_one("no_such_type_zzz")
    assert status["installed"] is False
    assert "未知" in status["note"]


def test_install_stdlib_noop():
    """标准库安装直接返回成功且不执行 pip。"""
    ok, message = pip_manager.install("sqlite", "sqlite3")
    assert ok is True
    assert "无需安装" in message


def test_uninstall_stdlib_rejected():
    ok, message = pip_manager.uninstall("sqlite")
    assert ok is False
    assert "不可卸载" in message
