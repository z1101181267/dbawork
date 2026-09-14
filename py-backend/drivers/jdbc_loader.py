"""JDBC 驱动加载（扩展位）。

当前版本 Python 侧以 pip 驱动为主；JAR 的上传 / 删除 / 激活仅做文件与状态管理
（见 rpc/servicers/driver_servicer.py）。后续如需通过 JPype 在 Python 进程内
直接加载 JDBC jar 并建立连接，可在此模块实现 load_jar / close_jar 等接口。
"""
from __future__ import annotations


def is_jpype_available() -> bool:
    """检测运行环境是否具备 jpype（可选依赖）。"""
    try:
        import jpype  # noqa: F401
        return True
    except ImportError:
        return False


def load_jar(jar_path: str, driver_class: str = ""):
    """加载 JDBC jar 并返回驱动类句柄（扩展位，暂未实现）。

    :raises NotImplementedError: 当前版本未实现 JDBC 直连，请使用 pip 驱动。
    """
    raise NotImplementedError(
        "JDBC 直连为扩展功能，当前版本未实现；请使用 pip 驱动或等待后续版本支持"
    )
