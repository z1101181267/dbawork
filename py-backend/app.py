"""DBAWORK Python 后端 FastAPI 入口。

提供 /health 健康检查；数据库/驱动逻辑通过 gRPC（rpc.server）对外服务。
独立启动：python app.py（API_PORT 环境变量可覆盖端口，默认 8000）
"""
from __future__ import annotations

import os

from fastapi import FastAPI

app = FastAPI(title="DBAWORK py-backend", version="0.1.0")


@app.get("/health")
def health():
    """健康检查。"""
    return {
        "status": "ok",
        "service": "py-backend",
        "grpc_port": int(os.environ.get("GRPC_PORT", "50051")),
    }


def main():
    """启动 uvicorn。"""
    import uvicorn

    port = int(os.environ.get("API_PORT", "8000"))
    uvicorn.run(app, host="0.0.0.0", port=port)


if __name__ == "__main__":
    main()
