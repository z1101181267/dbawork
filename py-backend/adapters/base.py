"""适配器基类。

统一接口：connect() / test() / close()。

conn 结构：
{
    "db_type": "mysql",
    "host": "10.0.0.1",        # 数据源登记的原始地址
    "port": 3306,              # 原始端口
    "db_name": "demo",
    "username": "root",
    "password": "***",
    "extra": {"charset": "utf8mb4", ...},
    "connect_host": "127.0.0.1",  # 隧道建立后的实际连接端点（无隧道时等于 host）
    "connect_port": 33306,
}
"""
from __future__ import annotations


class BaseAdapter:
    """所有数据库适配器的基类。"""

    #: 连接测试使用的 SQL（子类覆盖）
    TEST_SQL = "SELECT 1"

    def __init__(self, conn: dict):
        self.conn = dict(conn or {})
        self.conn.setdefault("extra", {})
        if not isinstance(self.conn.get("extra"), dict):
            self.conn["extra"] = {}
        self._conn = None

    # ---------------- 工具 ----------------

    def _endpoint(self) -> tuple[str, int]:
        """返回实际连接端点：隧道端点优先，无隧道时回退到原始地址。"""
        host = self.conn.get("connect_host") or self.conn.get("host") or ""
        port = self.conn.get("connect_port") or self.conn.get("port") or 0
        return str(host), int(port)

    def _extra(self) -> dict:
        return self.conn.get("extra") or {}

    # ---------------- 统一接口 ----------------

    def connect(self):
        """建立连接（子类实现），返回连接对象。"""
        raise NotImplementedError

    def test(self) -> tuple[bool, str, str]:
        """连接并执行探测 SQL，返回 (ok, error, db_version)。"""
        try:
            self.connect()
            cur = self._conn.cursor()
            try:
                cur.execute(self.TEST_SQL)
                row = cur.fetchone()
                version = self._format_version(row)
            finally:
                cur.close()
            return True, "", version
        except Exception as e:
            # 驱动异常类型繁多，统一转为可读信息
            return False, f"连接失败: {type(e).__name__}: {e}", ""
        finally:
            self.close()

    def _format_version(self, row) -> str:
        """把探测 SQL 的返回整形为单行版本字符串。"""
        if not row:
            return ""
        value = row[0]
        if value is None:
            return ""
        return " ".join(str(value).split())

    def close(self):
        """关闭连接（幂等）。"""
        conn, self._conn = self._conn, None
        if conn is None:
            return
        try:
            conn.close()
        except Exception:
            pass
