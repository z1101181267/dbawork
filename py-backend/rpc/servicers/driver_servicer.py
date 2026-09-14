"""DriverService 实现：驱动检测 / 安装 / 卸载 / JAR 上传与管理。"""
from __future__ import annotations

import json
import re
from pathlib import Path

from drivers import catalog, pip_manager
from ..gen import dbawork_pb2, dbawork_pb2_grpc

#: 进程内 active 映射（持久化由 Go 侧负责）
_ACTIVE_DRIVERS: dict[str, int] = {}


def _status_proto(s: dict) -> dbawork_pb2.DriverStatus:
    return dbawork_pb2.DriverStatus(
        db_type=s.get("db_type", ""),
        driver_kind=s.get("driver_kind", ""),
        package_name=s.get("package_name", ""),
        installed=bool(s.get("installed")),
        installed_version=s.get("installed_version", ""),
        note=s.get("note", ""),
    )


def _sanitize_segment(value: str) -> str:
    """清洗用作目录名的片段，防止路径穿越。"""
    cleaned = re.sub(r"[^0-9A-Za-z._\-]", "_", value or "")
    if cleaned in ("", ".", ".."):
        return "_"
    return cleaned


class DriverServicer(dbawork_pb2_grpc.DriverServiceServicer):
    """DriverService：8 个 RPC。"""

    # ---------------- 类型目录 ----------------

    def ListDbTypes(self, request, context):  # noqa: N802
        reply = dbawork_pb2.ListDbTypesReply()
        for t in catalog.CATALOG:
            # hidden 逻辑由 Go 侧处理，这里只返回内置目录
            reply.types.append(
                dbawork_pb2.DbTypeInfo(
                    key=t["key"],
                    name_zh=t.get("name_zh", ""),
                    name_en=t.get("name_en", ""),
                    driver_class_hint=t.get("driver_class_hint", ""),
                    is_jdbc=bool(t.get("is_jdbc")),
                    order=int(t.get("order", 0)),
                    is_custom=False,
                )
            )
        return reply

    # ---------------- 检测 ----------------

    def DetectAll(self, request, context):  # noqa: N802
        reply = dbawork_pb2.DetectAllReply()
        for s in pip_manager.detect_all():
            reply.statuses.append(_status_proto(s))
        return reply

    def DetectOne(self, request, context):  # noqa: N802
        return _status_proto(pip_manager.detect_one(request.db_type))

    # ---------------- 安装 / 卸载 ----------------

    def InstallDriver(self, request, context):  # noqa: N802
        ok, message = pip_manager.install(request.db_type, request.package_name, request.version)
        latest = pip_manager.detect_one(request.db_type)
        return dbawork_pb2.Result(
            ok=ok, message=message, detail=json.dumps(latest, ensure_ascii=False)
        )

    def UninstallDriver(self, request, context):  # noqa: N802
        ok, message = pip_manager.uninstall(request.db_type)
        latest = pip_manager.detect_one(request.db_type)
        return dbawork_pb2.Result(
            ok=ok, message=message, detail=json.dumps(latest, ensure_ascii=False)
        )

    # ---------------- JAR 管理 ----------------

    def UploadJar(self, request, context):  # noqa: N802
        db_type = _sanitize_segment((request.db_type or "").strip())
        version = _sanitize_segment((request.version or "").strip() or "default")
        filename = Path(request.jar_filename or "").name  # 防路径穿越
        if not (request.db_type or "").strip():
            return dbawork_pb2.UploadJarReply(ok=False, message="db_type 不能为空")
        if not filename:
            return dbawork_pb2.UploadJarReply(ok=False, message="jar 文件名不能为空")
        if not filename.lower().endswith(".jar"):
            return dbawork_pb2.UploadJarReply(ok=False, message="仅支持 .jar 文件")
        if not request.jar_bytes:
            return dbawork_pb2.UploadJarReply(ok=False, message="jar 内容为空")

        target_dir = Path(catalog.DRIVERS_DIR) / db_type / version
        try:
            target_dir.mkdir(parents=True, exist_ok=True)
            target = target_dir / filename
            target.write_bytes(request.jar_bytes)
        except OSError as e:
            return dbawork_pb2.UploadJarReply(ok=False, message=f"保存 jar 失败: {e}")
        return dbawork_pb2.UploadJarReply(
            ok=True,
            jar_path=str(target.resolve()),
            file_size=len(request.jar_bytes),
            message="上传成功",
        )

    def DeleteJar(self, request, context):  # noqa: N802
        raw = (request.jar_path or "").strip()
        if not raw:
            return dbawork_pb2.Result(ok=False, message="jar_path 不能为空")
        base = Path(catalog.DRIVERS_DIR).resolve()
        try:
            resolved = Path(raw).resolve()
            resolved.relative_to(base)  # 不在驱动目录内会抛 ValueError
        except ValueError:
            return dbawork_pb2.Result(ok=False, message=f"拒绝删除数据目录之外的路径: {raw}")
        if not resolved.is_file():
            return dbawork_pb2.Result(ok=False, message=f"文件不存在: {raw}")
        try:
            resolved.unlink()
        except OSError as e:
            return dbawork_pb2.Result(ok=False, message=f"删除失败: {e}")
        return dbawork_pb2.Result(ok=True, message="已删除")

    # ---------------- 激活 ----------------

    def ActivateDriver(self, request, context):  # noqa: N802
        if not (request.db_type or "").strip():
            return dbawork_pb2.Result(ok=False, message="db_type 不能为空")
        _ACTIVE_DRIVERS[request.db_type] = int(request.driver_id)
        return dbawork_pb2.Result(
            ok=True,
            message=f"已激活驱动: {request.db_type} -> id={request.driver_id}（持久化由 Go 侧完成）",
        )
