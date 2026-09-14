"""gRPC 服务入口。

独立启动：python -m rpc.server
端口：环境变量 GRPC_PORT，默认 50051，监听 0.0.0.0。
"""
from __future__ import annotations

import logging
import os
from concurrent import futures

import grpc

from .gen import dbawork_pb2_grpc
from .servicers import ConnectionServicer, DriverServicer

logger = logging.getLogger(__name__)

_server: grpc.Server | None = None


def create_server(port: int = 0) -> grpc.Server:
    """创建（未启动）gRPC server 实例。"""
    resolved = int(port or os.environ.get("GRPC_PORT", "50051"))
    server = grpc.server(
        futures.ThreadPoolExecutor(max_workers=8),
        options=[
            # JAR 上传可能较大，放宽消息大小限制
            ("grpc.max_receive_message_length", 128 * 1024 * 1024),
            ("grpc.max_send_message_length", 128 * 1024 * 1024),
        ],
    )
    dbawork_pb2_grpc.add_ConnectionServiceServicer_to_server(ConnectionServicer(), server)
    dbawork_pb2_grpc.add_DriverServiceServicer_to_server(DriverServicer(), server)
    bound = server.add_insecure_port(f"0.0.0.0:{resolved}")
    if bound == 0:
        raise RuntimeError(f"gRPC 端口绑定失败: {resolved}")
    logger.info("gRPC 监听 0.0.0.0:%s", bound)
    return server


def serve(port: int = 0):
    """启动服务并阻塞等待（供独立运行）。"""
    global _server
    _server = create_server(port)
    _server.start()
    try:
        _server.wait_for_termination()
    except KeyboardInterrupt:
        stop()


def stop(grace: float = 2.0):
    """停止服务（供测试 / 嵌入式调用，幂等）。"""
    global _server
    if _server is not None:
        _server.stop(grace)
        _server = None


if __name__ == "__main__":
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s: %(message)s")
    serve()
